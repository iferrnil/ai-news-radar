package radar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeProv is a fake provider endpoint: mode decides the answer, hits counts calls.
type fakeProv struct {
	srv  *httptest.Server
	mode atomic.Value // "ok" | "short" | "402" | "429" | "500" | "403credit"
	hits atomic.Int32
	auth atomic.Value
}

func newFakeProv(t *testing.T, id string) *fakeProv {
	p := &fakeProv{}
	p.mode.Store("ok")
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.hits.Add(1)
		p.auth.Store(r.Header.Get("Authorization"))
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		text := strings.Repeat(id+" markdown body sentence with facts. ", 30) + "\n\n![img](https://x/i.png)"
		switch p.mode.Load().(string) {
		case "402":
			w.WriteHeader(402)
			w.Write([]byte(`{"error":"Payment required: insufficient credits"}`))
			return
		case "403credit":
			w.WriteHeader(403)
			w.Write([]byte(`{"detail":"Insufficient balance"}`))
			return
		case "429":
			w.WriteHeader(429)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		case "500":
			w.WriteHeader(500)
			return
		case "short":
			text = "tiny"
		}
		switch id {
		case "jina":
			json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": map[string]any{"title": "J", "content": text, "url": r.URL.Path[1:]}})
		case "firecrawl":
			if !strings.Contains(string(body), `"markdown"`) || r.URL.Path != "/v2/scrape" {
				w.WriteHeader(400)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"markdown": text, "metadata": map[string]any{"title": "F", "statusCode": 200}}})
		case "crawl4ai":
			if r.URL.Path != "/scrape" {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("x-c4-balance", "812")
			w.Header().Set("x-c4-allowance-left", "300")
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "markdown": text})
		}
	}))
	t.Cleanup(p.srv.Close)
	return p
}

type provRig struct {
	st           *Store
	jina, fc, c4 *fakeProv
	selfURL      string
	selfHits     atomic.Int32
}

func newProvRig(t *testing.T) *provRig {
	t.Setenv("UPSTREAM_MOCK", "")
	d := t.TempDir()
	st, err := Open(Config{DBPath: filepath.Join(d, "news.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "secrets.json"), Desktop: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	g := &provRig{st: st, jina: newFakeProv(t, "jina"), fc: newFakeProv(t, "firecrawl"), c4: newFakeProv(t, "crawl4ai")}
	self := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.selfHits.Add(1)
		fmt.Fprintf(w, "<html><body><article>%s</article></body></html>", para("Self", 8))
	}))
	t.Cleanup(self.Close)
	g.selfURL = self.URL + "/post"
	oldF, oldC := firecrawlBase, crawl4aiBase
	firecrawlBase, crawl4aiBase = g.fc.srv.URL, g.c4.srv.URL
	t.Cleanup(func() { firecrawlBase, crawl4aiBase = oldF, oldC })
	sec := st.loadSecrets()
	sec.Fetch = FetchKeys{JinaKey: "jina-key-1", JinaBase: g.jina.srv.URL, FirecrawlKey: "fc-key-1", Crawl4aiKey: "c4-key-1"}
	st.saveSecrets(sec)
	gArticles.mu.Lock()
	gArticles.m = map[string]articleCached{}
	gArticles.mu.Unlock()
	return g
}

func (g *provRig) fetch(t *testing.T) articleRes {
	gArticles.mu.Lock()
	gArticles.m = map[string]articleCached{}
	gArticles.mu.Unlock()
	return g.st.fetchViaProviders([]string{g.selfURL})[g.selfURL]
}

func TestProviderSchedulerFailover(t *testing.T) {
	g := newProvRig(t)
	// 1. Jina first
	r := g.fetch(t)
	if r.Method != "jina" || r.Status != "full" || strings.Contains(r.Text, "![img]") || g.jina.auth.Load() != "Bearer jina-key-1" {
		t.Fatalf("jina: %+v", r)
	}
	// 2. Jina out of credit → exhausted, Firecrawl takes over; the next call skips Jina entirely
	g.jina.mode.Store("402")
	if r = g.fetch(t); r.Method != "firecrawl" || r.Status != "full" {
		t.Fatalf("after 402: %+v", r)
	}
	jh := g.jina.hits.Load()
	if r = g.fetch(t); r.Method != "firecrawl" || g.jina.hits.Load() != jh {
		t.Fatalf("exhausted jina retried: %+v", r)
	}
	// 3. Firecrawl rate limited → cooling, crawl4ai used; its balance headers fill the quota
	g.fc.mode.Store("429")
	if r = g.fetch(t); r.Method != "crawl4ai" {
		t.Fatalf("after 429: %+v", r)
	}
	pub := g.st.fetchAPIPublic()
	b, _ := json.Marshal(pub)
	js := string(b)
	for _, want := range []string{`"id":"jina"`, `"status":"exhausted"`, `"status":"cooling"`, `"remaining":812`, `"allowance":300`} {
		if !strings.Contains(js, want) {
			t.Fatalf("public state lacks %s: %s", want, js)
		}
	}
	if strings.Contains(js, "jina-key-1") || strings.Contains(js, "fc-key-1") || strings.Contains(js, "c4-key-1") {
		t.Fatal("key leaked to the UI")
	}
	// 4. crawl4ai down too → the self-written fetcher, reasons not needed since it got full text
	g.c4.mode.Store("500")
	sh := g.selfHits.Load()
	if r = g.fetch(t); r.Method != "self" || r.Status != "full" || g.selfHits.Load() != sh+1 {
		t.Fatalf("self fallback: %+v", r)
	}
	// 5. everything unavailable now → straight to self without calling any provider
	h := g.jina.hits.Load() + g.fc.hits.Load() + g.c4.hits.Load()
	if r = g.fetch(t); r.Method != "self" || g.jina.hits.Load()+g.fc.hits.Load()+g.c4.hits.Load() != h {
		t.Fatalf("unavailable providers called: %+v", r)
	}
	// 6. a new Jina key resets its exhaustion
	g.jina.mode.Store("ok")
	sec := g.st.loadSecrets()
	sec.Fetch.JinaKey = "jina-key-2"
	g.st.saveSecrets(sec)
	if r = g.fetch(t); r.Method != "jina" || g.jina.auth.Load() != "Bearer jina-key-2" {
		t.Fatalf("new key not used: %+v", r)
	}
}

func TestProviderShortAndCreditForbidden(t *testing.T) {
	g := newProvRig(t)
	g.jina.mode.Store("short")   // too short: keep as partial, try the next one
	g.fc.mode.Store("403credit") // 403 + credit wording = exhausted
	g.c4.mode.Store("short")
	selfSrv := g.selfURL
	r := g.fetch(t)
	if r.Method != "self" || r.Status != "full" {
		t.Fatalf("short everywhere: %+v", r)
	}
	if g.st.prov.available("firecrawl", "fc-key-1") || !g.st.prov.available("jina", "jina-key-1") {
		t.Fatal("403 credit not exhausted / short answer penalised")
	}
	_ = selfSrv
	// no keys at all: never touches a provider
	sec := g.st.loadSecrets()
	sec.Fetch = FetchKeys{}
	g.st.saveSecrets(sec)
	h := g.jina.hits.Load() + g.fc.hits.Load() + g.c4.hits.Load()
	if r = g.fetch(t); r.Method != "self" || g.jina.hits.Load()+g.fc.hits.Load()+g.c4.hits.Load() != h {
		t.Fatalf("no keys: %+v", r)
	}
}

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		id   string
		code int
		body string
		want provOutcome
	}{
		{"jina", 402, `{}`, provExhausted}, {"jina", 401, `{}`, provExhausted}, {"firecrawl", 403, `{"error":"Insufficient credits"}`, provExhausted},
		{"firecrawl", 403, `{"error":"site blocked"}`, provPage}, {"crawl4ai", 429, `{"detail":"monthly plan_cap reached"}`, provExhausted},
		{"crawl4ai", 429, `{}`, provRate}, {"crawl4ai", 502, `{}`, provPage}, {"jina", 503, ``, provDown}, {"jina", 422, `{}`, provPage},
	} {
		if got, _ := classify(c.id, &Resp{Status: c.code, Body: []byte(c.body), Header: http.Header{}}, nil); got != c.want {
			t.Fatalf("%s %d %s: %v want %v", c.id, c.code, c.body, got, c.want)
		}
	}
}

func TestQuotaRefresh(t *testing.T) {
	t.Setenv("UPSTREAM_MOCK", "")
	var c4Body atomic.Value
	c4Body.Store(`{"balance": {"credits": 1234.5}, "plan": {"allowance_per_month": 5000, "allowance_left": 420}, "resets_at": "2026-11-01T00:00:00Z"}`)
	var jinaFail atomic.Bool
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/team/credit-usage":
			fmt.Fprint(w, `{"success":true,"data":{"remainingCredits":4321,"planCredits":100000,"billingPeriodStart":"2026-10-01T00:00:00Z","billingPeriodEnd":"2026-11-01T00:00:00Z"}}`)
		case r.URL.Path == "/v1/billing/balance":
			w.Header().Set("x-c4-balance", "999")
			fmt.Fprint(w, c4Body.Load().(string))
		case r.URL.Path == "/api/v1/api_key/fe_user":
			if jinaFail.Load() || r.URL.Query().Get("api_key") != "jina-key-1" {
				w.WriteHeader(401)
				return
			}
			fmt.Fprint(w, `{"wallet":{"total_balance":9876543}}`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer fake.Close()
	oldF, oldC, oldJ := firecrawlBase, crawl4aiBase, jinaDashBase
	firecrawlBase, crawl4aiBase, jinaDashBase = fake.URL, fake.URL, fake.URL
	defer func() { firecrawlBase, crawl4aiBase, jinaDashBase = oldF, oldC, oldJ }()
	d := t.TempDir()
	st, err := Open(Config{DBPath: filepath.Join(d, "news.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "secrets.json"), Desktop: true})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sec := st.loadSecrets()
	sec.Fetch = FetchKeys{JinaKey: "jina-key-1", FirecrawlKey: "fc", Crawl4aiKey: "c4"}
	st.saveSecrets(sec)

	if q := st.refreshQuota("firecrawl"); !q.OK || *q.Remaining != 4321 || *q.Total != 100000 || q.ResetAt != "2026-11-01T00:00:00Z" {
		t.Fatalf("firecrawl: %+v", q)
	}
	if q := st.refreshQuota("crawl4ai"); !q.OK || *q.Remaining != 1234.5 || *q.Allowance != 420 || *q.Total != 5000 || q.ResetAt == "" {
		t.Fatalf("crawl4ai: %+v", q)
	}
	c4Body.Store(`{"weird": true}`) // unknown body: the balance header still counts
	if q := st.refreshQuota("crawl4ai"); !q.OK || *q.Remaining != 999 {
		t.Fatalf("crawl4ai header fallback: %+v", q)
	}
	c4Body.Store(`not json`)
	if q := st.refreshQuota("crawl4ai"); q == nil {
		t.Fatal("crawl4ai nil")
	}
	if q := st.refreshQuota("jina"); !q.OK || *q.Remaining != 9876543 {
		t.Fatalf("jina: %+v", q)
	}
	jinaFail.Store(true)
	q := st.refreshQuota("jina")
	if q.OK || !strings.HasPrefix(q.Error, "不可用") || strings.Contains(q.Error, "jina-key-1") {
		t.Fatalf("jina failure: %+v", q)
	}
}

func TestFetchAPIHandlers(t *testing.T) {
	t.Setenv("UPSTREAM_MOCK", "http://127.0.0.1:9")
	for _, desktop := range []bool{false, true} {
		d := t.TempDir()
		st, err := Open(Config{DBPath: filepath.Join(d, "news.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "secrets.json"), Desktop: desktop})
		if err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(st.Server(nil))
		var pub map[string]any
		code := postJSON(t, srv.URL+"/api/fetchapi", "PATCH", map[string]any{"jina": map[string]any{"key": "jk-secret-1234567890", "baseUrl": "https://r.jinaai.cn"}, "firecrawl": map[string]any{"key": "fc-secret-4567890123"}}, &pub)
		if !desktop {
			r, _ := http.Get(srv.URL + "/api/fetchapi")
			if code != 404 || r.StatusCode != 404 {
				t.Fatalf("web: PATCH %d GET %d", code, r.StatusCode)
			}
		} else {
			b, _ := json.Marshal(pub)
			if code != 200 || strings.Contains(string(b), "secret") || !strings.Contains(string(b), `"baseUrl":"https://r.jinaai.cn"`) || !strings.Contains(string(b), `"configured":true`) {
				t.Fatalf("desktop PATCH %d: %s", code, b)
			}
			fi, _ := os.Stat(filepath.Join(d, "secrets.json"))
			if fi.Mode().Perm() != 0o600 {
				t.Fatalf("secrets mode %v", fi.Mode())
			}
			if c := postJSON(t, srv.URL+"/api/fetchapi", "PATCH", map[string]any{"jina": map[string]any{"baseUrl": "ftp://bad"}}, nil); c != 400 {
				t.Fatalf("bad base accepted: %d", c)
			}
			postJSON(t, srv.URL+"/api/fetchapi", "PATCH", map[string]any{"jina": map[string]any{"clearKey": true, "baseUrl": ""}}, &pub)
			if k := st.fetchKeys(); k.JinaKey != "" || k.jinaBase() != jinaDefaultBase || k.FirecrawlKey != "fc-secret-4567890123" {
				t.Fatalf("clear: %+v", k.JinaBase)
			}
		}
		srv.Close()
		st.Close()
	}
}
