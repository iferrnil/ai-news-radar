package radar

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// 生成文章 (desktop only): the chosen items (≤2) → fetch the originals → factual per-item summaries → the
// platform's Skill (推特 / 公众号 / 小红书) writes the article → a fact-check pass. A Skill asks for a cover by
// putting a marker line in its output (see coverRe); it is generated with the 多模态模型 when that is configured,
// otherwise skipped with a short note.

// composeMax: 生成文章 takes at most 2 items (the originals go through the 抓取 API).
const composeMax = 2

var platformSkill = map[string]string{"twitter": "推特文章", "wechat": "公众号文章", "xhs": "小红书文章"}

// <!-- cover-image: prompt -->, <!-- cover-video size=1280x720: prompt -->
var coverRe = regexp.MustCompile(`(?m)^[ \t>]*` + "`?" + `<!--\s*cover-(image|video)(?:\s+size=(\d{2,5}x\d{2,5}))?\s*[:：]\s*([\s\S]+?)\s*-->` + "`?" + `[ \t]*$\n?`)

type coverReq struct {
	Kind   string `json:"kind"` // image | video
	Size   string `json:"size,omitempty"`
	Prompt string `json:"prompt"`
	Status string `json:"status"` // ok | skipped | error
	Note   string `json:"note,omitempty"`
	Path   string `json:"path,omitempty"`
}

type articleOut struct {
	Markdown    string      `json:"markdown"`
	Title       string      `json:"title"`
	Platform    string      `json:"platform"`
	Label       string      `json:"label"`
	Skill       string      `json:"skill"`
	Count       int         `json:"count"`
	Verified    bool        `json:"verified,omitempty"`
	NoFullText  []briefMiss `json:"noFullText,omitempty"`
	Covers      []coverReq  `json:"covers,omitempty"`
	VisionNote  string      `json:"visionNote,omitempty"`
	SelfCheck   []string    `json:"selfCheck,omitempty"` // what the self-check still flags after its revision pass
	GeneratedAt float64     `json:"generatedAt"`
}

// extractCovers removes the cover markers from md and returns them (at most one image and one video).
func extractCovers(md string) (string, []coverReq) {
	var out []coverReq
	seen := map[string]bool{}
	md = coverRe.ReplaceAllStringFunc(md, func(m string) string {
		g := coverRe.FindStringSubmatch(m)
		if !seen[g[1]] && strings.TrimSpace(g[3]) != "" {
			seen[g[1]] = true
			out = append(out, coverReq{Kind: g[1], Size: g[2], Prompt: strings.Join(strings.Fields(g[3]), " ")})
		}
		return ""
	})
	return strings.TrimSpace(md) + "\n", out
}

const articleOutputRules = "\n输出要求：用简体中文写（英文资料译成中文，专有名词保留原文）；只输出文章本身（Markdown），不要解释，不要输出自检过程；按 Skill 的规定放置 [[n]](url) 链接（Skill 没有规定时放在每条新闻引用处），编号与输入一致，不要改动 url。" +
	"若 Skill 要求封面，按 Skill 写明的格式单独一行输出 <!-- cover-image: 画面描述 --> 或 <!-- cover-video: 画面描述 -->；Skill 没有要求就不要输出。" + factRules

const articleVerify = `<stage>verify</stage>
你是事实核查编辑。用户消息给出一篇文章草稿 <brief> 和每条新闻的资料 <source n>。
逐句检查草稿：凡是资料里找不到依据的说法（数字、日期、人名、机构、因果、评价、引语）都删掉或改成资料里的原话；
数字和专名必须与资料一致；不要增加新内容；保持文章原有的结构、语气、平台格式和 [[n]](url) 链接不变。
只输出核查后的完整文章（简体中文 Markdown），不要解释。`

func (s *Store) composeArticle(section, platform string, ids []int64) (*articleOut, error) {
	label, ok := platformSkill[platform]
	if !ok {
		return nil, &httpError{400, "未知的文章类型"}
	}
	if len(ids) == 0 {
		return nil, &httpError{400, "请先选择条目"}
	}
	if section == "" || (section != "all" && !isSectionKey(s.DB, section)) {
		section = "all"
	}
	rule, err := s.buildRuleBrief(section, false, ids...)
	if err != nil {
		return nil, err
	}
	f, err := s.gatherFacts(rule, "article")
	if err != nil {
		return nil, err
	}
	sys, skill := s.skillSystemPrompt(platform)
	sys += articleOutputRules
	var ub strings.Builder
	ub.WriteString("## 选中的新闻\n\n")
	for i, it := range f.items {
		meta := it.Source
		if t := fmtBriefTime(it.Published); t != "" {
			meta += " · " + t
		}
		fmt.Fprintf(&ub, "%d. %s · %s · [[%d]](%s)\n", i+1, it.Title, meta, i+1, it.URL)
	}
	ub.WriteString("\n## 每条摘要（只用这些事实）\n\n")
	for i, it := range f.items {
		note := ""
		if it.Fetch.Status != "full" {
			note = " full_text=\"false\""
		}
		fmt.Fprintf(&ub, "<summary n=\"%d\"%s>\n%s\n</summary>\n\n", i+1, note, f.sums[i])
	}
	out := &articleOut{Platform: platform, Label: label, Skill: skill, Count: len(f.items), NoFullText: f.out.NoFullText, GeneratedAt: now()}
	var md string
	mm := s.mmConfig()
	if s.activeSkill(platform).Vision {
		var imgs []string
		for _, it := range f.items {
			if strings.HasPrefix(it.image, "https://") && len(imgs) < 6 {
				imgs = append(imgs, it.image)
			}
		}
		switch {
		case len(imgs) == 0:
		case mm.can("vision"):
			md, err = mm.mmVisionChat(sys, ub.String(), imgs, 0.4, 6000)
			if err != nil {
				out.VisionNote = "视觉模型调用失败，改用纯文字：" + err.Error()
				md = ""
			}
		default:
			out.VisionNote = "Skill 需要看图，未配置视觉模型，只用文字"
		}
	}
	if md == "" {
		if md, err = s.dsChat(sys, ub.String(), 0.4, 6000, false); err != nil {
			return nil, err
		}
	}
	md, covers := extractCovers(md)
	md, out.SelfCheck = s.selfCheck(md, platform)
	if os.Getenv("BRIEF_VERIFY") != "0" {
		var vb strings.Builder
		fmt.Fprintf(&vb, "<brief>\n%s\n</brief>\n\n", md)
		for i := range f.items {
			fmt.Fprintf(&vb, "<source n=\"%d\">\n摘要：%s\n原文：%s\n</source>\n\n", i+1, f.sums[i], capRunes(f.content[i], 1500))
		}
		if v, err := s.dsChat(articleVerify, vb.String(), 0, 6000, false); err == nil && len([]rune(v)) > len([]rune(md))/3 {
			md, out.Verified = v, true
			md, _ = extractCovers(md) // a marker the checker copied back
		}
	}
	out.Title = label
	if m := regexp.MustCompile(`(?m)^#\s+(.+)$`).FindStringSubmatch(md); m != nil {
		out.Title = strings.TrimSpace(m[1])
	}
	out.Covers = s.makeCovers(covers, platform, mm)
	out.Markdown = md
	return out, nil
}

func (s *Store) makeCovers(reqs []coverReq, platform string, mm MMConfig) []coverReq {
	for i := range reqs {
		c := &reqs[i]
		if !mm.can(c.Kind) {
			c.Status = "skipped"
			c.Note = map[string]string{"image": "未配置图片模型，已跳过封面", "video": "未配置视频模型，已跳过封面视频"}[c.Kind]
			continue
		}
		if !s.Cfg.Desktop {
			c.Status, c.Note = "skipped", "仅桌面版可用"
			continue
		}
		// generated into 缓存/ (cleaned after a day); 导出 copies it next to the article
		dst := s.cachePath(map[string]string{"image": "封面", "video": "视频"}[c.Kind], platformSkill[platform])
		var p string
		var err error
		if c.Kind == "image" {
			p, err = mm.mmImage(c.Prompt, c.Size, dst)
		} else {
			p, err = mm.mmVideo(c.Prompt, c.Size, dst)
		}
		if err != nil {
			c.Status, c.Note = "error", err.Error()
			continue
		}
		c.Status, c.Path = "ok", p
	}
	return reqs
}

// POST /api/compose {platform, ids, section}
func (s *Store) hCompose(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "生成文章仅桌面版可用") {
		return
	}
	if SettingsLocked() {
		detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1），无法调用 AI")
		return
	}
	b := readBody(r)
	var parts []string
	if xs, ok := b["ids"].([]any); ok {
		for _, x := range xs {
			parts = append(parts, fmt.Sprint(x))
		}
	}
	ids, err := parseIDsMax(strings.Join(parts, ","), envIntDef("COMPOSE_MAX_ITEMS", composeMax))
	if err != nil {
		detail(w, 400, err.Error())
		return
	}
	out, err := s.composeArticle(str(b["section"]), str(b["platform"]), ids)
	if err != nil {
		if he, ok := err.(*httpError); ok {
			detail(w, he.code, he.msg)
			return
		}
		detail(w, 500, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, out)
}
