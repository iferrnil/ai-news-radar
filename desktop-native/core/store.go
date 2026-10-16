package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// Source mirrors types.ts SourceRow (fields the desktop needs).
type Source struct {
	ID                                          int64
	Name, Region, Category, Type, URL, Homepage string
	Username, Priority, Origin, Grp, Section    string
	Enabled, Supported                          bool
	UnsupportedReason, LastError, Note          string
	PollInterval                                int
	LastFetched                                 float64
	LastNew                                     int
	Rule                                        Rule
	Count                                       int // items currently stored (visible)
}

const sourceCols = `s.id,s.name,COALESCE(s.region,''),COALESCE(s.category,''),s.type,s.url,COALESCE(s.homepage,''),COALESCE(s.username,''),
COALESCE(s.priority,''),COALESCE(s.origin,''),COALESCE(s.grp,''),COALESCE(s.section,''),COALESCE(s.enabled,1),COALESCE(s.supported,1),
COALESCE(s.unsupported_reason,''),COALESCE(s.last_error,''),COALESCE(s.note,''),COALESCE(s.poll_interval,600),COALESCE(s.last_fetched,0),
COALESCE(s.last_new,0),COALESCE(s.f_include,''),COALESCE(s.f_exclude,''),COALESCE(s.f_scope,'title'),COALESCE(s.f_cs,0)`

func scanSource(sc interface{ Scan(...any) error }, extra ...any) (Source, error) {
	var x Source
	var en, sup, cs int
	args := []any{&x.ID, &x.Name, &x.Region, &x.Category, &x.Type, &x.URL, &x.Homepage, &x.Username, &x.Priority, &x.Origin, &x.Grp, &x.Section,
		&en, &sup, &x.UnsupportedReason, &x.LastError, &x.Note, &x.PollInterval, &x.LastFetched, &x.LastNew,
		&x.Rule.Include, &x.Rule.Exclude, &x.Rule.Scope, &cs}
	err := sc.Scan(append(args, extra...)...)
	x.Enabled, x.Supported, x.Rule.CS = en == 1, sup == 1, cs == 1
	return x, err
}

func (s *Store) Sources() ([]Source, error) {
	rows, err := s.DB.Query(`SELECT ` + sourceCols + `, (SELECT COUNT(*) FROM items i WHERE i.source_id=s.id AND i.hidden=0 AND i.merged_into IS NULL)
		FROM sources s ORDER BY s.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var n int
		x, err := scanSource(rows, &n)
		if err != nil {
			return nil, err
		}
		x.Count = n
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) Source(id int64) (Source, error) {
	return scanSource(s.DB.QueryRow(`SELECT `+sourceCols+` FROM sources s WHERE s.id=?`, id))
}

// Item is one news entry as shown in the feed.
type Item struct {
	ID                                              int64
	Title, URL, Summary, Author, Image              string
	TitleZh, SummaryZh                              string
	Published                                       float64
	IsX, Read                                       bool
	AlsoOnX                                         []AlsoX
	SourceID                                        int64
	Source, SourceType, Priority, Section, Homepage string
}

type AlsoX struct {
	URL    string  `json:"url"`
	Author *string `json:"author"`
}

// Query selects visible items. section "" = 全部 (sections with in_all=1); sourceID>0 narrows to one source.
type Query struct {
	Section    string
	SourceID   int64
	UnreadOnly bool
	Search     string
	Limit      int
}

func (s *Store) Items(q Query) ([]Item, error) {
	where := []string{"i.merged_into IS NULL", "i.hidden=0", "s.enabled=1"}
	var args []any
	switch {
	case q.SourceID > 0:
		where = append(where, "s.id=?")
		args = append(args, q.SourceID)
	case q.Section != "":
		where = append(where, "s.section=?")
		args = append(args, q.Section)
	default:
		where = append(where, "COALESCE((SELECT in_all FROM sections WHERE key=s.section),1)=1")
	}
	if q.UnreadOnly {
		where = append(where, "r.item_id IS NULL")
	}
	if t := strings.TrimSpace(q.Search); t != "" {
		where = append(where, "(i.title LIKE ? OR i.summary LIKE ? OR COALESCE(i.title_zh,'') LIKE ?)")
		p := "%" + t + "%"
		args = append(args, p, p, p)
	}
	lim := q.Limit
	if lim <= 0 || lim > 2000 {
		lim = 500
	}
	rows, err := s.DB.Query(`SELECT i.id,COALESCE(i.title,''),COALESCE(i.url,''),COALESCE(i.summary,''),COALESCE(i.author,''),COALESCE(i.image,''),
		COALESCE(i.title_zh,''),COALESCE(i.summary_zh,''),i.published,i.is_x,r.item_id IS NOT NULL,COALESCE(i.also_on_x,'[]'),
		s.id,s.name,s.type,COALESCE(s.priority,''),COALESCE(s.section,''),COALESCE(s.homepage,'')
		FROM items i JOIN sources s ON s.id=i.source_id LEFT JOIN read_items r ON r.item_id=i.id
		WHERE `+strings.Join(where, " AND ")+` ORDER BY i.published DESC LIMIT ?`, append(args, lim)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		var isx int
		var also string
		if err := rows.Scan(&it.ID, &it.Title, &it.URL, &it.Summary, &it.Author, &it.Image, &it.TitleZh, &it.SummaryZh, &it.Published, &isx, &it.Read, &also,
			&it.SourceID, &it.Source, &it.SourceType, &it.Priority, &it.Section, &it.Homepage); err != nil {
			return nil, err
		}
		it.IsX = isx == 1
		it.Title, it.Summary = Decode(it.Title), Decode(it.Summary)
		json.Unmarshal([]byte(also), &it.AlsoOnX)
		out = append(out, it)
	}
	return out, rows.Err()
}

// UnreadCounts returns unread counts by section key ("" = 全部).
func (s *Store) UnreadCounts() map[string]int {
	out := map[string]int{}
	rows, err := s.DB.Query(`SELECT COALESCE(s.section,''), COALESCE(sec.in_all,1), COUNT(*) FROM items i JOIN sources s ON s.id=i.source_id
		LEFT JOIN sections sec ON sec.key=s.section LEFT JOIN read_items r ON r.item_id=i.id
		WHERE i.merged_into IS NULL AND i.hidden=0 AND s.enabled=1 AND r.item_id IS NULL GROUP BY 1,2`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var inAll, n int
		rows.Scan(&k, &inAll, &n)
		out[k] += n
		if inAll == 1 {
			out[""] += n
		}
	}
	return out
}

func (s *Store) SetRead(ids []int64, read bool) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if read {
			tx.Exec("INSERT OR IGNORE INTO read_items VALUES(?,?)", id, now())
		} else {
			tx.Exec("DELETE FROM read_items WHERE item_id=?", id)
		}
	}
	return tx.Commit()
}

var xHost = regexp.MustCompile(`^(x\.com|twitter\.com|t\.co)/`)

// StoreItems inserts new items (dedupe via seen), applies the source rule, merges tweets into linked articles.
// Returns ids of newly visible items.
func (s *Store) StoreItems(src Source, items []FetchedItem) ([]int64, error) {
	t := now()
	cutoff := t - RetentionHours*3600
	firstFetch := src.LastFetched == 0
	var keep []bool
	if src.Rule.Has() {
		texts := make([]string, len(items))
		for i, it := range items {
			texts[i] = RuleText(src.Rule.Scope, it.Title, it.Summary)
		}
		keep = src.Rule.Keeps(texts)
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var newIDs []int64
	for n, it := range items {
		var one int
		if tx.QueryRow("SELECT 1 FROM seen WHERE uid=?", it.UID).Scan(&one) == nil {
			continue
		}
		tx.Exec("INSERT OR IGNORE INTO seen VALUES(?,?)", it.UID, t)
		if keep != nil && !keep[n] {
			continue
		}
		if it.Published == 0 && firstFetch {
			continue
		}
		pub := t
		if it.Published != 0 && float64(it.Published) < t {
			pub = float64(it.Published)
		}
		if pub < cutoff {
			continue
		}
		ins := `INSERT OR IGNORE INTO items(uid,source_id,title,url,summary,published,fetched,is_x,author,merged_into,image) VALUES(?,?,?,?,?,?,?,?,?,?,?)`
		if it.IsX {
			var links []string
			for _, l := range it.Links {
				if nl := NormURL(l); !xHost.MatchString(nl) {
					links = append(links, nl)
				}
			}
			links = uniq(links)
			var artID int64
			var artAlso string
			for _, l := range links {
				if tx.QueryRow("SELECT id, COALESCE(also_on_x,'[]') FROM items WHERE uid=? AND is_x=0", l).Scan(&artID, &artAlso) == nil {
					break
				}
				artID = 0
			}
			var merged any
			if artID != 0 {
				merged = artID
			}
			res, err := tx.Exec(ins, it.UID, src.ID, it.Title, it.URL, it.Summary, pub, t, 1, nz(it.Author), merged, nz(it.Image))
			if err != nil {
				return nil, err
			}
			if c, _ := res.RowsAffected(); c == 0 {
				continue
			}
			xid, _ := res.LastInsertId()
			for _, l := range links {
				tx.Exec("INSERT OR IGNORE INTO post_links VALUES(?,?)", l, xid)
			}
			if artID != 0 {
				var also []AlsoX
				json.Unmarshal([]byte(artAlso), &also)
				also = append(also, AlsoX{it.URL, strPtr(it.Author)})
				b, _ := json.Marshal(also)
				tx.Exec("UPDATE items SET also_on_x=? WHERE id=?", string(b), artID)
			} else {
				newIDs = append(newIDs, xid)
			}
		} else {
			res, err := tx.Exec(ins, it.UID, src.ID, it.Title, it.URL, it.Summary, pub, t, 0, nz(it.Author), nil, nz(it.Image))
			if err != nil {
				return nil, err
			}
			if c, _ := res.RowsAffected(); c == 0 {
				continue
			}
			aid, _ := res.LastInsertId()
			rows, err := tx.Query("SELECT i.id, COALESCE(i.url,''), i.author FROM post_links p JOIN items i ON i.id=p.item_id WHERE p.norm_url=? AND i.merged_into IS NULL", it.UID)
			if err != nil {
				return nil, err
			}
			var also []AlsoX
			var xids []int64
			for rows.Next() {
				var id int64
				var u string
				var a sql.NullString
				rows.Scan(&id, &u, &a)
				var ap *string
				if a.Valid {
					ap = &a.String
				}
				also = append(also, AlsoX{u, ap})
				xids = append(xids, id)
			}
			rows.Close()
			if len(also) > 0 {
				b, _ := json.Marshal(also)
				tx.Exec("UPDATE items SET also_on_x=? WHERE id=?", string(b), aid)
				for _, x := range xids {
					tx.Exec("UPDATE items SET merged_into=? WHERE id=?", aid, x)
				}
			}
			newIDs = append(newIDs, aid)
		}
	}
	return newIDs, tx.Commit()
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Prune drops items past retention (48h) and old seen ids, then reclaims space.
func (s *Store) Prune() (int64, error) {
	cutoff := now() - RetentionHours*3600
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	tx.Exec("DELETE FROM post_links WHERE item_id IN (SELECT id FROM items WHERE published < ?)", cutoff)
	tx.Exec("DELETE FROM tcache WHERE ts < ?", now()-7*86400)
	tx.Exec("DELETE FROM seen WHERE ts < ?", now()-30*86400)
	res, err := tx.Exec("DELETE FROM items WHERE published < ?", cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		tx.Exec("DELETE FROM read_items WHERE item_id NOT IN (SELECT id FROM items)")
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.DB.Exec("PRAGMA incremental_vacuum")
	s.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return n, nil
}

// DeleteSources removes sources with their items; built-ins get a tombstone.
func (s *Store) DeleteSources(ids []int64) (int, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	n := 0
	for _, id := range ids {
		var name, origin string
		if tx.QueryRow("SELECT COALESCE(seed_name,name), COALESCE(origin,'') FROM sources WHERE id=?", id).Scan(&name, &origin) != nil {
			continue
		}
		tx.Exec("DELETE FROM seen WHERE uid IN (SELECT uid FROM items WHERE source_id=?)", id)
		tx.Exec("DELETE FROM post_links WHERE item_id IN (SELECT id FROM items WHERE source_id=?)", id)
		tx.Exec("DELETE FROM read_items WHERE item_id IN (SELECT id FROM items WHERE source_id=?)", id)
		tx.Exec("DELETE FROM items WHERE source_id=?", id)
		tx.Exec("DELETE FROM sources WHERE id=?", id)
		tx.Exec("DELETE FROM source_health WHERE source_id=?", id)
		if origin == "seed" {
			tx.Exec("INSERT OR REPLACE INTO deleted_seeds VALUES(?,?)", name, now())
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if n > 0 {
		s.DB.Exec("PRAGMA incremental_vacuum")
		s.emit(Event{Kind: "sources"})
	}
	return n, nil
}

// ReapplyRule re-evaluates a source's rule over stored items (retroactive hide/unhide). Returns hidden count.
func (s *Store) ReapplyRule(id int64) (int, error) {
	src, err := s.Source(id)
	if err != nil {
		return 0, err
	}
	rows, err := s.DB.Query("SELECT id, COALESCE(title,''), COALESCE(summary,'') FROM items WHERE source_id=?", id)
	if err != nil {
		return 0, err
	}
	var ids []int64
	var texts []string
	for rows.Next() {
		var i int64
		var t, sm string
		rows.Scan(&i, &t, &sm)
		ids = append(ids, i)
		texts = append(texts, RuleText(src.Rule.Scope, t, sm))
	}
	rows.Close()
	keep := src.Rule.Keeps(texts)
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	hidden := 0
	for n, i := range ids {
		h := b2i(!keep[n])
		hidden += h
		tx.Exec("UPDATE items SET hidden=? WHERE id=?", h, i)
	}
	return hidden, tx.Commit()
}

// SetRule validates, saves and retroactively applies a filter rule.
func (s *Store) SetRule(id int64, r Rule) (int, error) {
	r, err := r.Validate()
	if err != nil {
		return 0, err
	}
	if _, err := s.DB.Exec("UPDATE sources SET f_include=?, f_exclude=?, f_scope=?, f_cs=? WHERE id=?", nz(r.Include), nz(r.Exclude), r.Scope, b2i(r.CS), id); err != nil {
		return 0, err
	}
	n, err := s.ReapplyRule(id)
	s.emit(Event{Kind: "sources"})
	return n, err
}

func addOverride(cur, f string) string {
	if strings.Contains(cur, f) {
		return cur
	}
	if cur == "" {
		return f
	}
	return cur + "," + f
}

// RenameSource renames; built-ins remember their seed name so seed sync still matches.
func (s *Store) RenameSource(id int64, name string) error {
	n := cleanName(name)
	if n == "" {
		return errors.New("名称为空")
	}
	var other int64
	if s.DB.QueryRow("SELECT id FROM sources WHERE name=? AND id<>?", n, id).Scan(&other) == nil {
		return errors.New("已有同名数据源")
	}
	_, err := s.DB.Exec("UPDATE sources SET seed_name = CASE WHEN origin='seed' THEN COALESCE(seed_name,name) ELSE seed_name END, name=? WHERE id=?", n, id)
	s.emit(Event{Kind: "sources"})
	return err
}

// UpdateSource changes enabled/priority/section, recording user overrides.
func (s *Store) SetEnabled(id int64, on bool) error {
	return s.setField(id, "enabled", b2i(on))
}
func (s *Store) SetPriority(id int64, p string) error {
	if p != "P0" && p != "P1" && p != "P2" {
		return errors.New("优先级无效")
	}
	src, err := s.Source(id)
	if err != nil {
		return err
	}
	if err := s.setField(id, "priority", p); err != nil {
		return err
	}
	return s.setField(id, "poll_interval", IntervalFor(src.Type, p))
}
func (s *Store) SetSection(id int64, key string) error {
	if !isSectionKey(s.DB, key) {
		return errors.New("分区不存在")
	}
	_, err := s.DB.Exec("UPDATE sources SET section=? WHERE id=?", key, id)
	s.emit(Event{Kind: "sources"})
	return err
}
func (s *Store) setField(id int64, f string, v any) error {
	var ov string
	s.DB.QueryRow("SELECT overrides FROM sources WHERE id=?", id).Scan(&ov)
	_, err := s.DB.Exec("UPDATE sources SET "+f+"=?, overrides=? WHERE id=?", v, addOverride(ov, f), id)
	s.emit(Event{Kind: "sources"})
	return err
}

// AddSource inserts a user source (from Detect or import).
func (s *Store) AddSource(d Detected, section, priority string) (int64, error) {
	if !isSectionKey(s.DB, section) {
		return 0, errors.New("分区不存在")
	}
	if priority == "" {
		priority = "P1"
	}
	name := cleanName(d.Name)
	if name == "" {
		return 0, errors.New("名称为空")
	}
	reg, cat := MetaFor(section)
	res, err := s.DB.Exec(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,priority,poll_interval,origin,created,section)
		VALUES(?,?,?,?,?,?,?,1,1,?,?,'user',?,?)`, name, reg, cat, d.Type, d.URL, nz(d.Homepage), nz(d.Username), priority, IntervalFor(d.Type, priority), now(), section)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, errors.New("已有同名数据源")
		}
		return 0, err
	}
	s.emit(Event{Kind: "sources"})
	return res.LastInsertId()
}

var ctrlName = regexp.MustCompile(`[\x00-\x1f\x7f<>]`)

func cleanName(s string) string {
	return Trunc(strings.TrimSpace(wsRe.ReplaceAllString(ctrlName.ReplaceAllString(s, ""), " ")), 120, "")
}
