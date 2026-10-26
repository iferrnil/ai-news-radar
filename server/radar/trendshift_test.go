package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestTrendshiftKind(t *testing.T) {
	for in, want := range map[string]string{"https://trendshift.io/": "daily", "trendshift.io": "daily", "https://www.trendshift.io/weekly": "weekly",
		"https://trendshift.io/weekly/?x=1": "weekly", "https://trendshift.io/repositories/1": "", "https://example.com/": ""} {
		if k, _ := trendshiftKind(in); k != want {
			t.Errorf("%s: %q want %q", in, k, want)
		}
	}
}

func TestParseTrendshiftJSON(t *testing.T) {
	for _, kind := range []string{"daily", "weekly"} {
		raw, _ := os.ReadFile("testdata/trendshift_" + kind + ".json")
		items, err := ParseTrendshiftJSON(kind, raw, 1_800_000_000)
		if err != nil || len(items) != 6 {
			t.Fatalf("%s: %v %d", kind, err, len(items))
		}
		first := items[0]
		if first.Title != "morluto/rea" || first.URL != "https://github.com/morluto/rea" || first.Author != "morluto" {
			t.Fatalf("%s first: %+v", kind, first)
		}
		period := map[string]string{"daily": "2026-10-18", "weekly": "2026-W42"}[kind]
		if first.UID != "trendshift:"+kind+":"+period+":morluto/rea" {
			t.Errorf("uid %q", first.UID)
		}
		gained := map[string]string{"daily": "今日 +634 星", "weekly": "本周 +205,098 星"}[kind]
		if !strings.HasPrefix(first.Summary, "#1 · TypeScript · "+gained+" · Reverse engineer") {
			t.Errorf("summary %q", first.Summary)
		}
		if !strings.Contains(first.Content, "星标：307,919") || !strings.Contains(first.Content, "讨论平台：hackernews") {
			t.Errorf("content %q", first.Content)
		}
		for i := 1; i < len(items); i++ { // rank order, newest first in the feed
			if items[i].Published >= items[i-1].Published || !strings.HasPrefix(items[i].Summary, "#"+itoa(i+1)+" ") {
				t.Errorf("%s order at %d: %q", kind, i, items[i].Summary)
			}
			if !strings.Contains(items[i].UID, period) {
				t.Errorf("older period leaked: %s", items[i].UID)
			}
		}
	}
	if _, err := ParseTrendshiftJSON("daily", []byte(`[]`), 1); err == nil {
		t.Error("empty list should fail")
	}
	if _, err := ParseTrendshiftJSON("daily", []byte(`<html>`), 1); err == nil {
		t.Error("not JSON should fail")
	}
}

func TestParseTrendshiftHTML(t *testing.T) {
	raw, _ := os.ReadFile("testdata/trendshift_home.html")
	// another JSON-LD block (not the ranking) must be ignored
	page := strings.Replace(string(raw), "<head>", `<head><script type="application/ld+json">{"@type":"WebSite","name":"Trendshift"}</script>`, 1)
	items, err := ParseTrendshiftHTML("daily", []byte(page), 1_800_000_000)
	if err != nil || len(items) != 6 {
		t.Fatalf("%v %d", err, len(items))
	}
	if items[0].Title != "google-research/harness2" || items[0].UID != "trendshift:daily:2026-10-18:google-research/harness2" || !strings.HasPrefix(items[0].Summary, "#1 · Python") {
		t.Fatalf("%+v", items[0])
	}
	for _, it := range items {
		if strings.Contains(it.Title, "Spam") {
			t.Fatal("non-ranking block parsed")
		}
	}
	raw, _ = os.ReadFile("testdata/trendshift_weekly.html")
	items, err = ParseTrendshiftHTML("weekly", raw, 1_800_000_000)
	if err != nil || len(items) != 6 || items[0].UID != "trendshift:weekly:2026-W42:morluto/rea" || !strings.Contains(items[0].Summary, "Reverse engineer") {
		t.Fatalf("weekly: %v %+v", err, items)
	}
	if _, err := ParseTrendshiftHTML("daily", []byte("<html><a>owner/repo</a></html>"), 1); err == nil {
		t.Error("a page without the ranking should fail")
	}
}

// primary JSON → fallback page → both failing (an error; nothing stored is touched)
func TestFetchTrendshiftFallback(t *testing.T) {
	daily, _ := os.ReadFile("testdata/trendshift_daily.json")
	home, _ := os.ReadFile("testdata/trendshift_home.html")
	jsonOK, pageOK := true, true
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query().Get("u")
		switch {
		case strings.Contains(u, "quantavil.github.io"):
			if !jsonOK {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Last-Modified", "Sun, 18 Oct 2026 01:33:25 GMT")
			w.Write(daily)
		case strings.HasPrefix(u, "https://trendshift.io/"):
			ua = r.Header.Get("User-Agent")
			if !pageOK {
				w.WriteHeader(503)
				return
			}
			w.Write(home)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	t.Setenv("UPSTREAM_MOCK", srv.URL)
	ctx := context.Background()
	items, err := fetchTrendshift(ctx, "daily", false)
	if err != nil || len(items) != 6 || items[0].Title != "morluto/rea" || items[0].Published != 1792287205-1 {
		t.Fatalf("primary: %v %d %v", err, len(items), items)
	}
	jsonOK = false
	items, err = fetchTrendshift(ctx, "daily", false)
	if err != nil || len(items) != 6 || items[0].Title != "google-research/harness2" || !strings.Contains(ua, "Mozilla/5.0 (Macintosh") {
		t.Fatalf("fallback: %v %d ua=%q", err, len(items), ua)
	}
	pageOK = false
	if items, err = fetchTrendshift(ctx, "daily", false); err == nil || items != nil || !strings.Contains(err.Error(), "Trendshift") {
		t.Fatalf("both failing: %v %v", err, items)
	}
}

func TestTrendshiftCap25(t *testing.T) {
	rs := make([]tsRepo, 30)
	for i := range rs {
		rs[i] = tsRepo{Rank: i + 1, FullName: fmt.Sprintf("o/r%d", i+1), Period: "2026-10-18"}
	}
	got := trendshiftItems("daily", rs, 1792287205)
	if len(got) != 25 || got[24].Title != "o/r25" {
		t.Fatalf("cap: %d %s", len(got), got[len(got)-1].Title)
	}
}

// 日榜 and 周榜 in one section: a repo shows once (daily wins, then the newest period); other sections unaffected.
func TestTrendshiftDedupPerSection(t *testing.T) {
	st := testStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	src := func(name string) *Row {
		r, err := one(st.DB, "SELECT * FROM sources WHERE name=?", name)
		if err != nil || r == nil {
			t.Fatalf("no source %s", name)
		}
		r.Set("last_fetched", now())
		return r
	}
	daily, weekly := src("Trendshift 日榜"), src("Trendshift 周榜")
	at := int64(now()) - 600
	it := func(kind, period, repo string, off int64) FetchedItem {
		return FetchedItem{UID: "trendshift:" + kind + ":" + period + ":" + strings.ToLower(repo), Title: repo, URL: "https://github.com/" + repo, Published: at - off}
	}
	st.StoreItems(daily, []FetchedItem{it("daily", "2026-10-18", "a/one", 1), it("daily", "2026-10-18", "b/Two", 2), it("daily", "2026-10-17", "a/one", 3600)})
	st.StoreItems(weekly, []FetchedItem{it("weekly", "2026-W42", "B/two", 1), it("weekly", "2026-W42", "c/three", 2)})
	other, _ := one(st.DB, "SELECT * FROM sources WHERE name NOT LIKE 'Trendshift%' AND enabled=1 LIMIT 1")
	other.Set("last_fetched", now())
	st.StoreItems(other, []FetchedItem{{UID: "https://n.example/1", Title: "a/one", URL: "https://n.example/1", Published: at}}) // not Trendshift: untouched
	list := func(q string) []string {
		var items []map[string]any
		sec := ""
		if v, ok := strings.CutPrefix(q, "&section="); ok {
			sec, q = v, ""
		}
		r, _ := http.Get(srv.URL + "/api/items?limit=200" + q)
		json.NewDecoder(r.Body).Decode(&items)
		r.Body.Close()
		var out []string
		for _, x := range items {
			if sec != "" && x["section"] != sec {
				continue
			}
			out = append(out, x["source"].(string)+"|"+x["title"].(string))
		}
		sort.Strings(out)
		return out
	}
	want := func(q string, exp ...string) {
		t.Helper()
		got := list(q)
		var ts []string
		for _, g := range got {
			if strings.HasPrefix(g, "Trendshift") || strings.HasSuffix(g, "|a/one") {
				ts = append(ts, g)
			}
		}
		sort.Strings(exp)
		if strings.Join(ts, ",") != strings.Join(exp, ",") {
			t.Fatalf("%s:\n got %v\nwant %v", q, ts, exp)
		}
	}
	otherName := other.Str("name")
	want("&section=media", "Trendshift 日榜|a/one", "Trendshift 日榜|b/Two", "Trendshift 周榜|c/three")
	want("", "Trendshift 日榜|a/one", "Trendshift 日榜|b/Two", "Trendshift 周榜|c/three", otherName+"|a/one")
	// stable across a re-poll (same items again)
	st.StoreItems(daily, []FetchedItem{it("daily", "2026-10-18", "a/one", 1)})
	want("&section=media", "Trendshift 日榜|a/one", "Trendshift 日榜|b/Two", "Trendshift 周榜|c/three")
	// a favorite hidden by the dedup still shows in 收藏
	var wid int64
	st.DB.QueryRow("SELECT id FROM items WHERE uid='trendshift:weekly:2026-W42:b/two'").Scan(&wid)
	postJSON(t, srv.URL+"/api/favorites", "POST", map[string]any{"id": wid, "on": true}, nil)
	if got := list("&fav=1"); len(got) != 1 || got[0] != "Trendshift 周榜|B/two" {
		t.Fatalf("fav view: %v", got)
	}
	// moving 周榜 to another section: both show again
	if c := postJSON(t, srv.URL+fmt.Sprintf("/api/sources/%d", weekly.Get("id")), "PATCH", map[string]any{"section": "lab"}, nil); c != 200 {
		t.Fatalf("move %d", c)
	}
	want("&section=media", "Trendshift 日榜|a/one", "Trendshift 日榜|b/Two")
	want("&section=lab", "Trendshift 周榜|B/two", "Trendshift 周榜|c/three", otherName+"|a/one") // the non-Trendshift item lives in lab
}

func TestPublishFreshItemsSkipsTrendshiftLoser(t *testing.T) {
	st := testStore(t)
	ch := st.Bus.Subscribe()
	defer st.Bus.Unsubscribe(ch)
	asID := func(v any) int64 {
		switch x := v.(type) {
		case int64:
			return x
		case float64:
			return int64(x)
		case int:
			return int64(x)
		default:
			return 0
		}
	}
	drain := func() (items, removed []int64) {
		for {
			select {
			case m := <-ch:
				if m.Event == "item" && m.Item != nil {
					items = append(items, int64(m.Item.Num("id")))
				} else if m.Event == "removed" {
					removed = append(removed, asID(m.ID))
				}
			default:
				return
			}
		}
	}
	src := func(name string) *Row {
		r, _ := one(st.DB, "SELECT * FROM sources WHERE name=?", name)
		r.Set("last_fetched", now())
		return r
	}
	daily, weekly := src("Trendshift 日榜"), src("Trendshift 周榜")
	at := int64(now()) - 60
	it := func(kind, period, repo string) FetchedItem {
		return FetchedItem{UID: "trendshift:" + kind + ":" + period + ":" + strings.ToLower(repo), Title: repo, URL: "https://github.com/" + repo, Published: at}
	}
	// weekly first — publishes; then daily arrives and should remove the weekly twin from the live feed
	_, _, _, err := st.StoreItems(weekly, []FetchedItem{it("weekly", "2026-W44", "Acme/Repo")})
	if err != nil {
		t.Fatal(err)
	}
	got, rem := drain()
	if len(got) != 1 || len(rem) != 0 {
		t.Fatalf("weekly alone: items=%v removed=%v", got, rem)
	}
	wid := got[0]
	_, _, _, err = st.StoreItems(daily, []FetchedItem{it("daily", "2026-10-26", "Acme/Repo")})
	if err != nil {
		t.Fatal(err)
	}
	got, rem = drain()
	if len(got) != 1 || len(rem) != 1 || rem[0] != wid {
		t.Fatalf("daily displaces weekly: items=%v removed=%v want removed=%d", got, rem, wid)
	}
	// a pure weekly-only repo still publishes
	_, _, _, err = st.StoreItems(weekly, []FetchedItem{it("weekly", "2026-W44", "Only/Weekly")})
	if err != nil {
		t.Fatal(err)
	}
	got, rem = drain()
	if len(got) != 1 || len(rem) != 0 {
		t.Fatalf("weekly-only: items=%v removed=%v", got, rem)
	}
}

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
	// POST path: insert a user source the same way hSourcesPost does (Detect needs live upstream; unit-test the clamp + overrides + scheduler wiring).
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
	// PATCH can change it further (add-dialog edit path on sources page)
	var out map[string]any
	if c := postJSON(t, srv.URL+fmt.Sprintf("/api/sources/%d", id), "PATCH", map[string]any{"poll_interval": 3600}, &out); c != 200 {
		t.Fatalf("PATCH %d %v", c, out)
	}
	var got int
	st.DB.QueryRow("SELECT poll_interval FROM sources WHERE id=?", id).Scan(&got)
	if got != 3600 {
		t.Fatalf("after PATCH poll_interval=%d want 3600", got)
	}
	st.DB.Exec("UPDATE sources SET last_fetched=? WHERE id=?", now()-3601, id)
	due, _ := all(st.DB, `SELECT id FROM sources s LEFT JOIN source_health h ON h.source_id=s.id
		WHERE s.enabled=1 AND s.supported=1 AND (s.last_fetched IS NULL OR s.last_fetched + s.poll_interval <= ?)
		AND (h.open_until IS NULL OR h.open_until <= ?) AND s.id=?`, now(), now(), id)
	if len(due) != 1 {
		t.Fatalf("scheduler should pick custom interval source, got %d", len(due))
	}
	// too soon: not due
	st.DB.Exec("UPDATE sources SET last_fetched=? WHERE id=?", now()-10, id)
	due, _ = all(st.DB, `SELECT id FROM sources s LEFT JOIN source_health h ON h.source_id=s.id
		WHERE s.enabled=1 AND s.supported=1 AND (s.last_fetched IS NULL OR s.last_fetched + s.poll_interval <= ?)
		AND (h.open_until IS NULL OR h.open_until <= ?) AND s.id=?`, now(), now(), id)
	if len(due) != 0 {
		t.Fatalf("scheduler should wait for interval, got %d", len(due))
	}
}
