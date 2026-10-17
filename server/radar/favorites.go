package radar

import (
	"net/http"
	"strings"
)

// 选题收藏夹: favorites(item_id, ts). Favorited items are exempt from the retention prune (Prune), show in the
// 收藏 view (/api/items?fav=1, any age) and travel with the subscriptions JSON export (`favorites`), re-created on
// import (matched to a local source by name / X username / URL; the item row is re-inserted when it was pruned).

const favoritesSchema = `CREATE TABLE IF NOT EXISTS favorites(item_id INTEGER PRIMARY KEY, ts REAL NOT NULL);`

// favMax caps the favorites list (they are kept forever, so the cap keeps the DB small).
const favMax = 2000

// POST /api/favorites {id, on}
func (s *Store) hFavoritePost(w http.ResponseWriter, r *http.Request) {
	b := readBody(r)
	idf, ok := numOf(b["id"])
	on, okOn := b["on"].(bool)
	if !ok || !okOn || idf <= 0 {
		detail(w, 400, "需要 {id, on}")
		return
	}
	id := int64(idf)
	if on {
		var n int
		s.DB.QueryRow("SELECT COUNT(*) FROM favorites").Scan(&n)
		if n >= favMax {
			detail(w, 400, "收藏已达上限")
			return
		}
		res, _ := s.DB.Exec("INSERT INTO favorites(item_id, ts) SELECT id, ? FROM items WHERE id=? ON CONFLICT(item_id) DO NOTHING", now(), id)
		if res != nil {
			if c, _ := res.RowsAffected(); c == 0 {
				var one int
				if s.DB.QueryRow("SELECT 1 FROM favorites WHERE item_id=?", id).Scan(&one) != nil {
					detail(w, 404, "条目不存在")
					return
				}
			}
		}
	} else {
		s.DB.Exec("DELETE FROM favorites WHERE item_id=?", id)
	}
	s.Bus.Publish(Msg{Event: "fav", Data: obj("id", id, "on", on)})
	writeJSON(w, 200, obj("ok", true, "id", id, "on", on))
}

// GET /api/favorites/export: the favorites as portable rows (newest favorite first).
func (s *Store) hFavoritesExport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, Rows{List: s.favoriteRows(), DropNulls: true})
}

func (s *Store) favoriteRows() []*Row {
	rs, err := all(s.DB, `SELECT i.uid, i.title, i.url, i.summary, i.published, i.is_x, i.author, i.image, f.ts AS favorited,
		s.name AS source, s.type AS source_type, s.username, s.url AS source_url
		FROM favorites f JOIN items i ON i.id=f.item_id JOIN sources s ON s.id=i.source_id ORDER BY f.ts DESC LIMIT ?`, favMax)
	if err != nil {
		return []*Row{}
	}
	for _, x := range rs {
		x.Set("title", Decode(x.Str("title")))
		x.Set("summary", jsSlice(Decode(x.Str("summary")), 2000))
		x.Set("is_x", x.Num("is_x") == 1)
	}
	return rs
}

// POST /api/favorites/import {favorites: [...]} → {added, existing, skipped}
func (s *Store) hFavoritesImport(w http.ResponseWriter, r *http.Request) {
	if SettingsLocked() { // re-creates item rows: a data write like the subscriptions import
		detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1）")
		return
	}
	b := readBody(r)
	list, ok := b["favorites"].([]any)
	if !ok || len(list) > favMax {
		detail(w, 400, "请求格式错误")
		return
	}
	added, existing, skipped := s.importFavorites(list)
	if added+existing > 0 {
		s.Bus.Publish(Msg{Event: "fav", Data: obj("import", added+existing)})
	}
	writeJSON(w, 200, obj("added", added, "existing", existing, "skipped", skipped))
}

func (s *Store) importFavorites(list []any) (added, existing, skipped int) {
	local, _ := all(s.DB, "SELECT id, name, type, username, url FROM sources")
	byName, byUser, byURL := map[string]int64{}, map[string]int64{}, map[string]int64{}
	for _, x := range local {
		id := int64(x.Num("id"))
		byName[strings.ToLower(x.Str("name"))] = id
		if u := x.Str("username"); u != "" {
			byUser[strings.ToLower(u)] = id
		}
		byURL[NormURL(x.Str("url"))] = id
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, 0, len(list)
	}
	defer tx.Rollback()
	t := now()
	for _, raw := range list {
		m, ok := raw.(map[string]any)
		if !ok {
			skipped++
			continue
		}
		uid, title, url := strings.TrimSpace(str(m["uid"])), cleanName(str(m["title"])), strings.TrimSpace(str(m["url"]))
		if uid == "" || len(uid) > 2000 || title == "" {
			skipped++
			continue
		}
		if _, bad := httpURLCheck(url); bad != "" {
			skipped++
			continue
		}
		var sid int64
		if id, ok := byName[strings.ToLower(str(m["source"]))]; ok {
			sid = id
		} else if id, ok := byUser[strings.ToLower(strings.TrimPrefix(str(m["username"]), "@"))]; ok && str(m["username"]) != "" {
			sid = id
		} else if id, ok := byURL[NormURL(str(m["source_url"]))]; ok && str(m["source_url"]) != "" {
			sid = id
		}
		if sid == 0 {
			skipped++
			continue
		}
		pub, _ := numOf(m["published"])
		if pub <= 0 || pub > t {
			pub = t
		}
		isX := 0
		if m["is_x"] == true {
			isX = 1
		}
		var img any
		if u := str(m["image"]); strings.HasPrefix(u, "https://") && len(u) < 2000 {
			img = u
		}
		tx.Exec(`INSERT OR IGNORE INTO items(uid,source_id,title,url,summary,published,fetched,is_x,author,image) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			uid, sid, title, url, jsSlice(str(m["summary"]), 4000), pub, t, isX, nz(jsSlice(str(m["author"]), 120)), img)
		tx.Exec("INSERT OR IGNORE INTO seen VALUES(?,?)", uid, t)
		var id int64
		if tx.QueryRow("SELECT id FROM items WHERE uid=?", uid).Scan(&id) != nil {
			skipped++
			continue
		}
		fts, _ := numOf(m["favorited"])
		if fts <= 0 {
			fts = t
		}
		res, _ := tx.Exec("INSERT OR IGNORE INTO favorites(item_id, ts) VALUES(?,?)", id, fts)
		if c, _ := res.RowsAffected(); c > 0 {
			added++
		} else {
			existing++
		}
	}
	if tx.Commit() != nil {
		return 0, 0, len(list)
	}
	return
}
