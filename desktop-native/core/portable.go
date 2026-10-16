package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Import/export compatible with the web app (src/lib/portable*.ts, api/sources/import).
const (
	App           = "ai-news-radar"
	SchemaVersion = 2
	MaxBytes      = 1 << 20
	MaxEntries    = 2000
)

var legacyKeys = []string{"lab", "cn", "media", "x", "ent"}
var legacyZh = map[string]string{"lab": "实验室", "cn": "国内媒体", "media": "国外媒体", "x": "X 快讯", "ent": "娱乐"}

type exFilter struct {
	Include       string `json:"include,omitempty"`
	Exclude       string `json:"exclude,omitempty"`
	Scope         string `json:"scope"`
	CaseSensitive bool   `json:"caseSensitive"`
}
type exEntry struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	URL      string    `json:"url,omitempty"`
	Username string    `json:"username,omitempty"`
	Homepage string    `json:"homepage,omitempty"`
	Section  string    `json:"section"`
	Priority string    `json:"priority"`
	Interval int       `json:"interval"`
	Enabled  bool      `json:"enabled"`
	Filter   *exFilter `json:"filter,omitempty"`
}
type exSection struct {
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Icon  *string `json:"icon,omitempty"`
	InAll bool    `json:"inAll"`
}

func bjStamp(t time.Time) (iso, file string) {
	b := t.In(cst)
	return b.Format("2006-01-02T15:04:05") + "+08:00", b.Format("20060102-1504")
}

// ExportFileName is ai-news-radar-sources-YYYYMMDD-HHmm.<ext> (Beijing time).
func ExportFileName(ext string, t time.Time) string {
	_, f := bjStamp(t)
	return App + "-sources-" + f + "." + ext
}

func toEntry(s Source) exEntry {
	e := exEntry{Name: s.Name, Type: s.Type, Homepage: s.Homepage, Section: s.Section, Priority: s.Priority, Interval: s.PollInterval, Enabled: s.Enabled}
	if e.Priority == "" {
		e.Priority = "P1"
	}
	if s.Type == "x" {
		e.Username = s.Username
	} else {
		e.URL = s.URL
	}
	if s.Rule.Has() {
		sc := "title"
		if s.Rule.Scope == "all" {
			sc = "all"
		}
		e.Filter = &exFilter{s.Rule.Include, s.Rule.Exclude, sc, s.Rule.CS}
	}
	return e
}

// ExportJSON builds the v2 JSON export (sections used by the list are included).
func (s *Store) ExportJSON(t time.Time) ([]byte, error) {
	list, err := s.Sources()
	if err != nil {
		return nil, err
	}
	secs, _ := s.Sections()
	var used []exSection
	for _, c := range secs {
		if slices.ContainsFunc(list, func(x Source) bool { return x.Section == c.Key }) {
			var ic *string
			if c.Icon != "" {
				ic = &c.Icon
			}
			used = append(used, exSection{c.Key, c.Name, ic, c.InAll})
		}
	}
	entries := make([]exEntry, len(list))
	for i, x := range list {
		entries[i] = toEntry(x)
	}
	iso, _ := bjStamp(t)
	doc := struct {
		App        string      `json:"app"`
		Version    int         `json:"version"`
		ExportedAt string      `json:"exportedAt"`
		Count      int         `json:"count"`
		Sections   []exSection `json:"sections,omitempty"`
		Sources    []exEntry   `json:"sources"`
	}{App, SchemaVersion, iso, len(list), used, entries}
	b, err := json.MarshalIndent(doc, "", "  ")
	return append(b, '\n'), err
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// ExportOPML builds OPML 2.0 with one folder per section (X accounts/lists as custom outlines).
func (s *Store) ExportOPML(t time.Time) ([]byte, error) {
	list, err := s.Sources()
	if err != nil {
		return nil, err
	}
	secs, _ := s.Sections()
	out := []string{`<?xml version="1.0" encoding="UTF-8"?>`, `<opml version="2.0">`, `  <head>`, `    <title>AI News Radar subscriptions</title>`,
		`    <dateCreated>` + t.UTC().Format(http1123) + `</dateCreated>`, `    <docs>http://opml.org/spec2.opml</docs>`, `  </head>`, `  <body>`}
	type grp struct{ key, name, extra string }
	var groups []grp
	for _, c := range secs {
		ex := fmt.Sprintf(` sectionCustom="1" sectionKey="%s" sectionInAll="%t"`, esc(c.Key), c.InAll)
		if c.Icon != "" {
			ex += ` sectionIcon="` + esc(c.Icon) + `"`
		}
		groups = append(groups, grp{c.Key, c.Name, ex})
	}
	isKnown := func(k string) bool { return slices.ContainsFunc(secs, func(c Section) bool { return c.Key == k }) }
	if slices.ContainsFunc(list, func(x Source) bool { return !isKnown(x.Section) }) {
		groups = append(groups, grp{"__orphan", "未分类", ` sectionCustom="1"`})
	}
	for _, g := range groups {
		var rows []Source
		for _, x := range list {
			if (g.key == "__orphan" && !isKnown(x.Section)) || x.Section == g.key {
				rows = append(rows, x)
			}
		}
		if len(rows) == 0 {
			continue
		}
		out = append(out, fmt.Sprintf(`    <outline text="%s" title="%s"%s>`, esc(g.name), esc(g.name), g.extra))
		for _, x := range rows {
			e := toEntry(x)
			fa := ""
			if f := e.Filter; f != nil {
				if f.Include != "" {
					fa += ` filterInclude="` + esc(f.Include) + `"`
				}
				if f.Exclude != "" {
					fa += ` filterExclude="` + esc(f.Exclude) + `"`
				}
				fa += fmt.Sprintf(` filterScope="%s" filterCase="%t"`, f.Scope, f.CaseSensitive)
			}
			en := "false"
			if x.Enabled {
				en = "true"
			}
			common := fmt.Sprintf(`text="%s" title="%s" category="%s"%s priority="%s" interval="%d" enabled="%s"%s`, esc(x.Name), esc(x.Name), esc(g.name), g.extra, esc(e.Priority), x.PollInterval, en, fa)
			switch x.Type {
			case "x":
				out = append(out, fmt.Sprintf(`      <outline type="x" %s xUsername="%s" htmlUrl="https://x.com/%s"/>`, common, esc(x.Username), esc(x.Username)))
			case "x_list":
				out = append(out, fmt.Sprintf(`      <outline type="x_list" %s xmlUrl="%s"/>`, common, esc(x.URL)))
			default:
				typ := "rss"
				if x.Type == "api" {
					typ = "api"
				}
				hp := ""
				if x.Homepage != "" {
					hp = ` htmlUrl="` + esc(x.Homepage) + `"`
				}
				out = append(out, fmt.Sprintf(`      <outline type="%s" %s xmlUrl="%s"%s/>`, typ, common, esc(x.URL), hp))
			}
		}
		out = append(out, `    </outline>`)
	}
	out = append(out, `  </body>`, `</opml>`, ``)
	return []byte(strings.Join(out, "\n")), nil
}

const http1123 = "Mon, 02 Jan 2006 15:04:05 GMT"

// Entry is a validated import row.
type Entry struct {
	Name, Type, URL, Username, Homepage string
	Section, SectionName, SectionIcon   string
	SectionInAll                        *bool
	Rule                                Rule
	Priority                            string
	Interval                            int
	Enabled                             bool
}

type ParsedRow struct {
	Name  string
	Entry *Entry
	Error string
}

var xUser = regexp.MustCompile(`^@?[A-Za-z0-9_]{1,15}$`)

func httpURL(u string) bool {
	p, err := url.Parse(strings.TrimSpace(u))
	return err == nil && (p.Scheme == "http" || p.Scheme == "https") && len(u) <= 2000
}

// validateEntry mirrors EntrySchema (zod) defaults/catches.
func validateEntry(m map[string]any) (*Entry, string) {
	e := &Entry{Name: cleanName(js(m["name"])), Type: js(m["type"]), Priority: js(m["priority"]), Enabled: true}
	if e.Name == "" {
		return nil, "名称为空"
	}
	if !slices.Contains(Supported, e.Type) {
		return nil, "类型无效"
	}
	if u := strings.TrimSpace(js(m["url"])); u != "" {
		if !httpURL(u) {
			return nil, "只允许 http(s) URL"
		}
		e.URL = u
	}
	if u := strings.TrimSpace(js(m["username"])); u != "" {
		if !xUser.MatchString(u) {
			return nil, "X 用户名不合法"
		}
		e.Username = strings.TrimPrefix(u, "@")
	}
	if h := strings.TrimSpace(js(m["homepage"])); httpURL(h) {
		e.Homepage = h
	}
	e.Section = strings.TrimSpace(js(m["section"]))
	if e.Section == "" || len(e.Section) > 40 {
		e.Section = "media"
	}
	if n := strings.TrimSpace(js(m["sectionName"])); n != "" && len([]rune(n)) <= 24 {
		e.SectionName = n
	}
	if ic := strings.TrimSpace(js(m["sectionIcon"])); len([]rune(ic)) <= 8 {
		e.SectionIcon = ic
	}
	if b, ok := m["sectionInAll"].(bool); ok {
		e.SectionInAll = &b
	}
	if f := jo(m["filter"]); f != nil {
		e.Rule = Rule{Include: js(f["include"]), Exclude: js(f["exclude"]), Scope: js(f["scope"])}
		e.Rule.CS, _ = f["caseSensitive"].(bool)
	}
	if e.Priority != "P0" && e.Priority != "P1" && e.Priority != "P2" {
		e.Priority = "P1"
	}
	if iv, ok := m["interval"].(float64); ok && iv == float64(int(iv)) && iv >= 60 && iv <= 86400 {
		e.Interval = int(iv)
	}
	if b, ok := m["enabled"].(bool); ok {
		e.Enabled = b
	}
	if e.Type == "x" && e.Username == "" {
		return nil, "X 账号缺少 username"
	}
	if e.Type != "x" && e.URL == "" {
		return nil, "缺少 URL"
	}
	r, err := e.Rule.Validate()
	if err != nil {
		return nil, err.Error()
	}
	e.Rule = r
	return e, ""
}

// ParseImport parses a JSON (this app, v1/v2) or OPML file. opmlSection is the fallback section key for foreign OPML folders.
func (s *Store) ParseImport(data []byte, fileName, opmlSection string) (format string, rows []ParsedRow, err error) {
	if len(data) > MaxBytes {
		return "", nil, errors.New("文件超过 1 MB")
	}
	t := strings.TrimSpace(strings.TrimPrefix(string(data), "\uFEFF"))
	isJSON := strings.HasSuffix(strings.ToLower(fileName), ".json") || strings.HasPrefix(t, "{")
	var raws []map[string]any
	if isJSON {
		format = "json"
		var d struct {
			App      string
			Version  int
			Sources  []any
			Sections []struct {
				Key, Name string
				Icon      *string
				InAll     *bool `json:"inAll"`
			}
		}
		if json.Unmarshal([]byte(t), &d) != nil {
			return "", nil, errors.New("JSON 解析失败")
		}
		if d.App != App || d.Version < 1 || d.Version > SchemaVersion {
			return "", nil, fmt.Errorf(`不是本应用导出的 JSON（需要 app="%s" 且 version≤%d）`, App, SchemaVersion)
		}
		for _, r := range d.Sources {
			m := jo(r)
			if m == nil {
				m = map[string]any{}
			}
			for _, sec := range d.Sections {
				if sec.Key == js(m["section"]) {
					m["sectionName"] = sec.Name
					if sec.Icon != nil {
						m["sectionIcon"] = *sec.Icon
					}
					if sec.InAll != nil {
						m["sectionInAll"] = *sec.InAll
					}
				}
			}
			raws = append(raws, m)
		}
	} else {
		format = "opml"
		raws, err = s.opmlRows([]byte(t), opmlSection)
		if err != nil {
			return "", nil, err
		}
	}
	if len(raws) > MaxEntries {
		return "", nil, fmt.Errorf("条目过多（>%d）", MaxEntries)
	}
	for i, m := range raws {
		e, msg := validateEntry(m)
		name := Trunc(js(m["name"]), 120, "")
		if name == "" {
			name = "#" + strconv.Itoa(i+1)
		}
		rows = append(rows, ParsedRow{Name: name, Entry: e, Error: msg})
	}
	return format, rows, nil
}

func (s *Store) zhToKey(v string) string {
	t := strings.TrimSpace(v)
	if t == "" {
		return ""
	}
	parts := regexp.MustCompile(`[,/]`).Split(t, -1)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	secs, _ := s.Sections()
	for _, k := range secs {
		if t == k.Key || slices.Contains(parts, k.Name) {
			return k.Key
		}
	}
	for _, k := range legacyKeys {
		if t == k || slices.Contains(parts, legacyZh[k]) {
			return k
		}
	}
	return ""
}

func (s *Store) opmlRows(b []byte, fallback string) ([]map[string]any, error) {
	root, err := parseXML(b)
	if err != nil {
		return nil, errors.New("不是有效的 OPML 文件（缺少 <opml><body>）")
	}
	body := root.first("opml").first("body")
	if body == nil {
		return nil, errors.New("不是有效的 OPML 文件（缺少 <opml><body>）")
	}
	var rows []map[string]any
	var walk func(nodes []*node, folder string)
	walk = func(nodes []*node, folder string) {
		for _, o := range nodes {
			a := o.Attr
			name := firstNonEmpty(a["title"], a["text"])
			m := map[string]any{}
			if a["sectionCustom"] == "1" && a["category"] != "" {
				m["section"] = firstNonEmpty(a["sectionKey"], "custom")
				m["sectionName"] = a["category"]
				if a["sectionIcon"] != "" {
					m["sectionIcon"] = a["sectionIcon"]
				}
				if a["sectionInAll"] != "" {
					m["sectionInAll"] = a["sectionInAll"] != "false"
				}
			} else {
				m["section"] = firstNonEmpty(s.zhToKey(a["category"]), s.zhToKey(folder), fallback)
			}
			if a["priority"] != "" {
				m["priority"] = a["priority"]
			}
			if iv, err := strconv.Atoi(a["interval"]); err == nil {
				m["interval"] = float64(iv)
			}
			if a["enabled"] != "" {
				m["enabled"] = a["enabled"] != "false"
			}
			if a["filterInclude"] != "" || a["filterExclude"] != "" {
				m["filter"] = map[string]any{"include": a["filterInclude"], "exclude": a["filterExclude"], "scope": a["filterScope"], "caseSensitive": a["filterCase"] == "true"}
			}
			if a["type"] == "x" {
				m["name"], m["type"], m["username"] = name, "x", a["xUsername"]
				rows = append(rows, m)
			} else if a["xmlUrl"] != "" {
				typ := "rss"
				if a["type"] == "x_list" || a["type"] == "api" {
					typ = a["type"]
				}
				m["name"], m["type"], m["url"], m["homepage"] = firstNonEmpty(name, a["xmlUrl"]), typ, a["xmlUrl"], a["htmlUrl"]
				rows = append(rows, m)
			}
			if kids := o.all("outline"); len(kids) > 0 {
				f := folder
				if a["xmlUrl"] == "" {
					f = name
				}
				walk(kids, f)
			}
		}
	}
	walk(body.all("outline"), "")
	return rows, nil
}

// ImportPlan counts what an import would do.
type ImportPlan struct {
	Rows                               []PlanRow
	Remove                             []Source
	New, Restore, Dup, Update, Invalid int
}
type PlanRow struct {
	Name, Status, Reason, Match string
	entry                       *Entry
	match                       *Source
}

// PlanImport mirrors the web route's dry run. mode: merge | update | replace.
func (s *Store) PlanImport(rows []ParsedRow, mode string) (*ImportPlan, error) {
	local, err := s.Sources()
	if err != nil {
		return nil, err
	}
	tomb := map[string]bool{}
	for _, n := range s.strings("SELECT name FROM deleted_seeds") {
		tomb[n] = true
	}
	find := func(e *Entry, u string) *Source {
		for i := range local {
			l := &local[i]
			if e.Type == "x" && strings.EqualFold(l.Username, e.Username) {
				return l
			}
		}
		for i := range local {
			if NormURL(local[i].URL) == NormURL(u) {
				return &local[i]
			}
		}
		for i := range local {
			if strings.EqualFold(local[i].Name, e.Name) {
				return &local[i]
			}
		}
		return nil
	}
	p := &ImportPlan{}
	seen := map[string]bool{}
	keep := map[int64]bool{}
	for _, r := range rows {
		if r.Entry == nil {
			p.Rows = append(p.Rows, PlanRow{Name: r.Name, Status: "invalid", Reason: r.Error})
			p.Invalid++
			continue
		}
		e := r.Entry
		u := e.URL
		key := NormURL(u)
		if e.Type == "x" {
			u = "https://api.fxtwitter.com/2/profile/" + e.Username + "/statuses"
			key = "x:" + strings.ToLower(e.Username)
		}
		if seen[key] {
			p.Rows = append(p.Rows, PlanRow{Name: e.Name, Status: "invalid", Reason: "文件内重复"})
			p.Invalid++
			continue
		}
		seen[key] = true
		e.URL = u
		m := find(e, u)
		pr := PlanRow{Name: e.Name, entry: e, match: m}
		switch {
		case m != nil:
			keep[m.ID] = true
			pr.Match = m.Name
			pr.Reason = "已存在（URL / X 账号相同）"
			if strings.EqualFold(m.Name, e.Name) && NormURL(m.URL) != NormURL(u) {
				pr.Reason = "同名来源已存在"
			}
			if mode == "merge" {
				pr.Status = "dup"
				p.Dup++
			} else {
				pr.Status = "update"
				p.Update++
			}
		case tomb[e.Name]:
			pr.Status, pr.Reason = "restore", "已删除的内置源，将恢复"
			p.Restore++
		default:
			pr.Status = "new"
			p.New++
		}
		p.Rows = append(p.Rows, pr)
	}
	if mode == "replace" {
		for _, l := range local {
			if !keep[l.ID] {
				p.Remove = append(p.Remove, l)
			}
		}
	}
	return p, nil
}

func (s *Store) sectionFor(e *Entry) string {
	if e.SectionName != "" {
		var k string
		if s.DB.QueryRow("SELECT key FROM sections WHERE name=?", e.SectionName).Scan(&k) == nil {
			return k
		}
	}
	if _, legacy := legacyZh[e.Section]; legacy && isSectionKey(s.DB, e.Section) {
		return e.Section
	}
	if e.SectionName == "" && isSectionKey(s.DB, e.Section) {
		return e.Section
	}
	name := firstNonEmpty(e.SectionName, legacyZh[e.Section])
	if name != "" {
		inAll := e.Section != "ent"
		if e.SectionInAll != nil {
			inAll = *e.SectionInAll
		}
		if k, err := s.CreateSection(name, e.SectionIcon, inAll); err == nil {
			return k
		}
	}
	return s.seedTargetTx(s.DB, "media")
}

// ApplyImport executes a plan. Returns ids of added sources.
func (s *Store) ApplyImport(p *ImportPlan, mode string) ([]int64, error) {
	if len(p.Remove) > 0 {
		ids := make([]int64, len(p.Remove))
		for i, r := range p.Remove {
			ids[i] = r.ID
		}
		if _, err := s.DeleteSources(ids); err != nil {
			return nil, err
		}
	}
	const ov = "enabled,priority,poll_interval,section"
	var added []int64
	for _, r := range p.Rows {
		e := r.entry
		if e == nil || (r.match != nil && mode == "merge") {
			continue
		}
		sec := s.sectionFor(e)
		reg, cat := MetaFor(sec)
		iv := e.Interval
		if iv == 0 {
			iv = IntervalFor(e.Type, e.Priority)
		}
		if r.match != nil {
			if _, err := s.DB.Exec(`UPDATE sources SET enabled=?, priority=?, poll_interval=?, section=?, region=?, category=?, overrides=?, f_include=?, f_exclude=?, f_scope=?, f_cs=? WHERE id=?`,
				b2i(e.Enabled), e.Priority, iv, sec, reg, cat, ov, nz(e.Rule.Include), nz(e.Rule.Exclude), e.Rule.Scope, b2i(e.Rule.CS), r.match.ID); err != nil {
				return added, err
			}
			s.ReapplyRule(r.match.ID)
			continue
		}
		if r.Status == "restore" {
			s.DB.Exec("DELETE FROM deleted_seeds WHERE name=?", e.Name)
		}
		var user any
		if e.Type == "x" {
			user = e.Username
		}
		res, err := s.DB.Exec(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,priority,poll_interval,origin,created,section,overrides,f_include,f_exclude,f_scope,f_cs)
			VALUES(?,?,?,?,?,?,?,?,1,?,?,'user',?,?,?,?,?,?,?)`, e.Name, reg, cat, e.Type, e.URL, nz(e.Homepage), user, b2i(e.Enabled), e.Priority, iv, now(), sec, ov,
			nz(e.Rule.Include), nz(e.Rule.Exclude), e.Rule.Scope, b2i(e.Rule.CS))
		if err != nil {
			return added, fmt.Errorf("%s: %w", e.Name, err)
		}
		id, _ := res.LastInsertId()
		added = append(added, id)
	}
	s.emit(Event{Kind: "sources"})
	return added, nil
}
