package radar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MarshalJSON: types.ts FetchedItem (published null when unknown).
func (f FetchedItem) MarshalJSON() ([]byte, error) {
	m := &Row{}
	m.Set("uid", f.UID)
	m.Set("title", f.Title)
	m.Set("url", f.URL)
	m.Set("summary", f.Summary)
	if f.Published != 0 {
		m.Set("published", f.Published)
	} else {
		m.Set("published", nil)
	}
	m.Set("isX", f.IsX)
	m.Set("author", nz(f.Author))
	if f.Links != nil {
		m.Set("links", f.Links)
	}
	if f.Image != "" {
		m.Set("image", f.Image)
	}
	return m.MarshalJSON()
}

// Conditional GET: remember ETag / Last-Modified per URL and send them back (a 304 costs ~no bandwidth).
type validator struct{ etag, lm string }

var (
	valMu      sync.Mutex
	validators = map[string]validator{}
)

func condGet(ctx context.Context, u string, headers map[string]string) (*Resp, error) {
	h := map[string]string{}
	for k, v := range headers {
		h[k] = v
	}
	valMu.Lock()
	v := validators[u]
	valMu.Unlock()
	if v.etag != "" {
		h["If-None-Match"] = v.etag
	}
	if v.lm != "" {
		h["If-Modified-Since"] = v.lm
	}
	r, err := httpGet(ctx, u, 0, 0, h)
	if err != nil {
		return nil, err
	}
	if r.OK() {
		valMu.Lock()
		if e, lm := r.Header.Get("ETag"), r.Header.Get("Last-Modified"); e != "" || lm != "" {
			validators[u] = validator{e, lm}
		} else {
			delete(validators, u)
		}
		valMu.Unlock()
	}
	return r, nil
}

func fetchRSS(ctx context.Context, u string) ([]FetchedItem, error) {
	r, err := condGet(ctx, u, nil)
	if err != nil {
		return nil, err
	}
	if r.Status == 304 {
		return nil, nil
	}
	if !r.OK() {
		k := FailKind("http")
		if r.Status == 404 {
			k = "not_found"
		} else if r.Status == 429 {
			k = "rate_limit"
		}
		return nil, fetchErr(k, "HTTP %d", r.Status)
	}
	f := ParseFeed(r.Body)
	if f == nil {
		return nil, fetchErr("parse", "不是有效的 RSS/Atom")
	}
	return f.Items, nil
}

func fetchAPI(ctx context.Context, u string) ([]FetchedItem, error) {
	r, err := condGet(ctx, u, map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}
	if r.Status == 304 {
		return nil, nil
	}
	if !r.OK() {
		return nil, plainErr("HTTP %d", r.Status)
	}
	return ParseAPI(u, r.Body)
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
		return nil, &JSError{Name: "SyntaxError", Msg: "Unexpected token in JSON: " + err.Error()}
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
			out = append(out, FetchedItem{UID: NormURL(link), Title: js(a["title"]), URL: link, Summary: Clean(js(a["content"]), 280), Published: pub, Author: js(a["author"]), Content: longerContent(js(a["content"]), 280)})
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
		return nil, plainErr("未知 API（没有适配器）")
	}
	return out, nil
}

func orZero(v any) string {
	if s := js(v); s != "" {
		return s
	}
	return "0"
}

// ---- X via FxEmbed. FX_MIRRORS = comma-separated API bases tried in order; a mirror failing 3x in a row is skipped for 5 min.
type mirrorState struct {
	Base  string  `json:"base"`
	Fails int     `json:"fails"`
	Until float64 `json:"until"`
}

var (
	fxMu      sync.Mutex
	fxMirrors = map[string]*mirrorState{}
)

func fxBases() []string {
	v := os.Getenv("FX_MIRRORS")
	if v == "" {
		v = "https://api.fxtwitter.com/2"
	}
	var out []string
	for _, b := range strings.Split(v, ",") {
		if b = strings.TrimRight(strings.TrimSpace(b), "/"); b != "" {
			out = append(out, b)
		}
	}
	return out
}

func mirrorOrder() []string {
	fxMu.Lock()
	defer fxMu.Unlock()
	t := float64(time.Now().UnixMilli())
	var ok []string
	for _, b := range fxBases() {
		if m := fxMirrors[b]; m == nil || m.Until <= t {
			ok = append(ok, b)
		}
	}
	if len(ok) == 0 {
		return fxBases()
	}
	return ok
}

func markMirror(b string, ok bool) {
	fxMu.Lock()
	defer fxMu.Unlock()
	m := fxMirrors[b]
	if m == nil {
		m = &mirrorState{Base: b}
		fxMirrors[b] = m
	}
	if ok {
		m.Fails, m.Until = 0, 0
	} else if m.Fails++; m.Fails >= 3 {
		m.Until = float64(time.Now().UnixMilli()) + 5*60_000
	}
}

// FxMirrorState is /api/stats `fx`.
func FxMirrorState() []mirrorState {
	fxMu.Lock()
	defer fxMu.Unlock()
	out := []mirrorState{}
	for _, b := range fxBases() {
		if m := fxMirrors[b]; m != nil {
			out = append(out, *m)
		} else {
			out = append(out, mirrorState{Base: b})
		}
	}
	return out
}

// fxGet: first mirror that answers; 404/429/5xx/timeouts retried; status 404 only when every try said 404.
func fxGet(ctx context.Context, path string) (*Resp, error) {
	tries := envInt("FX_TRIES", 3)
	timeout := envMs("FX_TIMEOUT_MS", 12000)
	var last error
	var lastResp *Resp
	all404 := true
	for _, base := range mirrorOrder() {
		for i := 0; i < tries; i++ {
			r, err := netFetch(ctx, "GET", base+path, map[string]string{"User-Agent": "ai-news-crawler/1.0", "Accept": "application/json"}, nil, timeout)
			if err != nil {
				last, lastResp, all404 = err, nil, false
				if i < tries-1 {
					sleepCtx(ctx, backoffD(i, 800*time.Millisecond, 8*time.Second))
				}
				continue
			}
			if r.OK() || r.Status == 204 {
				markMirror(base, true)
				return r, nil
			}
			lastResp, last = r, nil
			if r.Status != 404 {
				all404 = false
			}
			switch r.Status {
			case 404, 429, 500, 502, 503, 504:
			default:
				markMirror(base, true)
				return r, nil // a real answer (e.g. 400)
			}
			if i < tries-1 {
				d, ok := retryAfter(r)
				if !ok {
					d = backoffD(i, 800*time.Millisecond, 8*time.Second)
				}
				sleepCtx(ctx, d)
			}
		}
		markMirror(base, false)
	}
	if all404 {
		return &Resp{Status: 404}, nil
	}
	if lastResp != nil {
		k := FailKind("http")
		if lastResp.Status == 429 {
			k = "rate_limit"
		}
		return nil, fetchErr(k, "FxEmbed HTTP %d", lastResp.Status)
	}
	if last != nil {
		return nil, fetchErr(kindOf(last), "FxEmbed %s", errMsg(last))
	}
	return nil, fetchErr("other", "FxEmbed failed")
}

var urlInText = regexp.MustCompile(`https?://\S+`)

func xItem(text, u, id, author string, ts int64, links []string) FetchedItem {
	first, _, _ := strings.Cut(strings.TrimSpace(urlInText.ReplaceAllString(text, "")), "\n")
	title := first
	if jsLen(first) > 120 {
		title = jsSlice(first, 120) + "…"
	}
	if title == "" {
		title = u
	}
	if links == nil {
		links = []string{}
	}
	content := "" // a long-form post: the whole text (Summary keeps the 4000-char cut the feed shows)
	if jsLen(text) > 4000 {
		content = Trunc(text, contentMaxRunes, "…")
	}
	return FetchedItem{UID: "x:" + id, Title: title, URL: u, Summary: jsSlice(text, 4000), Published: ts, IsX: true, Author: author, Links: links, Content: content}
}

func fxProfile(ctx context.Context, handle string) (jm, error) {
	r, err := fxGet(ctx, "/profile/"+url.PathEscape(handle))
	if err != nil {
		return nil, err
	}
	if r.Status == 404 {
		return nil, nil
	}
	if !r.OK() {
		return nil, plainErr("FxEmbed HTTP %d", r.Status)
	}
	var d jm
	json.Unmarshal(r.Body, &d)
	return jo(d["user"]), nil
}

// ParseXStatuses parses an FxEmbed statuses response (own posts only, no reposts/replies to others).
func ParseXStatuses(handle string, body []byte) ([]FetchedItem, error) {
	var d jm
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fetchErr("parse", "FxEmbed 返回的不是 JSON")
	}
	out := []FetchedItem{}
	for _, x := range ja(d["results"]) {
		p := jo(x)
		author := js(jo(p["author"])["screen_name"])
		if !strings.EqualFold(author, handle) {
			continue
		}
		if rt := js(jo(p["replying_to"])["screen_name"]); rt != "" && !strings.EqualFold(rt, handle) {
			continue
		}
		links := []string{}
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
	r, err := fxGet(ctx, "/profile/"+url.PathEscape(handle)+"/statuses?count=20")
	if err != nil {
		return nil, err
	}
	if r.Status == 204 {
		return nil, nil
	}
	if r.Status == 404 {
		prof, err := fxProfile(ctx, handle)
		if err == nil && prof == nil {
			return nil, fetchErr("not_found", "X 账号 @%s 不存在（FxEmbed 资料页也是 404）", handle)
		}
		return nil, fetchErr("false_404", "FxEmbed 伪 404（账号存在，时间线暂时取不到）")
	}
	if !r.OK() {
		return nil, fetchErr("http", "FxEmbed HTTP %d", r.Status)
	}
	return ParseXStatuses(handle, r.Body)
}

var nextData = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

func syndicationList(ctx context.Context, id string) (jm, []any, error) {
	r, err := httpGet(ctx, "https://syndication.twitter.com/srv/timeline-list/list-id/"+id, 20*time.Second, 2, map[string]string{"User-Agent": UA})
	if err != nil {
		return nil, nil, err
	}
	if !r.OK() {
		k := FailKind("http")
		if r.Status == 429 {
			k = "rate_limit"
		}
		return nil, nil, fetchErr(k, "syndication HTTP %d", r.Status)
	}
	return ParseSyndication(r.Body)
}

// ParseSyndication extracts the list header + entries from the syndication HTML.
func ParseSyndication(body []byte) (jm, []any, error) {
	m := nextData.FindSubmatch(body)
	if m == nil {
		return nil, nil, plainErr("syndication 响应里没有 __NEXT_DATA__")
	}
	var d jm
	if err := json.Unmarshal(m[1], &d); err != nil {
		return nil, nil, &JSError{Name: "SyntaxError", Msg: "Unexpected token in JSON"}
	}
	pp := jo(jo(d["props"])["pageProps"])
	h := jo(pp["headerProps"])
	if h == nil {
		h = jm{}
	}
	return h, ja(jo(pp["timeline"])["entries"]), nil
}

func XListItems(entries []any) []FetchedItem {
	out := []FetchedItem{}
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
		links := []string{}
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
		return nil, plainErr("无法识别列表 id")
	}
	_, entries, err := syndicationList(ctx, m[1])
	if err != nil {
		return nil, err
	}
	return XListItems(entries), nil
}

// FetchSource dispatches by type (fetchers.ts FETCHERS).
func FetchSource(ctx context.Context, src *Row) ([]FetchedItem, error) {
	switch src.Str("type") {
	case "rss":
		return fetchRSS(ctx, src.Str("url"))
	case "api":
		return fetchAPI(ctx, src.Str("url"))
	case "x":
		return fetchX(ctx, src.Str("username"))
	case "x_list":
		return fetchXList(ctx, src.Str("url"))
	}
	return nil, &JSError{Name: "TypeError", Msg: "FETCHERS[src.type] is not a function"}
}

// MarshalJSON keeps the key order of the Node build (X handles spread their src object first).
func (d *Detected) row() *Row {
	var u any
	if d.Username != nil {
		u = *d.Username
	}
	if d.Type == "x" {
		return obj("type", d.Type, "url", d.URL, "username", u, "name", d.Name, "homepage", d.Homepage, "sample", d.Sample, "count", d.Count)
	}
	return obj("type", d.Type, "name", d.Name, "url", d.URL, "homepage", d.Homepage, "username", u, "sample", d.Sample, "count", d.Count)
}

// Detected is the add-source probe result (fetchers.ts Detected).
type Detected struct {
	Type     string        `json:"type"`
	Name     string        `json:"name"`
	URL      string        `json:"url"`
	Homepage string        `json:"homepage"`
	Username *string       `json:"username"`
	Sample   []FetchedItem `json:"sample"`
	Count    int           `json:"count"`
}

var (
	listInRe = regexp.MustCompile(`^(?:https?://)?(?:www\.)?(?:x|twitter)\.com/i/lists/(\d+)`)
	xUserRe  = regexp.MustCompile(`^(?:@|(?:https?://)?(?:www\.)?(?:x|twitter)\.com/)?([A-Za-z0-9_]{1,15})/?$`)
	xDomRe   = regexp.MustCompile(`(x|twitter)\.com`)
	titleRe  = regexp.MustCompile(`(?i)<title[^>]*>([^<]*)`)
)

func head5(xs []FetchedItem) []FetchedItem {
	if xs == nil {
		return []FetchedItem{}
	}
	if len(xs) > 5 {
		return xs[:5]
	}
	return xs
}

func hostOf(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return ""
	}
	return p.Host
}

// Detect: X list URL, X handle, RSS/Atom URL, or a page with <link rel=alternate>.
func Detect(ctx context.Context, input string) (*Detected, error) {
	s := strings.TrimSpace(input)
	if m := listInRe.FindStringSubmatch(s); m != nil {
		u := "https://x.com/i/lists/" + m[1]
		h, _, err := syndicationList(ctx, m[1])
		if err != nil {
			return nil, err
		}
		items, err := fetchXList(ctx, u)
		if err != nil {
			return nil, err
		}
		ln := m[1]
		if v, ok := h["listName"]; ok && v != nil {
			ln = js(v)
		} else if v, ok := h["name"]; ok && v != nil {
			ln = js(v)
		}
		return &Detected{Type: "x_list", Name: strings.TrimSpace("X 列表 " + ln), URL: u, Homepage: u, Sample: head5(items), Count: len(items)}, nil
	}
	if m := xUserRe.FindStringSubmatch(s); m != nil && (!strings.Contains(s, ".") || xDomRe.MatchString(s)) {
		prof, err := fxProfile(ctx, m[1])
		if err != nil {
			return nil, err
		}
		if prof == nil {
			return nil, plainErr("X 账号 @%s 不存在或 FxEmbed 无法获取", m[1])
		}
		h := m[1]
		if v, ok := prof["screen_name"].(string); ok {
			h = v
		}
		items, err := fetchX(ctx, h)
		if err != nil {
			return nil, err
		}
		return &Detected{Type: "x", Name: fmt.Sprintf("X @%s (%s)", h, jsString(prof["name"])), URL: "https://api.fxtwitter.com/2/profile/" + h + "/statuses",
			Homepage: "https://x.com/" + h, Username: &h, Sample: head5(items), Count: len(items)}, nil
	}
	u := s
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	if p, err := url.Parse(u); err != nil || p.Host == "" {
		return nil, &JSError{Name: "TypeError", Msg: "Invalid URL"}
	}
	r, err := httpGet(ctx, u, 0, 0, nil)
	if err != nil {
		return nil, err
	}
	if !r.OK() {
		return nil, plainErr("HTTP %d", r.Status)
	}
	if f := ParseFeed(r.Body); f != nil && len(f.Items) > 0 {
		return &Detected{Type: "rss", Name: firstNonEmpty(f.Title, hostOf(u)), URL: r.URL, Homepage: firstNonEmpty(f.Link, r.URL), Sample: head5(f.Items), Count: len(f.Items)}, nil
	}
	for _, fu := range DiscoverFeeds(string(r.Body), r.URL) {
		r2, err := httpGet(ctx, fu, 0, 0, nil)
		if err != nil || !r2.OK() {
			continue
		}
		if f2 := ParseFeed(r2.Body); f2 != nil && len(f2.Items) > 0 {
			t := f2.Title
			if t == "" {
				if m := titleRe.FindSubmatch(r.Body); m != nil {
					t = strings.TrimSpace(string(m[1]))
				}
			}
			return &Detected{Type: "rss", Name: Clean(firstNonEmpty(t, hostOf(u)), 80), URL: r2.URL, Homepage: r.URL, Sample: head5(f2.Items), Count: len(f2.Items)}, nil
		}
	}
	return nil, plainErr("页面没有可用的 RSS/Atom（未找到 <link rel=alternate> 或没有条目）；该站需要爬取，暂不支持")
}

var _ = errors.New
var _ = strconv.Itoa
