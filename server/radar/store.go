package radar

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

const itemSQL = `SELECT i.id,i.title,i.url,i.summary,i.published,i.fetched,i.is_x,i.author,i.also_on_x,i.image,i.title_zh,i.summary_zh,
  s.id AS source_id, s.name AS source, s.category, s.region, s.type AS source_type, s.priority, s.homepage, s.username, s.grp, s.section,
  i.kw, fv.ts AS fav
  FROM items i JOIN sources s ON s.id=i.source_id LEFT JOIN favorites fv ON fv.item_id=i.id WHERE i.merged_into IS NULL AND i.hidden=0 AND s.enabled=1`

// PreviewChars: cards clamp to 3 lines; hover card / drawer fetch the full text via ?id=.
const PreviewChars = 280

// QueryItems mirrors store.ts queryItems. full: untruncated summary. zh: include title_zh/summary_zh.
func (s *Store) QueryItems(where string, args []any, limit int, full, zh bool) []*Row {
	limit = min(max(1, limit), 1000)
	rs, err := all(s.DB, fmt.Sprintf("%s %s ORDER BY i.published DESC LIMIT %d", itemSQL, where, limit), args...)
	if err != nil {
		return []*Row{}
	}
	clip := func(x string) string {
		if full || jsLen(x) <= PreviewChars {
			return x
		}
		return jsSlice(x, PreviewChars)
	}
	for _, r := range rs {
		tzh, szh := r.Get("title_zh"), r.Str("summary_zh")
		r.Del("fetched")
		r.Del("title_zh")
		r.Del("summary_zh")
		summary := Decode(r.Str("summary"))
		cut := clip(summary)
		r.Set("title", Decode(r.Str("title")))
		r.Set("summary", cut)
		var also any = json.RawMessage("[]")
		if a := r.Str("also_on_x"); a != "" {
			var v any
			if json.Unmarshal([]byte(a), &v) == nil {
				b, _ := marshalNoEscape(v)
				also = json.RawMessage(b)
			}
		}
		r.Set("also_on_x", also) // in place (spread keeps key order)
		r.Set("truncated", jsLen(cut) < jsLen(summary))
		if kw := kwList(r.Str("kw")); kw != nil {
			r.Set("kw", kw)
		} else {
			r.Del("kw")
		}
		if r.Has("fav") {
			r.Set("fav", true)
		}
		if t, ok := tzh.(string); zh && ok && t != "" {
			r.Set("title_zh", t)
			r.Set("summary_zh", clip(szh))
			if !full && clip(szh) != "" {
				r.Set("summary", "")
				r.Set("truncated", true)
			}
		}
	}
	return rs
}

func idsWhere(ids []int64) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return "AND i.id IN (" + placeholders(len(ids)) + ")", args
}

var xHost = regexp.MustCompile(`^(x\.com|twitter\.com|t\.co)/`)

// StoreItems mirrors store.ts storeItems: dedupe via `seen`, source rule, retention, tweet↔article merge; publishes events.
func (s *Store) StoreItems(src *Row, items []FetchedItem) (newIDs []int64, merged int, fresh []*Row, err error) {
	items = safeItems(items) // http(s) links only
	t := now()
	cutoff := t - retentionHours()*3600
	firstFetch := !src.Has("last_fetched")
	rule := ruleOf(src)
	var keep []bool
	if rule.Has() {
		texts := make([]string, len(items))
		for i, it := range items {
			texts[i] = RuleText(rule.Scope, it.Title, it.Summary)
		}
		keep = EvalRule(rule, texts, 250*time.Millisecond)
	}
	updated := map[int64]bool{}
	var updOrder []int64
	var removed []int64
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, 0, nil, err
	}
	defer tx.Rollback()
	srcID := src.Get("id")
	ins := `INSERT OR IGNORE INTO items(uid,source_id,title,url,summary,published,fetched,is_x,author,merged_into,image,content) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`
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
			var mergedInto any
			if artID != 0 {
				mergedInto = artID
			}
			res, err := tx.Exec(ins, it.UID, srcID, it.Title, it.URL, it.Summary, pub, t, 1, nz(it.Author), mergedInto, nz(it.Image), nz(it.Content))
			if err != nil {
				return nil, 0, nil, err
			}
			if c, _ := res.RowsAffected(); c == 0 {
				continue
			}
			xid, _ := res.LastInsertId()
			for _, l := range links {
				tx.Exec("INSERT OR IGNORE INTO post_links VALUES(?,?)", l, xid)
			}
			if artID != 0 {
				var also []any
				if json.Unmarshal([]byte(artAlso), &also) != nil {
					also = nil
				}
				also = append(also, alsoX{it.URL, nz(it.Author)})
				b, _ := marshalNoEscape(also)
				tx.Exec("UPDATE items SET also_on_x=? WHERE id=?", string(b), artID)
				if !updated[artID] {
					updated[artID] = true
					updOrder = append(updOrder, artID)
				}
			} else {
				newIDs = append(newIDs, xid)
			}
		} else {
			res, err := tx.Exec(ins, it.UID, srcID, it.Title, it.URL, it.Summary, pub, t, 0, nz(it.Author), nil, nz(it.Image), nz(it.Content))
			if err != nil {
				return nil, 0, nil, err
			}
			if c, _ := res.RowsAffected(); c == 0 {
				continue
			}
			aid, _ := res.LastInsertId()
			xs, _ := all(tx, "SELECT i.id, i.url, i.author FROM post_links p JOIN items i ON i.id=p.item_id WHERE p.norm_url=? AND i.merged_into IS NULL", it.UID)
			if len(xs) > 0 {
				also := make([]alsoX, len(xs))
				for i, x := range xs {
					also[i] = alsoX{x.Get("url"), x.Get("author")}
				}
				b, _ := marshalNoEscape(also)
				tx.Exec("UPDATE items SET also_on_x=? WHERE id=?", string(b), aid)
				for _, x := range xs {
					tx.Exec("UPDATE items SET merged_into=? WHERE id=?", aid, x.Get("id"))
					removed = append(removed, int64(x.Num("id")))
				}
			}
			newIDs = append(newIDs, aid)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, nil, err
	}
	s.tr.enqueue(newIDs)
	s.applyWatch(newIDs)
	for _, id := range removed {
		s.Bus.Publish(Msg{Event: "removed", ID: id})
	}
	if len(newIDs) > 0 {
		w, a := idsWhere(newIDs)
		fresh = s.QueryItems(w, a, len(newIDs), false, false)
	}
	s.publishFreshItems(fresh)
	for _, id := range updOrder {
		for _, r := range s.QueryItems("AND i.id=?", []any{id}, 1, false, false) {
			s.Bus.Publish(Msg{Event: "update", Item: r})
		}
	}
	return newIDs, len(updated), fresh, nil
}

// Prune drops items past retention and old seen ids, then gives the space back (incremental vacuum + WAL truncate).
func (s *Store) Prune() int64 {
	cutoff := now() - retentionHours()*3600
	tx, err := s.DB.Begin()
	if err != nil {
		return 0
	}
	defer tx.Rollback()
	// favorited items (选题收藏夹) are kept past the retention window
	tx.Exec("DELETE FROM post_links WHERE item_id IN (SELECT id FROM items WHERE published < ? AND id NOT IN (SELECT item_id FROM favorites))", cutoff)
	tx.Exec("DELETE FROM tcache WHERE ts < ?", now()-7*86400)
	tx.Exec("DELETE FROM seen WHERE ts < ?", now()-envNum("SEEN_DAYS", 30)*86400)
	res, err := tx.Exec("DELETE FROM items WHERE published < ? AND id NOT IN (SELECT item_id FROM favorites)", cutoff)
	if err != nil {
		return 0
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		tx.Exec("DELETE FROM read_items WHERE item_id NOT IN (SELECT id FROM items)")
	}
	if tx.Commit() != nil {
		return 0
	}
	s.DB.Exec("PRAGMA incremental_vacuum")
	s.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return n
}

// DeleteSources removes sources (any origin) with items, links, seen ids; built-ins get a tombstone.
func (s *Store) DeleteSources(ids []int64) (deleted, items int64) {
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, 0
	}
	defer tx.Rollback()
	for _, id := range ids {
		var name, origin string
		if tx.QueryRow("SELECT COALESCE(seed_name, name), COALESCE(origin,'') FROM sources WHERE id=?", id).Scan(&name, &origin) != nil {
			continue
		}
		tx.Exec("DELETE FROM seen WHERE uid IN (SELECT uid FROM items WHERE source_id=?)", id)
		tx.Exec("DELETE FROM post_links WHERE item_id IN (SELECT id FROM items WHERE source_id=?)", id)
		tx.Exec("DELETE FROM favorites WHERE item_id IN (SELECT id FROM items WHERE source_id=?)", id)
		res, _ := tx.Exec("DELETE FROM items WHERE source_id=?", id)
		if res != nil {
			n, _ := res.RowsAffected()
			items += n
		}
		tx.Exec("DELETE FROM sources WHERE id=?", id)
		tx.Exec("DELETE FROM source_health WHERE source_id=?", id)
		if origin == "seed" {
			tx.Exec("INSERT OR REPLACE INTO deleted_seeds VALUES(?,?)", name, now())
		}
		deleted++
	}
	if tx.Commit() != nil {
		return 0, 0
	}
	if deleted > 0 {
		s.DB.Exec("PRAGMA incremental_vacuum")
		s.Bus.Publish(Msg{Event: "sources"})
	}
	return deleted, items
}

// ReapplyRule re-evaluates a source's rule over its stored items (retroactive hide/unhide). Returns hidden count.
func (s *Store) ReapplyRule(id any) int {
	src, _ := one(s.DB, "SELECT f_include, f_exclude, f_scope, f_cs FROM sources WHERE id=?", id)
	if src == nil {
		return 0
	}
	rule := ruleOf(src)
	rows, _ := all(s.DB, "SELECT id, title, summary FROM items WHERE source_id=?", id)
	texts := make([]string, len(rows))
	for i, r := range rows {
		texts[i] = RuleText(rule.Scope, r.Str("title"), r.Str("summary"))
	}
	keep := EvalRule(rule, texts, time.Second)
	tx, err := s.DB.Begin()
	if err != nil {
		return 0
	}
	hidden := 0
	for n, r := range rows {
		h := 0
		if keep != nil && !keep[n] {
			h = 1
		}
		hidden += h
		tx.Exec("UPDATE items SET hidden=? WHERE id=?", h, r.Get("id"))
	}
	tx.Commit()
	return hidden
}
