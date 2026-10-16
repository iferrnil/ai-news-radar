package radar

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Original-article excerpts for 「AI 润色」: fetch the item's link (timeout, size cap via netFetch), pull the
// readable paragraphs out (readability-lite: drop boilerplate blocks, prefer <article>/<main>, keep the
// substantial <p>s), cap the length. Social / login-walled hosts are skipped; failures just mean no excerpt.

const (
	articleMaxRunes = 1800
	articleTimeout  = 8 * time.Second
	articleConc     = 6
	articleTTL      = 30 * time.Minute
)

var articleSkipHosts = []string{"x.com", "twitter.com", "t.co", "linkedin.com", "facebook.com", "instagram.com", "threads.net", "weibo.com", "youtube.com", "youtu.be", "bsky.app"}

var (
	reDropBlocks = regexp.MustCompile(`(?is)<(script|style|noscript|svg|nav|header|footer|aside|form|figure|iframe|template|button)\b[^>]*>.*?</\s*(?:script|style|noscript|svg|nav|header|footer|aside|form|figure|iframe|template|button)\s*>`)
	reComments   = regexp.MustCompile(`(?s)<!--.*?-->`)
	reArticle    = regexp.MustCompile(`(?is)<article\b[^>]*>(.*)</article>`)
	reMain       = regexp.MustCompile(`(?is)<main\b[^>]*>(.*)</main>`)
	reBody       = regexp.MustCompile(`(?is)<body\b[^>]*>(.*)</body>`)
	rePara       = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</p>`)
	reMetaDesc   = regexp.MustCompile(`(?is)<meta[^>]+(?:property|name)=["'](?:og:description|description)["'][^>]*content=["']([^"']+)["']`)
	reLoginWall  = regexp.MustCompile(`(?i)(sign in to continue|log in to continue|subscribe to continue|请登录|登录后查看|登录以继续|this content is for subscribers)`)
)

func skipArticleHost(u string) bool {
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "http" && p.Scheme != "https") {
		return true
	}
	h := strings.TrimPrefix(strings.ToLower(p.Hostname()), "www.")
	for _, s := range articleSkipHosts {
		if h == s || strings.HasSuffix(h, "."+s) {
			return true
		}
	}
	return false
}

// extractArticle returns the main text of an HTML page ("" when nothing useful).
func extractArticle(html string) string {
	html = reComments.ReplaceAllString(html, " ")
	desc := ""
	if m := reMetaDesc.FindStringSubmatch(html); m != nil {
		desc = Clean(Decode(m[1]), 600)
	}
	html = reDropBlocks.ReplaceAllString(html, " ")
	scope := html
	for _, re := range []*regexp.Regexp{reArticle, reMain, reBody} {
		if m := re.FindStringSubmatch(html); m != nil {
			scope = m[1]
			break
		}
	}
	var parts []string
	n := 0
	for _, m := range rePara.FindAllStringSubmatch(scope, -1) {
		t := Clean(m[1], 2000)
		rc := utf8.RuneCountInString(t)
		cjk := len(cjkRe.FindAllStringIndex(t, -1))
		if rc < 40 && cjk < 18 {
			continue // bylines, captions, buttons
		}
		parts = append(parts, t)
		n += rc
		if n >= articleMaxRunes {
			break
		}
	}
	text := strings.Join(parts, "\n")
	if text == "" {
		text = desc
	}
	if reLoginWall.MatchString(text) && utf8.RuneCountInString(text) < 400 {
		return ""
	}
	if r := []rune(text); len(r) > articleMaxRunes {
		text = string(r[:articleMaxRunes]) + "…"
	}
	return strings.TrimSpace(text)
}

type articleCacheT struct {
	mu sync.Mutex
	m  map[string]struct {
		at   time.Time
		text string
	}
}

var gArticles = &articleCacheT{m: map[string]struct {
	at   time.Time
	text string
}{}}

func fetchArticle(ctx context.Context, u string) string {
	if skipArticleHost(u) {
		return ""
	}
	c := gArticles
	c.mu.Lock()
	if e, ok := c.m[u]; ok && time.Since(e.at) < articleTTL {
		c.mu.Unlock()
		return e.text
	}
	c.mu.Unlock()
	r, err := netFetch(ctx, "GET", u, map[string]string{"Accept": "text/html,application/xhtml+xml", "User-Agent": "Mozilla/5.0 (compatible; AI-News-Radar)"}, nil, articleTimeout)
	text := ""
	if err == nil && r.OK() && !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "pdf") {
		b := r.Body
		if len(b) > 3<<20 {
			b = b[:3<<20]
		}
		if utf8.Valid(b) || strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "html") {
			text = extractArticle(string(b))
		}
	}
	c.mu.Lock()
	c.m[u] = struct {
		at   time.Time
		text string
	}{time.Now(), text}
	c.mu.Unlock()
	return text
}

// fetchArticles returns url → excerpt for the given urls (bounded concurrency, overall deadline).
func fetchArticles(urls []string) map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	out := map[string]string{}
	var mu sync.Mutex
	sem := make(chan struct{}, articleConc)
	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if t := fetchArticle(ctx, u); t != "" {
				mu.Lock()
				out[u] = t
				mu.Unlock()
			}
		}(u)
	}
	wg.Wait()
	return out
}
