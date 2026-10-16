package radar

import (
	"net/http"
	"strconv"
)

// hTranslateProgress: GET /api/translate/progress?section=<key>|source=<id> (neither = 全部).
// Counts the retained items of the view by translation state and reports the translator queue, so the top-right
// popover can show "待翻译 / 进行中 / 已完成 / 失败" for the current page (refreshed on the SSE "translate" event).
func (s *Store) hTranslateProgress(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	where, args := "", []any{}
	if v := q.Get("source"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		where, args = " AND s.id=?", []any{id}
	} else if v := q.Get("section"); v != "" && v != "all" {
		where, args = " AND s.section=?", []any{v}
	}
	base := `FROM items i JOIN sources s ON s.id=i.source_id WHERE i.merged_into IS NULL AND i.hidden=0 AND s.enabled=1` + where
	var total, translated, pending, skipped int64
	s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN COALESCE(i.title_zh,'')<>'' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN i.title_zh IS NULL THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN i.title_zh='' THEN 1 ELSE 0 END),0) `+base, args...).
		Scan(&total, &translated, &pending, &skipped)
	// which of the pending ones are in the translator's queue right now
	ids, _ := all(s.DB, "SELECT i.id "+base+" AND i.title_zh IS NULL", args...)
	t := s.tr
	t.mu.Lock()
	inQueue := 0
	for _, r := range ids {
		if t.queued[int64(r.Num("id"))] {
			inQueue++
		}
	}
	out := &Row{}
	out.Set("total", total)
	out.Set("translated", translated)
	out.Set("pending", pending)
	out.Set("skipped", skipped)
	out.Set("inQueue", inQueue)
	out.Set("queued", len(t.queue))
	out.Set("running", t.running)
	out.Set("done", t.done)
	out.Set("failed", t.failed)
	out.Set("lastError", t.lastError)
	t.mu.Unlock()
	reason := s.unavailableReason()
	out.Set("available", reason == "")
	if reason == "" {
		out.Set("reason", nil)
	} else {
		out.Set("reason", reason)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, out)
}
