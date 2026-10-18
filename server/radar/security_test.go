package radar

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// guardOn turns the SSRF guard on for one test (TestMain turns it off for the httptest-based tests).
func guardOn(t *testing.T) {
	t.Helper()
	t.Setenv("UPSTREAM_MOCK", "")
	privateOK = false
	t.Cleanup(func() { privateOK = true })
}

func desktopStore(t *testing.T) *Store {
	t.Helper()
	d := t.TempDir()
	st, err := Open(Config{DBPath: filepath.Join(d, "news.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "secrets.json"), Desktop: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func do(t *testing.T, srv string, method, path, body string, hdr map[string]string) (int, http.Header, string) {
	t.Helper()
	rq, _ := http.NewRequest(method, srv+path, strings.NewReader(body))
	for k, v := range hdr {
		if k == "Host" {
			rq.Host = v
			continue
		}
		rq.Header.Set(k, v)
	}
	r, err := http.DefaultClient.Do(rq)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, r.Header, string(b)
}

// Any website could post to the local API (simple text/plain / form posts need no CORS preflight) or read it after
// DNS rebinding; browser requests from another site are refused now.
func TestCrossSiteAPIRefused(t *testing.T) {
	st := testStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	cases := []struct {
		name   string
		method string
		path   string
		hdr    map[string]string
		want   int
	}{
		{"cross-site form post", "POST", "/api/sections", map[string]string{"Sec-Fetch-Site": "cross-site", "Content-Type": "text/plain"}, 403},
		{"same-site post", "POST", "/api/sections", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"cross-site GET (img / AI trigger)", "GET", "/api/settings", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"foreign Origin, no Sec-Fetch", "POST", "/api/sections", map[string]string{"Origin": "https://evil.example"}, 403},
		{"Origin null", "POST", "/api/sections", map[string]string{"Origin": "null"}, 403},
		{"same origin", "POST", "/api/sections", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://" + host}, 200},
		{"matching Origin (old browser)", "POST", "/api/sections", map[string]string{"Origin": "http://" + host}, 200},
		{"proxy: X-Forwarded-Host", "POST", "/api/sections", map[string]string{"Origin": "https://radar.example.com", "X-Forwarded-Host": "radar.example.com"}, 200},
		{"no browser headers (curl)", "POST", "/api/sections", nil, 200},
		{"typed URL", "GET", "/api/settings", map[string]string{"Sec-Fetch-Site": "none"}, 200},
	}
	for i, c := range cases {
		body := `{"name":"s` + string(rune('a'+i)) + `"}`
		code, h, out := do(t, srv.URL, c.method, c.path, body, c.hdr)
		if code != c.want {
			t.Errorf("%s: %d %s (want %d)", c.name, code, out, c.want)
		}
		if h.Get("X-Frame-Options") != "DENY" || h.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: security headers missing: %v", c.name, h)
		}
	}
}

func TestAllowedHosts(t *testing.T) {
	st := testStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	t.Setenv("ALLOWED_HOSTS", "radar.example.com, 127.0.0.1")
	if c, _, _ := do(t, srv.URL, "GET", "/api/health", "", map[string]string{"Host": "rebound.attacker.example:80"}); c != http.StatusMisdirectedRequest {
		t.Fatalf("rebound host: %d", c)
	}
	if c, _, _ := do(t, srv.URL, "GET", "/api/health", "", nil); c != 200 { // 127.0.0.1:<port>
		t.Fatalf("allowed host: %d", c)
	}
	if c, _, _ := do(t, srv.URL, "GET", "/api/health", "", map[string]string{"Host": "radar.example.com"}); c != 200 {
		t.Fatalf("allowed name: %d", c)
	}
}

// /api/settings/test {baseUrl} used to send the STORED DeepSeek key to any Base URL (key theft by one request).
func TestDeepseekTestNeverSendsStoredKeyElsewhere(t *testing.T) {
	st := testStore(t)
	t.Setenv("UPSTREAM_MOCK", "")
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	var leaked atomic.Int32
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Authorization"), "stored-test-key") {
			leaked.Add(1)
		}
		w.Write([]byte(`{"data":[]}`))
	}))
	defer evil.Close()
	if c := postJSON(t, srv.URL+"/api/settings", "PATCH", map[string]any{"deepseek": map[string]any{"key": "stored-test-key-0123"}}, nil); c != 200 {
		t.Fatalf("save key %d", c)
	}
	evilBase := strings.Replace(evil.URL, "127.0.0.1", "localhost", 1) + "/v1"
	var res map[string]any
	postJSON(t, srv.URL+"/api/settings/test", "POST", map[string]any{"baseUrl": evilBase}, &res)
	if res["ok"] != false || leaked.Load() != 0 {
		t.Fatalf("stored key went to a new base: %v leaked=%d", res, leaked.Load())
	}
	// with the key typed along, testing another base is fine (the key is the caller's own)
	postJSON(t, srv.URL+"/api/settings/test", "POST", map[string]any{"baseUrl": evilBase, "key": "typed-test-key-01234"}, &res)
	if res["ok"] != true {
		t.Fatalf("typed key test: %v", res)
	}
}

func TestSafeLinks(t *testing.T) {
	for in, want := range map[string]string{
		"https://example.com/a":                      "https://example.com/a",
		"http://example.com":                         "http://example.com",
		"file:///System/Applications/Calculator.app": "",
		"javascript:alert(1)":                        "",
		"smb://attacker.example/share":               "",
		"http://127.0.0.1:47821@evil.example/":       "",
		"x-apple.systempreferences:com.apple":        "",
		"/relative":                                  "",
	} {
		if got := SafeLink(in); got != want {
			t.Errorf("SafeLink(%q) = %q", in, got)
		}
	}
	got := safeItems([]FetchedItem{{UID: "1", URL: "https://ok.example/1"}, {UID: "2", URL: "file:///etc/passwd"}, {UID: "3", URL: "javascript:x"}, {UID: "4", URL: "/rel"}, {UID: "5"}})
	if len(got) != 3 || got[0].UID != "1" || got[1].UID != "4" || got[1].URL != "" || got[2].UID != "5" {
		t.Fatalf("safeItems: %+v", got)
	}
	// StoreItems applies it
	st := testStore(t)
	src, _ := one(st.DB, "SELECT * FROM sources WHERE enabled=1 LIMIT 1")
	src.Set("last_fetched", now())
	st.StoreItems(src, []FetchedItem{{UID: "evil-1", Title: "calc", URL: "file:///System/Applications/Calculator.app", Published: int64(now()) - 60},
		{UID: "ok-1", Title: "fine", URL: "https://ok.example/x", Published: int64(now()) - 60}})
	var n int
	st.DB.QueryRow("SELECT COUNT(*) FROM items WHERE url LIKE 'file:%' OR uid='evil-1'").Scan(&n)
	if n != 0 {
		t.Fatal("a file:// item was stored")
	}
}

// The 工作目录 setting could be pointed at / (or ~) and then /api/artifacts/file served any file; open could launch
// an .app bundle.
func TestArtifactsGuards(t *testing.T) {
	st := desktopStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	home, _ := os.UserHomeDir()
	for _, d := range []string{"/", home, "/Applications/Calculator.app", "/System", t.TempDir() + "/x.app"} {
		if c := postJSON(t, srv.URL+"/api/artifacts", "PATCH", map[string]any{"dir": d}, nil); c != 400 {
			t.Errorf("artifacts dir %q accepted (%d)", d, c)
		}
	}
	work := t.TempDir()
	if c := postJSON(t, srv.URL+"/api/artifacts", "PATCH", map[string]any{"dir": work}, nil); c != 200 {
		t.Fatalf("normal dir refused %d", c)
	}
	covers := filepath.Join(work, "缓存", "封面", "2026-10-18")
	os.MkdirAll(covers, 0o755)
	cover := filepath.Join(covers, "1-a.png")
	os.WriteFile(cover, []byte("\x89PNG\r\n\x1a\nxxxx"), 0o644)
	secret := filepath.Join(work, "secret.png") // inside the 工作目录 but not a cover
	os.WriteFile(secret, []byte("nope"), 0o644)
	link := filepath.Join(covers, "2-link.png")
	os.Symlink(st.Cfg.SecretsPath, link)
	os.WriteFile(st.Cfg.SecretsPath, []byte(`{"deepseekKey":"sk-x"}`), 0o600)
	get := func(p string) int {
		r, err := http.Get(srv.URL + "/api/artifacts/file?path=" + url.QueryEscape(p))
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if get(cover) != 200 {
		t.Fatal("cover not served")
	}
	for _, p := range []string{secret, link, st.Cfg.SecretsPath, "/etc/passwd", filepath.Join(covers, "..", "..", "..", "secret.png")} {
		if c := get(p); c != 404 {
			t.Errorf("served %s (%d)", p, c)
		}
	}
	// export copies covers only
	var ex map[string]any
	postJSON(t, srv.URL+"/api/artifacts/export", "POST", map[string]any{"kind": "article", "title": "t", "markdown": "# t", "covers": []string{cover, st.Cfg.SecretsPath, link}}, &ex)
	md, _ := os.ReadFile(str(ex["path"]))
	if strings.Count(string(md), "![封面]") != 1 {
		t.Fatalf("export covers: %s", md)
	}
	// open: folders only, never a bundle
	var opened []string
	DesktopHooks.OpenPath = func(p string) error { opened = append(opened, p); return nil }
	defer func() { DesktopHooks.OpenPath = nil }()
	app := filepath.Join(work, "文章", "Evil.app")
	os.MkdirAll(app, 0o755)
	var res map[string]any
	postJSON(t, srv.URL+"/api/artifacts/open", "POST", map[string]any{"path": app}, &res)
	for _, p := range opened {
		if strings.HasSuffix(p, ".app") {
			t.Fatalf("opened a bundle: %v", opened)
		}
	}
	if res["ok"] != false {
		t.Fatalf("open bundle: %v", res)
	}
}

func TestBlockedIP(t *testing.T) {
	for s, want := range map[string]bool{
		"127.0.0.1": true, "10.1.2.3": true, "192.168.1.1": true, "172.16.0.1": true, "169.254.169.254": true,
		"100.64.0.1": true, "0.0.0.0": true, "::1": true, "fe80::1": true, "fc00::1": true, "::ffff:127.0.0.1": true,
		"64:ff9b::7f00:1": true, "224.0.0.1": true, "8.8.8.8": false, "2606:4700::1111": false, "140.82.112.3": false,
	} {
		if got := blockedIP(net.ParseIP(s)); got != want {
			t.Errorf("blockedIP(%s) = %v", s, got)
		}
	}
}

// Item links, icons, Skill downloads and (web) add-source probes could reach 127.0.0.1 / the LAN / metadata.
func TestSSRFGuard(t *testing.T) {
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Local</title><item><title>secret</title><link>https://x.example/1</link><pubDate>Sun, 18 Oct 2026 10:00:00 GMT</pubDate></item></channel></rss>`))
	}))
	defer local.Close()
	guardOn(t)
	ctx := context.Background()
	if _, err := netFetch(PublicOnly(ctx), "GET", local.URL, nil, nil, 0); err == nil || !strings.Contains(err.Error(), "内网") {
		t.Fatalf("public-only fetch reached 127.0.0.1: %v", err)
	}
	host := strings.Replace(local.URL, "127.0.0.1", "localhost", 1)
	if _, err := netFetch(PublicOnly(ctx), "GET", host, nil, nil, 0); err == nil {
		t.Fatal("public-only fetch reached localhost")
	}
	if _, err := netFetch(ctx, "GET", local.URL, nil, nil, 0); err != nil { // unmarked (DeepSeek base etc.) still fine
		t.Fatalf("unmarked fetch: %v", err)
	}
	// a public URL that redirects to the LAN: the dial-time check catches the hop
	redir := httptest.NewServer(http.RedirectHandler(local.URL, http.StatusFound))
	defer redir.Close()
	if _, err := publicClient.Get(local.URL); err == nil || !errors.Is(err, errPrivateAddr) {
		t.Fatalf("guarded client dialed 127.0.0.1: %v", err)
	}
	// article fetcher (item links come from feeds)
	if _, err := gFetch.once(ctx, local.URL, 0+5e9); err == nil {
		t.Fatal("article fetcher reached 127.0.0.1")
	}
	// web server: add-source probe refused; desktop: allowed (local RSSHub)
	web := &Store{Cfg: Config{}}
	if _, err := Detect(web.fetchCtx(ctx), local.URL); err == nil {
		t.Fatal("web Detect reached 127.0.0.1")
	}
	desk := &Store{Cfg: Config{Desktop: true}}
	if d, err := Detect(desk.fetchCtx(ctx), local.URL); err != nil || d.Name != "Local" {
		t.Fatalf("desktop Detect of a local feed: %v %v", d, err)
	}
	// icons
	if b, _ := iconGet(ctx, local.URL+"/favicon.ico"); b != nil {
		t.Fatal("icon fetched from 127.0.0.1")
	}
	// ALLOW_PRIVATE_FETCH-style opt out
	privateOK = true
	if _, err := netFetch(PublicOnly(ctx), "GET", local.URL, nil, nil, 0); err != nil {
		t.Fatalf("opt-out: %v", err)
	}
}
