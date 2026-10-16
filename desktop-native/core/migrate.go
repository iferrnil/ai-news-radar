package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MigrateResult describes a one-time import from the webview build.
type MigrateResult struct {
	Done    bool   // something was imported now
	From    string // old data dir
	Sources int
	Items   int
	Read    int
	Secrets bool
	Skipped string // why nothing happened
}

const migratedMarker = ".migrated"

// MigrateFromWebview copies the webview app's data (oldDir/news.db + secrets.json) into newDir once:
// only when newDir has no news.db yet and no marker. The old files are opened read-only and never modified.
// The copy is a consistent snapshot (VACUUM INTO), so sources, sections, filter rules, tombstones, settings,
// items within retention, seen ids and read state all come across with their ids intact.
func MigrateFromWebview(oldDir, newDir string) (MigrateResult, error) {
	res := MigrateResult{From: oldDir}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return res, err
	}
	marker := filepath.Join(newDir, migratedMarker)
	newDB := filepath.Join(newDir, "news.db")
	oldDB := filepath.Join(oldDir, "news.db")
	if _, err := os.Stat(marker); err == nil {
		res.Skipped = "已迁移过"
		return res, nil
	}
	if _, err := os.Stat(newDB); err == nil {
		res.Skipped = "已有本地数据库"
		os.WriteFile(marker, []byte("existing db\n"), 0o644)
		return res, nil
	}
	if _, err := os.Stat(oldDB); err != nil {
		res.Skipped = "未发现旧版数据"
		return res, nil
	}
	if filepath.Clean(oldDB) == filepath.Clean(newDB) {
		return res, errors.New("新旧路径相同")
	}
	// read-only + immutable would skip the WAL; plain mode=ro still reads committed WAL frames.
	src, err := sql.Open("sqlite", "file:"+oldDB+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return res, err
	}
	defer src.Close()
	tmp := newDB + ".migrating"
	os.Remove(tmp)
	if _, err := src.Exec("VACUUM INTO ?", tmp); err != nil {
		os.Remove(tmp)
		return res, fmt.Errorf("复制旧数据库失败：%w", err)
	}
	src.Close()
	if err := os.Rename(tmp, newDB); err != nil {
		return res, err
	}
	if b, err := os.ReadFile(filepath.Join(oldDir, "secrets.json")); err == nil {
		if os.WriteFile(filepath.Join(newDir, "secrets.json"), b, 0o600) == nil {
			res.Secrets = true
		}
	}
	db, err := sql.Open("sqlite", "file:"+newDB)
	if err == nil {
		db.QueryRow("SELECT COUNT(*) FROM sources").Scan(&res.Sources)
		db.QueryRow("SELECT COUNT(*) FROM items").Scan(&res.Items)
		db.QueryRow("SELECT COUNT(*) FROM read_items").Scan(&res.Read)
		db.Exec("INSERT OR REPLACE INTO settings(k,v) VALUES('migrated_from', ?)", oldDB+" @ "+time.Now().Format(time.RFC3339))
		db.Close()
	}
	res.Done = true
	return res, os.WriteFile(marker, []byte("from "+oldDB+"\n"), 0o644)
}
