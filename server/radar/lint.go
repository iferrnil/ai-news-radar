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
	reTweetPart  = regexp.MustCompile(`(?m)^\s*(\d{1,2})/\s*`)
	reURL        = regexp.MustCompile(`https?://\S+`)
	reMdLinkURL  = regexp.MustCompile(`\[([^\]]*)\]\((https?://[^)\s]+)\)`)
	reH1         = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	reH2         = regexp.MustCompile(`(?m)^##\s+(.+)$`)
)

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
		locs := reTweetPart.FindAllStringIndex(body, -1)
		for i, l := range locs {
			end := len(body)
			if i+1 < len(locs) {
				end = locs[i+1][0]
			}
			part := strings.TrimSpace(body[l[0]:end])
			if w := xWeight(part); w > 280 {
				out = append(out, fmt.Sprintf("第 %d 条推文加权长度 %d（上限 280，汉字算 2，链接算 23）", i+1, w))
			}
		}
		if len(locs) > 12 {
			out = append(out, fmt.Sprintf("串推 %d 条（5–9 条为宜）", len(locs)))
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
func (s *Store) selfCheck(md, platform string) (string, []string) {
	issues := slopLint(md, platform)
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
	return v, slopLint(v, platform)
}
