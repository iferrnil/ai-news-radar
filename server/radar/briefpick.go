package radar

import (
	"slices"
	"sort"
	"strings"
	"unicode"
)

// Brief item selection (自动简报): stories are matched by URL and same / near-identical title (also across sources
// and between the original and the translated title); see autoPick. 手动简报 uses exactly the chosen items.
const (
	briefPick    = 10
	briefSimilar = 0.6 // Dice coefficient of character bigrams on normalized titles
)

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

// dedupeBrief keeps the first of every story (rows come P0 first, newest first).
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

// storyGroups clusters rows into stories (first row of a group = its representative; rows come P0 first, newest first).
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
		}
	}
	return groups
}

// autoPick (自动简报): ≤n rows → all of them (deduped). Otherwise first one item per story that several items /
// sources carry (hot topics, most-covered first), then the rest alternately from the oldest and the newest end of
// the window until n. The result is sorted newest first.
func autoPick(rs []*Row, n int) []*Row {
	groups := storyGroups(rs)
	var hot, rest []*Row
	hotG := [][]*Row{}
	for _, g := range groups {
		if len(g) > 1 {
			hotG = append(hotG, g)
		} else {
			rest = append(rest, g[0])
		}
	}
	var out []*Row
	if len(rs) <= n {
		for _, g := range groups {
			out = append(out, g[0])
		}
	} else {
		sort.SliceStable(hotG, func(i, j int) bool { return len(hotG[i]) > len(hotG[j]) })
		for _, g := range hotG {
			hot = append(hot, g[0])
		}
		if len(hot) > n {
			hot = hot[:n]
		}
		out = hot
		sort.SliceStable(rest, func(i, j int) bool { return rest[i].Num("published") < rest[j].Num("published") })
		lo, hi := 0, len(rest)-1
		for old := true; len(out) < n && lo <= hi; old = !old {
			if old {
				out = append(out, rest[lo])
				lo++
			} else {
				out = append(out, rest[hi])
				hi--
			}
		}
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
	return groupBrief(autoPick(rs, envIntDef("BRIEF_MAX_ITEMS", briefPick)))
}
