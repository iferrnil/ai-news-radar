package radar

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func para(prefix string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "<p>%s paragraph %d explains the details of the announcement, with enough words to read like prose, and it ends properly.</p>\n", prefix, i)
	}
	return b.String()
}

func TestExtractReadabilityTailwind(t *testing.T) {
	// the wrapper's arbitrary Tailwind class mentions "header"; a related-story card sits next to the body
	page := `<html><head><title>T</title><meta property="og:description" content="short teaser"></head><body>
<div class="[--header-h:64px] min-h-screen"><nav class="menu"><a href="/a">A</a><a href="/b">B</a></nav>
<main><div class="grid"><div class="col-span-8">` + para("Body", 12) + `</div>
<aside class="related"><a href="/x"><p>Related teaser story that is short.</p></a></aside></div></main>
<footer class="footer"><p>Copyright notice and links that should not appear in the article.</p></footer></div></body></html>`
	ex := extractPage(page)
	if !ex.Full || !strings.Contains(ex.Text, "Body paragraph 11") || strings.Contains(ex.Text, "Copyright") || strings.Contains(ex.Text, "Related teaser") {
		t.Fatalf("method %s full %v: %.300q", ex.Method, ex.Full, ex.Text)
	}
}

func TestExtractJSONLDAndNextData(t *testing.T) {
	body := strings.Repeat("JSON-LD article body sentence that carries the facts. ", 20)
	page := `<html><head><script type="application/ld+json">{"@context":"https://schema.org","@graph":[{"@type":"WebPage"},{"@type":"NewsArticle","headline":"H","articleBody":"` + body + `"}]}</script></head><body><div id="root"></div></body></html>`
	if ex := extractPage(page); ex.Method != "json-ld" || !ex.Full || !strings.Contains(ex.Text, "JSON-LD article body") {
		t.Fatalf("json-ld: %+v", ex)
	}
	nd := strings.Repeat("Embedded Next.js post content sentence with real words. ", 20)
	page = `<html><body><div id="__next"></div><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"post":{"title":"x","content":"<p>` + nd + `</p>"}}}}</script></body></html>`
	if ex := extractPage(page); ex.Method != "embedded" || !ex.Full || !strings.Contains(ex.Text, "Embedded Next.js") || strings.Contains(ex.Text, "<p>") {
		t.Fatalf("next data: %+v", ex)
	}
	// only meta: partial with the description
	if ex := extractPage(`<html><head><meta name="description" content="Only the meta description is here."></head><body></body></html>`); ex.Full || !strings.Contains(ex.Text, "meta description") {
		t.Fatalf("meta: %+v", ex)
	}
}

func TestCharsetDetection(t *testing.T) {
	zh := "人工智能公司今天发布了新的大模型，这是一段用来测试编码识别的中文正文。"
	gbk, _ := simplifiedchinese.GBK.NewEncoder().String(zh)
	for name, c := range map[string]struct{ body, ct string }{
		"meta":       {`<html><head><meta charset="gbk"></head><body><p>` + gbk + `</p></body></html>`, "text/html"},
		"header":     {`<html><body><p>` + gbk + `</p></body></html>`, "text/html; charset=GB2312"},
		"undeclared": {`<html><body><p>` + gbk + `</p></body></html>`, "text/html"},
		"xml":        {`<?xml version="1.0" encoding="gb18030"?><rss><p>` + gbk + `</p></rss>`, "application/xml"},
		"utf8-bom":   {"\xEF\xBB\xBF<p>" + zh + "</p>", "text/html; charset=gbk"},
		"utf8-plain": {"<p>" + zh + "</p>", "text/html"},
	} {
		got, _ := toUTF8([]byte(c.body), c.ct, "example.cn")
		if !strings.Contains(got, zh) {
			t.Fatalf("%s: %q", name, got)
		}
	}
	if got, _ := toUTF8([]byte("caf\xe9 na\xefve"), "text/html; charset=iso-8859-1", "e.com"); got != "café naïve" {
		t.Fatalf("latin1: %q", got)
	}
}

// articleMock serves pages for many "hosts" through UPSTREAM_MOCK (?u=<url>), recording concurrency per host.
type articleMock struct {
	mu            sync.Mutex
	cur, peak     map[string]int
	all, allPeak  int
	hits          map[string]int
	delay         time.Duration
	handler       func(w http.ResponseWriter, u *url.URL) bool
	lastUA, lastL string
}

func (m *articleMock) serve(w http.ResponseWriter, r *http.Request) {
	u, _ := url.Parse(r.URL.Query().Get("u"))
	h := u.Host
	m.mu.Lock()
	m.cur[h]++
	m.all++
	m.hits[u.String()]++
	if m.cur[h] > m.peak[h] {
		m.peak[h] = m.cur[h]
	}
	if m.all > m.allPeak {
		m.allPeak = m.all
	}
	m.lastUA, m.lastL = r.Header.Get("User-Agent"), r.Header.Get("Accept-Language")
	m.mu.Unlock()
	defer func() { m.mu.Lock(); m.cur[h]--; m.all--; m.mu.Unlock() }()
	time.Sleep(m.delay)
	if m.handler != nil && m.handler(w, u) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<html><body><article><h1>%s</h1>%s</article></body></html>", u.Path, para("Story "+u.Path, 8))
}

func newArticleMock(t *testing.T) *articleMock {
	m := &articleMock{cur: map[string]int{}, peak: map[string]int{}, hits: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(srv.Close)
	t.Setenv("UPSTREAM_MOCK", srv.URL)
	old := gFetch
	gFetch = newArtFetcher(10, 2)
	gFetch.retry = 20 * time.Millisecond
	t.Cleanup(func() { gFetch = old })
	gArticles.mu.Lock()
	gArticles.m = map[string]articleCached{}
	gArticles.mu.Unlock()
	return m
}

func TestFetcherConcurrencyLimits(t *testing.T) {
	m := newArticleMock(t)
	m.delay = 150 * time.Millisecond
	var urls []string
	for i := 0; i < 6; i++ { // 6 on one host, 14 on separate hosts
		urls = append(urls, fmt.Sprintf("https://same.example/a%d", i))
	}
	for i := 0; i < 14; i++ {
		urls = append(urls, fmt.Sprintf("https://h%d.example/a", i))
	}
	t0 := time.Now()
	res := fetchArticlesRes(urls)
	took := time.Since(t0)
	full := 0
	for _, r := range res {
		if r.Status == "full" && r.Method == "self" {
			full++
		}
	}
	if full != 20 || m.peak["same.example"] > 2 || m.allPeak > 10 || m.allPeak < 8 {
		t.Fatalf("full %d, same-host peak %d, global peak %d", full, m.peak["same.example"], m.allPeak)
	}
	if took > 2*time.Second {
		t.Fatalf("not concurrent: %v", took)
	}
	if !strings.Contains(m.lastUA, "Chrome/") || !strings.HasPrefix(m.lastL, "zh-CN") {
		t.Fatalf("headers: %q %q", m.lastUA, m.lastL)
	}
	if d := articleDeadline(urls[:10]); d < 20*time.Second || d > 75*time.Second {
		t.Fatalf("deadline for 10: %v", d)
	}
}

func TestFetcherEncodingsRetryAndBackoff(t *testing.T) {
	m := newArticleMock(t)
	var flaky atomic.Int32
	m.handler = func(w http.ResponseWriter, u *url.URL) bool {
		html := "<html><body><article>" + para("Enc "+u.Path, 8) + "</article></body></html>"
		switch u.Path {
		case "/gz":
			var b bytes.Buffer
			zw := gzip.NewWriter(&b)
			zw.Write([]byte(html))
			zw.Close()
			w.Header().Set("Content-Encoding", "gzip")
			w.Write(b.Bytes())
		case "/br":
			var b bytes.Buffer
			bw := brotli.NewWriter(&b)
			bw.Write([]byte(html))
			bw.Close()
			w.Header().Set("Content-Encoding", "br")
			w.Write(b.Bytes())
		case "/flaky":
			if flaky.Add(1) == 1 {
				w.WriteHeader(503)
				return true
			}
			w.Write([]byte(html))
		case "/limited":
			w.WriteHeader(429)
		default:
			return false
		}
		return true
	}
	res := fetchArticlesRes([]string{"https://a.example/gz", "https://b.example/br", "https://c.example/flaky", "https://d.example/limited"})
	for _, p := range []string{"https://a.example/gz", "https://b.example/br", "https://c.example/flaky"} {
		if r := res[p]; r.Status != "full" || !strings.Contains(r.Text, "Enc ") {
			t.Fatalf("%s: %+v", p, r)
		}
	}
	if flaky.Load() != 2 {
		t.Fatalf("flaky hits %d, want 1 retry", flaky.Load())
	}
	if r := res["https://d.example/limited"]; r.Status != "failed" || !strings.Contains(r.Reason, "429") {
		t.Fatalf("429: %+v", r)
	}
	// the host is cooling down: another url on it fails without a request
	r := fetchArticleRes(context.Background(), "https://d.example/other")
	if r.Status != "failed" || m.hits["https://d.example/other"] != 0 {
		t.Fatalf("no host backoff: %+v hits %d", r, m.hits["https://d.example/other"])
	}
}
