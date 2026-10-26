package radar

import (
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestClampPollInterval(t *testing.T) {
	for n, want := range map[float64]int{0: 0, -1: 0, 30: 60, 60: 60, 90.4: 90, 86400: 86400, 90000: 86400} {
		if got := clampPollInterval(n); got != want {
			t.Fatalf("clampPollInterval(%v)=%d want %d", n, got, want)
		}
	}
}

func TestSourcesPostPollInterval(t *testing.T) {
	st := testStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	iv := clampPollInterval(1800)
	if iv != 1800 {
		t.Fatalf("clamp 1800 -> %d", iv)
	}
	res, err := st.DB.Exec(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,priority,poll_interval,origin,created,section,overrides)
		VALUES('Poll IV Source','global','media','rss','https://example.com/feed.xml','https://example.com/',NULL,1,1,'P1',?,'user',?,?,?)`,
		iv, now(), "media", "poll_interval")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	var out map[string]any
	if c := postJSON(t, srv.URL+fmt.Sprintf("/api/sources/%d", id), "PATCH", map[string]any{"poll_interval": 3600}, &out); c != 200 {
		t.Fatalf("PATCH %d %v", c, out)
	}
	var got int
	st.DB.QueryRow("SELECT poll_interval FROM sources WHERE id=?", id).Scan(&got)
	if got != 3600 {
		t.Fatalf("after PATCH poll_interval=%d want 3600", got)
	}
	// too-small PATCH clamps to 60
	if c := postJSON(t, srv.URL+fmt.Sprintf("/api/sources/%d", id), "PATCH", map[string]any{"poll_interval": 10}, &out); c != 200 {
		t.Fatalf("PATCH clamp %d %v", c, out)
	}
	st.DB.QueryRow("SELECT poll_interval FROM sources WHERE id=?", id).Scan(&got)
	if got != 60 {
		t.Fatalf("after PATCH 10 poll_interval=%d want 60", got)
	}
	st.DB.Exec("UPDATE sources SET last_fetched=?, poll_interval=3600 WHERE id=?", now()-3601, id)
	due, _ := all(st.DB, `SELECT id FROM sources s LEFT JOIN source_health h ON h.source_id=s.id
		WHERE s.enabled=1 AND s.supported=1 AND (s.last_fetched IS NULL OR s.last_fetched + s.poll_interval <= ?)
		AND (h.open_until IS NULL OR h.open_until <= ?) AND s.id=?`, now(), now(), id)
	if len(due) != 1 {
		t.Fatalf("scheduler should pick custom interval source, got %d", len(due))
	}
	st.DB.Exec("UPDATE sources SET last_fetched=? WHERE id=?", now()-10, id)
	due, _ = all(st.DB, `SELECT id FROM sources s LEFT JOIN source_health h ON h.source_id=s.id
		WHERE s.enabled=1 AND s.supported=1 AND (s.last_fetched IS NULL OR s.last_fetched + s.poll_interval <= ?)
		AND (h.open_until IS NULL OR h.open_until <= ?) AND s.id=?`, now(), now(), id)
	if len(due) != 0 {
		t.Fatalf("scheduler should wait for interval, got %d", len(due))
	}
}
