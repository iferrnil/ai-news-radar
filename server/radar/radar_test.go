package radar

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/andybalholm/brotli"
)

func TestNormURL(t *testing.T) {
	for in, want := range map[string]string{
		"https://Example.com/a/?utm_source=x&b=1#frag": "example.com/a?b=1",
		"http://example.com/":                          "example.com/",
	} {
		if got := NormURL(in); got != want {
			t.Errorf("NormURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodeAndClean(t *testing.T) {
	if got := Decode("a&amp;b&nbsp;c&#39;d&#x4e2d;"); got != "a&b c'd中" {
		t.Fatalf("Decode: %q", got)
	}
	if got := Clean("<p>Hello <b>world</b></p>", 100); got != "Hello world" {
		t.Fatalf("Clean: %q", got)
	}
}

func TestParseFeedRSSAndAtom(t *testing.T) {
	rss := `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title><link>https://e.com</link>
<item><title>One &amp; two</title><link>https://e.com/1</link><pubDate>Mon, 12 Oct 2026 10:00:00 GMT</pubDate><description>&lt;p&gt;Hi&lt;/p&gt;</description></item>
</channel></rss>`
	f := ParseFeed([]byte(rss))
	if f == nil || len(f.Items) != 1 || f.Items[0].Title != "One & two" || f.Items[0].URL != "https://e.com/1" {
		t.Fatalf("rss: %+v", f)
	}
	atom := `<feed xmlns="http://www.w3.org/2005/Atom"><title>A</title><entry><title>E</title><link rel="alternate" href="https://e.com/e"/><updated>2026-10-12T10:00:00Z</updated></entry></feed>`
	f = ParseFeed([]byte(atom))
	if f == nil || len(f.Items) != 1 || f.Items[0].URL != "https://e.com/e" || f.Items[0].Published == 0 {
		t.Fatalf("atom: %+v", f)
	}
}

func TestRuleGuard(t *testing.T) {
	bad := []string{"^(a|a)*$", "^(a|aa)*$", "(a+)+b", "(\\w|\\d)+$"}
	for _, p := range bad {
		if _, err := ValidateRule(map[string]any{"exclude": p}, nil); err == "" {
			t.Errorf("%q accepted", p)
		}
	}
	if _, err := ValidateRule(map[string]any{"include": "(unclosed"}, nil); !strings.Contains(err, "Unterminated group") {
		t.Errorf("V8-style message expected, got %q", err)
	}
	ok := []string{"not much happened", "^((?!not much).)*$", "GPT-?5|Claude", "(foo|bar)+"}
	for _, p := range ok {
		if _, err := ValidateRule(map[string]any{"include": p}, []string{"foo bar", "GPT-5 is out"}); err != "" {
			t.Errorf("%q rejected: %s", p, err)
		}
	}
	r, _ := ValidateRule(map[string]any{"exclude": "not much"}, nil)
	got := EvalRule(r, []string{"Not much happened today", "big news"}, 100*time.Millisecond)
	if len(got) != 2 || got[0] || !got[1] {
		t.Fatalf("EvalRule (case-insensitive exclude): %v", got)
	}
}

func TestRowKeepsKeyOrderAndDropsNulls(t *testing.T) {
	r := obj("z", 1, "a", nil, "m", "x")
	b, _ := json.Marshal(r)
	if string(b) != `{"z":1,"a":null,"m":"x"}` {
		t.Fatal(string(b))
	}
	b, _ = json.Marshal(Rows{List: []*Row{r}, DropNulls: true})
	if string(b) != `[{"z":1,"m":"x"}]` {
		t.Fatal(string(b))
	}
	if jsNum(3) != "3" || jsNum(0.5) != "0.5" {
		t.Fatal(jsNum(3), jsNum(0.5))
	}
}

func TestValidateEntry(t *testing.T) {
	if _, err := ValidateEntry(map[string]any{"name": "JS", "type": "rss", "url": "javascript:alert(1)"}); err == "" {
		t.Fatal("javascript: URL accepted")
	}
	if _, err := ValidateEntry(map[string]any{"type": "rss", "url": "https://e.com/f"}); !strings.Contains(err, "received undefined") {
		t.Fatalf("missing name: %q", err)
	}
	if e, err := ValidateEntry(map[string]any{"name": " OK ", "type": "rss", "url": "https://e.com/feed"}); err != "" || e == nil {
		t.Fatalf("valid entry rejected: %q", err)
	}
}

func TestSectionNames(t *testing.T) {
	if got := CleanSectionName("  视频   生成  "); got != "视频 生成" {
		t.Fatalf("%q", got)
	}
	if jsLen("😀a") != 3 { // UTF-16 length like JS
		t.Fatal(jsLen("😀a"))
	}
}

func TestStaticHandlerFallbacks(t *testing.T) {
	ui := fstest.MapFS{
		"index.html":               {Data: []byte("home")},
		"sources.html":             {Data: []byte("sources")},
		"docs/rate-limits.html":    {Data: []byte("rl")},
		"404.html":                 {Data: []byte("nf")},
		"_next/static/chunks/a.js": {Data: []byte("js")},
	}
	h := StaticHandler(ui)
	for path, want := range map[string][2]string{
		"/":                         {"200", "home"},
		"/sources":                  {"200", "sources"},
		"/docs/rate-limits":         {"200", "rl"},
		"/_next/static/chunks/a.js": {"200", "js"},
		"/nope":                     {"404", "nf"},
	} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", path, nil))
		if code := rr.Code; http.StatusText(code) == "" || itoa(code) != want[0] || rr.Body.String() != want[1] {
			t.Errorf("%s: %d %q, want %s %q", path, code, rr.Body.String(), want[0], want[1])
		}
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/_next/static/chunks/a.js", nil))
	if !strings.Contains(rr.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("hashed assets should be immutable: %q", rr.Header().Get("Cache-Control"))
	}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func testStore(t *testing.T) *Store {
	t.Helper()
	t.Setenv("UPSTREAM_MOCK", "http://127.0.0.1:9")
	d := t.TempDir()
	st, err := Open(Config{DBPath: filepath.Join(d, "news.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "secrets.json")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestAPISmoke(t *testing.T) {
	if _, err := os.Stat("../../sources/sources.json"); err != nil {
		t.Skip("no sources.json")
	}
	st := testStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	get := func(p string) (int, string) {
		r, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(b)
	}
	post := func(p, body string) (int, string) {
		r, err := http.Post(srv.URL+p, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(b)
	}
	if c, _ := get("/api/health"); c != 200 {
		t.Fatal("health", c)
	}
	c, b := get("/api/sources")
	var srcs []map[string]any
	if c != 200 || json.Unmarshal([]byte(b), &srcs) != nil || len(srcs) < 20 {
		t.Fatalf("sources %d %d", c, len(srcs))
	}
	if c, b := get("/api/items?limit=5"); c != 200 || !strings.HasPrefix(b, "[") {
		t.Fatalf("items %d %s", c, b)
	}
	if c, _ := post("/api/sources/rule-check", `{"exclude":"^(a|a)*$"}`); c != 400 {
		t.Fatalf("catastrophic rule: %d", c)
	}
	if c, _ := post("/api/sources/rule-check", `{"exclude":"not much"}`); c != 200 {
		t.Fatalf("simple rule: %d", c)
	}
	if c, b := get("/api/sections"); c != 200 || !strings.Contains(b, `"key"`) {
		t.Fatalf("sections %d %s", c, b)
	}
	if c, _ := get("/api/does-not-exist"); c != 404 {
		t.Fatalf("unknown api: %d", c)
	}
	// settings never echo a key
	t.Setenv("DEEPSEEK_API_KEY", "sk-"+"test-not-real")
	if _, b := get("/api/settings"); strings.Contains(b, "test-not-real") {
		t.Fatal("settings leak the key")
	}
}

func TestSecretsFileMode(t *testing.T) {
	st := testStore(t)
	t.Setenv("SETTINGS_LOCKED", "")
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	req, _ := http.NewRequest("PATCH", srv.URL+"/api/settings", strings.NewReader(`{"deepseek":{"key":"sk-`+`abcdefghijklmnopqrstuvwx"}}`))
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	fi, err := os.Stat(st.Cfg.SecretsPath)
	if err != nil {
		t.Fatalf("settings PATCH %d did not write secrets (%v)", r.StatusCode, err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("secrets.json mode %o", fi.Mode().Perm())
	}
}

func TestStaticCompressed(t *testing.T) {
	big := strings.Repeat("console.log('radar');\n", 200)
	ui := fstest.MapFS{"index.html": {Data: []byte("<html>" + big + "</html>")}, "404.html": {Data: []byte("nf")},
		"_next/static/chunks/b.js": {Data: []byte(big)}, "_next/static/media/f.woff2": {Data: []byte(big)}}
	h := StaticHandler(ui)
	get := func(p, ae, inm string) *httptest.ResponseRecorder {
		rq := httptest.NewRequest("GET", p, nil)
		if ae != "" {
			rq.Header.Set("Accept-Encoding", ae)
		}
		if inm != "" {
			rq.Header.Set("If-None-Match", inm)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, rq)
		return rr
	}
	rr := get("/_next/static/chunks/b.js", "gzip, deflate, br", "")
	if rr.Header().Get("Content-Encoding") != "br" || rr.Body.Len() >= len(big)/4 || rr.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("br: %q %d", rr.Header().Get("Content-Encoding"), rr.Body.Len())
	}
	if out, err := io.ReadAll(brotli.NewReader(rr.Body)); err != nil || string(out) != big {
		t.Fatal("br body does not round-trip")
	}
	if r2 := get("/_next/static/chunks/b.js", "br", rr.Header().Get("ETag")); r2.Code != 304 {
		t.Fatalf("etag: %d", r2.Code)
	}
	rr = get("/", "gzip", "")
	zr, err := gzip.NewReader(rr.Body)
	if err != nil || rr.Header().Get("Content-Encoding") != "gzip" || rr.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("html gzip: %v %v", err, rr.Header())
	}
	if out, _ := io.ReadAll(zr); !strings.HasPrefix(string(out), "<html>") {
		t.Fatal("gzip body")
	}
	if rr := get("/_next/static/chunks/b.js", "", ""); rr.Header().Get("Content-Encoding") != "" || rr.Body.String() != big {
		t.Fatal("identity")
	}
	if rr := get("/_next/static/media/f.woff2", "br", ""); rr.Header().Get("Content-Encoding") != "" {
		t.Fatal("fonts are already compressed")
	}
}
