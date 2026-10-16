package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Section brief (分区简报): always-available rule-based digest + optional DeepSeek "AI 润色".

const (
	briefPerSource = 4
	briefMaxSrc    = 12
	briefMaxItems  = 40
	briefCacheTTL  = 5 * time.Minute
	briefSumLine   = 100
)

type briefOut struct {
	Markdown      string       `json:"markdown"`
	Title         string       `json:"title"`
	Scope         string       `json:"scope"`
	ScopeLabel    string       `json:"scopeLabel"`
	Count         int          `json:"count"`
	Sources       int          `json:"sources"`
	GeneratedAt   float64      `json:"generatedAt"`
	AI            bool         `json:"ai"`
	Cached        bool         `json:"cached"`
	AIAvailable   bool         `json:"aiAvailable"`
	AIReason      any          `json:"aiReason"` // string or null
	Groups        []briefGroup `json:"groups"`
	Translate     bool         `json:"translate"`
	TranslateNote string       `json:"translateNote,omitempty"`
	Skill         string       `json:"skill,omitempty"`
}

type briefCacheEntry struct {
	at   time.Time
	rule *briefOut
}

type briefCache struct {
	mu sync.Mutex
	m  map[string]*briefCacheEntry
}

var gBriefs = &briefCache{m: map[string]*briefCacheEntry{}}

func envIntDef(k string, def int) int {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > briefSumLine {
		return string([]rune(s)[:briefSumLine])
	}
	return s
}

func (s *Store) scopeLabel(key string) string {
	if key == "all" {
		return "全部"
	}
	var name string
	if s.DB.QueryRow("SELECT name FROM sections WHERE key=?", key).Scan(&name) == nil && name != "" {
		return name
	}
	return key
}

type briefItem struct {
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	URL        string  `json:"url"`
	Source     string  `json:"source"`
	Published  float64 `json:"published"`
	Translated bool    `json:"translated"`
}

type briefGroup struct {
	Source   string      `json:"source"`
	Priority string      `json:"priority"`
	Items    []briefItem `json:"items"`
}

func (s *Store) briefRows(scope string) ([]*Row, error) {
	q := `SELECT i.id, i.title, i.summary, i.title_zh, i.summary_zh, i.published, i.url, i.is_x,
		s.name AS source, s.id AS source_id, s.priority
		FROM items i JOIN sources s ON s.id=i.source_id
		WHERE i.merged_into IS NULL AND i.hidden=0 AND s.enabled=1`
	args := []any{}
	if scope != "all" {
		q += " AND s.section=?"
		args = append(args, scope)
	}
	q += ` ORDER BY CASE s.priority WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 ELSE 2 END, i.published DESC LIMIT ?`
	args = append(args, max(briefMaxItems*3, 120))
	return all(s.DB, q, args...)
}

// pickBrief groups rows per source (top sources, a few items each).
func pickBrief(rs []*Row) (order []int64, groups map[int64][]*Row, meta map[int64][2]string) {
	perSrc := envIntDef("BRIEF_PER_SOURCE", briefPerSource)
	maxSrc := envIntDef("BRIEF_MAX_SOURCES", briefMaxSrc)
	groups, meta = map[int64][]*Row{}, map[int64][2]string{}
	for _, r := range rs {
		sid := int64(r.Num("source_id"))
		if _, ok := groups[sid]; !ok {
			if len(groups) >= maxSrc {
				continue
			}
			groups[sid] = nil
			meta[sid] = [2]string{r.Str("source"), r.Str("priority")}
			order = append(order, sid)
		}
		if len(groups[sid]) < perSrc {
			groups[sid] = append(groups[sid], r)
		}
	}
	return
}

// buildRuleBrief: the rule-based digest. tr=true prefers Chinese translations (and, when a DeepSeek key is set,
// first translates the picked items that have none yet); tr=false keeps the original text.
func (s *Store) buildRuleBrief(scope string, tr bool) (*briefOut, error) {
	if scope == "" {
		scope = "all"
	}
	if scope != "all" && !isSectionKey(s.DB, scope) {
		return nil, &httpError{404, "分区不存在"}
	}
	rs, err := s.briefRows(scope)
	if err != nil {
		return nil, err
	}
	order, groups, meta := pickBrief(rs)
	reason := s.unavailableReason()
	trNote := ""
	if tr {
		var need []int64
		for _, sid := range order {
			for _, it := range groups[sid] {
				if it.Get("title_zh") == nil {
					need = append(need, int64(it.Num("id")))
				}
			}
		}
		if len(need) > 0 && reason == "" {
			if err := s.tr.runIDs(need); err != nil {
				trNote = "部分条目翻译失败：" + err.Error()
			} else if rs, err = s.briefRows(scope); err == nil { // re-read with the fresh translations
				order, groups, meta = pickBrief(rs)
			}
		} else if len(need) > 0 {
			trNote = "未配置可用的 DeepSeek key，仅使用已有译文（" + reason + "）"
		}
	}

	bj := time.Now().In(time.FixedZone("CST", 8*3600))
	label := s.scopeLabel(scope)
	title := fmt.Sprintf("「%s」简报 · %s", label, bj.Format("1/2 15:04"))
	var out []briefGroup
	count := 0
	for _, sid := range order {
		g := briefGroup{Source: meta[sid][0], Priority: meta[sid][1]}
		for _, it := range groups[sid] {
			t, sum, zh := strings.TrimSpace(it.Str("title")), strings.TrimSpace(it.Str("summary")), false
			if tr {
				if z := strings.TrimSpace(it.Str("title_zh")); z != "" {
					t, zh = z, true
					if zs := strings.TrimSpace(it.Str("summary_zh")); zs != "" || sum == "" {
						sum = zs
					}
				}
			}
			if t == "" {
				t = "(无标题)"
			}
			t, sum = oneLine(t), oneLine(sum)
			if sum == t {
				sum = ""
			}
			g.Items = append(g.Items, briefItem{Title: t, Summary: sum, URL: it.Str("url"), Source: g.Source, Published: it.Num("published"), Translated: zh})
		}
		count += len(g.Items)
		out = append(out, g)
	}
	res := &briefOut{
		Title: title, Scope: scope, ScopeLabel: label, Groups: out, Translate: tr,
		Count: count, Sources: len(order), GeneratedAt: now(), AIAvailable: reason == "",
	}
	if trNote != "" {
		res.TranslateNote = trNote
	}
	if reason != "" {
		res.AIReason = reason
	}
	res.Markdown = briefMarkdown(res)
	return res, nil
}

func fmtBriefTime(ms float64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(int64(ms)).In(time.FixedZone("CST", 8*3600)).Format("1/2 15:04")
}

// briefMarkdown mirrors the dialog layout: group header → per item a title heading, a summary paragraph and a
// separate 「来源 · 时间 · 原文链接」 line.
func briefMarkdown(b *briefOut) string {
	var w strings.Builder
	fmt.Fprintf(&w, "# %s\n\n", b.Title)
	fmt.Fprintf(&w, "> 共 %d 条 · %d 个来源 · 近 48 小时\n\n", b.Count, b.Sources)
	if b.Count == 0 {
		w.WriteString("_近 48 小时暂无内容。_\n")
	}
	for _, g := range b.Groups {
		tag := ""
		if g.Priority == "P0" {
			tag = " · P0"
		}
		fmt.Fprintf(&w, "## %s%s\n\n", g.Source, tag)
		for _, it := range g.Items {
			fmt.Fprintf(&w, "### %s\n\n", strings.ReplaceAll(it.Title, "\n", " "))
			if it.Summary != "" {
				fmt.Fprintf(&w, "%s\n\n", it.Summary)
			}
			meta := "来源：" + it.Source
			if t := fmtBriefTime(it.Published); t != "" {
				meta += " · " + t
			}
			fmt.Fprintf(&w, "%s · [原文链接](%s)\n\n", meta, it.URL)
		}
	}
	return strings.TrimRight(w.String(), "\n") + "\n"
}

// briefSystemPrompt: the polishing instructions = the active 简报 Skill (desktop; built-in default otherwise).
func (s *Store) briefSystemPrompt() string {
	name, body := s.activeSkill()
	return "你是简报编辑，按下面的 Skill（" + name + "）把用户给的 Markdown 分区简报润色成中文简报（仍用 Markdown）。" +
		"不要编造未出现的事实；保留原文链接；只输出润色后的 Markdown。\n\n<skill>\n" + body + "\n</skill>"
}

func (s *Store) polishBrief(rule *briefOut) (*briefOut, error) {
	if reason := s.unavailableReason(); reason != "" {
		return nil, &httpError{503, reason}
	}
	body, _ := json.Marshal(map[string]any{
		"model": s.deepseekModel(), "temperature": 0.3, "max_tokens": 4000,
		"messages": []map[string]string{
			{"role": "system", "content": s.briefSystemPrompt()},
			{"role": "user", "content": rule.Markdown},
		},
	})
	r, err := netFetch(context.Background(), "POST", s.deepseekBase()+"/chat/completions",
		map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + s.deepseekKey()},
		strings.NewReader(string(body)), 90*time.Second)
	if err != nil {
		return nil, &httpError{502, err.Error()}
	}
	if r.Status == 401 || r.Status == 402 {
		return nil, &httpError{502, fmt.Sprintf("DeepSeek HTTP %d", r.Status)}
	}
	if !r.OK() {
		return nil, &httpError{502, fmt.Sprintf("DeepSeek HTTP %d: %s", r.Status, jsSlice(string(r.Body), 200))}
	}
	var j struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(r.Body, &j) != nil || len(j.Choices) == 0 || strings.TrimSpace(j.Choices[0].Message.Content) == "" {
		return nil, &httpError{502, "DeepSeek 返回为空"}
	}
	md := strings.TrimSpace(j.Choices[0].Message.Content)
	if !strings.HasSuffix(md, "\n") {
		md += "\n"
	}
	out := *rule
	out.Markdown = md
	out.AI = true
	out.Cached = false
	out.GeneratedAt = now()
	out.Skill, _ = s.activeSkill()
	return &out, nil
}

func (s *Store) getBrief(scope string, ai, tr bool) (*briefOut, error) {
	if scope == "" {
		scope = "all"
	}
	ttl := briefCacheTTL
	if v := os.Getenv("BRIEF_CACHE_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttl = time.Duration(n) * time.Millisecond
		}
	}
	key := scope
	if tr {
		key += "|tr"
	}
	if ai {
		sk, _ := s.activeSkill()
		key += "|ai|" + sk
	}
	c := gBriefs
	c.mu.Lock()
	if hit := c.m[key]; hit != nil && time.Since(hit.at) < ttl && hit.rule != nil {
		out := *hit.rule
		out.Cached = true
		c.mu.Unlock()
		if !ai {
			reason := s.unavailableReason()
			out.AIAvailable = reason == ""
			out.AIReason = nil
			if reason != "" {
				out.AIReason = reason
			}
		}
		return &out, nil
	}
	c.mu.Unlock()
	res, err := s.buildRuleBrief(scope, tr)
	if err != nil {
		return nil, err
	}
	if ai {
		if res, err = s.polishBrief(res); err != nil {
			return nil, err
		}
	}
	c.mu.Lock()
	c.m[key] = &briefCacheEntry{at: time.Now(), rule: res}
	c.mu.Unlock()
	return res, nil
}

type httpError struct {
	code int
	msg  string
}

func (e *httpError) Error() string { return e.msg }

func (s *Store) hBrief(w http.ResponseWriter, r *http.Request) {
	sec := r.URL.Query().Get("section")
	if sec == "" {
		sec = "all"
	}
	ai := r.URL.Query().Get("ai") == "1" || r.URL.Query().Get("ai") == "true"
	tr := r.URL.Query().Get("tr") == "1" || r.URL.Query().Get("tr") == "true"
	if ai && SettingsLocked() {
		detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1），无法调用 AI 润色")
		return
	}
	out, err := s.getBrief(sec, ai, tr)
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
