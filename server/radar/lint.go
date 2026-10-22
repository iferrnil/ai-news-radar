package radar

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// slopLint is the mechanical half of the Skills' self-check (references/checklist.md): banned phrases, the
// patterns that only count when they pile up, and each platform's hard limits. composeArticle / polishBrief send
// what it finds back for one revision pass; the model does the judgement-based half itself.

var slopBanned = []string{
	// 开场 / 预告
	"在当今", "随着科技的", "随着人工智能的", "随着AI的", "近年来，", "众所周知", "本文将", "本期简报", "让我们一起", "让我们来看看", "今天给大家",
	// 过渡
	"值得一提的是", "值得注意的是", "不难发现", "毋庸置疑", "不可否认", "显而易见",
	// 收尾
	"综上所述", "总而言之", "总的来说", "总之，", "让我们拭目以待", "拭目以待", "未来可期", "敬请期待", "写在最后", "未来已来",
	// 夸饰 / 空话
	"重磅", "炸裂", "震撼", "颠覆性", "革命性", "里程碑式", "史诗级", "王炸", "遥遥领先", "引爆", "刷屏", "火出圈", "封神", "天花板", "杀疯了",
	"赋能", "助力", "全方位", "多维度", "深度融合", "持续发力", "开启新篇章", "注入新动能", "迈出了重要一步", "具有重要意义", "凸显了", "彰显了",
	// 自我指涉
	"作为一个AI", "作为AI", "以下是改写", "希望对你有帮助", "希望这篇", "如有疑问",
	// 小红书极限词 / 营销
	"全网最", "史上最", "绝绝子", "yyds", "闭眼入", "无脑冲", "必入", "100%有效",
}

var (
	reNotButCh   = regexp.MustCompile(`不是[^。！？\n]{1,30}而是`)
	reConnective = regexp.MustCompile(`此外|同时，|然而|另外，|与此同时|因此，|并且`)
	reAdverbSay  = regexp.MustCompile(`[\p{Han}]{2}地(表示|指出|强调|说|宣布|透露)`)
	reBoldRun    = regexp.MustCompile(`\*\*[^*\n]+\*\*`)
	reHashtag    = regexp.MustCompile(`(?:^|[\s　])#[^\s#]{1,30}`)
	reURL        = regexp.MustCompile(`https?://\S+`)
	reMdLinkURL  = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^)\s]+)\)`)
	reH1         = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	reH2         = regexp.MustCompile(`(?m)^##\s+(.+)$`)
	reRefNum     = regexp.MustCompile(`\[\[(\d{1,2})\]\]`)
)

func setOf(ms [][]string) map[string]bool {
	out := map[string]bool{}
	for _, m := range ms {
		out[m[1]] = true
	}
	return out
}

// titleLimit: the platforms whose title the pipeline enforces (runes, everything counted).
var titleLimit = map[string][2]int{"wechat": {15, 30}, "xhs": {6, 20}} // {ideal-from, max}

// articleTitle finds the title line: wechat's first "# " line; xhs's first line. at = -1 when there is none (an
// xhs note whose first line is really a body sentence).
func articleTitle(md, platform string) (title string, at int) {
	lines := strings.Split(md, "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if platform == "wechat" {
			if strings.HasPrefix(t, "# ") {
				return strings.TrimSpace(t[2:]), i
			}
			continue
		}
		t = strings.TrimSpace(strings.TrimLeft(t, "# "))
		if utf8.RuneCountInString(t) > 40 || strings.ContainsAny(t, "。！？") || strings.HasPrefix(t, "1️⃣") {
			return "", -1
		}
		return t, i
	}
	return "", -1
}

const titleSystem = `<stage>title</stage>
你是%s编辑。用户消息是一篇写好的文章。给它写 8 个标题候选。
硬性要求：每个标题最多 %d 个字符——汉字、每个英文字母、每个数字、空格和标点都各算 1 个，所以「DeepSeek」就占 8 个。目标 %d–%d 个字符。
技巧：最多保留一个英文名；去掉版本号、日期和单位；只留一个最关键的数字或变化；关键词或产品名放前面。
只用文章里出现的事实；不用问号、感叹号和夸饰词（重磅、炸裂、杀疯了、天花板）。每行一个标题，不要编号、引号和解释。`

// compactTitle drops the spaces next to CJK characters ("DeepSeek 新模型" → "DeepSeek新模型"), allowed in Chinese
// typography and worth 2–4 characters against a 20-character limit.
func compactTitle(t string) string {
	rs := []rune(t)
	var b strings.Builder
	for i, r := range rs {
		if r == ' ' && i > 0 && i+1 < len(rs) && (hasCJK(string(rs[i-1])) || hasCJK(string(rs[i+1])) || strings.ContainsRune("，：、", rs[i-1])) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func titleCandidates(v string) []string {
	var out []string
	for _, l := range strings.Split(v, "\n") {
		l = strings.Trim(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "#0123456789.、)-* ")), "「」“”\"'《》")
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// fixTitle: a title over the platform limit (or a missing xhs title) is replaced by the first fitting candidate
// from a dedicated short call — models keep missing character limits, counting is done here.
func (s *Store) fixTitle(md, platform string) (string, bool) {
	lim, ok := titleLimit[platform]
	if !ok {
		return md, false
	}
	title, at := articleTitle(md, platform)
	n := utf8.RuneCountInString(title)
	if at >= 0 && n <= lim[1] {
		return md, false
	}
	label := map[string]string{"wechat": "公众号", "xhs": "小红书"}[platform]
	sys := fmt.Sprintf(titleSystem, label, lim[1], lim[1]*3/5, lim[1]-2)
	user := bodyOnly(md)
	best := ""
	for round := 0; round < 2 && best == ""; round++ { // the model can't count; the code does, and asks once more
		v, err := s.dsChat(sys, user, 0.5, 400, false)
		if err != nil {
			return md, false
		}
		var long []string
		for _, l := range titleCandidates(v) {
			k := utf8.RuneCountInString(l)
			if k > lim[1] {
				l = compactTitle(l)
				k = utf8.RuneCountInString(l)
			}
			if k > lim[1] {
				long = append(long, fmt.Sprintf("- （%d 个字符）%s", k, l))
				continue
			}
			if k < lim[0]/2 || len(slopLint(l, "")) > 0 {
				continue
			}
			if best == "" || (utf8.RuneCountInString(best) < lim[0] && k >= lim[0]) {
				best = l
			}
		}
		user = bodyOnly(md) + "\n\n上一轮的候选都超过 " + fmt.Sprint(lim[1]) + " 个字符：\n" + strings.Join(long, "\n") + "\n再写 8 个更短的。"
	}
	if best == "" {
		return md, false
	}
	lines := strings.Split(md, "\n")
	switch {
	case at < 0 && platform == "xhs":
		return best + "\n\n" + strings.TrimLeft(md, "\n"), true
	case at < 0:
		return "# " + best + "\n\n" + strings.TrimLeft(md, "\n"), true
	case platform == "wechat":
		lines[at] = "# " + best
	default:
		lines[at] = best
	}
	return strings.Join(lines, "\n"), true
}

func isEmoji(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1FAFF) || (r >= 0x2600 && r <= 0x27BF) || r == 0x2B50 || r == 0x2705
}

func countEmoji(s string) int {
	n := 0
	for _, r := range s {
		if isEmoji(r) {
			n++
		}
	}
	return n
}

// xWeight: X's weighted length — CJK and most non-Latin characters count 2, a URL 23.
func xWeight(s string) int {
	s = reMdLinkURL.ReplaceAllString(s, "$1 $2")
	n := 23 * len(reURL.FindAllString(s, -1))
	s = reURL.ReplaceAllString(s, "")
	for _, r := range s {
		if r <= 0x10FF || (r >= 0x2000 && r <= 0x200D) || (r >= 0x2010 && r <= 0x201F) || (r >= 0x2032 && r <= 0x2037) {
			n++
		} else {
			n += 2
		}
	}
	return n
}

// bodyOnly: the text without the 原文链接 list (links do not count for style or length).
func bodyOnly(md string) string {
	if i := strings.Index(md, "\n## 原文链接"); i >= 0 {
		return md[:i]
	}
	return md
}

// tweetParts splits a twitter draft into posts: blank-line separated paragraphs. Leading "N/" numbering
// (older skill output) is stripped so length checks still apply. Empty parts are dropped.
func tweetParts(body string) []string {
	raw := strings.Split(body, "\n")
	var parts []string
	var cur []string
	flush := func() {
		p := strings.TrimSpace(strings.Join(cur, "\n"))
		cur = cur[:0]
		if p == "" {
			return
		}
		// drop legacy "1/" / "12/" prefixes (require space or EOL after the slash so "3/5" stays)
		if i := strings.IndexByte(p, '/'); i > 0 && i <= 2 {
			n := p[:i]
			ok := true
			for _, c := range n {
				if c < '0' || c > '9' {
					ok = false
					break
				}
			}
			if ok && (i+1 == len(p) || p[i+1] == ' ' || p[i+1] == '	') {
				p = strings.TrimSpace(p[i+1:])
			}
		}
		if p != "" {
			parts = append(parts, p)
		}
	}
	for _, ln := range raw {
		if strings.TrimSpace(ln) == "" {
			flush()
			continue
		}
		cur = append(cur, ln)
	}
	flush()
	return parts
}

func slopLint(md, platform string) []string {
	body := bodyOnly(md)
	var out []string
	flat := strings.ReplaceAll(body, " ", "")
	var hit []string
	for _, p := range slopBanned {
		if strings.Contains(flat, strings.ReplaceAll(p, " ", "")) {
			hit = append(hit, "「"+p+"」")
		}
	}
	if len(hit) > 0 {
		out = append(out, "禁用词句："+strings.Join(hit, "、"))
	}
	if n := len(reNotButCh.FindAllString(body, -1)); n > 1 {
		out = append(out, fmt.Sprintf("「不是……而是……」用了 %d 次（最多 1 次）", n))
	}
	if n := len(reConnective.FindAllString(body, -1)); n > 3 {
		out = append(out, fmt.Sprintf("连接词（此外 / 同时 / 然而 / 另外 / 因此 / 并且）用了 %d 次，删掉多余的，让句子直接相接", n))
	}
	if m := reAdverbSay.FindAllString(body, -1); len(m) > 0 {
		out = append(out, "说话动词前的「X地」状语："+strings.Join(uniq(m), "、"))
	}
	switch platform {
	case "wechat":
		if m := reH1.FindStringSubmatch(body); m == nil {
			out = append(out, "缺少 # 标题")
		} else if n := utf8.RuneCountInString(strings.TrimSpace(m[1])); n > 30 {
			out = append(out, fmt.Sprintf("标题 %d 字（不超过 30，最好 15–25）", n))
		}
		if n := len(reH2.FindAllString(body, -1)); n > 7 {
			out = append(out, fmt.Sprintf("小标题 %d 个（3–5 个为宜）", n))
		}
		if n := len(reBoldRun.FindAllString(body, -1)); n > 5 {
			out = append(out, fmt.Sprintf("加粗 %d 处（只加粗关键数据，最多 5 处）", n))
		}
		if n := countEmoji(body); n > 0 {
			out = append(out, fmt.Sprintf("公众号正文不用 emoji（有 %d 个）", n))
		}
	case "xhs":
		lines := strings.Split(strings.TrimSpace(body), "\n")
		title := strings.TrimSpace(strings.TrimLeft(lines[0], "# "))
		if n := utf8.RuneCountInString(title); n > 20 {
			out = append(out, fmt.Sprintf("标题 %d 字（不超过 20）", n))
		}
		if n := utf8.RuneCountInString(strings.TrimSpace(reMdLinkURL.ReplaceAllString(body, "$1"))); n > 1000 {
			out = append(out, fmt.Sprintf("正文 %d 字（含标点、emoji 和话题不超过 1000）", n))
		}
		if n := len(reHashtag.FindAllString(body, -1)); n == 0 || n > 6 {
			out = append(out, fmt.Sprintf("话题标签 %d 个（3–5 个）", n))
		}
		if n := countEmoji(body); n > 10 {
			out = append(out, fmt.Sprintf("emoji %d 个（最多 8 个）", n))
		}
	case "twitter":
		parts := tweetParts(body)
		for i, part := range parts {
			if w := xWeight(part); w > 280 {
				out = append(out, fmt.Sprintf("第 %d 条推文加权长度 %d（上限 280，汉字算 2，链接算 23）", i+1, w))
			}
		}
		refs := map[string]int{}
		for _, part := range parts {
			for n := range setOf(reRefNum.FindAllStringSubmatch(part, -1)) {
				refs[n]++
			}
		}
		for _, n := range sortedKeys(refs) {
			if refs[n] > 1 {
				out = append(out, fmt.Sprintf("链接 [[%s]] 出现在 %d 条推文里（全串只放一次，放在讲这条新闻的最后一条）", n, refs[n]))
			}
		}
		if len(parts) > 12 {
			out = append(out, fmt.Sprintf("串推 %d 条（5–9 条为宜）", len(parts)))
		}
		if n := len(reHashtag.FindAllString(body, -1)); n > 2 {
			out = append(out, fmt.Sprintf("话题标签 %d 个（最多 2 个）", n))
		}
		if n := len(reBoldRun.FindAllString(body, -1)); n > 0 {
			out = append(out, "推文不支持加粗，去掉 ** **")
		}
	}
	return out
}

const reviseSystem = `<stage>revise</stage>
你是中文编辑。用户消息给出一篇草稿 <draft> 和自检发现的问题 <issues>。
只改有问题的句子：删掉或换掉禁用词句（换成具体的事实或直接删除，不要换成同义的套话）；超长的就删次要信息，不改事实；
不增加任何新事实、数字、引语或评价；保持原有结构、平台格式、[[n]](url) 链接与「原文链接」列表不变；不要输出封面标记行。
只输出修改后的完整文章（简体中文 Markdown），不要解释。`

// selfCheck: lint → at most one revision pass → lint again. Returns the text and what is still flagged.
func (s *Store) selfCheck(md, platform string, refs ...string) (string, []string) {
	issues := articleLint(md, platform, refs...)
	if len(issues) == 0 {
		return md, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<draft>\n%s\n</draft>\n\n<issues>\n", md)
	for _, x := range issues {
		fmt.Fprintf(&b, "- %s\n", x)
	}
	b.WriteString("</issues>\n")
	v, err := s.dsChat(reviseSystem, b.String(), 0.2, 6000, false)
	if err != nil || utf8.RuneCountInString(v) < utf8.RuneCountInString(md)/3 {
		return md, issues
	}
	v, _ = extractCovers(v)
	return v, articleLint(v, platform, refs...)
}

// articleLint = slopLint + every input story's [[n]](url) link present (refs = the selected items' urls).
func articleLint(md, platform string, refs ...string) []string {
	out := slopLint(md, platform)
	if len(refs) > 0 {
		body := bodyOnly(md)
		var miss []string
		for i, u := range refs {
			if !strings.Contains(body, fmt.Sprintf("[[%d]]", i+1)) {
				miss = append(miss, fmt.Sprintf("[[%d]](%s)", i+1, u))
			}
		}
		if len(miss) > 0 {
			out = append(out, "缺少链接 "+strings.Join(miss, "、")+"（每条新闻至少引用一次，放在讲它的段落 / 推文末尾）")
		}
	}
	return out
}
