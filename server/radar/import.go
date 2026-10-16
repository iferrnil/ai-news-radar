package radar

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type planRow struct {
	I      int    `json:"i"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Match  string `json:"match,omitempty"`
	Reason string `json:"reason,omitempty"`
}

const importOV = "enabled,priority,poll_interval,section" // imported settings are user overrides: seed sync must not revert them

// hImport: POST { entries, mode: merge|update|replace, dryRun, test } — dryRun returns the plan only (sources/import route).
func (s *Store) hImport(w http.ResponseWriter, r *http.Request) {
	text, _ := io.ReadAll(io.LimitReader(r.Body, MaxBytes*2+1))
	if float64(len([]rune(string(text)))) > MaxBytes*1.5 {
		detail(w, 413, "请求过大（文件限制 1 MB）")
		return
	}
	var body map[string]any
	bad := func() { detail(w, 400, "请求格式错误") }
	if json.Unmarshal(text, &body) != nil || body == nil {
		bad()
		return
	}
	entries, ok := body["entries"].([]any)
	if !ok || len(entries) > MaxEntries {
		bad()
		return
	}
	if v, has := body["translateAll"]; has {
		if _, ok := v.(bool); !ok {
			bad()
			return
		}
	}
	mode := "merge"
	if v, has := body["mode"]; has {
		m, _ := v.(string)
		if m != "merge" && m != "update" && m != "replace" {
			bad()
			return
		}
		mode = m
	}
	dryRun, test := true, false
	if v, has := body["dryRun"]; has {
		b, ok := v.(bool)
		if !ok {
			bad()
			return
		}
		dryRun = b
	}
	if v, has := body["test"]; has {
		b, ok := v.(bool)
		if !ok {
			bad()
			return
		}
		test = b
	}

	local, _ := all(s.DB, "SELECT * FROM sources")
	byURL, byUser, byName := map[string]*Row{}, map[string]*Row{}, map[string]*Row{}
	for _, x := range local { // later rows win, like new Map(entries)
		byURL[NormURL(x.Str("url"))] = x
		if u := x.Str("username"); u != "" {
			byUser[strings.ToLower(u)] = x
		}
		byName[strings.ToLower(x.Str("name"))] = x
	}
	tomb := map[string]bool{}
	ts, _ := all(s.DB, "SELECT name FROM deleted_seeds")
	for _, t := range ts {
		tomb[t.Str("name")] = true
	}
	type validT struct {
		e       *Entry
		url     string
		match   *Row
		restore bool
	}
	rows := []planRow{}
	var valid []validT
	seenKeys := map[string]bool{}
	for i, raw := range entries {
		e, msg := ValidateEntry(raw)
		if e == nil {
			nm := ""
			if m, ok := raw.(map[string]any); ok && m["name"] != nil {
				nm = jsString(m["name"])
			} else {
				nm = "#" + jsNum(float64(i+1))
			}
			rows = append(rows, planRow{I: i, Name: jsSlice(nm, 120), Status: "invalid", Reason: msg})
			continue
		}
		fr := map[string]any{}
		if e.Filter != nil {
			fr = map[string]any{"include": e.Filter.Include, "exclude": e.Filter.Exclude, "scope": e.Filter.Scope, "cs": e.Filter.CaseSensitive}
		}
		if _, er := ValidateRule(fr, nil); er != "" {
			rows = append(rows, planRow{I: i, Name: e.Name, Status: "invalid", Reason: er})
			continue
		}
		u := e.URL
		key := NormURL(u)
		if e.Type == "x" {
			u = xProfileURL(e.Username)
			key = "x:" + strings.ToLower(e.Username)
		}
		if seenKeys[key] {
			rows = append(rows, planRow{I: i, Name: e.Name, Status: "invalid", Reason: "文件内重复"})
			continue
		}
		seenKeys[key] = true
		var match *Row
		if e.Type == "x" {
			match = byUser[strings.ToLower(e.Username)]
		}
		if match == nil {
			match = byURL[NormURL(u)]
		}
		if match == nil {
			match = byName[strings.ToLower(e.Name)]
		}
		restore := match == nil && tomb[e.Name]
		pr := planRow{I: i, Name: e.Name}
		switch {
		case match != nil:
			pr.Status = "update"
			if mode == "merge" {
				pr.Status = "dup"
			}
			pr.Match = match.Str("name")
			if strings.EqualFold(match.Str("name"), e.Name) && NormURL(match.Str("url")) != NormURL(u) {
				pr.Reason = "同名来源已存在"
			} else {
				pr.Reason = "已存在（URL / X 账号相同）"
			}
		case restore:
			pr.Status, pr.Reason = "restore", "已删除的内置源，将恢复"
		default:
			pr.Status = "new"
		}
		rows = append(rows, pr)
		valid = append(valid, validT{e, u, match, restore})
	}
	keep := map[any]bool{}
	for _, v := range valid {
		if v.match != nil {
			keep[v.match.Get("id")] = true
		}
	}
	remove := []*Row{}
	if mode == "replace" {
		for _, x := range local {
			if !keep[x.Get("id")] {
				rr := &Row{}
				rr.Set("id", x.Get("id"))
				rr.Set("name", x.Get("name"))
				remove = append(remove, rr)
			}
		}
	}
	counts := &Row{}
	for _, k := range []string{"new", "restore", "dup", "update", "invalid"} {
		counts.Set(k, 0)
	}
	counts.Set("remove", len(remove))
	for _, r := range rows {
		counts.Set(r.Status, int(counts.Num(r.Status))+1)
	}
	// counts values must be numbers: Set stored int, Num reads int64/float64 only — normalise
	for i, v := range counts.Vals {
		if n, ok := v.(int); ok {
			counts.Vals[i] = int64(n)
		}
	}
	plan := &Row{}
	plan.Set("rows", rows)
	plan.Set("remove", remove)
	plan.Set("counts", counts)
	if dryRun {
		writeJSON(w, 200, plan)
		return
	}

	var added []int64
	tx, err := s.DB.Begin()
	if err != nil {
		detail(w, 500, err.Error())
		return
	}
	secKey := func(e *Entry) string {
		byNm := func(n string) string {
			if n == "" {
				return ""
			}
			var k string
			tx.QueryRow("SELECT key FROM sections WHERE name=?", n).Scan(&k)
			return k
		}
		hit := byNm(e.SectionName)
		if hit == "" {
			if _, legacy := LegacyZh[e.Section]; legacy && isSectionKey(tx, e.Section) {
				hit = e.Section
			}
		}
		if hit == "" && isSectionKey(tx, e.Section) && e.SectionName == "" {
			hit = e.Section
		}
		if hit != "" {
			return hit
		}
		name := e.SectionName
		if name == "" {
			name = LegacyZh[e.Section]
		}
		if name != "" {
			inAll := e.Section != "ent"
			if e.SectionInAll != nil {
				inAll = *e.SectionInAll
			}
			raw := map[string]any{"name": name, "inAll": inAll}
			if e.SectionTranslate != nil {
				raw["tr"] = *e.SectionTranslate
			}
			if e.SectionIcon != "" {
				raw["icon"] = e.SectionIcon
			}
			if k, er := createSection(tx, raw); er == "" {
				return k
			}
		}
		return seedTarget(tx, "media")
	}
	type vals struct {
		enabled          int
		priority         string
		interval         int
		section, reg, ct string
		fi, fe           any
		fs               string
		fc               int
		tr               int // sources.tr
	}
	valsOf := func(e *Entry) vals {
		k := secKey(e)
		reg, ct := MetaFor(k)
		v := vals{enabled: b2i(e.Enabled), priority: e.Priority, interval: e.Interval, section: k, reg: reg, ct: ct, fs: "title"}
		if v.interval == 0 {
			v.interval = IntervalFor(e.Type, e.Priority)
		}
		if f := e.Filter; f != nil {
			v.fi, v.fe, v.fs, v.fc = nz(f.Include), nz(f.Exclude), f.Scope, b2i(f.CaseSensitive)
		}
		v.tr = 0
		if e.Translate == "on" { // inherit/off (older exports) = off
			v.tr = 1
		}
		return v
	}
	upd := func(v vals, id any) {
		tx.Exec(`UPDATE sources SET enabled=?, priority=?, poll_interval=?, section=?, region=?, category=?, overrides=?, f_include=?, f_exclude=?, f_scope=?, f_cs=?, tr=? WHERE id=?`,
			v.enabled, v.priority, v.interval, v.section, v.reg, v.ct, importOV, v.fi, v.fe, v.fs, v.fc, v.tr, id)
	}
	if len(remove) > 0 {
		tx.Commit() // deleteSources runs its own transaction (nested in a better-sqlite3 transaction in the Node build)
		ids := make([]int64, len(remove))
		for i, x := range remove {
			ids[i] = int64(x.Num("id"))
		}
		s.DeleteSources(ids)
		if tx, err = s.DB.Begin(); err != nil {
			detail(w, 500, err.Error())
			return
		}
	}
	restored := false
	for _, v := range valid {
		if v.restore {
			tx.Exec("DELETE FROM deleted_seeds WHERE name=?", v.e.Name)
			restored = true
		}
	}
	if restored {
		tx.Commit()
		s.Seed()
		if tx, err = s.DB.Begin(); err != nil {
			detail(w, 500, err.Error())
			return
		}
	}
	for _, v := range valid {
		if v.match != nil {
			if mode != "merge" {
				upd(valsOf(v.e), v.match.Get("id"))
			}
			continue
		}
		if v.restore {
			var id int64
			if tx.QueryRow("SELECT id FROM sources WHERE name=?", v.e.Name).Scan(&id) == nil {
				upd(valsOf(v.e), id)
				added = append(added, id)
				continue
			}
		}
		x := valsOf(v.e)
		var uname any
		if v.e.Type == "x" {
			uname = v.e.Username
		}
		res, err := tx.Exec(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,priority,poll_interval,origin,created,section,overrides,f_include,f_exclude,f_scope,f_cs,tr)
			VALUES(?,?,?,?,?,?,?,?,1,?,?,'user',?,?,?,?,?,?,?,?)`, v.e.Name, x.reg, x.ct, v.e.Type, v.url, nz(v.e.Homepage), uname, x.enabled, x.priority, x.interval, now(), x.section, importOV, x.fi, x.fe, x.fs, x.fc, x.tr)
		if err == nil {
			id, _ := res.LastInsertId()
			added = append(added, id)
		}
	}
	tx.Commit()
	for _, v := range valid {
		if v.match != nil && mode != "merge" {
			s.ReapplyRule(v.match.Get("id"))
		}
	}
	s.Bus.Publish(Msg{Event: "sources"})
	if body["translateAll"] == true {
		s.SetSetting("tr_all", "1") // the file had 「全部」 translation on
	}
	s.tr.backfill(true) // imported sources/sections may switch translation on (no-op when unavailable)
	if test && len(added) > 0 {
		args := make([]any, len(added))
		for i, id := range added {
			args[i] = id
		}
		list, _ := all(s.DB, "SELECT * FROM sources WHERE id IN ("+placeholders(len(added))+") AND enabled=1", args...)
		go func() {
			for k := 0; k < len(list); k += 4 {
				done := make(chan struct{}, 4)
				n := 0
				for _, src := range list[k:min(k+4, len(list))] {
					n++
					go func() { s.PollOne(src); done <- struct{}{} }()
				}
				for ; n > 0; n-- {
					<-done
				}
			}
		}()
	}
	plan.Set("applied", true)
	plan.Set("added", len(added))
	writeJSON(w, 200, plan)
}
