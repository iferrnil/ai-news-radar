package radar

import (
	"sort"
	"strings"
	"unicode"
)

// Brief item selection: dedupe (same URL, same / near-identical title — also across sources and between the
// original and the translated title), then at most briefPick items; when more remain, by time: the 4 oldest,
// 3 from the middle and the 3 newest, so a brief covers the whole window instead of the last hour.
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
	grams  []map[string]int
}

func keyOf(r *Row) briefKey {
	k := briefKey{url: NormURL(r.Str("url"))}
	for _, t := range []string{r.Str("title"), r.Str("title_zh")} {
		if n := normTitle(t); len([]rune(n)) >= 6 {
			k.titles = append(k.titles, n)
			k.grams = append(k.grams, bigrams(n))
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
			if ta == tb || dice(a.grams[i], b.grams[j]) >= briefSimilar {
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

// spreadPick returns at most n rows by time: all of them when they fit, else 4 oldest + 3 middle + 3 newest
// (scaled for other n). The result is sorted newest first.
func spreadPick(rs []*Row, n int) []*Row {
	s := append([]*Row(nil), rs...)
	sort.SliceStable(s, func(i, j int) bool { return s[i].Num("published") < s[j].Num("published") })
	if len(s) > n {
		old, newest := n*4/10, n*3/10
		mid := n - old - newest
		rest := s[old : len(s)-newest]
		start := (len(rest) - mid) / 2
		pick := append(append(append([]*Row(nil), s[:old]...), rest[start:start+mid]...), s[len(s)-newest:]...)
		s = pick
	}
	sort.SliceStable(s, func(i, j int) bool { return s[i].Num("published") > s[j].Num("published") })
	return s
}

// selectBrief: dedupe, then the time spread; grouped by source in order of each source's newest item.
func selectBrief(rs []*Row) (order []int64, groups map[int64][]*Row, meta map[int64][2]string) {
	picked := spreadPick(dedupeBrief(rs), envIntDef("BRIEF_MAX_ITEMS", briefPick))
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
