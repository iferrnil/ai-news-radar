package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// 抓取 API (desktop): third-party readers for 生成文章 — Jina Reader, Firecrawl, crawl4ai cloud. A scheduler picks
// the first provider (in that order) that has a key and is neither out of credit nor cooling down; a provider that
// answers 402 / insufficient balance / invalid key is marked exhausted, 429 or a server error puts it in a cool-down,
// and the url moves on to the next provider. Without any usable provider (or when all fail) the self-written fetcher
// is used. Keys live in secrets.json (0600) like the DeepSeek key and are never sent to the UI or logged.

type FetchKeys struct {
	JinaKey      string `json:"jinaKey,omitempty"`
	JinaBase     string `json:"jinaBase,omitempty"`
	FirecrawlKey string `json:"firecrawlKey,omitempty"`
	Crawl4aiKey  string `json:"crawl4aiKey,omitempty"`
}

const (
	jinaDefaultBase = "https://r.jina.ai" // r.jinaai.cn is the mainland mirror
	provExhaustTTL  = 6 * time.Hour       // re-probe an exhausted provider after this (credits may have been added)
)

// endpoint bases (tests point them at fake servers)
var (
	firecrawlBase = "https://api.firecrawl.dev"
	crawl4aiBase  = "https://api.crawl4ai.com"
	jinaDashBase  = "https://dash.jina.ai"
)

var provOrder = []string{"jina", "firecrawl", "crawl4ai"}
var provLabel = map[string]string{"jina": "Jina Reader", "firecrawl": "Firecrawl", "crawl4ai": "crawl4ai"}

// quota is a provider's usage as last seen (refresh or response headers). Nil numbers = unknown.
type quota struct {
	OK        bool     `json:"ok"`
	Remaining *float64 `json:"remaining,omitempty"`
	Total     *float64 `json:"total,omitempty"`
	Allowance *float64 `json:"allowance,omitempty"` // crawl4ai: credits left under this month's plan allowance
	Unit      string   `json:"unit,omitempty"`
	ResetAt   string   `json:"resetAt,omitempty"`
	At        float64  `json:"at"`
	Error     string   `json:"error,omitempty"`
}

type provState struct {
	coolUntil   time.Time
	exhausted   bool
	exhaustedAt time.Time
	why         string
	quota       *quota
	keyHash     string // state belongs to this key; a new key resets it
}

type provSched struct {
	mu sync.Mutex
	st map[string]*provState
}

func newProvSched() *provSched {
	return &provSched{st: map[string]*provState{"jina": {}, "firecrawl": {}, "crawl4ai": {}}}
}

func (s *Store) fetchKeys() FetchKeys { return s.loadSecrets().Fetch }

func (k FetchKeys) key(id string) string {
	switch id {
	case "jina":
		return k.JinaKey
	case "firecrawl":
		return k.FirecrawlKey
	case "crawl4ai":
		return k.Crawl4aiKey
	}
	return ""
}

func (k FetchKeys) jinaBase() string {
	if k.JinaBase != "" {
		return k.JinaBase
	}
	return jinaDefaultBase
}

func (ps *provSched) state(id, key string) *provState {
	st := ps.st[id]
	if h := hashOf(id, key); st.keyHash != h { // key changed: forget exhaustion / cool-down / quota
		*st = provState{keyHash: h}
	}
	return st
}

// available: has a key, not exhausted (or exhausted long enough ago), not cooling down.
func (ps *provSched) available(id, key string) bool {
	if key == "" {
		return false
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	st := ps.state(id, key)
	if st.exhausted && time.Since(st.exhaustedAt) < provExhaustTTL {
		return false
	}
	return !time.Now().Before(st.coolUntil)
}

func (ps *provSched) markExhausted(id, key, why string) {
	ps.mu.Lock()
	st := ps.state(id, key)
	st.exhausted, st.exhaustedAt, st.why = true, time.Now(), why
	ps.mu.Unlock()
}

func (ps *provSched) markCool(id, key string, d time.Duration, why string) {
	ps.mu.Lock()
	st := ps.state(id, key)
	if t := time.Now().Add(d); t.After(st.coolUntil) {
		st.coolUntil, st.why = t, why
	}
	ps.mu.Unlock()
}

func (ps *provSched) markOK(id, key string) {
	ps.mu.Lock()
	st := ps.state(id, key)
	st.exhausted, st.why = false, ""
	ps.mu.Unlock()
}

func (ps *provSched) setQuota(id, key string, q *quota) {
	ps.mu.Lock()
	ps.state(id, key).quota = q
	ps.mu.Unlock()
}

// provOutcome classifies one provider call.
type provOutcome int

const (
	provOK        provOutcome = iota
	provPage                  // the page itself failed (blocked, 404, empty): try the next provider, provider stays usable
	provExhausted             // no credit / plan cap / invalid key
	provRate                  // 429
	provDown                  // 5xx, network, timeout
)

type provResult struct {
	text, title string
	outcome     provOutcome
	why         string
	retryAfter  time.Duration
}

var reCreditWords = regexp.MustCompile(`(?i)insufficient|balance|credit|quota|payment|billing|no_credit|plan_cap|spend_cap|exceed`)

// classify maps an HTTP answer to an outcome (body for the 403 / 429 wording).
func classify(id string, r *Resp, err error) (provOutcome, string) {
	if err != nil {
		return provDown, "请求失败：" + fetchErrText(err)
	}
	body := jsSlice(string(r.Body), 600)
	msg := provErrMsg(r.Body)
	switch {
	case r.OK():
		return provOK, ""
	case r.Status == 401:
		return provExhausted, "key 无效（401）"
	case r.Status == 402:
		return provExhausted, "额度用尽（402" + suffix(msg) + "）"
	case r.Status == 403 && reCreditWords.MatchString(body):
		return provExhausted, "额度不足（403" + suffix(msg) + "）"
	case r.Status == 429 && reCreditWords.MatchString(msg) && !strings.Contains(strings.ToLower(msg), "rate"):
		return provExhausted, "额度不足（429" + suffix(msg) + "）"
	case r.Status == 429:
		return provRate, "限流（429）"
	case id == "crawl4ai" && r.Status == 502: // no-answer / origin-error: the site, not the service
		return provPage, "站点无响应（502" + suffix(msg) + "）"
	case id == "crawl4ai" && r.Status == 503: // fleet-busy: retry in a few seconds
		return provRate, "繁忙（503" + suffix(msg) + "）"
	case r.Status >= 500:
		return provDown, fmt.Sprintf("服务错误（HTTP %d）", r.Status)
	}
	return provPage, fmt.Sprintf("HTTP %d%s", r.Status, suffix(msg))
}

func suffix(m string) string {
	if m == "" {
		return ""
	}
	return " " + jsSlice(m, 60)
}

// provErrMsg: the short error text of a JSON error body ({"error":…} / {"message":…} / {"detail":…} / {"name":…}).
func provErrMsg(b []byte) string {
	var j map[string]any
	if json.Unmarshal(b, &j) != nil {
		return ""
	}
	for _, k := range []string{"error", "message", "detail", "readableMessage", "name", "code"} {
		switch v := j[k].(type) {
		case string:
			if v != "" {
				return v
			}
		case map[string]any:
			if s, ok := v["message"].(string); ok {
				return s
			}
		}
	}
	return ""
}

func num(v any) *float64 {
	switch x := v.(type) {
	case float64:
		return &x
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return &f
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(x, ",", "")), 64); err == nil {
			return &f
		}
	}
	return nil
}

// ---- provider calls

func (s *Store) callProvider(ctx context.Context, id string, k FetchKeys, target string) provResult {
	key := k.key(id)
	var r *Resp
	var err error
	switch id {
	case "jina":
		r, err = netFetch(ctx, "GET", strings.TrimRight(k.jinaBase(), "/")+"/"+target, map[string]string{
			"Authorization": "Bearer " + key, "Accept": "application/json", "X-Return-Format": "markdown", "X-Timeout": "30", "X-Retain-Images": "none",
		}, nil, 45*time.Second)
	case "firecrawl":
		b, _ := json.Marshal(map[string]any{"url": target, "formats": []string{"markdown"}, "onlyMainContent": true, "timeout": 45000})
		r, err = netFetch(ctx, "POST", firecrawlBase+"/v2/scrape", map[string]string{"Authorization": "Bearer " + key, "Content-Type": "application/json"},
			strings.NewReader(string(b)), 60*time.Second)
	case "crawl4ai":
		b, _ := json.Marshal(map[string]any{"url": target, "format": "md"})
		r, err = netFetch(ctx, "POST", crawl4aiBase+"/scrape", map[string]string{"Authorization": "Bearer " + key, "Content-Type": "application/json"},
			strings.NewReader(string(b)), 120*time.Second) // a cold page can take a while; their docs ask for 120 s
	default:
		return provResult{outcome: provPage, why: "unknown provider"}
	}
	if r != nil && id == "crawl4ai" {
		s.c4Headers(key, r.Header)
	}
	oc, why := classify(id, r, err)
	res := provResult{outcome: oc, why: why}
	if oc == provRate && r != nil {
		res.retryAfter, _ = retryAfter(r)
	}
	if oc != provOK {
		return res
	}
	var j map[string]any
	if json.Unmarshal(r.Body, &j) != nil {
		if id == "jina" && len(r.Body) > 0 { // plain markdown answer
			res.text = string(r.Body)
		} else {
			return provResult{outcome: provDown, why: "返回的不是 JSON"}
		}
	}
	switch id {
	case "jina": // {code, status, data:{title, content, url}}
		if d, ok := j["data"].(map[string]any); ok {
			res.text, res.title = str(d["content"]), str(d["title"])
		}
	case "firecrawl": // {success, data:{markdown, metadata:{title, statusCode}}}
		if d, ok := j["data"].(map[string]any); ok {
			res.text = str(d["markdown"])
			if m, ok := d["metadata"].(map[string]any); ok {
				res.title = jsString(m["title"])
				if sc := num(m["statusCode"]); sc != nil && *sc >= 400 {
					return provResult{outcome: provPage, why: fmt.Sprintf("站点 HTTP %d", int(*sc))}
				}
			}
		}
		if j["success"] == false {
			return provResult{outcome: provPage, why: "抓取失败" + suffix(provErrMsg(r.Body))}
		}
	case "crawl4ai": // {ok, markdown, html, content_hash, ms, …}: no title field — the first "# " heading
		res.text = str(j["markdown"])
		if m := reH1.FindStringSubmatch(res.text); m != nil {
			res.title = strings.TrimSpace(m[1])
		}
		if j["ok"] == false {
			return provResult{outcome: provPage, why: "抓取失败" + suffix(provErrMsg(r.Body))}
		}
	}
	res.text = tidyMarkdown(res.text)
	if strings.TrimSpace(res.text) == "" {
		return provResult{outcome: provPage, why: "没有正文"}
	}
	return res
}

var (
	reMdImage = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reMdBlank = regexp.MustCompile(`\n{3,}`)
)

// tidyMarkdown drops images and excess blank lines; capped like self-fetched text.
func tidyMarkdown(t string) string {
	t = reMdImage.ReplaceAllString(strings.ReplaceAll(t, "\r\n", "\n"), "")
	t = reMdBlank.ReplaceAllString(t, "\n\n")
	return capArticle(t)
}

// c4Headers: crawl4ai reports the running balance on every call.
func (s *Store) c4Headers(key string, h http.Header) {
	bal, left := num(h.Get("x-c4-balance")), num(h.Get("x-c4-allowance-left"))
	if bal == nil && left == nil {
		return
	}
	s.prov.mu.Lock()
	st := s.prov.state("crawl4ai", key)
	q := &quota{OK: true, Unit: "credits", At: now()}
	if st.quota != nil && st.quota.OK {
		cp := *st.quota
		q = &cp
		q.At = now()
	}
	if bal != nil {
		q.Remaining = bal
	}
	if left != nil {
		q.Allowance = left
	}
	st.quota = q
	s.prov.mu.Unlock()
}

// nextProvider: the first provider in order that is usable and not tried yet for this url.
func (s *Store) nextProvider(k FetchKeys, tried map[string]bool) string {
	for _, id := range provOrder {
		if !tried[id] && s.prov.available(id, k.key(id)) {
			return id
		}
	}
	return ""
}

// fetchViaProviders (生成文章): url → result, each url on its own goroutine walking the provider order; a url
// with no full text from any provider falls back to the self-written fetcher.
func (s *Store) fetchViaProviders(urls []string) map[string]articleRes {
	urls = uniq(urls)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	k := s.fetchKeys()
	out := map[string]articleRes{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, u := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			res := s.fetchOneViaProviders(ctx, k, u)
			mu.Lock()
			out[u] = res
			mu.Unlock()
		}(u)
	}
	wg.Wait()
	return out
}

func (s *Store) fetchOneViaProviders(ctx context.Context, k FetchKeys, u string) articleRes {
	if skipArticleHost(u) {
		return articleRes{Status: "skipped", Reason: "社交平台 / 需登录的站点，不抓取", Method: "self"}
	}
	if _, skip := articleRoute(u); skip != "" {
		return fetchArticleRes(ctx, u)
	}
	if c, ok := gArticles.get(u); ok && c.Status == "full" {
		return c
	}
	best, tried := s.providersOnly(ctx, k, u)
	if best.Status == "full" {
		return best
	}
	self := fetchArticleRes(ctx, u) // fallback: the self-written fetcher
	if tried && self.Status != "full" {
		self.Reason = strings.TrimLeft(best.Reason+"；自抓："+self.Reason, "；")
	}
	if best.Text != "" && self.Status != "full" && utf8.RuneCountInString(best.Text) > utf8.RuneCountInString(self.Text) {
		best.Reason = self.Reason
		return best
	}
	return self
}

// providersOnly walks the provider order for u (no self-written fetcher). tried = some provider was called. A
// result that is not full carries the providers' notes in Reason.
func (s *Store) providersOnly(ctx context.Context, k FetchKeys, u string) (articleRes, bool) {
	if c, ok := gArticles.get(u); ok && c.Status == "full" {
		return c, true
	}
	t0 := time.Now()
	tried := map[string]bool{}
	var best articleRes
	var notes []string
	for {
		id := s.nextProvider(k, tried)
		if id == "" || ctx.Err() != nil {
			break
		}
		tried[id] = true
		key := k.key(id)
		r := s.callProvider(ctx, id, k, u)
		switch r.outcome {
		case provOK:
			s.prov.markOK(id, key)
			res := articleRes{Text: r.text, Title: r.title, Method: id, Status: "full", Took: time.Since(t0)}
			if utf8.RuneCountInString(r.text) < articleFullMin {
				res.Status, res.Reason = "partial", "只取到摘要 / 开头"
			}
			if res.Status == "full" {
				gArticles.put(u, res)
				return res, true
			}
			if utf8.RuneCountInString(res.Text) > utf8.RuneCountInString(best.Text) {
				best = res
			}
			notes = append(notes, provLabel[id]+"：内容过短")
			continue
		case provExhausted:
			s.prov.markExhausted(id, key, r.why)
		case provRate:
			d := r.retryAfter
			switch {
			case id == "crawl4ai" && strings.Contains(r.why, "503"):
				d = 15 * time.Second
			case d < 30*time.Second:
				d = 60 * time.Second
			}
			s.prov.markCool(id, key, d, r.why)
		case provDown:
			s.prov.markCool(id, key, 2*time.Minute, r.why)
		}
		notes = append(notes, provLabel[id]+"："+r.why)
	}
	best.Reason = strings.Join(notes, "；")
	if best.Text == "" {
		best.Status = "failed"
	}
	return best, len(tried) > 0
}

// ---- quota

func (s *Store) refreshQuota(id string) *quota {
	k := s.fetchKeys()
	key := k.key(id)
	if key == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	q := &quota{At: now()}
	fail := func(why string) *quota {
		q.OK, q.Error = false, strings.ReplaceAll(why, key, "***")
		s.prov.setQuota(id, key, q)
		return q
	}
	switch id {
	case "firecrawl":
		r, err := netFetch(ctx, "GET", firecrawlBase+"/v2/team/credit-usage", map[string]string{"Authorization": "Bearer " + key}, nil, 20*time.Second)
		if oc, why := classify(id, r, err); oc != provOK {
			return fail(why)
		}
		var j struct {
			Success bool `json:"success"`
			Data    struct {
				Remaining any    `json:"remainingCredits"`
				Plan      any    `json:"planCredits"`
				Start     string `json:"billingPeriodStart"`
				End       string `json:"billingPeriodEnd"`
			} `json:"data"`
		}
		if json.Unmarshal(r.Body, &j) != nil || num(j.Data.Remaining) == nil {
			return fail("无法解析")
		}
		q.OK, q.Remaining, q.Total, q.Unit, q.ResetAt = true, num(j.Data.Remaining), num(j.Data.Plan), "credits", j.Data.End
	case "crawl4ai":
		r, err := netFetch(ctx, "GET", crawl4aiBase+"/v1/billing/balance", map[string]string{"Authorization": "Bearer " + key}, nil, 20*time.Second)
		if r != nil {
			s.c4Headers(key, r.Header)
		}
		if oc, why := classify(id, r, err); oc != provOK {
			return fail(why)
		}
		var j map[string]any
		dec := json.NewDecoder(strings.NewReader(string(r.Body)))
		dec.UseNumber()
		if dec.Decode(&j) != nil {
			return fail("无法解析")
		}
		// {"free":"999.75","free_mc":999750,"credit":"0.00","credit_mc":0,"free_period":"2026-10","allowance_mc":2500000,
		//  "spend_month_mc":250,"plan_month_mc":0,"tier":"community",…}: _mc = millicredits. What can be spent = the
		// monthly free credit + bought credit; the free credit renews on the 1st of the month after free_period.
		// allowance_mc is the monthly spend cap (x-c4-allowance-left = allowance − this month's spend).
		mc := func(k, plain string) *float64 {
			if v := num(j[k]); v != nil {
				f := *v / 1000
				return &f
			}
			return num(j[plain])
		}
		free, credit := mc("free_mc", "free"), mc("credit_mc", "credit")
		if free == nil && credit == nil {
			return fail("无法解析")
		}
		rem := 0.0
		for _, v := range []*float64{free, credit} {
			if v != nil {
				rem += *v
			}
		}
		q.Remaining = &rem
		if allow := mc("allowance_mc", ""); allow != nil && *allow > 0 {
			left := *allow
			if spent := mc("spend_month_mc", "spend_month"); spent != nil {
				left -= *spent
			}
			q.Allowance = &left
		}
		if plan := mc("plan_month_mc", ""); plan != nil && *plan > 0 {
			q.Total = plan
		}
		if t, err := time.Parse("2006-01", str(j["free_period"])); err == nil {
			q.ResetAt = t.AddDate(0, 1, 0).Format("2006-01-02")
		}
		q.Unit = "credits"
		// the balance endpoint trails the per-call x-c4-balance header by about a minute: a lower figure seen on
		// a call in the last two minutes is the current one
		s.prov.mu.Lock()
		if prev := s.prov.state(id, key).quota; prev != nil && prev.OK && prev.Remaining != nil && now()-prev.At < 120 && *prev.Remaining < rem {
			v := *prev.Remaining
			q.Remaining = &v
		}
		s.prov.mu.Unlock()
		q.OK = true
	case "jina":
		// no official endpoint: the dashboard's own call (best effort). The URL carries the key: never log it.
		r, err := netFetch(ctx, "GET", jinaDashBase+"/api/v1/api_key/fe_user?api_key="+url.QueryEscape(key), map[string]string{"Accept": "application/json"}, nil, 20*time.Second)
		if err != nil {
			return fail("不可用")
		}
		if !r.OK() {
			return fail(fmt.Sprintf("不可用（HTTP %d）", r.Status))
		}
		var j map[string]any
		dec := json.NewDecoder(strings.NewReader(string(r.Body)))
		dec.UseNumber()
		if dec.Decode(&j) != nil {
			return fail("不可用")
		}
		var bal *float64
		for _, root := range []any{j["data"], j} {
			if m, ok := root.(map[string]any); ok {
				if w, ok := m["wallet"].(map[string]any); ok {
					if bal = num(w["total_balance"]); bal != nil {
						break
					}
				}
			}
		}
		if bal == nil {
			return fail("不可用")
		}
		q.OK, q.Remaining, q.Unit = true, bal, "tokens"
	default:
		return nil
	}
	s.prov.setQuota(id, key, q)
	return q
}

// findNum: the first numeric value under one of the keys (in key order), searched breadth-first.
func findNum(v any, keys ...string) *float64 {
	for _, k := range keys {
		if f := findKey(v, k, 0); f != nil {
			if n := num(f); n != nil {
				return n
			}
		}
	}
	return nil
}

func findStr(v any, keys ...string) string {
	for _, k := range keys {
		if s, ok := findKey(v, k, 0).(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func findKey(v any, key string, depth int) any {
	m, ok := v.(map[string]any)
	if !ok || depth > 4 {
		return nil
	}
	if x, ok := m[key]; ok {
		if _, isMap := x.(map[string]any); !isMap {
			return x
		}
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if x := findKey(m[n], key, depth+1); x != nil {
			return x
		}
	}
	return nil
}

// ---- API (desktop only)

func (s *Store) fetchAPIPublic() *Row {
	k := s.fetchKeys()
	var list []any
	for _, id := range provOrder {
		key := k.key(id)
		s.prov.mu.Lock()
		st := s.prov.state(id, key)
		status, why := "off", ""
		if key != "" {
			status = "ok"
			if st.exhausted && time.Since(st.exhaustedAt) < provExhaustTTL {
				status, why = "exhausted", st.why
			} else if time.Now().Before(st.coolUntil) {
				status, why = "cooling", st.why
			}
		}
		var q any
		if st.quota != nil {
			cp := *st.quota
			q = cp
		}
		s.prov.mu.Unlock()
		r := obj("id", id, "label", provLabel[id], "configured", key != "", "status", status)
		if why != "" {
			r.Set("why", why)
		}
		if q != nil {
			r.Set("quota", q)
		}
		if id == "jina" {
			r.Set("baseUrl", k.JinaBase)
			r.Set("baseUrlDefault", jinaDefaultBase)
		}
		list = append(list, r)
	}
	return obj("providers", list, "locked", SettingsLocked())
}

func (s *Store) hFetchAPIGet(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "抓取 API 仅桌面版可用") {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, s.fetchAPIPublic())
}

// PATCH /api/fetchapi {jina:{key, clearKey, baseUrl}, firecrawl:{key, clearKey}, crawl4ai:{key, clearKey}}
func (s *Store) hFetchAPIPatch(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "抓取 API 仅桌面版可用") {
		return
	}
	if SettingsLocked() {
		detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1）")
		return
	}
	b := readBody(r)
	st := s.loadSecrets()
	k := st.Fetch
	for _, id := range provOrder {
		p, ok := b[id].(map[string]any)
		if !ok {
			continue
		}
		var kp *string
		switch id {
		case "jina":
			kp = &k.JinaKey
		case "firecrawl":
			kp = &k.FirecrawlKey
		default:
			kp = &k.Crawl4aiKey
		}
		if p["clearKey"] == true {
			*kp = ""
		}
		if v, has := p["key"]; has && strings.TrimSpace(jsString(v)) != "" {
			c, ok := cleanKey(v)
			if !ok {
				detail(w, 400, provLabel[id]+" key 格式不对")
				return
			}
			*kp = c
		}
		if v, has := p["baseUrl"]; has && id == "jina" {
			u, ok := cleanBase(v)
			if !ok {
				detail(w, 400, "Base URL 无效（需 https://）")
				return
			}
			k.JinaBase = u
		}
	}
	st.Fetch = k
	if err := s.saveSecrets(st); err != nil {
		detail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, s.fetchAPIPublic())
}

// POST /api/fetchapi/quota[?id=] — refresh the usage of the configured providers.
func (s *Store) hFetchAPIQuota(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "抓取 API 仅桌面版可用") {
		return
	}
	only := r.URL.Query().Get("id")
	var wg sync.WaitGroup
	for _, id := range provOrder {
		if only != "" && only != id {
			continue
		}
		wg.Add(1)
		go func(id string) { defer wg.Done(); s.refreshQuota(id) }(id)
	}
	wg.Wait()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, s.fetchAPIPublic())
}
