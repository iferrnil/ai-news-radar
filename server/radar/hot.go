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

// 热点追踪: stories that several sources report. Items of the 48 h window (sections counted in 全部, enabled
// sources) are clustered with the brief dedupe (same URL / near-identical title, briefpick.go) plus shared
// distinctive names (product / model / company tokens such as "V4.1", "816B", "Muse", weighted by rarity in the
// window), so an English article, a Chinese write-up and the lab's own post land in one topic. A tweet merged
// into an article ("也在 X") counts as a report of its own source. Heat = Σ reports (1 for a source's first
// report, 0.3 for more) × a 12 h half-life, so fresh, widely covered stories rank first.

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

type HotTopic struct {
	ID      int64       `json:"id"` // representative item
	Title   string      `json:"title"`
	Items   []int64     `json:"items"` // visible items, earliest first
	Rows    []hotItem   `json:"rows"`  // the same, for the list (title prefers the translation)
	Sources []hotSource `json:"sources"`
	Reports int         `json:"reports"`
	First   struct {
		Source    string  `json:"source"`
		Published float64 `json:"published"`
		Item      int64   `json:"item"`
	} `json:"first"`
	Last  float64 `json:"last"`
	Heat  float64 `json:"heat"`
	Spark []int   `json:"spark"` // reports per hour, the last 24 h (oldest first)
}

const (
	hotSparkHours = 24
	hotWindowSame = 36 * 3600 // two reports more than 36 h apart are not one story
)

var hotTokRe = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9.+]*`)

var hotStop = map[string]bool{}

// hotOrgs: companies / product families that appear in many unrelated stories — they can support a match but
// never make one on their own.
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

// hotTokens: lowercase name-like tokens of a text ("DeepSeek-V4.1-Flash-1017" → deepseek, v4.1, flash, 1017;
// "GPT-6" → gpt6). Plain words of 1–2 letters, stopwords and lone years are dropped.
// The value is true for a name-like token: written with a capital letter or a digit somewhere in the text
// ("Roadster", "DeepSeek", "816B"), as opposed to plain words ("parameters").
func hotTokens(text string) map[string]bool {
	out := map[string]bool{}
	for _, loc := range hotTokRe.FindAllStringIndex(text, -1) {
		raw := text[loc[0]:loc[1]]
		// capital at a sentence start says nothing ("Build your …"); digits / inner capitals always do
		name := strings.ContainsFunc(raw, func(r rune) bool { return r >= '0' && r <= '9' }) ||
			strings.ContainsFunc(raw[1:], func(r rune) bool { return r >= 'A' && r <= 'Z' }) ||
			(raw[0] >= 'A' && raw[0] <= 'Z' && (!sentenceStart(text[:loc[0]]) || nameFollows(text[loc[1]:])))
		raw = strings.ToLower(strings.TrimRight(raw, ".+"))
		parts := strings.FieldsFunc(raw, func(r rune) bool { return r == '+' })
		for _, p := range parts {
			p = strings.Trim(p, ".")
			if p == "" || hotStop[p] {
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
				continue // "6", "12", "2026"
			case digits == 0 && len(p) < 3:
				continue
			}
			out[p] = out[p] || name
		}
	}
	return out
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

// hotRoundup: digests / weekly round-ups mention many stories; they only join a topic by an identical title.
var hotRoundup = regexp.MustCompile(`(?i)\b(this week|week in|weekly|roundup|recap|digest|newsletter)\b|周报|一周|本周|日报|早报|晚报|速览|汇总|盘点`)

// hotText is what the tokens come from: title + the start of the summary (X posts: the post text).
func hotText(r *Row) string {
	return r.Str("title") + "\n" + r.Str("title_zh") + "\n" + capRunes(r.Str("summary"), 300)
}

// joinNumbered rejoins "gpt-6" style names before tokenizing, so the version stays part of the name.
var hotDashNum = regexp.MustCompile(`(?i)\b([a-z]{2,})(?:-(\d{1,2}(?:\.\d+)?)|\s(\d{1,2}\.\d+))\b`)
var hotURL = regexp.MustCompile(`https?://\S+|\b[\w.-]+\.(?:com|ai|io|org|net|co)/\S*|@\w+`)

type hotDoc struct {
	row     *Row
	key     briefKey
	toks    map[string]bool // title + summary start (value: name-like)
	title   map[string]bool // title only
	pub     float64
	roundup bool
}

type hotIdx struct {
	df map[string]int
	n  int
}

// newHotIdx: document frequency counted per SOURCE (how many sources use the name), so a story one account posts
// a 5-tweet thread about doesn't make its own names look common.
func newHotIdx(docs []*hotDoc) hotIdx {
	x := hotIdx{df: map[string]int{}}
	seen := map[string]bool{}
	srcs := map[float64]bool{}
	for _, d := range docs {
		sid := d.row.Num("source_id")
		srcs[sid] = true
		for t := range d.toks {
			k := t + "\x00" + strconv.FormatFloat(sid, 'f', 0, 64)
			if !seen[k] {
				seen[k] = true
				x.df[t]++
			}
		}
	}
	x.n = len(srcs)
	return x
}

func (x hotIdx) idf(t string) float64 { return math.Log(float64(x.n+1) / float64(x.df[t])) }
func (x hotIdx) rare(t string) bool   { return x.df[t] <= max(4, x.n/5) }

// distinctive: a rare name with letters and digits ("816b", "v4.1", "gpt6") or a name of 5+ letters that at
// most 3 texts of the window use ("roadster"; not "openai", "google" — they are in every other story)
func (x hotIdx) distinctive(t string) bool {
	if !x.rare(t) || hotOrgs[t] {
		return false
	}
	hasD, hasL := false, false
	for _, r := range t {
		if r >= '0' && r <= '9' {
			hasD = true
		} else if r != '.' {
			hasL = true
		}
	}
	return (hasD && hasL && len(t) >= 3) || (hasL && !hasD && len(t) >= 5 && x.df[t] <= 3)
}

// hotScore: rare names both texts use, how many of them are distinctive, and their share of the smaller
// text's names (idf-weighted). Plain words never link two stories.
func (x hotIdx) hotScore(a, b *hotDoc) (rare, dist int, cover float64) {
	var shared, ta, tb float64
	inTitle := false
	for t, name := range a.toks {
		if !name {
			continue
		}
		w := x.idf(t)
		ta += w
		if b.toks[t] {
			shared += w
			if x.rare(t) {
				rare++
				if x.distinctive(t) {
					dist++
				}
				inTitle = inTitle || a.title[t] || b.title[t]
			}
		}
	}
	for t, name := range b.toks {
		if name {
			tb += x.idf(t)
		}
	}
	if ta == 0 || tb == 0 || !inTitle { // a name both share must be in one of the headlines, not just mentioned
		return 0, 0, 0
	}
	return rare, dist, shared / math.Min(ta, tb)
}

func (x hotIdx) same(a, b *hotDoc) bool {
	if math.Abs(a.pub-b.pub) > hotWindowSame {
		return false
	}
	if sameStory(a.key, b.key) {
		return true
	}
	if a.roundup || b.roundup {
		return false
	}
	if a.row.Num("source_id") == b.row.Num("source_id") {
		// one source: only the parts of one X thread (posted within 3 minutes); a source's recurring names
		// ("Intelligence Index") are rare across sources but say nothing about the story
		// and beyond a minute only when they share a name (two unrelated posts minutes apart are not one thread)
		return a.row.Num("is_x") == 1 && b.row.Num("is_x") == 1 && (math.Abs(a.pub-b.pub) <= 60 || math.Abs(a.pub-b.pub) <= 180 && shareName(a, b))
	}
	rare, dist, cover := x.hotScore(a, b)
	return (dist >= 2 && cover >= 0.25) || (rare >= 2 && dist >= 1 && cover >= 0.3) || (dist >= 1 && cover >= 0.5)
}

func shareName(a, b *hotDoc) bool {
	for t, name := range a.toks {
		if name && b.toks[t] {
			return true
		}
	}
	return false
}

// clusterHot: connected components of the "same story" relation (union-find over all pairs; the window is a few
// hundred items, so O(n²) small set intersections stay in the low milliseconds). Groups keep the input order.
func clusterHot(docs []*hotDoc) [][]*hotDoc {
	idx := newHotIdx(docs)
	parent := make([]int, len(docs))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i := range docs {
		for j := i + 1; j < len(docs); j++ {
			if find(i) != find(j) && idx.same(docs[i], docs[j]) {
				parent[find(j)] = find(i)
			}
		}
	}
	pos := map[int]int{}
	var groups [][]*hotDoc
	for i, d := range docs {
		r := find(i)
		g, ok := pos[r]
		if !ok {
			g = len(groups)
			pos[r] = g
			groups = append(groups, nil)
		}
		groups[g] = append(groups[g], d)
	}
	return groups
}

func hotPrep(t string) string {
	return hotDashNum.ReplaceAllString(hotURL.ReplaceAllString(t, " "), "$1$2$3")
}

func newHotDoc(r *Row) *hotDoc {
	return &hotDoc{row: r, key: keyOf(r), toks: hotTokens(hotPrep(hotText(r))), title: hotTokens(hotPrep(r.Str("title") + "\n" + r.Str("title_zh"))), pub: r.Num("published"),
		roundup: hotRoundup.MatchString(r.Str("title") + " " + r.Str("title_zh"))}
}

// hotTopics computes the topics (≥ 2 sources) sorted by heat.
func (s *Store) hotTopics(t float64) []HotTopic {
	cut := t - retentionHours()*3600
	rs, _ := all(s.DB, `SELECT i.id, i.title, i.title_zh, i.summary, i.url, i.published, i.is_x, i.merged_into, i.uid,
		s.id AS source_id, s.name AS source, s.priority
		FROM items i JOIN sources s ON s.id=i.source_id LEFT JOIN sections sc ON sc.key=s.section
		WHERE i.hidden=0 AND s.enabled=1 AND i.published >= ? AND COALESCE(sc.in_all, 1)=1
		ORDER BY i.published ASC`, cut)
	var docs []*hotDoc
	merged := map[int64][]*Row{} // article id → tweets merged into it
	for _, r := range rs {
		if m := r.Num("merged_into"); m > 0 {
			merged[int64(m)] = append(merged[int64(m)], r)
			continue
		}
		docs = append(docs, newHotDoc(r))
	}
	var out []HotTopic
	for _, g := range clusterHot(docs) {
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
		ht := HotTopic{Reports: len(reports), Spark: make([]int, hotSparkHours)}
		sort.SliceStable(reports, func(i, j int) bool { return reports[i].Num("published") < reports[j].Num("published") })
		seenSrc := map[int64]bool{}
		for _, r := range reports {
			p := r.Num("published")
			w := 0.3
			if sid := int64(r.Num("source_id")); !seenSrc[sid] {
				seenSrc[sid], w = true, 1
			}
			age := math.Max(0, t-p) / 3600
			ht.Heat += w * math.Pow(0.5, age/12)
			if h := int(age); h < hotSparkHours {
				ht.Spark[hotSparkHours-1-h]++
			}
			ht.Last = math.Max(ht.Last, p)
		}
		ht.Heat = math.Round(ht.Heat*100) / 100
		f := reports[0]
		ht.First.Source, ht.First.Published = f.Str("source"), f.Num("published")
		if m := f.Num("merged_into"); m > 0 {
			ht.First.Item = int64(m)
		} else {
			ht.First.Item = int64(f.Num("id"))
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
	return out
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
