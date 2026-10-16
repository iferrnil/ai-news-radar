package core

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Section struct {
	Key, Name, Icon string
	InAll           bool
	Pos             int
}

var DefaultSections = []Section{
	{"lab", "实验室", "lucide:lab", true, 1}, {"cn", "国内媒体", "lucide:cn", true, 2}, {"media", "国外媒体", "lucide:media", true, 3},
	{"x", "X 快讯", "lucide:x", true, 4}, {"ent", "娱乐", "lucide:ent", false, 5},
}

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

func CleanSectionName(s string) string {
	return Trunc(strings.TrimSpace(wsRe.ReplaceAllString(ctrlRe.ReplaceAllString(s, ""), " ")), 24, "")
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

func (s *Store) Sections() ([]Section, error) {
	rows, err := s.DB.Query("SELECT key, name, COALESCE(icon,''), in_all, pos FROM sections ORDER BY pos, key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Section
	for rows.Next() {
		var x Section
		var ia int
		rows.Scan(&x.Key, &x.Name, &x.Icon, &ia, &x.Pos)
		x.InAll = ia == 1
		out = append(out, x)
	}
	return out, rows.Err()
}

type execer interface {
	Exec(string, ...any) (sql.Result, error)
	QueryRow(string, ...any) *sql.Row
}

func isSectionKey(e execer, k string) bool {
	var one int
	return e.QueryRow("SELECT 1 FROM sections WHERE key=?", k).Scan(&one) == nil
}

// CreateSection creates (or reuses by name) a section and returns its key.
func (s *Store) CreateSection(name, icon string, inAll bool) (string, error) {
	return createSection(s.DB, name, icon, inAll)
}

func createSection(e execer, rawName, icon string, inAll bool) (string, error) {
	name := CleanSectionName(rawName)
	if name == "" {
		return "", errors.New("分区名称不能为空")
	}
	if name == "全部" {
		return "", errors.New("「全部」是保留名称")
	}
	var key string
	if e.QueryRow("SELECT key FROM sections WHERE name=?", name).Scan(&key) == nil {
		return key, nil
	}
	var n, pos int
	e.QueryRow("SELECT COALESCE(MAX(CAST(substr(key,2) AS INTEGER)),0)+1 FROM sections WHERE key GLOB 'c[0-9]*'").Scan(&n)
	for isSectionKey(e, fmt.Sprintf("c%d", n)) {
		n++
	}
	e.QueryRow("SELECT COALESCE(MAX(pos),0)+1 FROM sections").Scan(&pos)
	key = fmt.Sprintf("c%d", n)
	_, err := e.Exec("INSERT INTO sections(key,name,icon,in_all,pos,created) VALUES(?,?,?,?,?,?)", key, name, cleanIcon(icon), b2i(inAll), pos, now())
	return key, err
}

func (s *Store) seedTargetTx(e execer, key string) string {
	if isSectionKey(e, key) {
		return key
	}
	k, err := createSection(e, "未分类", "📥", true)
	if err != nil {
		return key
	}
	return k
}

func (s *Store) migrateSections() error {
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
			tx.Exec("INSERT OR IGNORE INTO sections(key,name,icon,in_all,pos,created) VALUES(?,?,?,?,?,?)", d.Key, name, d.Icon, b2i(d.InAll), i+1, now())
		}
		tx.Exec("INSERT INTO settings VALUES('sections_seeded','1')")
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	rows, err := s.DB.Query("SELECT id, COALESCE(grp,''), COALESCE(region,''), COALESCE(category,''), type FROM sources WHERE section IS NULL")
	if err != nil {
		return err
	}
	type t struct {
		id                 int64
		grp, reg, cat, typ string
	}
	var todo []t
	for rows.Next() {
		var x t
		rows.Scan(&x.id, &x.grp, &x.reg, &x.cat, &x.typ)
		todo = append(todo, x)
	}
	rows.Close()
	for _, x := range todo {
		s.DB.Exec("UPDATE sources SET section=? WHERE id=?", s.seedTargetTx(s.DB, LegacyKey(x.grp, x.reg, x.cat, x.typ)), x.id)
	}
	return nil
}

// UpdateSection renames / re-icons / toggles 全部 inclusion.
func (s *Store) UpdateSection(key, name, icon string, inAll bool) error {
	n := CleanSectionName(name)
	if n == "" {
		return errors.New("分区名称不能为空")
	}
	if n == "全部" {
		return errors.New("「全部」是保留名称")
	}
	var other string
	if s.DB.QueryRow("SELECT key FROM sections WHERE name=? AND key<>?", n, key).Scan(&other) == nil {
		return errors.New("已有同名分区")
	}
	_, err := s.DB.Exec("UPDATE sections SET name=?, icon=?, in_all=? WHERE key=?", n, cleanIcon(icon), b2i(inAll), key)
	if err == nil {
		s.emit(Event{Kind: "sources"})
	}
	return err
}

// DeleteSection removes a section; its sources move to moveTo (must exist).
func (s *Store) DeleteSection(key, moveTo string) error {
	if key == moveTo || !isSectionKey(s.DB, moveTo) {
		return errors.New("请选择要移入的分区")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec("UPDATE sources SET section=? WHERE section=?", moveTo, key)
	tx.Exec("DELETE FROM sections WHERE key=?", key)
	if err = tx.Commit(); err == nil {
		s.emit(Event{Kind: "sources"})
	}
	return err
}

// MoveSection swaps a section with its neighbour (delta -1 up / +1 down).
func (s *Store) MoveSection(key string, delta int) error {
	secs, err := s.Sections()
	if err != nil {
		return err
	}
	for i, x := range secs {
		if x.Key == key {
			j := i + delta
			if j < 0 || j >= len(secs) {
				return nil
			}
			secs[i], secs[j] = secs[j], secs[i]
			for p, y := range secs {
				s.DB.Exec("UPDATE sections SET pos=? WHERE key=?", p+1, y.Key)
			}
			s.emit(Event{Kind: "sources"})
			return nil
		}
	}
	return nil
}
