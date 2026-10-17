package radar

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// condense shrinks an original text to about budget runes before it goes to the model, without a model call:
// boilerplate lines (share / subscribe / copyright / related links / image credits / navigation crumbs, Markdown
// images and bare links, repeated paragraphs) are dropped, then whole paragraphs are kept by score — the lead,
// paragraphs with numbers, quotes and names — in their original order. Very long paragraphs are split into
// sentences first. A gap is marked with a line "…". Text already within budget only loses the boilerplate.

var (
	reBoilerLine  = regexp.MustCompile(`(?i)^(\W{0,3})(分享到|分享|点赞|在看|收藏|关注我们|关注公众号|扫码|扫描二维码|长按识别|阅读原文|责任编辑|编辑[:：]|校对[:：]|排版[:：]|作者[:：]|来源[:：]|图源|图片来源|题图|本文来自|本文转载|原标题|版权所有|版权声明|免责声明|未经授权|未经许可|转载请|广告|推荐阅读|相关阅读|相关文章|相关推荐|延伸阅读|热门文章|往期回顾|往期推荐|点击(这里|此处|上方|下方)|订阅|上一篇|下一篇|返回顶部|copyright|©|all rights reserved|subscribe|sign up|sign in|log in|share (this|on)|follow us|related( articles| stories)?[:：]?$|read more|read next|more from|advertisement|sponsored|cookie|newsletter|image[:：]|photo[:：]|credit[:：]|getty images|skip to)`)
	reMdImageLine = regexp.MustCompile(`^!\[[^\]]*\]\([^)]*\)$`)
	reBareLink    = regexp.MustCompile(`^(\[[^\]]*\]\([^)]*\)[\s|·,，、/]*)+$|^https?://\S+$`)
	reNumber      = regexp.MustCompile(`\d[\d,.]*\s*(%|％|亿|万|千|百万|倍|美元|元|美金|港元|欧元|英镑|日元|tokens?|[kKmMbB]\b|GB|TB|MB|ms|秒|分钟|小时|天|个月|年)?|[$¥€£]\s?\d`)
	reQuote       = regexp.MustCompile(`[“「『"][^“”「」『』"]{4,}[”」』"]`)
	reNameLatin   = regexp.MustCompile(`\b[A-Z][A-Za-z0-9]+(?:[-.][A-Za-z0-9]+)*\b`)
	reSentenceZh  = regexp.MustCompile(`[^。！？!?；;]+[。！？!?；;]+["”」』]?|[^。！？!?；;]+$`)
	reSentenceEn  = regexp.MustCompile(`[^.!?]+(?:[.!?]+["”]?\s+|$)`)
)

const shortLine = 24 // runes: a line this short is a cell / label, not a paragraph

func boilerplate(p string) bool {
	t := strings.TrimSpace(p)
	if t == "" || reMdImageLine.MatchString(t) || reBareLink.MatchString(t) {
		return true
	}
	n := utf8.RuneCountInString(t)
	if n <= 60 && reBoilerLine.MatchString(t) {
		return true
	}
	return false
}

func paraScore(p string, idx int) float64 {
	sc := 0.0
	switch idx {
	case 0:
		sc += 4
	case 1:
		sc += 1.5
	case 2:
		sc += 0.5
	}
	sc += minF(4, 0.8*float64(len(reNumber.FindAllString(p, -1))))
	sc += minF(3, 1.5*float64(len(reQuote.FindAllString(p, -1))))
	sc += minF(2, 0.25*float64(len(reNameLatin.FindAllString(p, -1))))
	n := utf8.RuneCountInString(p)
	switch {
	case n < 25:
		sc -= 1.5
	case n > 600:
		sc -= 0.5
	}
	return sc
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func splitSentences(p string) []string {
	re := reSentenceZh
	cjk := 0
	for _, r := range p {
		if unicode.Is(unicode.Han, r) {
			cjk++
		}
	}
	if cjk*4 < utf8.RuneCountInString(p) {
		re = reSentenceEn
	}
	var out []string
	for _, s := range re.FindAllString(p, -1) {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return []string{p}
	}
	return out
}

func condense(text string, budget int) string {
	text = strings.ReplaceAll(text, "\r", "")
	// runs of short lines (table cells, lists of names and amounts, crumbs) become one line "a · b · c", so a
	// table keeps its rows; repeated long paragraphs (teasers, pull quotes) are kept once
	var paras, run []string
	flush := func() {
		if len(run) > 0 {
			paras = append(paras, strings.Join(run, " · "))
			run = nil
		}
	}
	seen := map[string]bool{}
	for _, p := range strings.Split(text, "\n") {
		p = strings.TrimSpace(p)
		if boilerplate(p) {
			continue
		}
		if utf8.RuneCountInString(p) <= shortLine {
			run = append(run, p)
			continue
		}
		flush()
		k := strings.Join(strings.Fields(p), " ")
		if seen[k] {
			continue
		}
		seen[k] = true
		paras = append(paras, p)
	}
	flush()
	total := 0
	for _, p := range paras {
		total += utf8.RuneCountInString(p)
	}
	if budget <= 0 || total <= budget {
		return strings.Join(paras, "\n\n")
	}
	// units: paragraphs; a paragraph longer than a third of the budget is split into sentence runs
	type unit struct {
		text  string
		para  int
		score float64
		n     int
	}
	var units []unit
	for i, p := range paras {
		base := paraScore(p, i)
		if utf8.RuneCountInString(p) <= budget/3 {
			units = append(units, unit{p, i, base, utf8.RuneCountInString(p)})
			continue
		}
		for j, s := range splitSentences(p) {
			if !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
				continue // a stray bracket / quote left by the split
			}
			sc := paraScore(s, i) - 0.3*float64(j) // earlier sentences of a long paragraph first
			if i == 0 && j == 0 {
				sc += 4
			}
			units = append(units, unit{s, i, sc, utf8.RuneCountInString(s)})
		}
	}
	order := make([]int, len(units))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return units[order[a]].score > units[order[b]].score })
	keep := make([]bool, len(units))
	used := 0
	for _, k := range order {
		if used+units[k].n > budget {
			continue
		}
		keep[k] = true
		used += units[k].n
	}
	if used == 0 { // even the best unit is too long
		return Trunc(units[order[0]].text, budget, "…")
	}
	var b strings.Builder
	last := -1
	for i, u := range units {
		if !keep[i] {
			continue
		}
		switch {
		case last < 0:
		case i != last+1:
			b.WriteString("\n…\n")
		case u.para != units[last].para:
			b.WriteString("\n\n")
		case !unicode.Is(unicode.Han, firstRune(u.text)):
			b.WriteString(" ")
		}
		b.WriteString(u.text)
		last = i
	}
	return strings.TrimSpace(b.String())
}

func firstRune(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

// condenseAll condenses each text to perItem runes and all of them to about total runes (the shorter texts
// leave their unused share to the longer ones).
func condenseAll(texts []string, perItem, total int) []string {
	out := make([]string, len(texts))
	if len(texts) == 0 {
		return out
	}
	idx := make([]int, len(texts))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return utf8.RuneCountInString(texts[idx[a]]) < utf8.RuneCountInString(texts[idx[b]])
	})
	left := total
	for k, i := range idx {
		share := left / (len(idx) - k)
		if share > perItem {
			share = perItem
		}
		out[i] = condense(texts[i], share)
		left -= utf8.RuneCountInString(out[i])
		if left < 0 {
			left = 0
		}
	}
	return out
}
