package radar

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
)

// Handler returns the HTTP API (every former Next.js route handler under /api).
func (s *Store) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", s.hHealth)
	m.HandleFunc("GET /api/items", s.hItems)
	m.HandleFunc("GET /api/read", s.hReadGet)
	m.HandleFunc("POST /api/read", s.hReadPost)
	m.HandleFunc("GET /api/sections", s.hSectionsGet)
	m.HandleFunc("POST /api/sections", s.hSectionsPost)
	m.HandleFunc("PATCH /api/sections/{key}", s.hSectionPatch)
	m.HandleFunc("DELETE /api/sections/{key}", s.hSectionDelete)
	m.HandleFunc("GET /api/settings", s.hSettingsGet)
	m.HandleFunc("PATCH /api/settings", s.hSettingsPatch)
	m.HandleFunc("POST /api/settings/test", s.hSettingsTest)
	m.HandleFunc("POST /api/settings/notify-test", s.hNotifyTest)
	m.HandleFunc("GET /api/sources", s.hSourcesGet)
	m.HandleFunc("POST /api/sources", s.hSourcesPost)
	m.HandleFunc("PATCH /api/sources/{id}", s.hSourcePatch)
	m.HandleFunc("DELETE /api/sources/{id}", s.hSourceDelete)
	m.HandleFunc("POST /api/sources/{id}/fetch", s.hSourceFetch)
	m.HandleFunc("POST /api/sources/bulk", s.hBulk)
	m.HandleFunc("GET /api/sources/deleted", s.hDeletedGet)
	m.HandleFunc("POST /api/sources/deleted", s.hDeletedPost)
	m.HandleFunc("POST /api/sources/import", s.hImport)
	m.HandleFunc("POST /api/sources/rule-check", s.hRuleCheck)
	m.HandleFunc("POST /api/sources/test", s.hSourceTest)
	m.HandleFunc("GET /api/brief", s.hBrief)
	m.HandleFunc("GET /api/translate/progress", s.hTranslateProgress)
	m.HandleFunc("GET /api/skills", s.hSkillsList)
	m.HandleFunc("GET /api/skills/{name}", s.hSkillGet)
	m.HandleFunc("POST /api/skills/install", s.hSkillInstall)
	m.HandleFunc("POST /api/skills/upload", s.hSkillUpload)
	m.HandleFunc("PATCH /api/skills/active", s.hSkillActive)
	m.HandleFunc("DELETE /api/skills/{name}", s.hSkillDelete)
	m.HandleFunc("GET /api/desktop/notify", s.hDesktopNotifyStatus)
	m.HandleFunc("POST /api/desktop/notify-open-settings", s.hDesktopNotifyOpen)
	m.HandleFunc("POST /api/desktop/open-data", s.hDesktopOpenData)
	m.HandleFunc("GET /api/stats", s.hStats)
	m.HandleFunc("GET /api/stream", s.hStream)
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		if s.notFound != nil {
			s.notFound.ServeHTTP(w, r) // like Next.js: the app's 404 page
			return
		}
		writeJSON(w, 404, obj("detail", "not found"))
	})
	return m
}

// ---- helpers

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := marshalNoEscape(v)
	if err != nil {
		status, b = 500, []byte(`{"detail":"internal error"}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(b)
}

func detail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, obj("detail", msg))
}

// readBody parses a JSON object body; anything else becomes {} (req.json().catch(() => ({}))).
func readBody(r *http.Request) map[string]any {
	b, _ := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	var m map[string]any
	if json.Unmarshal(b, &m) != nil || m == nil {
		return map[string]any{}
	}
	return m
}

var brRe = regexp.MustCompile(`\bbr\b`)
var gzRe = regexp.MustCompile(`\bgzip\b`)

// jsonResponse: content ETag (304 on If-None-Match) + brotli q5 / gzip for bodies > 1 KB (http.ts).
func jsonResponse(w http.ResponseWriter, r *http.Request, body []byte) {
	h := sha1.Sum(body)
	etag := `W/"` + base64.RawURLEncoding.EncodeToString(h[:])[:16] + `"`
	hd := w.Header()
	hd.Set("Content-Type", "application/json; charset=utf-8")
	hd.Set("Cache-Control", "private, no-cache")
	hd.Set("Vary", "Accept-Encoding")
	hd.Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(304)
		return
	}
	ae := r.Header.Get("Accept-Encoding")
	if len(body) > 1024 && brRe.MatchString(ae) {
		var b bytes.Buffer
		bw := brotli.NewWriterOptions(&b, brotli.WriterOptions{Quality: 5})
		bw.Write(body)
		bw.Close()
		hd.Set("Content-Encoding", "br")
		w.Write(b.Bytes())
		return
	}
	if len(body) > 1024 && gzRe.MatchString(ae) {
		var b bytes.Buffer
		gw, _ := gzip.NewWriterLevel(&b, 6)
		gw.Write(body)
		gw.Close()
		hd.Set("Content-Encoding", "gzip")
		w.Write(b.Bytes())
		return
	}
	w.Write(body)
}

// jsNumber is JavaScript Number(v) for query/path strings (NaN -> ok=false).
func jsNumber(v string) (float64, bool) {
	t := strings.TrimSpace(v)
	if t == "" {
		return 0, true
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || math.IsNaN(f) {
		return 0, false
	}
	return f, true
}

func numOf(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		return jsNumber(x)
	case bool:
		return float64(b2i(x)), true
	case nil:
		return 0, true
	}
	return 0, false
}

func idsFrom(v any, cap int, positiveInt bool) []int64 {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []int64
	for _, x := range arr {
		f, ok := numOf(x)
		if !ok || math.IsInf(f, 0) {
			continue
		}
		if positiveInt && (f != math.Trunc(f) || f <= 0) {
			continue
		}
		out = append(out, int64(f))
		if cap > 0 && len(out) >= cap {
			break
		}
	}
	return out
}

func (s *Store) readSync() bool { return s.Cfg.Desktop || os.Getenv("READ_SYNC") == "1" }

func (s *Store) sourceByID(id any) *Row {
	r, _ := one(s.DB, "SELECT * FROM sources WHERE id=?", id)
	return r
}

func pathID(r *http.Request) (float64, bool) {
	return jsNumber(r.PathValue("id"))
}

// ---- routes

func (s *Store) hHealth(w http.ResponseWriter, r *http.Request) {
	if _, err := s.DB.Exec("SELECT 1"); err != nil {
		writeJSON(w, 503, obj("ok", false, "error", err.Error()))
		return
	}
	writeJSON(w, 200, obj("ok", true))
}

func (s *Store) hItems(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query()
	var wh []string
	var a []any
	if v := p.Get("category"); v != "" {
		wh, a = append(wh, "s.category=?"), append(a, v)
	}
	if v := p.Get("region"); v != "" {
		wh, a = append(wh, "s.region=?"), append(a, v)
	}
	switch p.Get("stype") {
	case "x":
		wh = append(wh, "i.is_x=1")
	case "web":
		wh = append(wh, "i.is_x=0")
	}
	if v := p.Get("priority"); v != "" {
		wh, a = append(wh, "s.priority=?"), append(a, v)
	}
	if v := p.Get("source"); v != "" {
		n, _ := jsNumber(v)
		wh, a = append(wh, "s.id=?"), append(a, n)
	}
	if q := p.Get("q"); q != "" {
		l := "%" + q + "%"
		wh, a = append(wh, "(i.title LIKE ? OR i.summary LIKE ? OR s.name LIKE ?)"), append(a, l, l, l)
	}
	if v := p.Get("before"); v != "" {
		n, _ := jsNumber(v)
		wh, a = append(wh, "i.published < ?"), append(a, n)
	}
	id := p.Get("id")
	if id != "" {
		n, _ := jsNumber(id)
		wh, a = append(wh, "i.id=?"), append(a, n)
	}
	limit := 50
	if v, has := p["limit"]; has {
		if n, ok := jsNumber(v[0]); ok {
			limit = int(n)
		}
	}
	where := ""
	for _, x := range wh {
		where += " AND " + x
	}
	items := s.QueryItems(where, a, limit, id != "", p.Get("zh") == "1")
	b, _ := Rows{List: items, DropNulls: true}.MarshalJSON()
	jsonResponse(w, r, b)
}

func (s *Store) hReadGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.readSync() {
		writeJSON(w, 200, obj("sync", false, "desktop", s.Cfg.Desktop))
		return
	}
	rs, _ := all(s.DB, "SELECT item_id FROM read_items ORDER BY ts DESC LIMIT 5000")
	ids := make([]any, len(rs))
	for i, x := range rs {
		ids[i] = x.Get("item_id")
	}
	out := &Row{}
	out.Set("sync", true)
	out.Set("desktop", s.Cfg.Desktop)
	out.Set("ids", ids)
	writeJSON(w, 200, out)
}

func (s *Store) hReadPost(w http.ResponseWriter, r *http.Request) {
	if !s.readSync() {
		detail(w, 404, "read sync is off")
		return
	}
	b := readBody(r)
	t := now()
	tx, err := s.DB.Begin()
	if err == nil {
		for _, id := range idsFrom(b["add"], 5000, true) {
			tx.Exec("INSERT INTO read_items(item_id, ts) SELECT ?, ? WHERE EXISTS (SELECT 1 FROM items WHERE id=?) ON CONFLICT(item_id) DO NOTHING", id, t, id)
		}
		for _, id := range idsFrom(b["remove"], 5000, true) {
			tx.Exec("DELETE FROM read_items WHERE item_id=?", id)
		}
		tx.Commit()
	}
	writeJSON(w, 200, obj("ok", true))
}

func (s *Store) hSectionsGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, Rows{List: listSections(s.DB)})
}

func (s *Store) hSectionsPost(w http.ResponseWriter, r *http.Request) {
	key, e := createSection(s.DB, readBody(r))
	if e != "" {
		detail(w, 400, e)
		return
	}
	s.Bus.Publish(Msg{Event: "sources"})
	writeJSON(w, 200, obj("key", key))
}

func (s *Store) hSectionPatch(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	b := readBody(r)
	if !isSectionKey(s.DB, key) {
		detail(w, 404, "分区不存在")
		return
	}
	if v, has := b["name"]; has {
		name := CleanSectionName(jsString(v))
		if name == "" {
			detail(w, 400, "分区名称不能为空")
			return
		}
		if name == "全部" {
			detail(w, 400, "「全部」是保留名称")
			return
		}
		var one int
		if s.DB.QueryRow("SELECT 1 FROM sections WHERE name=? AND key<>?", name, key).Scan(&one) == nil {
			detail(w, 409, "已有同名分区")
			return
		}
		s.DB.Exec("UPDATE sections SET name=? WHERE key=?", name, key)
	}
	if v, has := b["icon"]; has {
		s.DB.Exec("UPDATE sections SET icon=? WHERE key=?", cleanIcon(jsString(v)), key)
	}
	if v, ok := b["inAll"].(bool); ok {
		s.DB.Exec("UPDATE sections SET in_all=? WHERE key=?", b2i(v), key)
	}
	if mv, ok := b["move"].(float64); ok && (mv == -1 || mv == 1) {
		list := listSections(s.DB)
		i := -1
		for n, x := range list {
			if x.Str("key") == key {
				i = n
			}
		}
		j := i + int(mv)
		if i >= 0 && j >= 0 && j < len(list) {
			list[i], list[j] = list[j], list[i]
			tx, err := s.DB.Begin()
			if err == nil {
				for n, x := range list {
					tx.Exec("UPDATE sections SET pos=? WHERE key=?", n+1, x.Get("key"))
				}
				tx.Commit()
			}
		}
	}
	s.Bus.Publish(Msg{Event: "sources"})
	writeJSON(w, 200, obj("ok", true))
}

func (s *Store) hSectionDelete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !isSectionKey(s.DB, key) {
		detail(w, 404, "分区不存在")
		return
	}
	var n int
	s.DB.QueryRow("SELECT COUNT(*) FROM sources WHERE section=?", key).Scan(&n)
	to := r.URL.Query().Get("moveTo")
	if n > 0 && (to == "" || to == key || !isSectionKey(s.DB, to)) {
		out := &Row{}
		out.Set("detail", fmt.Sprintf("该分区还有 %d 个来源，请选择要移入的分区", n))
		out.Set("count", n)
		writeJSON(w, 409, out)
		return
	}
	tx, err := s.DB.Begin()
	if err == nil {
		if n > 0 {
			tx.Exec("UPDATE sources SET section=?, overrides = CASE WHEN instr(overrides,'section') THEN overrides ELSE trim(overrides || ',section', ',') END WHERE section=?", to, key)
		}
		tx.Exec("DELETE FROM sections WHERE key=?", key)
		tx.Commit()
	}
	s.Bus.Publish(Msg{Event: "sources"})
	out := &Row{}
	out.Set("ok", true)
	out.Set("moved", n)
	writeJSON(w, 200, out)
}

func (s *Store) settingsOut(w http.ResponseWriter) {
	out := &Row{}
	out.Set("translate", s.translateStatus())
	out.Set("deepseek", s.deepseekPublic())
	out.Set("notify", s.NotifyPrefs())
	out.Set("desktop", s.Cfg.Desktop)
	if s.Cfg.Desktop {
		out.Set("dataDir", filepath.Dir(s.Cfg.DBPath))
	}
	if s.Cfg.Version != "" {
		out.Set("version", s.Cfg.Version)
	} else {
		out.Set("version", nil)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, out)
}

func (s *Store) hSettingsGet(w http.ResponseWriter, r *http.Request) { s.settingsOut(w) }

func (s *Store) hSettingsPatch(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	if d, ok := b["deepseek"].(map[string]any); ok {
		if SettingsLocked() {
			detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1），请通过环境变量配置")
			return
		}
		var p dsUpdate
		if k, has := d["key"]; has && k != "" {
			c, ok := cleanKey(k)
			if !ok {
				detail(w, 400, "API key 格式不正确")
				return
			}
			p.Key = c
		}
		if d["clearKey"] == true {
			p.ClearKey = true
		}
		if v, has := d["baseUrl"]; has {
			u, ok := cleanBase(v)
			if !ok {
				detail(w, 400, "Base URL 需为 https:// 地址（本机可用 http://localhost）")
				return
			}
			p.BaseURL = &u
		}
		if v, has := d["model"]; has {
			m, ok := cleanModel(v)
			if !ok {
				detail(w, 400, "模型名格式不正确")
				return
			}
			p.Model = &m
		}
		s.updateDeepseek(p)
		s.tr.resetAuth()
	}
	if n, ok := b["notify"].(map[string]any); ok {
		if SettingsLocked() {
			detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1）")
			return
		}
		var en *bool
		if v, ok := n["enabled"].(bool); ok {
			en = &v
		}
		secs, _ := n["sections"].([]any)
		var sys *bool
		if v, ok := n["system"].(bool); ok {
			sys = &v
		}
		scope, _ := n["scope"].(string)
		s.SetNotifyPrefs(en, secs, sys, scope)
	}
	if v, has := b["translate"]; has {
		// boolean = 「全部」; otherwise { scope: all|section|source, key, on }
		var t map[string]any
		if on, ok := v.(bool); ok {
			t = map[string]any{"scope": "all", "on": on}
		} else {
			t, _ = v.(map[string]any)
		}
		scope, _ := t["scope"].(string)
		on, okOn := t["on"].(bool)
		if t == nil || (scope != "all" && scope != "section" && scope != "source") || !okOn {
			detail(w, 400, "translate 需为 { scope: all|section|source, key, on }")
			return
		}
		// the per-view switches are the user's own control (no SETTINGS_LOCKED check); without a key they stay
		// stored but nothing is translated (the popover explains why)
		if !s.setTrScope(scope, t["key"], on) {
			if scope == "section" {
				detail(w, 404, "分区不存在")
			} else {
				detail(w, 404, "来源不存在")
			}
			return
		}
	}
	if _, ok := b["deepseek"].(map[string]any); ok {
		s.tr.backfill(true) // a new key may make queued scopes runnable
	}
	s.settingsOut(w)
}

func (s *Store) hSettingsTest(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	str := func(k string) string {
		if !truthy(b[k]) {
			return ""
		}
		return strings.TrimSpace(jsString(b[k]))
	}
	typed := str("key") != "" || str("baseUrl") != ""
	if typed && SettingsLocked() {
		writeJSON(w, 403, obj("ok", false, "error", "设置已锁定"))
		return
	}
	key := ""
	if truthy(b["key"]) {
		k, ok := cleanKey(b["key"])
		if !ok {
			writeJSON(w, 200, obj("ok", false, "error", "API key 格式不正确"))
			return
		}
		key = k
	}
	base := ""
	if truthy(b["baseUrl"]) {
		u, ok := cleanBase(b["baseUrl"])
		if !ok || u == "" {
			writeJSON(w, 200, obj("ok", false, "error", "Base URL 不合法"))
			return
		}
		base = u
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, s.testDeepseek(key, base))
}

func (s *Store) hNotifyTest(w http.ResponseWriter, r *http.Request) {
	// desktop: show a native banner directly (independent of the notify switches) and report the permission
	if DesktopHooks.NotifyTest != nil {
		st, err := DesktopHooks.NotifyTest()
		if err != nil {
			writeJSON(w, 200, obj("ok", false, "state", st, "error", err.Error()))
			return
		}
		writeJSON(w, 200, obj("ok", true, "state", st, "count", 1))
		return
	}
	if SettingsLocked() {
		detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1）")
		return
	}
	row, _ := one(s.DB, `SELECT i.id, COALESCE(NULLIF(i.title_zh,''), i.title) title, s.name source, s.section, i.url, i.published FROM items i JOIN sources s ON s.id=i.source_id
		WHERE s.priority='P0' AND i.hidden=0 AND i.merged_into IS NULL AND s.enabled=1 ORDER BY i.published DESC LIMIT 1`)
	it := NotifyItem{ID: 0, Title: "这是一条测试通知", Source: "AI Radar", Section: nil, URL: "", Published: now()}
	if row != nil {
		it = NotifyItem{ID: row.Get("id"), Title: row.Str("title"), Source: row.Str("source"), Section: row.Get("section"), URL: row.Str("url"), Published: row.Num("published")}
	}
	b := s.NotifyTest([]NotifyItem{it})
	out := &Row{}
	out.Set("ok", true)
	out.Set("count", b.Count)
	writeJSON(w, 200, out)
}

func (s *Store) hSourcesGet(w http.ResponseWriter, r *http.Request) {
	rows, _ := all(s.DB, `SELECT s.*, (SELECT COUNT(*) FROM items i WHERE i.source_id=s.id AND i.hidden=0) item_count, (SELECT COUNT(*) FROM items i WHERE i.source_id=s.id AND i.hidden=1) hidden_count,
    (SELECT MAX(published) FROM items i WHERE i.source_id=s.id) latest, `+HealthCols+` FROM sources s LEFT JOIN source_health h ON h.source_id=s.id
    ORDER BY s.enabled DESC, s.supported DESC, COALESCE(s.priority,'P9'), s.category, s.name`)
	b, _ := Rows{List: rows, DropNulls: true}.MarshalJSON()
	jsonResponse(w, r, b)
}

// overlap: share of sample titles already stored (same content under another feed URL).
func (s *Store) overlap(sample []FetchedItem) float64 {
	n, hit := 0, 0
	for _, it := range sample {
		if it.Title == "" {
			continue
		}
		n++
		var one int
		if s.DB.QueryRow("SELECT 1 FROM items WHERE title=? LIMIT 1", it.Title).Scan(&one) == nil {
			hit++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(hit) / float64(n)
}

func detectErr(err error) string {
	return strings.TrimPrefix(errMsg(err), "Error: ")
}

func (s *Store) hSourcesPost(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	d, err := Detect(r.Context(), jsString(b["input"]))
	if err != nil {
		detail(w, 400, detectErr(err))
		return
	}
	name := d.Name
	if truthy(b["name"]) {
		name = jsString(b["name"])
	}
	name = strings.TrimSpace(name)
	var one int
	if s.DB.QueryRow("SELECT 1 FROM sources WHERE url=? OR name=?", d.URL, name).Scan(&one) == nil {
		detail(w, 409, "已存在相同 URL 或名称的来源")
		return
	}
	if ov := s.overlap(d.Sample); ov >= 0.5 && !truthy(b["force"]) {
		detail(w, 409, fmt.Sprintf("疑似重复：示例条目中 %d%% 的标题已由现有来源收录。确需添加请勾选「强制添加」", int(math.Round(ov*100))))
		return
	}
	priority := "P1"
	if p, _ := b["priority"].(string); p == "P0" || p == "P1" || p == "P2" {
		priority = p
	}
	ruleRaw, _ := b["rule"].(map[string]any)
	if ruleRaw == nil {
		ruleRaw = map[string]any{}
	}
	titles := make([]string, len(d.Sample))
	for i, x := range d.Sample {
		titles[i] = x.Title
	}
	rule, e := ValidateRule(ruleRaw, titles)
	if e != "" {
		detail(w, 400, e)
		return
	}
	fbKey := "media"
	if strings.HasPrefix(d.Type, "x") {
		fbKey = "x"
	}
	var fallback string
	if isSectionKey(s.DB, fbKey) {
		fallback = fbKey
	} else if l := listSections(s.DB); len(l) > 0 {
		fallback = l[0].Str("key")
	}
	sec := fallback
	if isSectionKey(s.DB, b["section"]) {
		sec = b["section"].(string)
	}
	ns, hasNew := b["newSection"].(map[string]any)
	if !hasNew && truthy(b["newSection"]) {
		ns, hasNew = map[string]any{}, true
	}
	if sec == "" && !hasNew {
		detail(w, 400, "请先选择或新建一个分区")
		return
	}
	if hasNew {
		k, e := createSection(s.DB, ns)
		if e != "" {
			detail(w, 400, e)
			return
		}
		sec = k
	}
	region, category := MetaFor(sec)
	var uname any
	if d.Username != nil {
		uname = *d.Username
	}
	res, err := s.DB.Exec(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,priority,poll_interval,origin,created,section,f_include,f_exclude,f_scope,f_cs)
    VALUES(?,?,?,?,?,?,?,1,1,?,?,'user',?,?,?,?,?,?)`, name, region, category, d.Type, d.URL, d.Homepage, uname,
		priority, IntervalFor(d.Type, priority), now(), sec, rule.Include, rule.Exclude, rule.Scope, b2i(rule.CS))
	if err != nil {
		detail(w, 409, "已存在相同 URL 或名称的来源")
		return
	}
	id, _ := res.LastInsertId()
	src := s.sourceByID(id)
	out := &Row{}
	out.Set("source", src)
	out.Set("fetch", s.PollOne(src))
	writeJSON(w, 200, out)
}

var nameStrip = regexp.MustCompile(`[\x00-\x1f\x7f<>]`)

func (s *Store) hSourcePatch(w http.ResponseWriter, r *http.Request) {
	idf, ok := pathID(r)
	b := readBody(r)
	if !ok || s.sourceByID(idf) == nil {
		detail(w, 404, "not found")
		return
	}
	id := idf
	name := ""
	if v, has := b["name"]; has {
		name = strings.TrimSpace(wsRe.ReplaceAllString(nameStrip.ReplaceAllString(jsString(v), ""), " "))
		if name == "" {
			detail(w, 400, "名称不能为空")
			return
		}
		if len([]rune(name)) > 80 {
			detail(w, 400, "名称最多 80 个字符")
			return
		}
		var one int
		if s.DB.QueryRow("SELECT 1 FROM sources WHERE name=? COLLATE NOCASE AND id<>?", name, id).Scan(&one) == nil {
			detail(w, 409, "已有同名来源")
			return
		}
	}
	var rule *Rule
	if rr, ok := b["rule"].(map[string]any); ok && truthy(b["rule"]) {
		v, e := ValidateRule(rr, nil)
		if e != "" {
			detail(w, 400, e)
			return
		}
		rule = &v
	}
	sec := ""
	if isSectionKey(s.DB, b["section"]) {
		sec = b["section"].(string)
	}
	if truthy(b["newSection"]) {
		ns, _ := b["newSection"].(map[string]any)
		if ns == nil {
			ns = map[string]any{}
		}
		k, e := createSection(s.DB, ns)
		if e != "" {
			detail(w, 400, e)
			return
		}
		sec = k
	}
	tx, err := s.DB.Begin()
	if err != nil {
		detail(w, 500, err.Error())
		return
	}
	mark := func(f string) {
		tx.Exec("UPDATE sources SET overrides = CASE WHEN instr(overrides, ?) THEN overrides ELSE trim(overrides || ',' || ?, ',') END WHERE id=?", f, f, id)
	}
	if v, ok := b["enabled"].(bool); ok {
		tx.Exec("UPDATE sources SET enabled=? WHERE id=?", b2i(v), id)
		mark("enabled")
	}
	if v, has := b["poll_interval"]; has && v != nil {
		n, _ := numOf(v)
		tx.Exec("UPDATE sources SET poll_interval=? WHERE id=?", math.Max(30, n), id)
		mark("poll_interval")
	}
	if p, _ := b["priority"].(string); p == "P0" || p == "P1" || p == "P2" {
		tx.Exec("UPDATE sources SET priority=? WHERE id=?", p, id)
		mark("priority")
	}
	if name != "" {
		tx.Exec("UPDATE sources SET seed_name = CASE WHEN origin='seed' THEN COALESCE(seed_name, name) END, name=? WHERE id=?", name, id)
		mark("name")
	}
	if sec != "" {
		tx.Exec("UPDATE sources SET section=? WHERE id=?", sec, id)
		mark("section")
	}
	if rule != nil {
		tx.Exec("UPDATE sources SET f_include=?, f_exclude=?, f_scope=?, f_cs=? WHERE id=?", rule.Include, rule.Exclude, rule.Scope, b2i(rule.CS), id)
	}
	tx.Commit()
	out := &Row{}
	out.Set("ok", true)
	if rule != nil {
		out.Set("hidden", s.ReapplyRule(id))
	}
	s.Bus.Publish(Msg{Event: "sources", ID: id})
	writeJSON(w, 200, out)
}

func (s *Store) hSourceDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		detail(w, 404, "not found")
		return
	}
	d, n := s.DeleteSources([]int64{int64(id)})
	if d == 0 {
		detail(w, 404, "not found")
		return
	}
	out := &Row{}
	out.Set("deleted", d)
	out.Set("items", n)
	writeJSON(w, 200, out)
}

func (s *Store) hSourceFetch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	var src *Row
	if ok {
		src = s.sourceByID(id)
	}
	if src == nil {
		detail(w, 404, "not found")
		return
	}
	if !truthy(src.Get("supported")) {
		detail(w, 400, "该来源类型暂不支持抓取")
		return
	}
	writeJSON(w, 200, s.PollOne(src))
}

func (s *Store) hBulk(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	if _, ok := b["ids"].([]any); b["action"] != "delete" || !ok {
		detail(w, 400, "bad request")
		return
	}
	d, n := s.DeleteSources(idsFrom(b["ids"], 0, false))
	out := &Row{}
	out.Set("deleted", d)
	out.Set("items", n)
	writeJSON(w, 200, out)
}

func (s *Store) hDeletedGet(w http.ResponseWriter, r *http.Request) {
	rs, _ := all(s.DB, "SELECT name, ts FROM deleted_seeds ORDER BY ts DESC")
	writeJSON(w, 200, Rows{List: rs})
}

func (s *Store) hDeletedPost(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	var n int64
	if b["names"] == "all" {
		res, _ := s.DB.Exec("DELETE FROM deleted_seeds")
		if res != nil {
			n, _ = res.RowsAffected()
		}
	} else if arr, ok := b["names"].([]any); ok {
		tx, err := s.DB.Begin()
		if err == nil {
			for _, nm := range arr {
				if res, err := tx.Exec("DELETE FROM deleted_seeds WHERE name=?", jsString(nm)); err == nil {
					c, _ := res.RowsAffected()
					n += c
				}
			}
			tx.Commit()
		}
	}
	s.Seed()
	s.Bus.Publish(Msg{Event: "sources"})
	writeJSON(w, 200, obj("restored", n))
}

func (s *Store) hRuleCheck(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	type tr struct{ title, summary string }
	var rows []tr
	if truthy(b["sourceId"]) {
		n, _ := numOf(b["sourceId"])
		rs, _ := all(s.DB, "SELECT title, summary FROM items WHERE source_id=? ORDER BY published DESC LIMIT 40", n)
		for _, x := range rs {
			rows = append(rows, tr{x.Str("title"), x.Str("summary")})
		}
	} else if arr, ok := b["samples"].([]any); ok {
		for i, t := range arr {
			if i >= 40 {
				break
			}
			rows = append(rows, tr{jsSlice(jsString(t), 500), ""})
		}
	}
	scope, _ := b["scope"].(string)
	texts := make([]string, len(rows))
	for i, x := range rows {
		texts[i] = RuleText(scope, x.title, x.summary)
	}
	rule, e := ValidateRule(b, texts)
	if e != "" {
		detail(w, 400, e)
		return
	}
	for i, x := range rows {
		texts[i] = RuleText(rule.Scope, x.title, x.summary)
	}
	keep := EvalRule(rule, texts, 250*time.Millisecond)
	items := make([]*Row, len(rows))
	for i, x := range rows {
		it := &Row{}
		it.Set("title", x.title)
		it.Set("keep", keep == nil || keep[i])
		items[i] = it
	}
	ro := &Row{}
	ro.Set("include", rule.Include)
	ro.Set("exclude", rule.Exclude)
	ro.Set("scope", rule.Scope)
	ro.Set("cs", rule.CS)
	out := &Row{}
	out.Set("rule", ro)
	out.Set("items", items)
	writeJSON(w, 200, out)
}

func (s *Store) hSourceTest(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	d, err := Detect(r.Context(), jsString(b["input"]))
	if err != nil {
		detail(w, 400, detectErr(err))
		return
	}
	out := d.row()
	out.Set("overlap", s.overlap(d.Sample))
	writeJSON(w, 200, out)
}

func (s *Store) hStats(w http.ResponseWriter, r *http.Request) {
	items, _ := one(s.DB, `SELECT COUNT(*) total, COALESCE(SUM(is_x),0) x, COALESCE(SUM(CASE WHEN also_on_x!='[]' THEN 1 ELSE 0 END),0) with_x,
    COALESCE(SUM(CASE WHEN merged_into IS NOT NULL THEN 1 ELSE 0 END),0) merged FROM items i JOIN sources s ON s.id=i.source_id WHERE s.enabled=1`)
	sources, _ := one(s.DB, `SELECT COUNT(*) total, COALESCE(SUM(enabled AND supported),0) active, COALESCE(SUM(supported=0),0) unsupported,
    COALESCE(SUM(last_error IS NOT NULL AND enabled AND supported),0) errors,
    COALESCE(SUM(last_fetched IS NOT NULL AND enabled AND supported),0) fetched FROM sources`)
	out := &Row{}
	out.Set("items", items)
	out.Set("sources", sources)
	out.Set("retentionHours", retentionHours())
	out.Set("scheduler", s.schedulerState())
	out.Set("fx", FxMirrorState())
	out.Set("notify", s.notifyState())
	writeJSON(w, 200, out)
}

func (s *Store) hStream(w http.ResponseWriter, r *http.Request) {
	s.StartScheduler()
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	io.WriteString(w, "retry: 3000\n\n")
	fl.Flush()
	ch := s.Bus.Subscribe()
	defer s.Bus.Unsubscribe(ch)
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		case m := <-ch:
			b, _ := marshalNoEscape(m)
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", m.Event, b); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
