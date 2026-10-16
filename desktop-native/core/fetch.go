package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var client = &http.Client{Timeout: 20 * time.Second}

type validator struct{ etag, lm string }

var (
	valMu      sync.Mutex
	validators = map[string]validator{}
)

func backoff(i int, base, max time.Duration) time.Duration {
	d := base << i
	if d > max {
		d = max
	}
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}

// httpGet: GET with retries on 429/5xx/network errors (Retry-After wins). cond=true sends ETag/Last-Modified.
func httpGet(ctx context.Context, u string, hdr map[string]string, cond bool, tries int) (*http.Response, []byte, error) {
	if tries <= 0 {
		tries = 3
	}
	var last error
	for i := 0; i < tries; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("User-Agent", UA)
		req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml;q=0.9, text/html;q=0.8, application/json;q=0.8, */*;q=0.5")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		if cond {
			valMu.Lock()
			v := validators[u]
			valMu.Unlock()
			if v.etag != "" {
				req.Header.Set("If-None-Match", v.etag)
			}
			if v.lm != "" {
				req.Header.Set("If-Modified-Since", v.lm)
			}
		}
		r, err := client.Do(req)
		if err != nil {
			last = err
			if i < tries-1 {
				sleepCtx(ctx, backoff(i, 1500*time.Millisecond, 15*time.Second))
			}
			continue
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		r.Body.Close()
		if err != nil {
			last = err
			continue
		}
		if (r.StatusCode == 429 || r.StatusCode >= 500) && i < tries-1 {
			d := backoff(i, 1500*time.Millisecond, 15*time.Second)
			if ra, e := strconv.Atoi(r.Header.Get("Retry-After")); e == nil && ra > 0 && ra < 120 {
				d = time.Duration(ra) * time.Second
			}
			sleepCtx(ctx, d)
			continue
		}
		if cond && r.StatusCode == 200 {
			valMu.Lock()
			if e, lm := r.Header.Get("ETag"), r.Header.Get("Last-Modified"); e != "" || lm != "" {
				validators[u] = validator{e, lm}
			} else {
				delete(validators, u)
			}
			valMu.Unlock()
		}
		return r, body, nil
	}
	if last == nil {
		last = errors.New("请求失败")
	}
	return nil, nil, last
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// FetchSource dispatches by type (rss/api/x/x_list).
func FetchSource(ctx context.Context, src Source) ([]FetchedItem, error) {
	switch src.Type {
	case "rss":
		return fetchRSS(ctx, src.URL)
	case "api":
		return fetchAPI(ctx, src.URL)
	case "x":
		return fetchX(ctx, src.Username)
	case "x_list":
		return fetchXList(ctx, src.URL)
	}
	return nil, fmt.Errorf("不支持的类型 %s", src.Type)
}

func fetchRSS(ctx context.Context, u string) ([]FetchedItem, error) {
	r, body, err := httpGet(ctx, u, nil, true, 3)
	if err != nil {
		return nil, err
	}
	if r.StatusCode == 304 {
		return nil, nil
	}
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", r.StatusCode)
	}
	f := ParseFeed(body)
	if f == nil {
		return nil, errors.New("不是有效的 RSS/Atom")
	}
	return f.Items, nil
}

var jsonpRe = regexp.MustCompile(`(?s)^[\w$.]+\((.*)\)\s*;?$`)

type jm = map[string]any

func js(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case nil:
		return ""
	}
	return fmt.Sprint(v)
}
func jn(v any) float64 { f, _ := v.(float64); return f }
func jo(v any) jm      { m, _ := v.(map[string]any); return m }
func ja(v any) []any   { a, _ := v.([]any); return a }

var cst = time.FixedZone("CST", 8*3600)

// ParseAPI implements the per-site JSON adapters of fetchers.ts fetchApi.
func ParseAPI(u string, raw []byte) ([]FetchedItem, error) {
	t := strings.TrimSpace(string(raw))
	if m := jsonpRe.FindStringSubmatch(t); m != nil {
		t = m[1]
	}
	var d any
	if err := json.Unmarshal([]byte(t), &d); err != nil {
		return nil, fmt.Errorf("JSON 解析失败：%v", err)
	}
	var out []FetchedItem
	switch {
	case strings.Contains(u, "ent.163.com/special/"):
		re := regexp.MustCompile(`(\d\d)/(\d\d)/(\d{4}) (\d\d):(\d\d):(\d\d)`)
		for _, x := range ja(d) {
			a := jo(x)
			if js(a["docurl"]) == "" || js(a["title"]) == "" {
				continue
			}
			var pub int64
			if m := re.FindStringSubmatch(js(a["time"])); m != nil {
				tm, _ := time.ParseInLocation("2006-01-02 15:04:05", m[3]+"-"+m[1]+"-"+m[2]+" "+m[4]+":"+m[5]+":"+m[6], cst)
				pub = tm.Unix()
			}
			var kw []string
			for _, k := range ja(a["keywords"]) {
				if s := js(jo(k)["keyname"]); s != "" {
					kw = append(kw, s)
				} else if s := js(k); s != "" && jo(k) == nil {
					kw = append(kw, s)
				}
			}
			out = append(out, FetchedItem{UID: NormURL(js(a["docurl"])), Title: Clean(js(a["title"]), 200), URL: js(a["docurl"]), Summary: strings.Join(kw, " · "), Published: pub})
		}
	case strings.Contains(u, "api.bilibili.com/x/web-interface/popular"):
		for _, x := range ja(jo(jo(d)["data"])["list"]) {
			v := jo(x)
			link := "https://www.bilibili.com/video/" + js(v["bvid"])
			owner := js(jo(v["owner"])["name"])
			img := ""
			if p := js(v["pic"]); p != "" {
				img = strings.Replace(p, "http:", "https:", 1) + "@480w_270h_1c.webp"
			}
			out = append(out, FetchedItem{UID: NormURL(link), Title: Clean(js(v["title"]), 200), URL: link, Summary: Clean("UP主 "+owner+" · "+js(v["desc"]), 280),
				Published: int64(jn(v["pubdate"])), Author: owner, Image: img})
		}
	case strings.Contains(u, "api.jandan.net/api/v1/comment/list/"):
		for _, x := range ja(jo(d)["data"]) {
			c := jo(x)
			id := js(c["id"])
			text := Clean(strings.ReplaceAll(js(c["content"]), "#img#", ""), 300)
			title := Trunc(text, 80, "")
			if title == "" {
				pt := js(c["post_title"])
				if pt == "" {
					pt = "煎蛋"
				}
				title = pt + " · " + js(c["author"])
			}
			sum := fmt.Sprintf("OO %s / XX %s", orZero(c["vote_positive"]), orZero(c["vote_negative"]))
			if text != "" {
				sum = text + " · " + sum
			}
			img := ""
			if imgs := ja(c["images"]); len(imgs) > 0 {
				img = js(jo(imgs[0])["url"])
			}
			out = append(out, FetchedItem{UID: "jandan:" + id, Title: title, URL: "https://jandan.net/t/" + id, Summary: sum, Published: ParseDate(js(c["date"])), Author: js(c["author"]), Image: img})
		}
	case strings.Contains(u, "hotsearch/billboard/word"):
		for _, x := range ja(jo(d)["word_list"]) {
			w := jo(x)
			word := js(w["word"])
			if word == "" {
				continue
			}
			hv := js(w["hot_value"])
			if hv == "" {
				hv = "?"
			}
			out = append(out, FetchedItem{UID: "douyin:" + word, Title: word, URL: "https://www.douyin.com/search/" + url.PathEscape(word), Summary: "抖音热搜 · 热度 " + hv})
		}
	case strings.Contains(u, "hn.algolia.com"):
		for _, x := range ja(jo(d)["hits"]) {
			h := jo(x)
			oid := js(h["objectID"])
			link := js(h["url"])
			if link == "" {
				link = "https://news.ycombinator.com/item?id=" + oid
			}
			pub := int64(jn(h["created_at_i"]))
			if pub == 0 {
				pub = ParseDate(js(h["created_at"]))
			}
			out = append(out, FetchedItem{UID: NormURL(link), Title: js(h["title"]), URL: link, Author: js(h["author"]), Published: pub,
				Summary: fmt.Sprintf("HN %s points · %s comments · https://news.ycombinator.com/item?id=%s", orZero(h["points"]), orZero(h["num_comments"]), oid)})
		}
	case strings.Contains(u, "huggingface.co/api/daily_papers"):
		for _, x := range ja(d) {
			e := jo(x)
			p := jo(e["paper"])
			link := "https://huggingface.co/papers/" + js(p["id"])
			title := js(p["title"])
			if title == "" {
				title = js(e["title"])
			}
			out = append(out, FetchedItem{UID: NormURL(link), Title: Clean(title, 300), URL: link, Summary: Clean(js(p["summary"]), 280),
				Published: ParseDate(firstNonEmpty(js(e["publishedAt"]), js(p["publishedAt"])))})
		}
	case strings.Contains(u, "jiqizhixin.com"):
		re := regexp.MustCompile(`(\d{4})/(\d\d)/(\d\d) (\d\d):(\d\d)`)
		for _, x := range ja(jo(d)["articles"]) {
			a := jo(x)
			link := "https://www.jiqizhixin.com/articles/" + js(a["slug"])
			var pub int64
			if m := re.FindStringSubmatch(js(a["publishedAt"])); m != nil {
				tm, _ := time.ParseInLocation("2006-01-02 15:04", m[1]+"-"+m[2]+"-"+m[3]+" "+m[4]+":"+m[5], cst)
				pub = tm.Unix()
			}
			out = append(out, FetchedItem{UID: NormURL(link), Title: js(a["title"]), URL: link, Summary: Clean(js(a["content"]), 280), Published: pub, Author: js(a["author"])})
		}
	case strings.Contains(u, "openrouter.ai/api/v1/models"):
		for _, x := range ja(jo(d)["data"]) {
			m := jo(x)
			id := js(m["id"])
			link := "https://openrouter.ai/" + id
			name := js(m["name"])
			if name == "" {
				name = id
			}
			price := ""
			if pr := jo(m["pricing"]); pr != nil && pr["prompt"] != nil {
				p, _ := strconv.ParseFloat(js(pr["prompt"]), 64)
				c, _ := strconv.ParseFloat(js(pr["completion"]), 64)
				price = fmt.Sprintf(" · $%.2f/$%.2f 每百万 token", p*1e6, c*1e6)
			}
			ctxl := js(m["context_length"])
			if ctxl == "" {
				ctxl = "?"
			}
			out = append(out, FetchedItem{UID: NormURL(link), Title: "OpenRouter 上线新模型：" + name, URL: link,
				Summary: id + " · context " + ctxl + price + " · " + Clean(js(m["description"]), 200), Published: int64(jn(m["created"]))})
		}
	default:
		return nil, errors.New("未知 API（没有适配器）")
	}
	return out, nil
}

func orZero(v any) string {
	if s := js(v); s != "" {
		return s
	}
	return "0"
}

func fetchAPI(ctx context.Context, u string) ([]FetchedItem, error) {
	r, body, err := httpGet(ctx, u, map[string]string{"Accept": "application/json"}, true, 3)
	if err != nil {
		return nil, err
	}
	if r.StatusCode == 304 {
		return nil, nil
	}
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", r.StatusCode)
	}
	return ParseAPI(u, body)
}

// ---- X via FxEmbed
var fxBases = []string{"https://api.fxtwitter.com/2"}

func fxGet(ctx context.Context, path string) (int, []byte, error) {
	all404 := true
	var lastErr error
	lastStatus := 0
	for _, base := range fxBases {
		for i := 0; i < 3; i++ {
			c, cancel := context.WithTimeout(ctx, 12*time.Second)
			r, body, err := httpGet(c, base+path, map[string]string{"User-Agent": "ai-news-crawler/1.0", "Accept": "application/json"}, false, 1)
			cancel()
			if err != nil {
				lastErr, all404 = err, false
			} else {
				if r.StatusCode == 200 || r.StatusCode == 204 {
					return r.StatusCode, body, nil
				}
				lastStatus = r.StatusCode
				if r.StatusCode != 404 {
					all404 = false
				}
				switch r.StatusCode {
				case 404, 429, 500, 502, 503, 504:
				default:
					return r.StatusCode, body, nil
				}
			}
			if i < 2 {
				sleepCtx(ctx, backoff(i, 800*time.Millisecond, 8*time.Second))
			}
		}
	}
	if all404 {
		return 404, nil, nil
	}
	if lastStatus != 0 {
		return 0, nil, fmt.Errorf("FxEmbed HTTP %d", lastStatus)
	}
	return 0, nil, fmt.Errorf("FxEmbed %v", lastErr)
}

func xItem(text, u, id, author string, ts int64, links []string) FetchedItem {
	first := strings.TrimSpace(regexp.MustCompile(`https?://\S+`).ReplaceAllString(text, ""))
	first, _, _ = strings.Cut(first, "\n")
	first = strings.TrimSpace(first)
	title := Trunc(first, 120, "…")
	if title == "" {
		title = u
	}
	return FetchedItem{UID: "x:" + id, Title: title, URL: u, Summary: Trunc(text, 4000, ""), Published: ts, IsX: true, Author: author, Links: links}
}

func fxProfile(ctx context.Context, handle string) (jm, error) {
	st, body, err := fxGet(ctx, "/profile/"+url.PathEscape(handle))
	if err != nil {
		return nil, err
	}
	if st == 404 {
		return nil, nil
	}
	if st != 200 {
		return nil, fmt.Errorf("FxEmbed HTTP %d", st)
	}
	var d jm
	json.Unmarshal(body, &d)
	return jo(d["user"]), nil
}

// ParseXStatuses parses an FxEmbed statuses response (own posts only, no reposts/replies to others).
func ParseXStatuses(handle string, body []byte) ([]FetchedItem, error) {
	var d jm
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, errors.New("FxEmbed 返回的不是 JSON")
	}
	var out []FetchedItem
	for _, x := range ja(d["results"]) {
		p := jo(x)
		author := js(jo(p["author"])["screen_name"])
		if !strings.EqualFold(author, handle) {
			continue
		}
		if rt := js(jo(p["replying_to"])["screen_name"]); rt != "" && !strings.EqualFold(rt, handle) {
			continue
		}
		var links []string
		for _, f := range ja(jo(p["raw_text"])["facets"]) {
			fm := jo(f)
			if js(fm["type"]) == "url" && js(fm["replacement"]) != "" {
				links = append(links, js(fm["replacement"]))
			}
		}
		out = append(out, xItem(js(p["text"]), js(p["url"]), js(p["id"]), author, int64(jn(p["created_timestamp"])), links))
	}
	return out, nil
}

func fetchX(ctx context.Context, handle string) ([]FetchedItem, error) {
	st, body, err := fxGet(ctx, "/profile/"+url.PathEscape(handle)+"/statuses?count=20")
	if err != nil {
		return nil, err
	}
	if st == 204 {
		return nil, nil
	}
	if st == 404 {
		prof, err := fxProfile(ctx, handle)
		if err == nil && prof == nil {
			return nil, fmt.Errorf("X 账号 @%s 不存在（FxEmbed 资料页也是 404）", handle)
		}
		return nil, errors.New("FxEmbed 伪 404（账号存在，时间线暂时取不到）")
	}
	if st != 200 {
		return nil, fmt.Errorf("FxEmbed HTTP %d", st)
	}
	return ParseXStatuses(handle, body)
}

var nextData = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

func syndicationList(ctx context.Context, id string) (jm, []any, error) {
	r, body, err := httpGet(ctx, "https://syndication.twitter.com/srv/timeline-list/list-id/"+id, nil, false, 2)
	if err != nil {
		return nil, nil, err
	}
	if r.StatusCode != 200 {
		return nil, nil, fmt.Errorf("syndication HTTP %d", r.StatusCode)
	}
	return ParseSyndication(body)
}

// ParseSyndication extracts the list header + entries from the syndication HTML.
func ParseSyndication(body []byte) (jm, []any, error) {
	m := nextData.FindSubmatch(body)
	if m == nil {
		return nil, nil, errors.New("syndication 响应里没有 __NEXT_DATA__")
	}
	var d jm
	if err := json.Unmarshal(m[1], &d); err != nil {
		return nil, nil, err
	}
	pp := jo(jo(d["props"])["pageProps"])
	return jo(pp["headerProps"]), ja(jo(pp["timeline"])["entries"]), nil
}

func XListItems(entries []any) []FetchedItem {
	var out []FetchedItem
	for _, x := range entries {
		e := jo(x)
		if js(e["type"]) != "tweet" {
			continue
		}
		t := jo(jo(e["content"])["tweet"])
		text := firstNonEmpty(js(t["full_text"]), js(t["text"]))
		if t["retweeted_status"] != nil || strings.HasPrefix(text, "RT @") {
			continue
		}
		sn := js(jo(t["user"])["screen_name"])
		var links []string
		for _, u := range ja(jo(t["entities"])["urls"]) {
			if s := js(jo(u)["expanded_url"]); s != "" {
				links = append(links, s)
			}
		}
		id := js(t["id_str"])
		out = append(out, xItem(text, "https://x.com/"+sn+"/status/"+id, id, sn, ParseDate(js(t["created_at"])), links))
	}
	return out
}

var listIDRe = regexp.MustCompile(`(\d{5,})`)

func fetchXList(ctx context.Context, u string) ([]FetchedItem, error) {
	m := listIDRe.FindStringSubmatch(u)
	if m == nil {
		return nil, errors.New("无法识别列表 id")
	}
	_, entries, err := syndicationList(ctx, m[1])
	if err != nil {
		return nil, err
	}
	return XListItems(entries), nil
}

// Detected is the result of probing user input in the add-source dialog.
type Detected struct {
	Type, Name, URL, Homepage, Username string
	Sample                              []FetchedItem
	Count                               int
}

var (
	listInRe = regexp.MustCompile(`^(?:https?://)?(?:www\.)?(?:x|twitter)\.com/i/lists/(\d+)`)
	xUserRe  = regexp.MustCompile(`^(?:@|(?:https?://)?(?:www\.)?(?:x|twitter)\.com/)?([A-Za-z0-9_]{1,15})/?$`)
	xDomRe   = regexp.MustCompile(`(x|twitter)\.com`)
	titleRe  = regexp.MustCompile(`(?i)<title[^>]*>([^<]*)`)
)

func head5(xs []FetchedItem) []FetchedItem {
	if len(xs) > 5 {
		return xs[:5]
	}
	return xs
}

// Detect: X list URL, X handle, RSS/Atom URL, or a page with <link rel=alternate>.
func Detect(ctx context.Context, input string) (Detected, error) {
	s := strings.TrimSpace(input)
	if m := listInRe.FindStringSubmatch(s); m != nil {
		u := "https://x.com/i/lists/" + m[1]
		h, entries, err := syndicationList(ctx, m[1])
		if err != nil {
			return Detected{}, err
		}
		items := XListItems(entries)
		ln := firstNonEmpty(js(h["listName"]), js(h["name"]), m[1])
		return Detected{Type: "x_list", Name: strings.TrimSpace("X 列表 " + ln), URL: u, Homepage: u, Sample: head5(items), Count: len(items)}, nil
	}
	if m := xUserRe.FindStringSubmatch(s); m != nil && (!strings.Contains(s, ".") || xDomRe.MatchString(s)) {
		prof, err := fxProfile(ctx, m[1])
		if err != nil {
			return Detected{}, err
		}
		if prof == nil {
			return Detected{}, fmt.Errorf("X 账号 @%s 不存在或 FxEmbed 无法获取", m[1])
		}
		h := firstNonEmpty(js(prof["screen_name"]), m[1])
		items, err := fetchX(ctx, h)
		if err != nil {
			return Detected{}, err
		}
		return Detected{Type: "x", Name: fmt.Sprintf("X @%s (%s)", h, js(prof["name"])), URL: "https://api.fxtwitter.com/2/profile/" + h + "/statuses",
			Homepage: "https://x.com/" + h, Username: h, Sample: head5(items), Count: len(items)}, nil
	}
	u := s
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	r, body, err := httpGet(ctx, u, nil, false, 3)
	if err != nil {
		return Detected{}, err
	}
	if r.StatusCode != 200 {
		return Detected{}, fmt.Errorf("HTTP %d", r.StatusCode)
	}
	final := r.Request.URL.String()
	host := r.Request.URL.Host
	if f := ParseFeed(body); f != nil && len(f.Items) > 0 {
		return Detected{Type: "rss", Name: firstNonEmpty(f.Title, host), URL: final, Homepage: firstNonEmpty(f.Link, final), Sample: head5(f.Items), Count: len(f.Items)}, nil
	}
	for _, fu := range DiscoverFeeds(string(body), final) {
		r2, b2, err := httpGet(ctx, fu, nil, false, 2)
		if err != nil || r2.StatusCode != 200 {
			continue
		}
		if f2 := ParseFeed(b2); f2 != nil && len(f2.Items) > 0 {
			t := f2.Title
			if t == "" {
				if m := titleRe.FindSubmatch(body); m != nil {
					t = strings.TrimSpace(string(m[1]))
				}
			}
			return Detected{Type: "rss", Name: Clean(firstNonEmpty(t, host), 80), URL: r2.Request.URL.String(), Homepage: final, Sample: head5(f2.Items), Count: len(f2.Items)}, nil
		}
	}
	return Detected{}, errors.New("页面没有可用的 RSS/Atom（未找到 <link rel=alternate> 或没有条目）；该站需要爬取，暂不支持")
}
