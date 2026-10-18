package radar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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
