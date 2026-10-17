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

// Original-article text for briefs (AI 润色) and 生成文章: fetch the item's link with the self-written fetcher
// (fetch.go: browser headers, charsets, retry, host limits) and extract the main text (extract.go). Every fetch
// reports how it went (articleRes.Status) so the brief can say which items had no full text; social /
// login-walled hosts are skipped (an X post's own text is its full content).

const (
	articleMaxRunes = 6000
	articleFullMin  = 400 // fewer readable runes than this = partial (teaser / meta description only)
	articleTimeout  = 10 * time.Second
	articleTTL      = 30 * time.Minute
	articleFailTTL  = 2 * time.Minute // failures are retried sooner
)

var articleSkipHosts = []string{"x.com", "twitter.com", "t.co", "linkedin.com", "facebook.com", "instagram.com", "threads.net", "weibo.com", "youtube.com", "youtu.be", "bsky.app"}

var reLoginWall = regexp.MustCompile(`(?i)(sign in to continue|log in to continue|subscribe to continue|请登录|登录后查看|登录以继续|this content is for subscribers)`)

// articleRoute: per-site handling. Some pages are JavaScript apps whose text sits behind a public JSON endpoint
// (fetched instead, then read by the embedded-JSON extractor); some links have no article at all.
var (
	reJQZXArticle = regexp.MustCompile(`^https?://(?:www\.)?jiqizhixin\.com/articles/([\w-]+)`)
	reBiliVideo   = regexp.MustCompile(`^https?://(?:www\.|m\.)?bilibili\.com/video/(BV\w+)`)
)

func articleRoute(u string) (fetch, skip string) {
	if m := reJQZXArticle.FindStringSubmatch(u); m != nil {
		return "https://www.jiqizhixin.com/api/article_library/articles/" + m[1] + ".json", ""
	}
	if m := reBiliVideo.FindStringSubmatch(u); m != nil {
		return "https://api.bilibili.com/x/web-interface/view?bvid=" + m[1], ""
	}
	h := artHost(u)
	switch {
	case (h == "douyin.com" || strings.HasSuffix(h, ".douyin.com")) && strings.Contains(u, "/search/"):
		return "", "抖音热搜词，没有正文页"
	case h == "jandan.net" && strings.Contains(u, "/t/"):
		return "", "煎蛋评论：条目本身就是全文"
	}
	return u, ""
}

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
func extractArticle(page string) string { return extractPage(page).Text }

// articleRes is one fetch: Status full (≥ articleFullMin readable runes) | partial (teaser or meta description)
// | skipped (social / login host) | failed; Reason says why it is not full. Method = self | jina | firecrawl |
// crawl4ai (who fetched it); Extract = json-ld | dom | embedded | meta (self only).
type articleRes struct {
	Text    string        `json:"-"`
	Title   string        `json:"-"`
	Status  string        `json:"status"`
	Reason  string        `json:"reason,omitempty"`
	Method  string        `json:"method,omitempty"`
	Extract string        `json:"extract,omitempty"`
	Took    time.Duration `json:"-"`
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

func (c *articleCacheT) get(u string) (articleRes, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[u]
	if !ok {
		if r, ok := gDisk.load(u); ok { // 原文缓存 (desktop, 24 h)
			c.m[u] = articleCached{time.Now(), r}
			return r, true
		}
		return articleRes{}, false
	}
	ttl := articleTTL
	if e.res.Status == "failed" {
		ttl = articleFailTTL
	}
	if time.Since(e.at) >= ttl {
		delete(c.m, u)
		return articleRes{}, false
	}
	return e.res, true
}

func (c *articleCacheT) put(u string, r articleRes) {
	c.mu.Lock()
	if len(c.m) > 2000 { // simple bound: drop everything stale
		for k, e := range c.m {
			if time.Since(e.at) > articleTTL {
				delete(c.m, k)
			}
		}
	}
	c.m[u] = articleCached{time.Now(), r}
	c.mu.Unlock()
	if r.Status == "full" {
		gDisk.save(u, r)
	}
}

func fetchArticle(ctx context.Context, u string) string { return fetchArticleRes(ctx, u).Text }

func fetchArticleRes(ctx context.Context, u string) articleRes {
	if skipArticleHost(u) {
		return articleRes{Status: "skipped", Reason: "社交平台 / 需登录的站点，不抓取", Method: "self"}
	}
	if r, ok := gArticles.get(u); ok {
		return r
	}
	t0 := time.Now()
	res := articleRes{Status: "failed", Method: "self"}
	if t, blurb := techmemeTarget(ctx, u); t != "" && t != u { // aggregator page → the story it links to
		res = fetchArticleRes(ctx, t)
		if res.Status != "full" {
			res.Reason = "Techmeme 原文（" + artHost(t) + "）：" + res.Reason
			if len([]rune(blurb)) > len([]rune(res.Text)) {
				res.Text, res.Status, res.Extract = blurb, "partial", "techmeme" // Techmeme's own headline + excerpt
			}
		}
		res.Took = time.Since(t0)
		if ctx.Err() == nil {
			gArticles.put(u, res)
		}
		return res
	}
	target, skip := articleRoute(u)
	if skip != "" {
		res.Status, res.Reason = "skipped", skip
		gArticles.put(u, res)
		return res
	}
	r, err := gFetch.get(ctx, target, articleTimeout)
	ct := ""
	if r != nil {
		ct = strings.ToLower(r.Header.Get("Content-Type"))
	}
	switch {
	case err != nil:
		res.Reason = "请求失败：" + jsSlice(fetchErrText(err), 80)
	case r.Status < 200 || r.Status >= 300:
		res.Reason = "HTTP " + strconv.Itoa(r.Status)
	case strings.Contains(ct, "pdf"):
		res.Reason = "PDF，未解析"
	case strings.Contains(ct, "json"): // a site's JSON endpoint (articleRoute)
		res.Extract = "embedded"
		switch t := capArticle(embeddedBody([]string{r.Text})); {
		case utf8.RuneCountInString(t) >= articleFullMin:
			res.Text, res.Status = t, "full"
		case t != "":
			res.Text, res.Status, res.Reason = t, "partial", "只取到摘要 / 开头"
		default:
			res.Reason = "接口没有正文"
		}
	case ct != "" && !strings.Contains(ct, "html") && !strings.Contains(ct, "xml") && !strings.Contains(ct, "text/plain"):
		res.Reason = "不是网页（" + ct + "）"
	default:
		ex := extractPage(r.Text)
		res.Title, res.Extract = ex.Title, ex.Method
		switch {
		case ex.Text == "":
			res.Reason = "页面没有可读正文（可能需要 JavaScript 渲染或登录）"
		case !ex.Full || utf8.RuneCountInString(ex.Text) < articleFullMin:
			res.Text, res.Status, res.Reason = ex.Text, "partial", "只取到摘要 / 开头"
		default:
			res.Text, res.Status = ex.Text, "full"
		}
	}
	res.Took = time.Since(t0)
	if ctx.Err() == nil { // a deadline cut is not the page's fault: don't remember it
		gArticles.put(u, res)
	}
	return res
}

func fetchErrText(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "deadline exceeded") || strings.Contains(s, "Client.Timeout") || strings.Contains(s, "timeout"):
		return "超时"
	case strings.Contains(s, "no such host"):
		return "域名解析失败"
	case strings.Contains(s, "connection refused"):
		return "连接被拒绝"
	case strings.Contains(s, "connection reset") || strings.Contains(s, "EOF"):
		return "连接被重置"
	case strings.Contains(s, "certificate"):
		return "证书错误"
	}
	return s
}

// articleDeadline: enough for every url to get its turn — one wave per two urls of the busiest host (per-host
// cap), plus a retry; ten distinct hosts = one wave.
func articleDeadline(urls []string) time.Duration {
	per := map[string]int{}
	waves := 1
	for _, u := range urls {
		h := artHost(u)
		per[h]++
		if w := (per[h] + gFetch.perHost - 1) / gFetch.perHost; w > waves {
			waves = w
		}
	}
	if g := (len(urls) + cap(gFetch.global) - 1) / cap(gFetch.global); g > waves {
		waves = g
	}
	d := time.Duration(waves)*articleTimeout + articleTimeout + 5*time.Second // + retry / Techmeme hop
	return min(d, 75*time.Second)
}

// fetchArticlesRes returns url → result for the given urls (global + per-host limits, overall deadline).
func fetchArticlesRes(urls []string) map[string]articleRes {
	urls = uniq(urls)
	dl := articleDeadline(urls)
	ctx, cancel := context.WithTimeout(context.Background(), dl)
	defer cancel()
	out := map[string]articleRes{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			res := fetchArticleRes(ctx, u)
			if ctx.Err() != nil && res.Status == "failed" {
				res.Reason = "超时（整体 " + strconv.Itoa(int(dl.Seconds())) + " 秒）"
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

// the Techmeme front page is shared by all its items: fetched once per couple of minutes
var tmPages = struct {
	mu sync.Mutex
	m  map[string]*tmPage
}{m: map[string]*tmPage{}}

type tmPage struct {
	once sync.Once
	at   time.Time
	html string
}

func techmemePage(ctx context.Context, page string) string {
	tmPages.mu.Lock()
	p := tmPages.m[page]
	if p == nil || (time.Since(p.at) > 3*time.Minute && !p.at.IsZero()) {
		p = &tmPage{}
		tmPages.m[page] = p
	}
	tmPages.mu.Unlock()
	p.once.Do(func() {
		if r, err := gFetch.get(ctx, page, articleTimeout); err == nil && r.Status == 200 {
			p.html = r.Text
		}
		p.at = time.Now()
		if p.html == "" { // failed: let the next caller try again
			tmPages.mu.Lock()
			if tmPages.m[page] == p {
				delete(tmPages.m, page)
			}
			tmPages.mu.Unlock()
		}
	})
	return p.html
}

// techmemeTarget: a Techmeme item link (…/261016/p13#a261016p13) → the headline's original article.
func techmemeTarget(ctx context.Context, u string) (target, blurb string) {
	p, err := url.Parse(u)
	if err != nil || !strings.HasSuffix(strings.ToLower(p.Hostname()), "techmeme.com") || p.Fragment == "" {
		return "", ""
	}
	page := *p
	page.Fragment = ""
	html := techmemePage(ctx, page.String())
	if html == "" {
		return "", ""
	}
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
		return strings.TrimPrefix(strings.ToLower(p.Hostname()), "www.")
	}
	return u
}
