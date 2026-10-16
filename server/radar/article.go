package radar

import (
	"context"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Original-article text for 「AI 润色」: fetch the item's link (timeout, size cap via netFetch), pull the
// readable paragraphs out (readability-lite: drop boilerplate blocks, prefer <article>/<main>, keep the
// substantial <p>s), cap the length. Every fetch reports how it went (articleRes.Status) so the brief can say
// which items had no full text; social / login-walled hosts are skipped.

const (
	articleMaxRunes = 6000
	articleFullMin  = 400 // fewer readable runes than this = partial (teaser / meta description only)
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
func extractArticle(html string) string { t, _ := extractArticleKind(html); return t }

// extractArticleKind also tells whether the text came from the page's paragraphs or only its meta description.
func extractArticleKind(html string) (string, bool) {
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
	fromDesc := false
	if text == "" {
		text, fromDesc = desc, true
	}
	if reLoginWall.MatchString(text) && utf8.RuneCountInString(text) < 400 {
		return "", false
	}
	if r := []rune(text); len(r) > articleMaxRunes {
		text = string(r[:articleMaxRunes]) + "…"
	}
	return strings.TrimSpace(text), !fromDesc
}

// articleRes is one fetch: Status full (≥ articleFullMin readable runes) | partial (teaser or meta description)
// | skipped (social / login host) | failed; Reason says why it is not full.
type articleRes struct {
	Text   string `json:"-"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type articleCacheT struct {
	mu sync.Mutex
	m  map[string]articleCached
}

type articleCached struct {
	at  time.Time
	res articleRes
}

var gArticles = &articleCacheT{m: map[string]articleCached{}}

func fetchArticle(ctx context.Context, u string) string { return fetchArticleRes(ctx, u).Text }

func fetchArticleRes(ctx context.Context, u string) articleRes {
	if skipArticleHost(u) {
		return articleRes{Status: "skipped", Reason: "社交平台 / 需登录的站点，不抓取"}
	}
	c := gArticles
	c.mu.Lock()
	if e, ok := c.m[u]; ok && time.Since(e.at) < articleTTL {
		c.mu.Unlock()
		return e.res
	}
	c.mu.Unlock()
	res := articleRes{Status: "failed"}
	if t, blurb := techmemeTarget(ctx, u); t != "" && t != u { // aggregator page → the story it links to
		res = fetchArticleRes(ctx, t)
		if res.Status != "full" {
			res.Reason = "Techmeme 原文（" + artHost(t) + "）：" + res.Reason
			if len([]rune(blurb)) > len([]rune(res.Text)) {
				res.Text, res.Status = blurb, "partial" // Techmeme's own headline + excerpt
			}
		}
		c.mu.Lock()
		c.m[u] = articleCached{time.Now(), res}
		c.mu.Unlock()
		return res
	}
	r, err := netFetch(ctx, "GET", u, map[string]string{"Accept": "text/html,application/xhtml+xml", "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15 AI-News-Radar"}, nil, articleTimeout)
	ct := ""
	if r != nil {
		ct = strings.ToLower(r.Header.Get("Content-Type"))
	}
	switch {
	case err != nil:
		res.Reason = "请求失败：" + jsSlice(err.Error(), 80)
	case !r.OK():
		res.Reason = "HTTP " + strconv.Itoa(r.Status)
	case strings.Contains(ct, "pdf"):
		res.Reason = "PDF，未解析"
	case !utf8.Valid(r.Body) && !strings.Contains(ct, "html"):
		res.Reason = "不是网页（" + ct + "）"
	default:
		b := r.Body
		if len(b) > 3<<20 {
			b = b[:3<<20]
		}
		text, paras := extractArticleKind(string(b))
		switch {
		case text == "":
			res.Reason = "页面没有可读正文（可能需要 JavaScript 渲染或登录）"
		case !paras || utf8.RuneCountInString(text) < articleFullMin:
			res.Text, res.Status, res.Reason = text, "partial", "只取到摘要 / 开头"
		default:
			res.Text, res.Status = text, "full"
		}
	}
	c.mu.Lock()
	c.m[u] = articleCached{time.Now(), res}
	c.mu.Unlock()
	return res
}

// fetchArticlesRes returns url → result for the given urls (bounded concurrency, overall deadline).
func fetchArticlesRes(urls []string) map[string]articleRes {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out := map[string]articleRes{}
	var mu sync.Mutex
	sem := make(chan struct{}, articleConc)
	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			res := articleRes{Status: "failed", Reason: "超时（整体 30 秒）"}
			select {
			case sem <- struct{}{}:
				res = fetchArticleRes(ctx, u)
				<-sem
			case <-ctx.Done():
			}
			mu.Lock()
			out[u] = res
			mu.Unlock()
		}(u)
	}
	wg.Wait()
	return out
}

// fetchArticles returns url → text (only the urls with some text).
func fetchArticles(urls []string) map[string]string {
	out := map[string]string{}
	for u, r := range fetchArticlesRes(urls) {
		if r.Text != "" {
			out[u] = r.Text
		}
	}
	return out
}

var (
	reTMAnchor = regexp.MustCompile(`(?is)CLASS="ourh"\s+HREF="([^"]+)"`)
	reTMBlurb  = regexp.MustCompile(`(?is)<DIV CLASS="ii">(.*?)</DIV>`)
)

// techmemeTarget: a Techmeme item link (…/261016/p13#a261016p13) → the headline's original article.
func techmemeTarget(ctx context.Context, u string) (target, blurb string) {
	p, err := url.Parse(u)
	if err != nil || !strings.HasSuffix(strings.ToLower(p.Hostname()), "techmeme.com") || p.Fragment == "" {
		return "", ""
	}
	page := *p
	page.Fragment = ""
	r, err := netFetch(ctx, "GET", page.String(), map[string]string{"User-Agent": "Mozilla/5.0 (Macintosh) AI-News-Radar"}, nil, articleTimeout)
	if err != nil || !r.OK() {
		return "", ""
	}
	html := string(r.Body)
	i := strings.Index(html, `NAME="`+p.Fragment+`"`)
	if i < 0 {
		return "", ""
	}
	end := min(len(html), i+6000)
	if m := reTMBlurb.FindStringSubmatch(html[i:end]); m != nil {
		blurb = Clean(Decode(m[1]), 1200)
	}
	if m := reTMAnchor.FindStringSubmatch(html[i:end]); m != nil && strings.HasPrefix(m[1], "http") {
		return Decode(m[1]), blurb
	}
	return "", ""
}

func artHost(u string) string {
	if p, err := url.Parse(u); err == nil {
		return strings.TrimPrefix(p.Hostname(), "www.")
	}
	return u
}
