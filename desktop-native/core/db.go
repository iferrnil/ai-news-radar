package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite, no cgo
)

const RetentionHours = 48

var Supported = []string{"rss", "api", "x", "x_list"}

type tier struct{ X, Feed int }

var Tiers = map[string]tier{"P0": {120, 300}, "P1": {300, 600}, "P2": {1800, 1800}}

func IntervalFor(typ, priority string) int {
	t, ok := Tiers[priority]
	if !ok {
		t = Tiers["P1"]
	}
	if typ == "x" || typ == "x_list" {
		return t.X
	}
	return t.Feed
}

// Schema is byte-for-byte compatible with src/lib/db.ts (+ health.ts), so the same news.db opens in both.
const schema = `
CREATE TABLE IF NOT EXISTS sources(
  id INTEGER PRIMARY KEY, name TEXT UNIQUE NOT NULL, region TEXT, category TEXT, type TEXT NOT NULL, url TEXT NOT NULL,
  homepage TEXT, username TEXT, enabled INTEGER DEFAULT 1, supported INTEGER DEFAULT 1, unsupported_reason TEXT,
  priority TEXT, poll_interval INTEGER, last_fetched REAL, last_error TEXT, last_new INTEGER DEFAULT 0,
  origin TEXT DEFAULT 'seed', grp TEXT, note TEXT, created REAL);
CREATE TABLE IF NOT EXISTS items(
  id INTEGER PRIMARY KEY, uid TEXT UNIQUE NOT NULL, source_id INTEGER NOT NULL, title TEXT, url TEXT, summary TEXT,
  published REAL NOT NULL, fetched REAL NOT NULL, is_x INTEGER DEFAULT 0, author TEXT, also_on_x TEXT DEFAULT '[]', merged_into INTEGER);
CREATE INDEX IF NOT EXISTS items_pub ON items(published DESC);
CREATE TABLE IF NOT EXISTS post_links(norm_url TEXT NOT NULL, item_id INTEGER NOT NULL, PRIMARY KEY(norm_url, item_id));
CREATE INDEX IF NOT EXISTS post_links_item ON post_links(item_id);
CREATE TABLE IF NOT EXISTS seen(uid TEXT PRIMARY KEY, ts REAL NOT NULL) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS seen_ts ON seen(ts);
CREATE INDEX IF NOT EXISTS items_src_pub ON items(source_id, published DESC);
DROP INDEX IF EXISTS items_src;
CREATE TABLE IF NOT EXISTS tcache(h TEXT PRIMARY KEY, t TEXT, s TEXT, ts REAL NOT NULL) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS deleted_seeds(name TEXT PRIMARY KEY, ts REAL NOT NULL) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS settings(k TEXT PRIMARY KEY, v TEXT) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS read_items(item_id INTEGER PRIMARY KEY, ts REAL NOT NULL);
CREATE TABLE IF NOT EXISTS sections(key TEXT PRIMARY KEY, name TEXT UNIQUE NOT NULL, icon TEXT, in_all INTEGER NOT NULL DEFAULT 1, pos INTEGER NOT NULL DEFAULT 0, created REAL) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS source_health(source_id INTEGER PRIMARY KEY, runs TEXT NOT NULL DEFAULT '', last_ok REAL, last_err TEXT,
  last_err_at REAL, last_kind TEXT, consec INTEGER NOT NULL DEFAULT 0, lat_ms INTEGER, last_ms INTEGER, open_until REAL);
`

// Store wraps the database and everything that hangs off it.
type Store struct {
	DB          *sql.DB
	Dir         string // data dir (secrets.json lives here)
	OnEvent     func(Event)
	Thumbs      *ThumbCache // optional; pruned with retention
	translating *translator
	notifier    *notifier
}

// Event is what the UI listens to (replaces the web app's SSE bus).
type Event struct {
	Kind  string // "items", "sources", "notify", "translate"
	Batch *NotifyBatch
}

func (s *Store) emit(e Event) {
	if s.OnEvent != nil {
		s.OnEvent(e)
	}
}

// Open opens (creating/migrating) dir/news.db. seedJSON may be nil.
func Open(dir string, seedJSON []byte) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "news.db")
	fresh := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fresh = true
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-4000)&_pragma=temp_store(MEMORY)&_pragma=journal_size_limit(4194304)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // one writer; the app is single-user and queries are tiny
	if fresh {
		if _, err := db.Exec("PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
			return nil, err
		}
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	s := &Store{DB: db, Dir: dir}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	if seedJSON != nil {
		if err := s.Seed(seedJSON); err != nil {
			return nil, fmt.Errorf("seed: %w", err)
		}
	}
	s.translating = newTranslator(s)
	s.notifier = newNotifier(s)
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) cols(table string) map[string]bool {
	rows, err := s.DB.Query("SELECT name FROM pragma_table_info(?)", table)
	out := map[string]bool{}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		rows.Scan(&n)
		out[n] = true
	}
	return out
}

func (s *Store) migrate() error {
	ic, sc := s.cols("items"), s.cols("sources")
	var stmts []string
	add := func(have bool, q ...string) {
		if !have {
			stmts = append(stmts, q...)
		}
	}
	add(sc["overrides"], "ALTER TABLE sources ADD COLUMN overrides TEXT NOT NULL DEFAULT ''")
	add(ic["image"], "ALTER TABLE items ADD COLUMN image TEXT")
	add(sc["f_include"], "ALTER TABLE sources ADD COLUMN f_include TEXT", "ALTER TABLE sources ADD COLUMN f_exclude TEXT",
		"ALTER TABLE sources ADD COLUMN f_scope TEXT", "ALTER TABLE sources ADD COLUMN f_cs INTEGER NOT NULL DEFAULT 0")
	add(sc["seed_name"], "ALTER TABLE sources ADD COLUMN seed_name TEXT")
	add(sc["section"], "ALTER TABLE sources ADD COLUMN section TEXT")
	add(ic["hidden"], "ALTER TABLE items ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0")
	add(ic["title_zh"], "ALTER TABLE items ADD COLUMN title_zh TEXT", "ALTER TABLE items ADD COLUMN summary_zh TEXT")
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return fmt.Errorf("%s: %w", q, err)
		}
	}
	return s.migrateSections()
}

type seedSource struct {
	Name, Region, Category, Type, URL, Homepage, Username, Priority, Group, Status, Note string
	Enabled                                                                              *bool
	PollInterval                                                                         int `json:"pollInterval"`
}

// Seed syncs sources.json like db.ts seed(): tombstones respected, user overrides win, new rows get a section.
func (s *Store) Seed(data []byte) error {
	var d struct{ Sources []seedSource }
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	tomb := map[string]bool{}
	for _, n := range s.strings("SELECT name FROM deleted_seeds") {
		tomb[n] = true
	}
	have := map[string]int64{}
	rows, err := s.DB.Query("SELECT id, COALESCE(seed_name, name) FROM sources")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var n string
		rows.Scan(&id, &n)
		have[n] = id
	}
	rows.Close()
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, x := range d.Sources {
		if tomb[x.Name] {
			continue
		}
		st := x.Status
		if st == "" {
			st = "works"
		}
		sup := slices.Contains(Supported, x.Type) && st == "works"
		var reason any
		if !sup {
			if x.Type == "html" {
				reason = "需要爬取（HTML，无 feed）"
			} else {
				reason = "状态 " + st + "（被拦截/不可用）"
			}
		}
		en := 1
		if x.Enabled != nil && !*x.Enabled {
			en = 0
		}
		iv := x.PollInterval
		if iv == 0 {
			iv = IntervalFor(x.Type, x.Priority)
		}
		if id, ok := have[x.Name]; ok {
			_, err = tx.Exec(`UPDATE sources SET
				enabled = CASE WHEN instr(overrides,'enabled') THEN enabled ELSE ? END,
				priority = CASE WHEN instr(overrides,'priority') THEN priority ELSE ? END,
				poll_interval = CASE WHEN instr(overrides,'poll_interval') OR instr(overrides,'priority') THEN poll_interval ELSE ? END,
				url=?, username=?, grp = CASE WHEN instr(overrides,'grp') THEN grp ELSE ? END
				WHERE id=? AND origin='seed'`, en, nz(x.Priority), iv, x.URL, nz(x.Username), nz(x.Group), id)
		} else {
			sec := s.seedTargetTx(tx, LegacyKey(x.Group, x.Region, x.Category, x.Type))
			_, err = tx.Exec(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,unsupported_reason,priority,poll_interval,origin,grp,note,created,section)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,'seed',?,?,?,?)`, x.Name, nz(x.Region), nz(x.Category), x.Type, x.URL, nz(x.Homepage), nz(x.Username),
				en, b2i(sup), reason, nz(x.Priority), iv, nz(x.Group), nz(x.Note), now(), sec)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// nz maps "" to SQL NULL.
func nz(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) strings(q string, args ...any) []string {
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v sql.NullString
		rows.Scan(&v)
		out = append(out, v.String)
	}
	return out
}

func (s *Store) GetSetting(k string) (string, bool) {
	var v sql.NullString
	err := s.DB.QueryRow("SELECT v FROM settings WHERE k=?", k).Scan(&v)
	return v.String, err == nil
}
func (s *Store) SetSetting(k, v string) error {
	_, err := s.DB.Exec("INSERT INTO settings(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v", k, v)
	return err
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }
