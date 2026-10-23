package radar

import (
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Brief item selection (自动简报): current section · last BRIEF_HOURS (default 24) only; never uses the hot-topics
// page. Stories are matched by URL and same / near-identical title (also across sources and between the original
// and the translated title). Cap BRIEF_MAX_ITEMS (default 20); never pad. 手动简报 uses exactly the chosen items.
const (
	briefPick      = 20  // 自动简报 default (BRIEF_MAX_ITEMS)
	briefManualCap = 10  // 手动简报 UI / ids= cap (unchanged)
	briefHours     = 24  // pool window (BRIEF_HOURS); 全部 uses only its own 24h pool
	briefSimilar   = 0.6 // Dice coefficient of character bigrams on normalized titles
	// briefShellMin: after stripping URLs + whitespace, content|summary below this many runes is an
	// empty shell (link-only / a few words; ~20 keeps short-but-real RSS blurbs) — skipped at 自动简报 pick so in-app + Telegram never get them.
	// Manual picks (ids=) are unchanged. AI polish still uses briefDropMin (120) on fetched full text.
	briefShellMin = 20
)


// strip URLs so a body that is only "https://…" counts as empty.
var briefURLRe = regexp.MustCompile(`(?i)https?://\S+`)

// briefBodyText is the stored body used at pick time (full content, else summary).
func briefBodyText(r *Row) string {
	if c := strings.TrimSpace(r.Str("content")); c != "" {
		return c
	}
	return strings.TrimSpace(r.Str("summary"))
}

// isBriefShell: a *stored* body that is only a URL or a few words after stripping links/whitespace.
// Title-only rows (no summary/content yet) are kept — common for thin RSS; AI polish still uses briefDropMin.
func isBriefShell(r *Row) bool {
	raw := briefBodyText(r)
	if raw == "" {
		return false
	}
	s := briefURLRe.ReplaceAllString(raw, " ")
	s = strings.Join(strings.Fields(s), " ")
	return utf8.RuneCountInString(s) < briefShellMin
}

// titleSuffixes are publisher tails ("… - The Verge", "… | 机器之心") that make one story look different.
var titleSep = []string{" | ", " - ", " — ", " – ", "｜", " · "}

// normTitle lowercases, drops a trailing publisher part and keeps only letters / digits.
func normTitle(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	for _, sep := range titleSep {
		if i := strings.LastIndex(t, sep); i > 0 && len([]rune(t[i+len(sep):])) <= 24 && len([]rune(t[:i])) >= 12 {
			t = t[:i]
		}
	}
	var b strings.Builder
	for _, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func bigrams(s string) map[string]int {
	r := []rune(s)
	m := map[string]int{}
	for i := 0; i+1 < len(r); i++ {
		m[string(r[i:i+2])]++
	}
	return m
}

// dice is the Dice coefficient of the bigram multisets of a and b (1 = identical).
func dice(a, b map[string]int) float64 {
	na, nb, inter := 0, 0, 0
	for _, v := range a {
		na += v
	}
	for k, v := range b {
		nb += v
		inter += min(v, a[k])
	}
	if na+nb == 0 {
		return 0
	}
	return 2 * float64(inter) / float64(na+nb)
}

type briefKey struct {
	url    string
	titles []string
	grams  [][]gramC // bigram multisets, sorted by gram
	sums   []int     // bigram totals of grams
}

// gramC: one character bigram (two runes packed) and its count.
type gramC struct {
	g uint64
	n int
}

func gramVec(s string) ([]gramC, int) {
	r := []rune(s)
	if len(r) < 2 {
		return nil, 0
	}
	gs := make([]uint64, 0, len(r)-1)
	for i := 0; i+1 < len(r); i++ {
		gs = append(gs, uint64(uint32(r[i]))<<32|uint64(uint32(r[i+1])))
	}
	slices.Sort(gs)
	var out []gramC
	for _, g := range gs {
		if n := len(out); n > 0 && out[n-1].g == g {
			out[n-1].n++
		} else {
			out = append(out, gramC{g, 1})
		}
	}
	return out, len(gs)
}

// diceVec: dice of two sorted bigram vectors with their totals (same result as dice on the maps).
func diceVec(a, b []gramC, na, nb int) float64 {
	if na+nb == 0 {
		return 0
	}
	inter := 0
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i].g == b[j].g:
			inter += min(a[i].n, b[j].n)
			i++
			j++
		case a[i].g < b[j].g:
			i++
		default:
			j++
		}
	}
	return 2 * float64(inter) / float64(na+nb)
}

func keyOf(r *Row) briefKey {
	k := briefKey{url: NormURL(r.Str("url"))}
	for _, t := range []string{r.Str("title"), r.Str("title_zh")} {
		if n := normTitle(t); len([]rune(n)) >= 6 {
			k.titles = append(k.titles, n)
			g, sum := gramVec(n)
			k.grams = append(k.grams, g)
			k.sums = append(k.sums, sum)
		}
	}
	return k
}

func sameStory(a, b briefKey) bool {
	if a.url != "" && a.url == b.url {
		return true
	}
	for i, ta := range a.titles {
		for j, tb := range b.titles {
			na, nb := a.sums[i], b.sums[j]
			if ta == tb || (2*float64(min(na, nb)) >= briefSimilar*float64(na+nb) && diceVec(a.grams[i], b.grams[j], na, nb) >= briefSimilar) {
				return true
			}
		}
	}
	return false
}

func prioRank(p string) int {
	switch p {
	case "P0":
		return 0
	case "P1":
		return 1
	default:
		return 2
	}
}

// betterRep: prefer P0, then newer (for the story representative).
func betterRep(a, b *Row) bool {
	pa, pb := prioRank(a.Str("priority")), prioRank(b.Str("priority"))
	if pa != pb {
		return pa < pb
	}
	return a.Num("published") > b.Num("published")
}

// isMorningSH: published today 00:00–08:00 Asia/Shanghai (relative to at).
func isMorningSH(pub float64, at time.Time) bool {
	t := time.Unix(int64(pub), 0).In(cst)
	day0 := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, cst)
	return !t.Before(day0) && t.Before(day0.Add(8*time.Hour))
}

// fillLess: sort key for remaining slots — morning first, then P0, then newer.
func fillLess(a, b *Row, at time.Time) bool {
	ma, mb := isMorningSH(a.Num("published"), at), isMorningSH(b.Num("published"), at)
	if ma != mb {
		return ma // morning before non-morning
	}
	pa, pb := prioRank(a.Str("priority")), prioRank(b.Str("priority"))
	if pa != pb {
		return pa < pb
	}
	return a.Num("published") > b.Num("published")
}

// dedupeBrief keeps the first of every story (rows come preferred-rep first).
func dedupeBrief(rs []*Row) []*Row {
	var out []*Row
	var keys []briefKey
	for _, r := range rs {
		k := keyOf(r)
		dup := false
		for _, o := range keys {
			if sameStory(k, o) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, r)
			keys = append(keys, k)
		}
	}
	return out
}

// storyGroups clusters rows into stories. The first row of a group is the representative (P0 then newer).
func storyGroups(rs []*Row) [][]*Row {
	var groups [][]*Row
	var keys []briefKey
	for _, r := range rs {
		k := keyOf(r)
		hit := -1
		for i, o := range keys {
			if sameStory(k, o) {
				hit = i
				break
			}
		}
		if hit < 0 {
			groups = append(groups, []*Row{r})
			keys = append(keys, k)
		} else {
			groups[hit] = append(groups[hit], r)
			// keep the preferred representative at [0]
			if betterRep(r, groups[hit][0]) {
				g := groups[hit]
				g[0], g[len(g)-1] = g[len(g)-1], g[0]
				keys[hit] = k
			}
		}
	}
	return groups
}

func briefCutoff() float64 {
	return now() - float64(envIntDef("BRIEF_HOURS", briefHours))*3600
}

func briefMax() int { return envIntDef("BRIEF_MAX_ITEMS", briefPick) }

// autoPick (自动简报): one item per story; multi-coverage first (higher coverage first); remaining slots prefer
// today 00:00–08:00 Asia/Shanghai, then other hours, with P0 weighted; never pad past the pool. Final dedupe.
// exclude skips already-kept ids (refill after a full-text drop).
func autoPick(rs []*Row, n int) []*Row {
	return autoPickAt(rs, n, time.Now().In(cst), nil)
}

func autoPickAt(rs []*Row, n int, at time.Time, exclude map[int64]bool) []*Row {
	if n <= 0 {
		return nil
	}
	var pool []*Row
	for _, r := range rs {
		if exclude != nil && exclude[int64(r.Num("id"))] {
			continue
		}
		if isBriefShell(r) {
			continue // empty shell: no real body for a briefing
		}
		pool = append(pool, r)
	}
	groups := storyGroups(pool)
	var hotG, restG [][]*Row
	for _, g := range groups {
		if len(g) > 1 {
			hotG = append(hotG, g)
		} else {
			restG = append(restG, g)
		}
	}
	sort.SliceStable(hotG, func(i, j int) bool {
		if len(hotG[i]) != len(hotG[j]) {
			return len(hotG[i]) > len(hotG[j])
		}
		return betterRep(hotG[i][0], hotG[j][0])
	})
	var out []*Row
	for _, g := range hotG {
		if len(out) >= n {
			break
		}
		out = append(out, g[0])
	}
	if len(out) < n {
		rest := make([]*Row, 0, len(restG))
		for _, g := range restG {
			rest = append(rest, g[0])
		}
		sort.SliceStable(rest, func(i, j int) bool { return fillLess(rest[i], rest[j], at) })
		for _, r := range rest {
			if len(out) >= n {
				break
			}
			out = append(out, r)
		}
	}
	// final dedupe pass (same URL / near title / same story)
	out = dedupeBrief(out)
	if len(out) > n {
		out = out[:n]
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Num("published") > out[j].Num("published") })
	return out
}

// groupBrief groups picked rows by source in order of each source's newest item.
func groupBrief(picked []*Row) (order []int64, groups map[int64][]*Row, meta map[int64][2]string) {
	groups, meta = map[int64][]*Row{}, map[int64][2]string{}
	for _, r := range picked {
		sid := int64(r.Num("source_id"))
		if _, ok := groups[sid]; !ok {
			order = append(order, sid)
			meta[sid] = [2]string{r.Str("source"), r.Str("priority")}
		}
		groups[sid] = append(groups[sid], r)
	}
	return
}

// selectBrief: the 自动简报 pick (autoPick), grouped by source.
func selectBrief(rs []*Row) (order []int64, groups map[int64][]*Row, meta map[int64][2]string) {
	return groupBrief(autoPick(rs, briefMax()))
}

// selectBriefRefill picks up to need more rows from rs, skipping exclude ids / stories already represented.
func selectBriefRefill(rs []*Row, need int, exclude map[int64]bool, takenKeys []briefKey) []*Row {
	if need <= 0 {
		return nil
	}
	ex := map[int64]bool{}
	for id := range exclude {
		ex[id] = true
	}
	// also skip same-story as already taken
	var filtered []*Row
	for _, r := range rs {
		if ex[int64(r.Num("id"))] {
			continue
		}
		k := keyOf(r)
		dup := false
		for _, o := range takenKeys {
			if sameStory(k, o) {
				dup = true
				break
			}
		}
		if !dup {
			filtered = append(filtered, r)
		}
	}
	return autoPickAt(filtered, need, time.Now().In(cst), nil)
}
