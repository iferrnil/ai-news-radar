package core

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestThumbCache(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/a.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("PNGDATA-a"))
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	tc := NewThumbCache(t.TempDir(), 12)
	ctx := context.Background()
	b, err := tc.Get(ctx, srv.URL+"/a.png", "")
	if err != nil || string(b) != "PNGDATA-a" {
		t.Fatal(err, string(b))
	}
	tc.Get(ctx, srv.URL+"/a.png", "")
	if hits != 1 {
		t.Errorf("cache miss: hits=%d", hits)
	}
	if _, err := tc.Get(ctx, srv.URL+"/html", ""); err == nil {
		t.Error("html must be rejected")
	}
	if _, err := tc.Get(ctx, srv.URL+"/html", ""); err == nil || hits != 2 {
		t.Errorf("failure should be remembered: hits=%d", hits)
	}
	// size cap + retention prune
	os.WriteFile(tc.path("u1"), make([]byte, 10), 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(tc.path("u1"), old, old)
	os.WriteFile(tc.path("u2"), make([]byte, 10), 0o644)
	tc.Prune([]string{"u1", "u2", srv.URL + "/a.png"})
	if _, err := os.Stat(tc.path("u1")); err == nil {
		t.Error("LRU file should be evicted over the cap")
	}
	tc.Prune(nil)
	if tc.Size() != 0 {
		t.Errorf("unreferenced thumbs should be pruned, size=%d", tc.Size())
	}
}

func TestMigrateFromWebview(t *testing.T) {
	old := t.TempDir()
	s, err := Open(old, []byte(`{"sources":[{"name":"A","type":"rss","url":"https://a.test/f","priority":"P0"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	k, _ := s.CreateSection("我的分区", "⭐", true)
	a := srcByNameStore(s, "A")
	s.SetSection(a.ID, k)
	s.SetRule(a.ID, Rule{Exclude: "广告"})
	ids, _ := s.StoreItems(a, []FetchedItem{{UID: "a/1", Title: "t", URL: "https://a.test/1", Published: time.Now().Unix()}})
	s.SetRead(ids, true)
	s.SetSetting("notify", `{"enabled":true,"sections":["`+k+`"]}`)
	os.WriteFile(filepath.Join(old, "secrets.json"), []byte(`{"deepseekKey":"sk-0123456789abcdef"}`), 0o600)
	s.Close()
	before, _ := os.ReadFile(filepath.Join(old, "news.db"))

	nd := filepath.Join(old, "native")
	r, err := MigrateFromWebview(old, nd)
	if err != nil || !r.Done || r.Sources != 1 || r.Items != 1 || r.Read != 1 || !r.Secrets {
		t.Fatalf("%v %+v", err, r)
	}
	n, err := Open(nd, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := srcByNameStore(n, "A")
	if got.Rule.Exclude != "广告" || n.sectionNameOf(got.Section) != "我的分区" || n.NotifyPrefs().Sections[0] != k || n.Secrets().DeepseekKey == "" {
		t.Errorf("migrated: %+v", got)
	}
	if u := n.UnreadCounts()[""]; u != 0 {
		t.Errorf("read state lost: unread=%d", u)
	}
	n.Close()
	// once only
	if r2, _ := MigrateFromWebview(old, nd); r2.Done {
		t.Error("migrated twice")
	}
	after, _ := os.ReadFile(filepath.Join(old, "news.db"))
	if len(after) != len(before) {
		t.Error("old db modified")
	}
	if r3, _ := MigrateFromWebview(t.TempDir(), t.TempDir()); r3.Done || r3.Skipped == "" {
		t.Error("no old data should skip")
	}
	_ = sql.ErrNoRows
}

func TestHealth(t *testing.T) {
	s := newStore(t)
	a := srcByNameStore(s, "Seed A")
	for i := 0; i < 6; i++ {
		s.recordRun(a, false, time.Millisecond, "HTTP 500")
	}
	hs, err := s.Health()
	if err != nil {
		t.Fatal(err)
	}
	var h Health
	for _, x := range hs {
		if x.ID == a.ID {
			h = x
		}
	}
	if h.Consec != 6 || h.OpenUntil <= now() || h.LastErr != "HTTP 500" || h.SuccessRate() != 0 {
		t.Errorf("%+v", h)
	}
	s.recordRun(a, true, 5*time.Millisecond, "")
	hs, _ = s.Health()
	for _, x := range hs {
		if x.ID == a.ID && (x.Consec != 0 || x.OpenUntil != 0) {
			t.Errorf("breaker not closed: %+v", x)
		}
	}
}

func srcByNameStore(s *Store, name string) Source {
	l, _ := s.Sources()
	for _, x := range l {
		if x.Name == name {
			return x
		}
	}
	return Source{}
}

func (s *Store) sectionNameOf(k string) string {
	var n string
	s.DB.QueryRow("SELECT name FROM sections WHERE key=?", k).Scan(&n)
	return n
}

func TestReleasePick(t *testing.T) {
	v, link, _ := PickRelease([]relInfo{{"desktop-v0.9.0", "a"}, {"desktop-native-v0.4.2", "b"}, {"desktop-native-v0.10.0", "c"}, {"desktop-native-v0.4.10", "d"}}, "desktop-native-v")
	if v != "0.10.0" || link != "c" {
		t.Errorf("%s %s", v, link)
	}
	if !NewerVersion("0.4.10", "0.4.9") || NewerVersion("0.4.0", "0.4.0") || NewerVersion("0.3.9", "0.4.0") {
		t.Error("NewerVersion")
	}
}
