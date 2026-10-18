package radar

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIconLinks(t *testing.T) {
	base, _ := url.Parse("https://ex.com/blog/")
	got := iconLinks(`<link rel="stylesheet" href="/a.css"><link rel="icon" href="/f16.png" sizes="16x16"><link rel="mask-icon" href="/m.svg">
		<link rel="apple-touch-icon" href="t.png"><link rel="shortcut icon" href="https://cdn.ex.com/f.ico"><link rel=icon href=/f64.png sizes=64x64>`, base)
	want := "https://ex.com/blog/t.png https://ex.com/f64.png https://cdn.ex.com/f.ico https://ex.com/f16.png"
	if strings.Join(got, " ") != want {
		t.Fatalf("%v", got)
	}
}

func TestIconProxy(t *testing.T) {
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("x", 64)
	hits := map[string]int{}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query().Get("u")
		hits[u]++
		switch u {
		case "https://openai.com/":
			w.Write([]byte(`<html><head><link rel="icon" href="/icon-64.png" sizes="64x64"></head></html>`))
		case "https://openai.com/icon-64.png", "https://unavatar.io/x/OpenAI?fallback=false":
			w.Write([]byte(png))
		default:
			w.WriteHeader(404)
		}
	}))
	defer up.Close()
	st := testStore(t)
	t.Setenv("UPSTREAM_MOCK", up.URL)
	srv := httptest.NewServer(st.Handler())
	defer srv.Close()
	get := func(q string) (*http.Response, string) {
		r, err := http.Get(srv.URL + "/api/icon?" + q)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Body.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return r, b.String()
	}
	r, body := get("host=openai.com")
	if r.StatusCode != 200 || body != png || r.Header.Get("Content-Type") != "image/png" || !strings.Contains(r.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("site icon: %d %q", r.StatusCode, r.Header)
	}
	get("host=openai.com")
	if hits["https://openai.com/icon-64.png"] != 1 {
		t.Errorf("not cached: %v", hits)
	}
	if r, _ := get("host=evil.example.com"); r.StatusCode != 404 || hits["https://evil.example.com/"] != 0 {
		t.Errorf("open proxy: %d", r.StatusCode)
	}
	if r, body := get("x=OpenAI"); r.StatusCode != 200 || body != png {
		t.Errorf("x avatar %d", r.StatusCode)
	}
	if r, _ := get("x=../etc"); r.StatusCode != 400 {
		t.Errorf("bad handle %d", r.StatusCode)
	}
	// a host without any icon: a miss (204, no failed-resource log), remembered
	if r, _ := get("host=www.qbitai.com"); r.StatusCode != 204 {
		t.Errorf("miss %d", r.StatusCode)
	}
	n := hits["https://www.qbitai.com/favicon.ico"]
	get("host=www.qbitai.com")
	if hits["https://www.qbitai.com/favicon.ico"] != n || n == 0 {
		t.Errorf("miss not remembered: %v", hits)
	}
}
