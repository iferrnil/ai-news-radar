package radar

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

type defSection struct {
	Key, Name, Icon string
	InAll           int
}

// DefaultSections: the original defaults (migration + legacy mapping for v1 files / seed `group`).
var DefaultSections = []defSection{
	{"lab", "实验室", "lucide:lab", 1}, {"cn", "国内媒体", "lucide:cn", 1}, {"media", "国外媒体", "lucide:media", 1},
	{"x", "X 快讯", "lucide:x", 1}, {"ent", "娱乐", "lucide:ent", 0},
}

// LegacyZh: default key -> name.
var LegacyZh = map[string]string{"lab": "实验室", "cn": "国内媒体", "media": "国外媒体", "x": "X 快讯", "ent": "娱乐"}

var grpKey = map[string]string{"官方实验室": "lab", "国内": "cn", "开发与工具": "media", "媒体与聚合": "media", "娱乐": "ent", "Newsletter": "media", "X 快讯": "x", "爆料与榜单X": "x", "人物与产品X": "x"}

// LegacyKey derives the default section key from seed group / region / category / type.
func LegacyKey(grp, region, category, typ string) string {
	switch {
	case strings.HasPrefix(grp, "sec:"):
		return grp[4:]
	case grpKey[grp] != "":
		return grpKey[grp]
	case strings.HasPrefix(typ, "x"):
		return "x"
	case region == "cn":
		return "cn"
	case category == "official":
		return "lab"
	}
	return "media"
}

// MetaFor derives internal region/category from a section key.
func MetaFor(k string) (region, category string) {
	region, category = "global", "media"
	if k == "cn" {
		region = "cn"
	}
	if k == "lab" {
		category = "official"
	} else if k == "x" {
		category = "community"
	}
	return
}

var ctrlRe = regexp.MustCompile(`[\x00-\x1f\x7f<>]`)

// jsSlice is String#slice(0,n) on UTF-16 code units (keeps surrogate pairs whole when possible).
func jsSlice(s string, n int) string {
	u := 0
	for i, r := range s {
		w := 1
		if r > 0xffff {
			w = 2
		}
		if u+w > n {
			if u+1 == n && w == 2 { // JS would cut the pair in half; keep the whole rune instead
				return s[:i+len(string(r))]
			}
			return s[:i]
		}
		u += w
	}
	return s
}

// jsLen is String#length (UTF-16 code units).
func jsLen(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xffff {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func CleanSectionName(s string) string {
	return jsSlice(strings.TrimSpace(wsRe.ReplaceAllString(ctrlRe.ReplaceAllString(s, ""), " ")), 24)
}

var lucideRe = regexp.MustCompile(`^lucide:[a-z]+$`)

func cleanIcon(s string) any {
	t := strings.TrimSpace(s)
	if lucideRe.MatchString(t) {
		return t
	}
	if t == "" {
		return nil
	}
	return Trunc(t, 2, "")
}

type execer interface {
	Exec(string, ...any) (sql.Result, error)
	QueryRow(string, ...any) *sql.Row
	Query(string, ...any) (*sql.Rows, error)
}

func isSectionKey(e execer, k any) bool {
	ks, ok := k.(string)
	if !ok {
		return false
	}
	var one int
	return e.QueryRow("SELECT 1 FROM sections WHERE key=?", ks).Scan(&one) == nil
}

func listSections(e querier) []*Row {
	rs, _ := all(e, "SELECT key, name, icon, in_all, pos, tr FROM sections ORDER BY pos, key")
	return rs
}

// createSection creates (or reuses by name) a section. raw: {name, icon, inAll, tr}.
func createSection(e execer, raw map[string]any) (string, string) {
	name := CleanSectionName(jsString(raw["name"]))
	if name == "" {
		return "", "分区名称不能为空"
	}
	if name == "全部" {
		return "", "「全部」是保留名称"
	}
	var key string
	if e.QueryRow("SELECT key FROM sections WHERE name=?", name).Scan(&key) == nil {
		return key, ""
	}
	var n, pos int
	e.QueryRow("SELECT COALESCE(MAX(CAST(substr(key,2) AS INTEGER)),0)+1 FROM sections WHERE key GLOB 'c[0-9]*'").Scan(&n)
	for isSectionKey(e, fmt.Sprintf("c%d", n)) {
		n++
	}
	e.QueryRow("SELECT COALESCE(MAX(pos),0)+1 FROM sections").Scan(&pos)
	key = fmt.Sprintf("c%d", n)
	inAll := 1
	if b, ok := raw["inAll"].(bool); ok && !b {
		inAll = 0
	}
	tr := 0
	if b, ok := raw["tr"].(bool); ok && b {
		tr = 1
	}
	if _, err := e.Exec("INSERT INTO sections(key,name,icon,in_all,pos,created,tr) VALUES(?,?,?,?,?,?,?)", key, name, cleanIcon(jsString(raw["icon"])), inAll, pos, now(), tr); err != nil {
		return "", err.Error()
	}
	return key, ""
}

// seedTarget: a NEW seed source goes to its default section if it still exists, else 未分类 (auto-created).
func seedTarget(e execer, key string) string {
	if isSectionKey(e, key) {
		return key
	}
	k, errs := createSection(e, map[string]any{"name": "未分类", "icon": "📥"})
	if errs != "" {
		return key
	}
	return k
}

func (s *Store) migrateSections() error {
	// per-section translation switch (default on; the global switch stays the master toggle)
	secCols := s.cols("sections")
	if !secCols["tr"] {
		if _, err := s.DB.Exec("ALTER TABLE sections ADD COLUMN tr INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
	}
	if secCols["translate"] {
		if _, err := s.DB.Exec("ALTER TABLE sections DROP COLUMN translate"); err != nil {
			return err
		}
	}
	if _, ok := s.GetSetting("sections_seeded"); !ok {
		tx, err := s.DB.Begin()
		if err != nil {
			return err
		}
		tx.Exec("UPDATE sections SET pos = pos + ?", len(DefaultSections))
		for i, d := range DefaultSections {
			name := d.Name
			var one int
			if tx.QueryRow("SELECT 1 FROM sections WHERE name=?", name).Scan(&one) == nil {
				name += "（默认）"
			}
			tx.Exec("INSERT OR IGNORE INTO sections(key,name,icon,in_all,pos,created) VALUES(?,?,?,?,?,?)", d.Key, name, d.Icon, d.InAll, i+1, now())
		}
		tx.Exec("INSERT INTO settings VALUES('sections_seeded','1')")
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	todo, err := all(s.DB, "SELECT id, grp, region, category, type FROM sources WHERE section IS NULL")
	if err != nil || len(todo) == 0 {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	for _, x := range todo {
		tx.Exec("UPDATE sources SET section=? WHERE id=?", seedTarget(tx, LegacyKey(x.Str("grp"), x.Str("region"), x.Str("category"), x.Str("type"))), x.Get("id"))
	}
	return tx.Commit()
}

// jsString is String(v ?? "") for JSON-decoded values.
func jsString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return jsNum(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = jsString(e)
		}
		return strings.Join(parts, ",")
	case map[string]any:
		return "[object Object]"
	}
	return fmt.Sprint(v)
}
