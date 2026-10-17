package radar

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	_ "modernc.org/sqlite" // pure-Go SQLite, no cgo
)

var Supported = []string{"rss", "api", "x", "x_list"}

type tier struct{ X, Feed int }

func tiers() map[string]tier {
	return map[string]tier{
		"P0": {envInt("P0_X", 120), envInt("P0_FEED", 300)},
		"P1": {envInt("P1_X", 300), envInt("P1_FEED", 600)},
		"P2": {envInt("P2_X", 1800), envInt("P2_FEED", 1800)},
	}
}

// IntervalFor mirrors db.ts intervalFor.
func IntervalFor(typ, priority string) int {
	ts := tiers()
	t, ok := ts[priority]
	if !ok {
		t = ts["P1"]
	}
	if typ == "x" || typ == "x_list" {
		return t.X
	}
	return t.Feed
}

// Schema kept identical to the one the former TypeScript build created, so older news.db files open unchanged.
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
CREATE INDEX IF NOT EXISTS items_src ON items(source_id);
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
CREATE TABLE IF NOT EXISTS favorites(item_id INTEGER PRIMARY KEY, ts REAL NOT NULL);
CREATE TABLE IF NOT EXISTS source_health(source_id INTEGER PRIMARY KEY, runs TEXT NOT NULL DEFAULT '', last_ok REAL, last_err TEXT,
  last_err_at REAL, last_kind TEXT, consec INTEGER NOT NULL DEFAULT 0, lat_ms INTEGER, last_ms INTEGER, open_until REAL);
`

// Store is the whole backend: database, event bus, scheduler, notifier, translator, secrets.
type Store struct {
	DB       *sql.DB
	Cfg      Config
	Bus      *Bus
	sched    *scheduler
	notif    *notifier
	tr       *translator
	prov     *provSched // 抓取 API scheduler state (fetchapi.go)
	secMu    sync.Mutex
	notFound http.Handler
	sec      *Stored
}

// Open opens (creating / migrating) the database and syncs the seed file, like getDb() in db.ts.
func Open(cfg Config) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, err
	}
	dsn := "file:" + cfg.DBPath + "?_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-4000)&_pragma=temp_store(MEMORY)&_pragma=journal_size_limit(4194304)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	var av int
	db.QueryRow("PRAGMA auto_vacuum").Scan(&av)
	if av != 2 { // convert once (must precede WAL on existing DBs)
		if _, err := db.Exec("PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
			return nil, err
		}
		if _, err := db.Exec("VACUUM"); err != nil {
			return nil, fmt.Errorf("vacuum: %w", err)
		}
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	s := &Store{DB: db, Cfg: cfg, Bus: NewBus()}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	if err := s.Seed(); err != nil {
		log.Printf("[db] seed: %v", err)
	}
	s.repairEntities()
	s.applyWatch(nil) // items not matched yet (new column, or stored while the list was being changed)
	s.sched = newScheduler(s)
	s.notif = newNotifier(s)
	s.tr = newTranslator(s)
	s.prov = newProvSched()
	s.useDiskCache()
	return s, nil
}

// Close checkpoints the WAL and closes the database.
func (s *Store) Close() error {
	s.dropDiskCache()
	s.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return s.DB.Close()
}

func (s *Store) cols(table string) map[string]bool {
	out := map[string]bool{}
	rs, _ := all(s.DB, "SELECT name FROM pragma_table_info(?)", table)
	for _, r := range rs {
		out[r.Str("name")] = true
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
	add(ic["content"], "ALTER TABLE items ADD COLUMN content TEXT") // full feed / post text when longer than summary (briefs)
	add(ic["kw"], "ALTER TABLE items ADD COLUMN kw TEXT")           // matched watch keywords ("\n"-joined; '' = none; NULL = not checked), watch.go
	// translation scope flags (see translate.go): sources.tr / sections.tr = 1 when the user switched translation on for that view,
	// settings tr_all = 「全部」. Replaces the per-source inherit/on/off `translate` column (only explicit "on" carries over).
	add(sc["tr"], "ALTER TABLE sources ADD COLUMN tr INTEGER NOT NULL DEFAULT 0")
	if !sc["tr"] && sc["translate"] {
		stmts = append(stmts, "UPDATE sources SET tr=1 WHERE translate=1")
	}
	if sc["translate"] {
		stmts = append(stmts, "ALTER TABLE sources DROP COLUMN translate")
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return fmt.Errorf("%s: %w", q, err)
		}
	}
	if _, ok := s.GetSetting("tr_all"); !ok {
		// the old global 抓取时翻译 switch: only an explicit "on" becomes 「全部」 (default is off now)
		old, _ := s.GetSetting("translate")
		v := "0"
		if old == "1" {
			v = "1"
		}
		s.SetSetting("tr_all", v)
		s.DB.Exec("DELETE FROM settings WHERE k='translate'")
	}
	return s.migrateSections()
}

// repairEntities: one-off fix for entities double-escaped in older rows (e.g. "&mdash;").
func (s *Store) repairEntities() {
	rs, _ := all(s.DB, "SELECT id,title,summary FROM items WHERE title LIKE '%&%;%' OR summary LIKE '%&%;%'")
	if len(rs) == 0 {
		return
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return
	}
	for _, r := range rs {
		tx.Exec("UPDATE items SET title=?, summary=? WHERE id=?", decodeJS(r.Str("title")), decodeJS(r.Str("summary")), r.Get("id"))
	}
	tx.Commit()
}

type seedSource struct {
	Name, Type, URL, Status                                     string
	Region, Category, Homepage, Username, Priority, Group, Note *string // nil = absent (`?? null`)
	Enabled                                                     *bool
	PollInterval                                                int `json:"pollInterval"`
}

func sp(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
func sv(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Seed syncs sources.json like db.ts seed(): tombstones respected, user overrides win, new rows get a section.
func (s *Store) Seed() error {
	data := s.Cfg.SeedData
	if data == nil {
		var err error
		if data, err = os.ReadFile(s.Cfg.SeedPath); err != nil {
			return nil // no seed file: nothing to do
		}
	}
	var d struct{ Sources []seedSource }
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	tomb := map[string]bool{}
	rs, _ := all(s.DB, "SELECT name FROM deleted_seeds")
	for _, r := range rs {
		tomb[r.Str("name")] = true
	}
	have := map[string]any{}
	rs, _ = all(s.DB, "SELECT id, COALESCE(seed_name, name) name FROM sources")
	for _, r := range rs {
		have[r.Str("name")] = r.Get("id")
	}
	sync := os.Getenv("SYNC_SEED") != "0"
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
				reason = "状态 " + x.Status + "（被拦截/不可用）"
			}
		}
		en := 1
		if x.Enabled != nil && !*x.Enabled {
			en = 0
		}
		iv := x.PollInterval
		if iv == 0 {
			iv = IntervalFor(x.Type, sv(x.Priority))
		}
		if id, ok := have[x.Name]; ok {
			if !sync {
				continue
			}
			_, err = tx.Exec(`UPDATE sources SET
				enabled = CASE WHEN instr(overrides,'enabled') THEN enabled ELSE ? END,
				priority = CASE WHEN instr(overrides,'priority') THEN priority ELSE ? END,
				poll_interval = CASE WHEN instr(overrides,'poll_interval') OR instr(overrides,'priority') THEN poll_interval ELSE ? END,
				url=?, username=?, grp = CASE WHEN instr(overrides,'grp') THEN grp ELSE ? END
				WHERE id=? AND origin='seed'`, en, sp(x.Priority), iv, x.URL, sp(x.Username), sp(x.Group), id)
		} else {
			sec := seedTarget(tx, LegacyKey(sv(x.Group), sv(x.Region), sv(x.Category), x.Type))
			_, err = tx.Exec(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,unsupported_reason,priority,poll_interval,origin,grp,note,created,section)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,'seed',?,?,?,?)`, x.Name, sp(x.Region), sp(x.Category), x.Type, x.URL, sp(x.Homepage), sp(x.Username),
				en, b2i(sup), reason, sp(x.Priority), iv, sp(x.Group), sp(x.Note), now(), sec)
		}
		if err != nil {
			return err
		}
	}
	if os.Getenv("SYNC_DISABLE_USER") == "1" {
		tx.Exec("UPDATE sources SET enabled=0 WHERE origin='user'")
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

func (s *Store) GetSetting(k string) (string, bool) {
	var v sql.NullString
	err := s.DB.QueryRow("SELECT v FROM settings WHERE k=?", k).Scan(&v)
	return v.String, err == nil && v.Valid
}
func (s *Store) SetSetting(k, v string) error {
	_, err := s.DB.Exec("INSERT INTO settings(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v", k, v)
	return err
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

// VacuumInto writes a compact copy of the database (used by the e2e fixture script).
func (s *Store) VacuumInto(path string) error {
	_, err := s.DB.Exec("VACUUM INTO ?", path)
	return err
}
