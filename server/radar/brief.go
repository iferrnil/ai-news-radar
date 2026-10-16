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
	Markdown    string  `json:"markdown"`
	Title       string  `json:"title"`
	Scope       string  `json:"scope"`
	ScopeLabel  string  `json:"scopeLabel"`
	Count       int     `json:"count"`
	Sources     int     `json:"sources"`
	GeneratedAt float64 `json:"generatedAt"`
	AI          bool    `json:"ai"`
	Cached      bool    `json:"cached"`
	AIAvailable bool    `json:"aiAvailable"`
	AIReason    any     `json:"aiReason"` // string or null
}

type briefCacheEntry struct {
	at   time.Time
	rule *briefOut
	ai   *briefOut
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

func (s *Store) buildRuleBrief(scope string) (*briefOut, error) {
	if scope == "" {
		scope = "all"
	}
	if scope != "all" && !isSectionKey(s.DB, scope) {
		return nil, &httpError{404, "分区不存在"}
	}
	perSrc := envIntDef("BRIEF_PER_SOURCE", briefPerSource)
	maxSrc := envIntDef("BRIEF_MAX_SOURCES", briefMaxSrc)
	limit := max(briefMaxItems*3, 120)

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
	args = append(args, limit)
	rs, err := all(s.DB, q, args...)
	if err != nil {
		return nil, err
	}

	type grp struct {
		name, pri string
		items     []*Row
	}
	order := []int64{}
	groups := map[int64]*grp{}
	for _, r := range rs {
		sid := int64(r.Num("source_id"))
		g, ok := groups[sid]
		if !ok {
			if len(groups) >= maxSrc {
				continue
			}
			g = &grp{name: r.Str("source"), pri: r.Str("priority")}
			groups[sid] = g
			order = append(order, sid)
		}
		if len(g.items) < perSrc {
			g.items = append(g.items, r)
		}
	}

	bj := time.Now().In(time.FixedZone("CST", 8*3600)).Format("1/2 15:04")
	label := s.scopeLabel(scope)
	title := fmt.Sprintf("「%s」简报 · %s", label, bj)
	var b strings.Builder
	count := 0
	fmt.Fprintf(&b, "# %s\n\n", title)
	for _, sid := range order {
		count += len(groups[sid].items)
	}
	fmt.Fprintf(&b, "共 %d 条 · %d 个来源 · 近 48 小时\n\n", count, len(order))
	if count == 0 {
		b.WriteString("_近 48 小时暂无内容。_\n")
	}
	for _, sid := range order {
		g := groups[sid]
		tag := ""
		if g.pri == "P0" {
			tag = " · P0"
		}
		fmt.Fprintf(&b, "## %s%s\n\n", g.name, tag)
		for _, it := range g.items {
			t := strings.TrimSpace(it.Str("title_zh"))
			if t == "" {
				t = strings.TrimSpace(it.Str("title"))
			}
			if t == "" {
				t = "(无标题)"
			}
			t = oneLine(t)
			sum := strings.TrimSpace(it.Str("summary_zh"))
			if sum == "" {
				sum = strings.TrimSpace(it.Str("summary"))
			}
			sum = oneLine(sum)
			fmt.Fprintf(&b, "- **[%s](%s)**", t, it.Str("url"))
			if sum != "" && sum != t {
				fmt.Fprintf(&b, " — %s", sum)
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	reason := s.unavailableReason()
	out := &briefOut{
		Markdown: strings.TrimRight(b.String(), "\n") + "\n",
		Title:    title, Scope: scope, ScopeLabel: label,
		Count: count, Sources: len(order), GeneratedAt: now(),
		AI: false, Cached: false, AIAvailable: reason == "",
	}
	if reason != "" {
		out.AIReason = reason
	} else {
		out.AIReason = nil
	}
	return out, nil
}

func (s *Store) polishBrief(rule *briefOut) (*briefOut, error) {
	if reason := s.unavailableReason(); reason != "" {
		return nil, &httpError{503, reason}
	}
	sys := `你是简报编辑。把用户给的 Markdown 分区简报润色成一篇简短、可读的中文摘要（仍用 Markdown）。
要求：保留所有来源分组与条目链接；每条保留标题与一句要点；可合并同类表述、去掉重复；开头用 2–4 句总览；不要编造未出现的事实；不要解释你的做法；只输出润色后的 Markdown。`
	body, _ := json.Marshal(map[string]any{
		"model": s.deepseekModel(), "temperature": 0.3, "max_tokens": 4000,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": rule.Markdown},
		},
	})
	r, err := netFetch(context.Background(), "POST", s.deepseekBase()+"/chat/completions",
		map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + s.deepseekKey()},
		strings.NewReader(string(body)), 60*time.Second)
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
	return &out, nil
}

func (s *Store) getBrief(scope string, ai bool) (*briefOut, error) {
	if scope == "" {
		scope = "all"
	}
	ttl := briefCacheTTL
	if v := os.Getenv("BRIEF_CACHE_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttl = time.Duration(n) * time.Millisecond
		}
	}
	c := gBriefs
	c.mu.Lock()
	hit := c.m[scope]
	fresh := hit != nil && time.Since(hit.at) < ttl
	if !ai && fresh && hit.rule != nil {
		out := *hit.rule
		out.Cached = true
		reason := s.unavailableReason()
		out.AIAvailable = reason == ""
		if reason != "" {
			out.AIReason = reason
		} else {
			out.AIReason = nil
		}
		c.mu.Unlock()
		return &out, nil
	}
	if ai && fresh && hit.ai != nil {
		out := *hit.ai
		out.Cached = true
		c.mu.Unlock()
		return &out, nil
	}
	c.mu.Unlock()

	rule, err := s.buildRuleBrief(scope)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	ent := c.m[scope]
	if ent == nil || !fresh {
		ent = &briefCacheEntry{at: time.Now()}
		c.m[scope] = ent
	}
	ent.rule = rule
	ent.at = time.Now()
	if !ai {
		c.mu.Unlock()
		return rule, nil
	}
	c.mu.Unlock()

	polished, err := s.polishBrief(rule)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.m[scope] == nil {
		c.m[scope] = &briefCacheEntry{at: time.Now()}
	}
	c.m[scope].ai = polished
	c.mu.Unlock()
	return polished, nil
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
	if ai && SettingsLocked() {
		detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1），无法调用 AI 润色")
		return
	}
	out, err := s.getBrief(sec, ai)
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
