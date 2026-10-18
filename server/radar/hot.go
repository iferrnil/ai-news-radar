package radar

import (
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// 热点追踪: stories that several sources report — pure code, no AI. Items of the 48 h window (sections counted in
// 全部, enabled sources) are grouped in two steps:
//   1. units: the brief dedupe (same URL / near-identical title, briefpick.go) and the parts of one X thread;
//   2. units are merged greedily, strongest link first, by shared distinctive names (product / model / company
//      tokens such as "v4.1", "816b", "beam", 「可灵」 → kling; Chinese names via the lexicon in hotlex.go). A link
//      needs a shared name that is the MAIN SUBJECT of both items (in both headlines, or in one headline and
//      repeated in the other's lead; names after a comparison cue — 「超过 GPT-6」, "beats Claude" — don't count),
//      and a merge needs a core name that every unit of the merged group mentions and at least half of them are
//      about, so A~B~C chains can't drift into one topic. Fragments of one story that only share a core name are
//      reconnected afterwards.
// A tweet merged into an article ("也在 X") counts as a report of its own source. Heat = Σ reports × source weight
// (first-party 1.5 · article 1 · X repost 0.7 · non-P0 ×0.6; a source's later reports 0.3×) × a 12 h half-life,
// plus 0.5 × the weighted reports of the last hour (velocity). The first report of every topic is remembered for
// 7 days (hot_history), so the first reporter stays known after its item leaves the 48 h window.

type hotSource struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	First float64 `json:"first"`
	Count int     `json:"count"`
	X     bool    `json:"x,omitempty"`
}

type hotItem struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	Source    string  `json:"source"`
	Published float64 `json:"published"`
	X         bool    `json:"x,omitempty"`
	P0        bool    `json:"p0,omitempty"`
}

type hotFirst struct {
	Source    string  `json:"source"`
	Published float64 `json:"published"`
	Item      int64   `json:"item,omitempty"` // 0 when the first report has left the window (archived)
	Title     string  `json:"title,omitempty"`
	URL       string  `json:"url,omitempty"`
	Archived  bool    `json:"archived,omitempty"`
}

type HotTopic struct {
	ID       int64       `json:"id"` // representative item
	Title    string      `json:"title"`
	Core     string      `json:"core,omitempty"` // the shared name the topic is about
	Items    []int64     `json:"items"`          // visible items, earliest first
	Rows     []hotItem   `json:"rows"`           // the same, for the list (title prefers the translation)
	Sources  []hotSource `json:"sources"`
	Reports  int         `json:"reports"`
	First    hotFirst    `json:"first"`
	Last     float64     `json:"last"`
	Heat     float64     `json:"heat"`
	Velocity float64     `json:"velocity"` // weighted reports in the last hour
	Spark    []int       `json:"spark"`    // reports per hour, the last 24 h (oldest first)
	key      string
}

const (
	hotSparkHours = 24
	hotWindowSame = 36 * 3600 // two reports more than 36 h apart are not one story
	hotHistKeep   = 7 * 24 * 3600
	hotHistMatch  = 72 * 3600 // a remembered first report counts for a topic starting within 3 days of it
)

var hotTokRe = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9.+]*`)

var hotStop = map[string]bool{}

// hotOrgs: companies / product families / outlets that appear in many unrelated stories — they can support a match
// but never make one on their own (plus the lexicon's * entries).
var hotOrgs = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`openai anthropic google deepmind meta microsoft nvidia apple amazon aws tesla spacex xai mistral alibaba tencent baidu
		bytedance huawei samsung intel amd qualcomm oracle ibm chatgpt claude gemini gpt llama qwen grok copilot codex deepseek kimi moonshot zhipu
		minimax perplexity cursor github hugging huggingface arena artificial analysis index benchmark techmeme reuters bloomberg verge`) {
		hotOrgs[w] = true
	}
	for _, w := range strings.Fields(`the and for with its says said say new now here into onto over from that this than then they them their there these those what when where which while who why how
		are was were has have had will would can could may might must should our ours you your yours his her she him it's its not but all any one two three more most some such just also only very much
		about after before again against between during under above below off out per via via.. today week year years day days month months time first last next best top big
		a an as at be by do if in is it of on or so to up us we me my no yes vs etc inc ltd co com www http https
		ai model models open source release releases released launch launches launched update updates updated
		agent agents app apps api apis data based using use used user users people company companies startup startups
		report reports sources source says new news post blog thread read full details more learn link here try test`) {
		hotStop[w] = true
	}
}

// ---- text → name tokens

// hotNorm: full-width ASCII → ASCII, every dash → '-'
func hotNorm(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E && r != 0xFF0C && r != 0xFF1A && r != 0xFF1B && r != 0xFF01 && r != 0xFF1F && r != 0xFF08 && r != 0xFF09:
			return r - 0xFEE0 // keep ，：；！？（） — they are Chinese punctuation, handled below
		case r == 0x3000:
			return ' '
		case r == '‐' || r == '‑' || r == '‒' || r == '–' || r == '—' || r == '―' || r == '−' || r == '﹣':
			return '-'
		}
		return r
	}, s)
}

// hotDashNum rejoins "Beam-1.0" / "Matrix-Game 4.0" style names before tokenizing, so the version stays part of the
// name (the families in hotlex.go also join "GPT 6" / "GPT-6" / "gpt6").
var hotDashNum = regexp.MustCompile(`(?i)\b([a-z]{2,})(?:-(\d{1,2}(?:\.\d+)?)|\s(\d{1,2}\.\d+))\b`)
var hotURL = regexp.MustCompile(`https?://\S+|\b[\w.-]+\.(?:com|ai|io|org|net|co)/\S*|@\w+`)

// hotCue: a comparison — names after it (up to the next punctuation) are what the item is compared WITH, not about
// (「登顶 RoboDojo，超过 GPT-6 Astra」 is not a GPT-6 story).
var hotCue = regexp.MustCompile(`(?i)超过|超越|击败|打败|碾压|吊打|媲美|比肩|对标|优于|胜过|反超|赶超|\b(?:beats?|beating|outperform(?:s|ed|ing)?|surpass(?:es|ed|ing)?|vs\.?|versus|rivals?|rivaling|than)\b`)

const hotCueEnd = ",.;:!?\n，。；：！？（("

// versioned name: letters then a version ("gpt6", "beam1.0", "v4.1f", "kling4.0")
var hotVer = regexp.MustCompile(`^([a-z]+)(\d+(?:\.\d+)*)([a-z]{0,2})$`)

type hotScan struct {
	toks   map[string]bool   // every token (value: name-like), comparison targets included
	count  map[string]int    // occurrences outside comparisons
	base   map[string]string // versioned token → its base name ("beam1.0" → "beam")
	phrase map[string]bool   // two adjacent names ("muse spark")
}

// prep: the regexps only run when the text can match them (a URL / handle needs '/' or '@', a family or dash
// version needs a digit).
func (x *hotLex) prep(text string) (string, []string) {
	t := hotNorm(text)
	if strings.ContainsAny(t, "/@") {
		t = hotURL.ReplaceAllString(t, " ")
	}
	t, hits := x.hanReplace(t)
	if strings.ContainsAny(t, "0123456789") {
		t = x.family.ReplaceAllString(t, "$1$2")
		t = hotDashNum.ReplaceAllString(t, "$1$2$3")
	}
	return t, hits
}

const hotMemoMax = 8192

// scan is memoized per lexicon: every recompute scans the same leads / headlines again. The result is shared, so
// callers only read it.
func (x *hotLex) scan(text string) hotScan {
	x.memoMu.Lock()
	sc, ok := x.memo[text]
	x.memoMu.Unlock()
	if ok {
		return sc
	}
	sc = x.scanText(text)
	x.memoMu.Lock()
	if x.memo == nil || len(x.memo) >= hotMemoMax {
		x.memo = map[string]hotScan{}
	}
	x.memo[text] = sc
	x.memoMu.Unlock()
	return sc
}

// scan: lowercase name-like tokens of a text ("DeepSeek-V4.1-Flash-1017" → deepseek, v4.1, flash, 1017 and the
// phrases "deepseek v4.1", "v4.1 flash", …; 「字节跳动」/"ByteDance" → bytedance). Plain words of 1–2 letters,
// stopwords and lone years are dropped. A token is name-like when written with a capital letter or a digit, or
// next to Chinese text ("Meta杀回开源"), or found by the lexicon.
func (x *hotLex) scanText(text string) hotScan {
	sc := hotScan{toks: map[string]bool{}, count: map[string]int{}, base: map[string]string{}, phrase: map[string]bool{}}
	t, hits := x.prep(text)
	lexHit := map[string]bool{}
	for _, h := range hits {
		lexHit[h] = true
	}
	var demoted [][2]int
	for _, m := range hotCue.FindAllStringIndex(t, -1) {
		end := len(t)
		if i := strings.IndexAny(t[m[1]:], hotCueEnd); i >= 0 {
			end = m[1] + i
		}
		demoted = append(demoted, [2]int{m[1], end})
	}
	isDemoted := func(p int) bool {
		for _, d := range demoted {
			if p >= d[0] && p < d[1] {
				return true
			}
		}
		return false
	}
	prev, prevEnd := "", -10
	add := func(p string, name, dem bool) {
		sc.toks[p] = sc.toks[p] || name
		if !dem {
			sc.count[p]++
		}
	}
	for _, loc := range hotTokRe.FindAllStringIndex(t, -1) {
		raw := t[loc[0]:loc[1]]
		before, after := t[:loc[0]], t[loc[1]:]
		hanNext := func(s string, last bool) bool {
			s = strings.TrimSpace(s)
			if s == "" {
				return false
			}
			var r rune
			if last {
				r, _ = utf8.DecodeLastRuneInString(s)
			} else {
				r, _ = utf8.DecodeRuneInString(s)
			}
			return unicode.Is(unicode.Han, r)
		}
		// capital at a sentence start says nothing ("Build your …"); digits / inner capitals / Chinese context always do
		name := strings.ContainsFunc(raw, func(r rune) bool { return r >= '0' && r <= '9' }) ||
			strings.ContainsFunc(raw[1:], func(r rune) bool { return r >= 'A' && r <= 'Z' }) ||
			(raw[0] >= 'A' && raw[0] <= 'Z' && (!sentenceStart(before) || nameFollows(after))) ||
			hanNext(before, true) || hanNext(after, false)
		dem := isDemoted(loc[0])
		raw = strings.ToLower(strings.TrimRight(raw, ".+"))
		for _, p := range strings.FieldsFunc(raw, func(r rune) bool { return r == '+' }) {
			p = strings.Trim(p, ".")
			if id := x.latin[p]; id != "" {
				p = id
			} else if m := hotVer.FindStringSubmatch(p); m != nil && x.latin[m[1]] != "" {
				p = x.latin[m[1]] + m[2] + m[3]
			}
			if p == "" || hotStop[p] {
				prev = ""
				continue
			}
			digits, letters := 0, 0
			for _, r := range p {
				if r >= '0' && r <= '9' {
					digits++
				} else if r != '.' {
					letters++
				}
			}
			switch {
			case letters == 0 && (digits < 3 || (len(p) == 4 && (strings.HasPrefix(p, "19") || strings.HasPrefix(p, "20")))):
				prev = ""
				continue // "6", "12", "2026"
			case digits == 0 && len(p) < 3:
				prev = ""
				continue
			}
			nm := name || lexHit[p]
			add(p, nm, dem)
			if m := hotVer.FindStringSubmatch(p); m != nil {
				if len(m[1]) >= 3 && !hotStop[m[1]] {
					add(m[1], true, dem)
					sc.base[p] = m[1]
				}
				if m[3] != "" && strings.Contains(m[2], ".") { // "v4.1f" is also "v4.1"
					add(m[1]+m[2], true, dem)
					sc.base[p] = m[1] + m[2]
				}
			}
			if nm && prev != "" && loc[0]-prevEnd <= 1 && !dem {
				ph := prev + " " + p
				sc.toks[ph] = true
				sc.count[ph]++
				sc.phrase[ph] = true
			}
			if nm {
				prev = p
			} else {
				prev = ""
			}
		}
		prevEnd = loc[1]
	}
	return sc
}

// sentenceStart: nothing but spaces / quotes / emoji since the start of the text or a sentence end
func sentenceStart(before string) bool {
	for i := len(before); i > 0; {
		r, sz := utf8.DecodeLastRuneInString(before[:i])
		i -= sz
		switch {
		case r == ' ' || r == '\t' || r == '"' || r == '\'' || r == '“' || r == '‘' || r == '(' || r == '*' || r == '-' || r == '#':
			continue
		case strings.ContainsRune(".!?:;\n。！？：；…", r):
			return true
		case r > 0x2000 && !unicode.IsLetter(r) && !unicode.IsNumber(r): // emoji / symbols
			continue
		}
		return false
	}
	return true
}

// nameFollows: a sentence-initial capitalized word is still a name when a possessive or another capitalized word
// follows ("Reflection’s first model", "Reflection AI releases")
func nameFollows(after string) bool {
	if strings.HasPrefix(after, "’s") || strings.HasPrefix(after, "'s") {
		return true
	}
	t := strings.TrimLeft(after, " ")
	return len(t) < len(after) && t != "" && t[0] >= 'A' && t[0] <= 'Z'
}

// hotTokens: the name tokens of a text with the built-in lexicon (tests / callers outside a topic run)
func hotTokens(text string) map[string]bool { return hotLexDefault().scan(text).toks }

// hotRoundup: digests / weekly round-ups mention many stories; they only join a topic by an identical title.
var hotRoundup = regexp.MustCompile(`(?i)\b(this week|week in|weekly|roundup|recap|digest|newsletter)\b|周报|一周|本周|日报|早报|晚报|速览|汇总|盘点`)

// hotHeadline: what an item is about — its title (+ translation); for an X post the first sentence of the text
func hotHeadline(r *Row) string {
	if r.Num("is_x") == 1 {
		t := r.Str("summary")
		if t == "" {
			t = r.Str("title")
		}
		t = hotURL.ReplaceAllString(t, " ")
		rs := []rune(strings.TrimSpace(t))
		cut := len(rs)
		for i, c := range rs {
			if i >= 12 && (c == '\n' || c == '。' || c == '！' || c == '？' || ((c == '.' || c == '!' || c == '?') && (i+1 == len(rs) || rs[i+1] == ' ' || rs[i+1] == '\n'))) {
				cut = i + 1
				break
			}
		}
		return string(rs[:min(cut, 120)]) + "\n" + r.Str("title_zh")
	}
	return r.Str("title") + "\n" + r.Str("title_zh")
}

// hotLead: the headline plus the start of the text (X posts: the text itself)
func hotLead(r *Row) string {
	if r.Num("is_x") == 1 {
		t := r.Str("summary")
		if t == "" {
			t = r.Str("title")
		}
		return capRunes(t, 300) + "\n" + r.Str("title_zh")
	}
	return r.Str("title") + "\n" + r.Str("title_zh") + "\n" + capRunes(r.Str("summary"), 300)
}

type hotDoc struct {
	row     *Row
	key     briefKey
	all     hotScan        // lead text: every name (cover / mentions)
	head    map[string]int // headline names outside comparisons
	pub     float64
	sid     float64
	roundup bool
}

func newHotDocLex(x *hotLex, r *Row) *hotDoc {
	return &hotDoc{row: r, key: keyOf(r), all: x.scan(hotLead(r)), head: x.scan(hotHeadline(r)).count, pub: r.Num("published"), sid: r.Num("source_id"),
		roundup: hotRoundup.MatchString(r.Str("title") + " " + r.Str("title_zh"))}
}

func newHotDoc(r *Row) *hotDoc { return newHotDocLex(hotLexDefault(), r) }

// subject: the item is about t — t is in its headline, or repeated in its lead
func (d *hotDoc) subject(t string) bool { return d.head[t] > 0 || d.all.count[t] >= 2 }

// ---- names shared across the window

type hotIdx struct {
	df      map[string]int
	w       map[string]float64 // idf per name, computed once (score() asks for it per name and item pair)
	w0      float64            // idf of a name no source uses
	n       int
	product map[string]bool // bases of versioned names seen in the window ("beam" of "beam1.0"), lexicon products
	lex     *hotLex
}

// newHotIdx: document frequency counted per SOURCE (how many sources use the name), so a story one account posts
// a 5-tweet thread about doesn't make its own names look common.
func newHotIdx(x *hotLex, docs []*hotDoc) hotIdx {
	ix := hotIdx{df: map[string]int{}, product: map[string]bool{}, lex: x}
	seen := map[string]bool{}
	srcs := map[float64]bool{}
	for _, d := range docs {
		srcs[d.sid] = true
		for t := range d.all.toks {
			k := t + "\x00" + strconv.FormatFloat(d.sid, 'f', 0, 64)
			if !seen[k] {
				seen[k] = true
				ix.df[t]++
			}
		}
		for _, b := range d.all.base {
			ix.product[b] = true
		}
	}
	for id, on := range x.user {
		ix.product[id] = on
	}
	for _, id := range x.han {
		if !x.org[id] {
			ix.product[id] = true
		}
	}
	ix.n = len(srcs)
	ix.w = make(map[string]float64, len(ix.df))
	for t, n := range ix.df {
		ix.w[t] = math.Log(float64(ix.n+1) / float64(max(1, n)))
	}
	ix.w0 = math.Log(float64(ix.n + 1))
	return ix
}

func (ix hotIdx) org(t string) bool { return hotOrgs[t] || ix.lex.org[t] }
func (ix hotIdx) idf(t string) float64 {
	if w, ok := ix.w[t]; ok {
		return w
	}
	if ix.w == nil { // a literal hotIdx (tests)
		return math.Log(float64(ix.n+1) / float64(max(1, ix.df[t])))
	}
	return ix.w0
}

// rare: at most a third of the window's sources (≥ 5) use the name — a story five outlets cover keeps its names
func (ix hotIdx) rare(t string) bool { return ix.df[t] <= max(5, ix.n/3) }

// few: a plain-word name (no digits) must be rarer: at most a quarter of the sources (≥ 3)
func (ix hotIdx) few(t string) bool { return ix.df[t] <= max(3, ix.n/4) }

// distinctive: a rare name with letters and digits ("816b", "v4.1", "gpt6"), a name of 5+ letters that few
// sources of the window use ("roadster"; not "openai", "google" — they are in every other story), a product name
// (the base of a versioned name: "beam" of "Beam-1.0"; a lexicon product: 豆包, 可灵) or a phrase of two names
// ("muse spark") that few sources use
func (ix hotIdx) distinctive(t string) bool {
	if !ix.rare(t) || ix.org(t) {
		return false
	}
	if a, b, ok := strings.Cut(t, " "); ok {
		return ix.few(t) && len(a) >= 3 && len(b) >= 3 && !(ix.org(a) && ix.org(b))
	}
	hasD, hasL := false, false
	for _, r := range t {
		if r >= '0' && r <= '9' {
			hasD = true
		} else if r != '.' {
			hasL = true
		}
	}
	return (hasD && hasL && len(t) >= 3) || (hasL && !hasD && len(t) >= 5 && ix.few(t)) || (ix.product[t] && hasL && len(t) >= 3 && ix.few(t))
}

type hotLink struct {
	rare, dist int
	cover      float64
	subj       []string // shared distinctive names that are the subject of both items
}

// score: rare names both texts use, how many of them are distinctive, their share of the smaller text's names
// (idf-weighted) and the shared names both items are about. Plain words never link two stories.
func (ix hotIdx) score(a, b *hotDoc) hotLink {
	var l hotLink
	var shared, ta, tb float64
	phraseDist := false
	for t, name := range a.all.toks {
		if !name {
			continue
		}
		ph := a.all.phrase[t]
		w := ix.idf(t)
		if !ph { // phrases repeat their words: cover counts words only
			ta += w
		}
		if !b.all.toks[t] {
			continue
		}
		if !ph {
			shared += w
		}
		if !ix.rare(t) {
			continue
		}
		l.rare++
		if !ix.distinctive(t) {
			continue
		}
		if (a.head[t] > 0 && b.subject(t)) || (b.head[t] > 0 && a.subject(t)) {
			l.subj = append(l.subj, t)
		}
		switch {
		case a.all.phrase[t]:
			phraseDist = true
		case ix.sharedVersionOf(a, b, t):
			// "beam" next to a shared "beam1.0" is the same name, counted once
		default:
			l.dist++
		}
	}
	for t, name := range b.all.toks {
		if name && !b.all.phrase[t] {
			tb += ix.idf(t)
		}
	}
	if phraseDist {
		l.dist++
	}
	if ta == 0 || tb == 0 {
		return hotLink{}
	}
	l.cover = shared / math.Min(ta, tb)
	sort.Strings(l.subj)
	return l
}

func (ix hotIdx) sharedVersionOf(a, b *hotDoc, t string) bool {
	for v, base := range a.all.base {
		if base == t && b.all.toks[v] {
			return true
		}
	}
	return false
}

func (l hotLink) ok() bool {
	// long texts (an analyst's thread, a news write-up) dilute cover: two shared names both items are about suffice
	return len(l.subj) > 0 && ((l.dist >= 2 && l.cover >= 0.25) || (len(l.subj) >= 2 && l.dist >= 2 && l.cover >= 0.15) || (l.rare >= 2 && l.dist >= 1 && l.cover >= 0.3) || (l.dist >= 1 && l.cover >= 0.5))
}

func (l hotLink) strength() float64 { return float64(l.dist) + float64(len(l.subj)) + 2*l.cover }

// sameUnit: one report in two rows — the same URL / near-identical title, or the parts of one X thread
func sameUnit(a, b *hotDoc) bool {
	if math.Abs(a.pub-b.pub) > hotWindowSame {
		return false
	}
	if a.sid == b.sid && a.row.Num("is_x") == 1 && b.row.Num("is_x") == 1 && a.key.url != b.key.url {
		// one X account: its boilerplate posts ("Dive into the Arena: …") look alike but are not one report
	} else if sameStory(a.key, b.key) {
		return true
	}
	// one X account: posts within a minute are one thread, within 3 minutes when they share a name (two unrelated
	// posts minutes apart are not one thread); a source's recurring names ("Intelligence Index") say nothing
	return a.sid == b.sid && a.row.Num("is_x") == 1 && b.row.Num("is_x") == 1 &&
		(math.Abs(a.pub-b.pub) <= 60 || math.Abs(a.pub-b.pub) <= 180 && shareName(a, b))
}

func shareName(a, b *hotDoc) bool {
	for t, name := range a.all.toks {
		if name && b.all.toks[t] {
			return true
		}
	}
	return false
}

type hotUnit struct {
	docs []*hotDoc
	pub  float64 // earliest
}

func (u *hotUnit) mentions(t string) bool {
	for _, d := range u.docs {
		if d.all.toks[t] {
			return true
		}
	}
	return false
}

func (u *hotUnit) about(t string) bool {
	for _, d := range u.docs {
		if d.subject(t) {
			return true
		}
	}
	return false
}

type hotGroup struct {
	units []*hotUnit
	cores []string
}

// coreOf: the names every unit mentions and at least half of the units are about (sorted, most-about first)
func coreOf(units []*hotUnit, cands []string, minAbout int) []string {
	type c struct {
		t     string
		about int
	}
	var out []c
	for _, t := range cands {
		ok, about := true, 0
		for _, u := range units {
			if !u.mentions(t) {
				ok = false
				break
			}
			if u.about(t) {
				about++
			}
		}
		if ok && about >= minAbout {
			out = append(out, c{t, about})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].about != out[j].about {
			return out[i].about > out[j].about
		}
		if len(out[i].t) != len(out[j].t) {
			return len(out[i].t) > len(out[j].t)
		}
		return out[i].t < out[j].t
	})
	var ts []string
	for _, x := range out {
		ts = append(ts, x.t)
	}
	return ts
}

func groupSpan(a, b *hotGroup) bool {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, g := range []*hotGroup{a, b} {
		for _, u := range g.units {
			for _, d := range u.docs {
				lo, hi = math.Min(lo, d.pub), math.Max(hi, d.pub)
			}
		}
	}
	return hi-lo <= hotWindowSame
}

// clusterHot groups the docs (input order kept inside a group) and returns each group's core names.
func clusterHot(x *hotLex, docs []*hotDoc) ([][]*hotDoc, [][]string) {
	ix := newHotIdx(x, docs)
	// 1. units (union-find: these relations are exact, chaining is fine)
	parent := make([]int, len(docs))
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i := range docs {
		for j := i + 1; j < len(docs); j++ {
			if find(i) != find(j) && sameUnit(docs[i], docs[j]) {
				parent[find(j)] = find(i)
			}
		}
	}
	unitOf := make([]int, len(docs))
	var units []*hotUnit
	pos := map[int]int{}
	for i, d := range docs {
		r := find(i)
		u, ok := pos[r]
		if !ok {
			u = len(units)
			pos[r] = u
			units = append(units, &hotUnit{pub: d.pub})
		}
		units[u].docs = append(units[u].docs, d)
		unitOf[i] = u
	}
	// 2. links between units, strongest first
	type link struct {
		a, b int
		l    hotLink
	}
	var links []link
	// a link needs a shared distinctive name (hotLink.ok wants a subject): only score the pairs that have one
	// (scoring all n² pairs was most of a warm 热点 pass)
	cand := make([]bool, len(docs)*len(docs))
	byName := map[string][]int{}
	for i, d := range docs {
		for t, name := range d.all.toks {
			if name && ix.distinctive(t) {
				byName[t] = append(byName[t], i)
			}
		}
	}
	for _, is := range byName {
		for x, i := range is {
			for _, j := range is[x+1:] {
				cand[min(i, j)*len(docs)+max(i, j)] = true
			}
		}
	}
	for i := range docs {
		for j := i + 1; j < len(docs); j++ {
			a, b := docs[i], docs[j]
			if !cand[i*len(docs)+j] || unitOf[i] == unitOf[j] || a.sid == b.sid || a.roundup || b.roundup || math.Abs(a.pub-b.pub) > hotWindowSame {
				continue
			}
			if l := ix.score(a, b); l.ok() {
				links = append(links, link{unitOf[i], unitOf[j], l})
			}
		}
	}
	sort.SliceStable(links, func(i, j int) bool { return links[i].l.strength() > links[j].l.strength() })
	groups := make([]*hotGroup, len(units))
	gOf := make([]*hotGroup, len(units))
	for i, u := range units {
		groups[i] = &hotGroup{units: []*hotUnit{u}}
		gOf[i] = groups[i]
	}
	merge := func(ga, gb *hotGroup, cores []string) {
		ga.units = append(ga.units, gb.units...)
		ga.cores = cores
		for i, g := range gOf {
			if g == gb {
				gOf[i] = ga
			}
		}
		gb.units = nil
	}
	for _, lk := range links {
		ga, gb := gOf[lk.a], gOf[lk.b]
		if ga == gb || !groupSpan(ga, gb) {
			continue
		}
		all := append(append([]*hotUnit{}, ga.units...), gb.units...)
		if cores := coreOf(all, lk.l.subj, (len(all)+1)/2); len(cores) > 0 {
			merge(ga, gb, cores)
		}
	}
	// 3. fragments of one story: groups of ≥ 2 units whose units all mention a distinctive name that at least one
	// unit of each group is about
	for changed := true; changed; {
		changed = false
		for i, ga := range groups {
			if len(ga.units) < 2 {
				continue
			}
			for _, gb := range groups[i+1:] {
				if len(gb.units) < 2 || !groupSpan(ga, gb) {
					continue
				}
				var cands []string
				seen := map[string]bool{}
				for _, u := range ga.units {
					for _, d := range u.docs {
						for t := range d.all.toks {
							if !seen[t] && ix.distinctive(t) {
								seen[t] = true
								cands = append(cands, t)
							}
						}
					}
				}
				sort.Strings(cands)
				all := append(append([]*hotUnit{}, ga.units...), gb.units...)
				var cores []string
				for _, t := range coreOf(all, cands, 2) {
					if anyAbout(ga, t) && anyAbout(gb, t) {
						cores = append(cores, t)
					}
				}
				if len(cores) > 0 {
					merge(ga, gb, cores)
					changed = true
				}
			}
		}
	}
	idxOf := map[*hotDoc]int{}
	for i, d := range docs {
		idxOf[d] = i
	}
	type res struct {
		docs  []*hotDoc
		cores []string
	}
	var rs []res
	for _, g := range groups {
		if len(g.units) == 0 {
			continue
		}
		var ds []*hotDoc
		for _, u := range g.units {
			ds = append(ds, u.docs...)
		}
		sort.SliceStable(ds, func(i, j int) bool { return idxOf[ds[i]] < idxOf[ds[j]] })
		rs = append(rs, res{ds, g.cores})
	}
	sort.SliceStable(rs, func(i, j int) bool { return idxOf[rs[i].docs[0]] < idxOf[rs[j].docs[0]] }) // input order
	out, cores := make([][]*hotDoc, len(rs)), make([][]string, len(rs))
	for i, r := range rs {
		out[i], cores[i] = r.docs, r.cores
	}
	return out, cores
}

func anyAbout(g *hotGroup, t string) bool {
	for _, u := range g.units {
		if u.about(t) {
			return true
		}
	}
	return false
}

// ---- topics

type hotHist struct {
	Core      string
	Source    string
	SourceID  int64
	Published float64
	Title     string
	URL       string
	UID       string
}

// hotWeight: how much one report of a source counts — the lab's / company's own channel most, then articles, then
// X accounts that repost; P1 / P2 sources less
func hotWeight(x *hotLex, r *Row, names map[string]bool) float64 {
	w := 1.0
	fp := r.Str("section") == "lab"
	if !fp {
		for t := range x.scan(r.Str("source") + " " + strings.ReplaceAll(r.Str("username"), "_", " ")).toks {
			if len(t) >= 4 && names[t] {
				fp = true
				break
			}
		}
	}
	switch {
	case fp:
		w = 1.5
	case r.Num("is_x") == 1:
		w = 0.7
	}
	if p := r.Str("priority"); p != "" && p != "P0" {
		w *= 0.6
	}
	return w
}

// hotCompute is the pure part of 热点: rows of the window (published ascending; merged tweets included) → topics with
// ≥ 2 sources sorted by heat, and the first reports to remember (new topics, or an earlier first report).
func hotCompute(x *hotLex, rs []*Row, t float64, hist map[string]hotHist) ([]HotTopic, []hotHist) {
	var docs []*hotDoc
	merged := map[int64][]*Row{} // article id → tweets merged into it
	byUID := map[string]int64{}
	for _, r := range rs {
		byUID[r.Str("uid")] = int64(r.Num("id"))
		if m := r.Num("merged_into"); m > 0 {
			merged[int64(m)] = append(merged[int64(m)], r)
			continue
		}
		docs = append(docs, newHotDocLex(x, r))
	}
	groups, cores := clusterHot(x, docs)
	var out []HotTopic
	var upd []hotHist
	for gi, g := range groups {
		var reports []*Row
		for _, d := range g {
			reports = append(reports, d.row)
			reports = append(reports, merged[int64(d.row.Num("id"))]...)
		}
		srcs := map[int64]*hotSource{}
		var order []int64
		for _, r := range reports {
			sid := int64(r.Num("source_id"))
			if srcs[sid] == nil {
				srcs[sid] = &hotSource{ID: sid, Name: r.Str("source"), First: r.Num("published"), X: r.Num("is_x") == 1}
				order = append(order, sid)
			}
			hs := srcs[sid]
			hs.Count++
			hs.First = math.Min(hs.First, r.Num("published"))
		}
		if len(srcs) < 2 {
			continue
		}
		// the topic's names: its cores and what most of its items mention (for first-party sources)
		names := map[string]bool{}
		cnt := map[string]int{}
		for _, d := range g {
			for tk, nm := range d.all.toks {
				if nm {
					cnt[tk]++
				}
			}
		}
		for tk, n := range cnt {
			if 2*n >= len(g) {
				names[tk] = true
			}
		}
		for _, c := range cores[gi] {
			names[c] = true
			for _, w := range strings.Fields(c) {
				names[w] = true
			}
		}
		ht := HotTopic{Reports: len(reports), Spark: make([]int, hotSparkHours)}
		if len(cores[gi]) > 0 {
			ht.Core = cores[gi][0]
		}
		sort.SliceStable(reports, func(i, j int) bool { return reports[i].Num("published") < reports[j].Num("published") })
		seenSrc := map[int64]bool{}
		for _, r := range reports {
			p := r.Num("published")
			w := hotWeight(x, r, names)
			if sid := int64(r.Num("source_id")); !seenSrc[sid] {
				seenSrc[sid] = true
			} else {
				w *= 0.3
			}
			age := math.Max(0, t-p) / 3600
			ht.Heat += w * math.Pow(0.5, age/12)
			if age <= 1 {
				ht.Velocity += w
			}
			if h := int(age); h < hotSparkHours {
				ht.Spark[hotSparkHours-1-h]++
			}
			ht.Last = math.Max(ht.Last, p)
		}
		ht.Heat = math.Round((ht.Heat+0.5*ht.Velocity)*100) / 100
		ht.Velocity = math.Round(ht.Velocity*10) / 10
		f := reports[0]
		ht.First = hotFirst{Source: f.Str("source"), Published: f.Num("published"), Title: hotTitle(f), URL: f.Str("url")}
		if m := f.Num("merged_into"); m > 0 {
			ht.First.Item = int64(m)
		} else {
			ht.First.Item = int64(f.Num("id"))
		}
		ht.key = ht.Core
		if ht.key == "" {
			ht.key = "u:" + NormURL(g[0].row.Str("url"))
		}
		cur := hotHist{Core: ht.key, Source: f.Str("source"), SourceID: int64(f.Num("source_id")), Published: f.Num("published"), Title: ht.First.Title, URL: f.Str("url"), UID: f.Str("uid")}
		if h, ok := hist[ht.key]; ok && h.Published < cur.Published {
			if cur.Published-h.Published <= hotHistMatch {
				ht.First = hotFirst{Source: h.Source, Published: h.Published, Title: h.Title, URL: h.URL, Item: byUID[h.UID]}
				ht.First.Archived = ht.First.Item == 0
			}
		} else if !ok || cur.Published < h.Published {
			upd = append(upd, cur)
		}
		for _, d := range g {
			r := d.row
			ht.Items = append(ht.Items, int64(r.Num("id")))
			ht.Rows = append(ht.Rows, hotItem{ID: int64(r.Num("id")), Title: hotTitle(r), Source: r.Str("source"), Published: r.Num("published"),
				X: r.Num("is_x") == 1, P0: r.Str("priority") == "P0"})
		}
		for _, sid := range order {
			ht.Sources = append(ht.Sources, *srcs[sid])
		}
		sort.SliceStable(ht.Sources, func(i, j int) bool { return ht.Sources[i].First < ht.Sources[j].First })
		rep := hotRep(g)
		ht.ID, ht.Title = int64(rep.Num("id")), hotTitle(rep)
		out = append(out, ht)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Heat != out[j].Heat {
			return out[i].Heat > out[j].Heat
		}
		return out[i].Last > out[j].Last
	})
	return out, upd
}

const hotRowsSQL = `SELECT i.id, i.title, i.title_zh, i.summary, i.url, i.published, i.is_x, i.merged_into, i.uid,
	s.id AS source_id, s.name AS source, s.priority, s.section, s.username
	FROM items i JOIN sources s ON s.id=i.source_id LEFT JOIN sections sc ON sc.key=s.section
	WHERE i.hidden=0 AND s.enabled=1 AND i.published >= ? AND COALESCE(sc.in_all, 1)=1 AND i.uid NOT LIKE 'trendshift:%'
	ORDER BY i.published ASC, i.id ASC`

// hotTopics computes the topics (≥ 2 sources) sorted by heat, and keeps hot_history (first reports, 7 days).
func (s *Store) hotTopics(t float64) []HotTopic {
	rs, _ := all(s.DB, hotRowsSQL, t-retentionHours()*3600)
	hist := map[string]hotHist{}
	hs, _ := all(s.DB, "SELECT core, source, source_id, published, title, url, uid FROM hot_history WHERE published >= ?", t-hotHistKeep)
	for _, h := range hs {
		hist[h.Str("core")] = hotHist{Core: h.Str("core"), Source: h.Str("source"), SourceID: int64(h.Num("source_id")), Published: h.Num("published"),
			Title: h.Str("title"), URL: h.Str("url"), UID: h.Str("uid")}
	}
	topics, upd := hotCompute(s.hotLexicon(), rs, t, hist)
	if tx, err := s.DB.Begin(); err == nil {
		{
			for _, h := range upd {
				tx.Exec(`INSERT INTO hot_history(core, source, source_id, published, title, url, uid, seen) VALUES(?,?,?,?,?,?,?,?)
					ON CONFLICT(core) DO UPDATE SET source=excluded.source, source_id=excluded.source_id, published=excluded.published,
					title=excluded.title, url=excluded.url, uid=excluded.uid, seen=excluded.seen`, h.Core, h.Source, h.SourceID, h.Published, h.Title, h.URL, h.UID, t)
			}
			tx.Exec("DELETE FROM hot_history WHERE published < ?", t-hotHistKeep) // its own 7 days, not the 48 h item window
			tx.Commit()
		}
	}
	return topics
}

// hotRep: the topic's headline item — Chinese titles first (the UI is Chinese), then articles before X posts,
// then the source priority; ties keep the earliest report.
func hotRep(g []*hotDoc) *Row {
	var best *Row
	rank := func(r *Row) int {
		p := 2
		switch r.Str("priority") {
		case "P0":
			p = 0
		case "P1":
			p = 1
		}
		if r.Num("is_x") == 1 {
			p += 5
		}
		n := len([]rune(r.Str("title")))
		if (!hasCJK(r.Str("title")) || n < 12) && r.Str("title_zh") == "" {
			p += 10
		}
		return p*100 - min(n, 60) // then the more telling (longer) headline
	}
	for _, d := range g {
		if best == nil || rank(d.row) < rank(best) {
			best = d.row
		}
	}
	return best
}

func hasCJK(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.Is(unicode.Han, r) })
}

var hotByline = regexp.MustCompile(`\s*\([^()]{2,80}/[^()]{2,80}\)\s*$`) // Techmeme "(Author / Outlet)"

func hotTitle(r *Row) string {
	t := Decode(r.Str("title"))
	if z := r.Str("title_zh"); z != "" {
		t = Decode(z)
	}
	return jsSlice(strings.TrimSpace(hotByline.ReplaceAllString(t, "")), 160)
}

var hotCache struct {
	sync.Mutex
	at     time.Time
	sig    string
	topics []HotTopic
}

// GET /api/hot → {topics, at}; cached until the item set changes (or 60 s, for the decay / sparkline)
func (s *Store) hHot(w http.ResponseWriter, r *http.Request) {
	var sig string
	s.DB.QueryRow("SELECT COUNT(*) || ':' || COALESCE(MAX(id),0) || ':' || COALESCE(SUM(merged_into IS NOT NULL),0) FROM items").Scan(&sig)
	hotCache.Lock()
	if sig != hotCache.sig || time.Since(hotCache.at) > time.Minute || hotCache.topics == nil {
		hotCache.topics, hotCache.sig, hotCache.at = s.hotTopics(now()), sig, time.Now()
		if hotCache.topics == nil {
			hotCache.topics = []HotTopic{}
		}
	}
	topics := hotCache.topics
	hotCache.Unlock()
	b, _ := marshalNoEscape(obj("topics", topics, "at", now()))
	jsonResponse(w, r, b)
}
