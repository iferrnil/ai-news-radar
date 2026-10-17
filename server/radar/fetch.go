package radar

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// Self-written article fetcher (briefs; the fallback of 生成文章): browser-like requests, gzip / br / deflate,
// charset detection (incl. GBK / GB18030), one retry with backoff on transient errors, a cool-down for hosts that
// answer 403 / 429, and two levels of concurrency — global (ARTICLE_CONC, 10) and per host (ARTICLE_PER_HOST, 2) —
// so ten different sites are fetched in parallel while one site never gets more than two requests at a time.

const (
	articleBodyMax   = 5 << 20 // decoded bytes kept
	articleRetryBase = 600 * time.Millisecond
)

var articleHeaders = map[string]string{
	"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"Accept-Language":           "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
	"Accept-Encoding":           "gzip, deflate, br",
	"Cache-Control":             "no-cache",
	"Pragma":                    "no-cache",
	"Upgrade-Insecure-Requests": "1",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Sec-Ch-Ua":                 `"Google Chrome";v="141", "Not?A_Brand";v="8", "Chromium";v="141"`,
	"Sec-Ch-Ua-Mobile":          "?0",
	"Sec-Ch-Ua-Platform":        `"macOS"`,
}

type hostGate struct {
	sem   chan struct{}
	until time.Time // 403 / 429 cool-down
	why   string
}

type artFetcher struct {
	global  chan struct{}
	perHost int
	mu      sync.Mutex
	hosts   map[string]*hostGate
	client  *http.Client
	retry   time.Duration
}

func newArtFetcher(conc, perHost int) *artFetcher {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableCompression = true // Accept-Encoding is ours (br too), so is decoding
	tr.MaxIdleConnsPerHost = 4
	tr.ResponseHeaderTimeout = 15 * time.Second
	return &artFetcher{
		global: make(chan struct{}, max(1, conc)), perHost: max(1, perHost), hosts: map[string]*hostGate{}, retry: articleRetryBase,
		client: &http.Client{Transport: tr, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return errors.New("stopped after 8 redirects")
			}
			return nil
		}},
	}
}

var gFetch = newArtFetcher(envIntDef("ARTICLE_CONC", 10), envIntDef("ARTICLE_PER_HOST", 2))

func (f *artFetcher) gate(host string) *hostGate {
	f.mu.Lock()
	defer f.mu.Unlock()
	g := f.hosts[host]
	if g == nil {
		g = &hostGate{sem: make(chan struct{}, f.perHost)}
		f.hosts[host] = g
	}
	return g
}

// cooling reports a host that recently answered 403 / 429 (requests to it fail fast until then).
func (f *artFetcher) cooling(host string) (string, bool) {
	g := f.gate(host)
	f.mu.Lock()
	defer f.mu.Unlock()
	if time.Now().Before(g.until) {
		return g.why, true
	}
	return "", false
}

func (f *artFetcher) coolDown(host string, d time.Duration, why string) {
	g := f.gate(host)
	f.mu.Lock()
	if t := time.Now().Add(d); t.After(g.until) {
		g.until, g.why = t, why
	}
	f.mu.Unlock()
}

// acquire takes the host slot first, then a global one (a request waiting for its host never holds a global slot,
// so other hosts keep running).
func (f *artFetcher) acquire(ctx context.Context, host string) (func(), error) {
	g := f.gate(host)
	select {
	case g.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case f.global <- struct{}{}:
	case <-ctx.Done():
		<-g.sem
		return nil, ctx.Err()
	}
	return func() { <-f.global; <-g.sem }, nil
}

// pageResp is one fetched page, body already decompressed and converted to UTF-8.
type pageResp struct {
	Status  int
	Header  http.Header
	Text    string
	Charset string
	URL     string
	Raw     int // bytes on the wire
}

func transientStatus(c int) bool {
	return c == 408 || c == 425 || c == 500 || c == 502 || c == 503 || c == 504
}

// get fetches u (bounded by the host / global limits) with one retry on a transient failure.
func (f *artFetcher) get(ctx context.Context, u string, timeout time.Duration) (*pageResp, error) {
	host := artHost(u)
	if why, ok := f.cooling(host); ok {
		return nil, fmt.Errorf("%s，稍后再试", why)
	}
	release, err := f.acquire(ctx, host)
	if err != nil {
		return nil, err
	}
	defer release()
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			sleepCtx(ctx, backoffD(0, f.retry*2, 4*time.Second))
			if ctx.Err() != nil {
				break
			}
		}
		r, err := f.once(ctx, u, timeout)
		if err != nil {
			last = err
			if ctx.Err() != nil || !transientErr(err) {
				return nil, err
			}
			continue
		}
		switch {
		case r.Status == 429:
			d, ok := retryAfter(&Resp{Header: r.Header})
			if !ok || d < time.Minute {
				d = 2 * time.Minute
			}
			f.coolDown(host, d, "站点限流（HTTP 429）")
			return r, nil
		case r.Status == 403:
			f.coolDown(host, 5*time.Minute, "站点拒绝访问（HTTP 403）")
			return r, nil
		case transientStatus(r.Status) && attempt == 0:
			last = fmt.Errorf("HTTP %d", r.Status)
			continue
		}
		return r, nil
	}
	return nil, last
}

func transientErr(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true // timeouts, resets, refused
	}
	s := err.Error()
	return strings.Contains(s, "EOF") || strings.Contains(s, "connection reset") || strings.Contains(s, "connection refused")
}

func (f *artFetcher) once(ctx context.Context, u string, timeout time.Duration) (*pageResp, error) {
	target := u
	mock := os.Getenv("UPSTREAM_MOCK")
	if mock != "" {
		target = strings.TrimRight(mock, "/") + "/?u=" + url.QueryEscape(u)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range articleHeaders {
		req.Header.Set(k, v)
	}
	r, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, articleBodyMax))
	if err != nil && len(raw) == 0 {
		return nil, err
	}
	body, derr := decodeBody(raw, r.Header.Get("Content-Encoding"))
	if derr != nil {
		body = raw // mislabeled encoding: try as is
	}
	final := r.Request.URL.String()
	if mock != "" {
		final = u
	}
	text, cs := toUTF8(body, r.Header.Get("Content-Type"), artHost(final))
	return &pageResp{Status: r.StatusCode, Header: r.Header, Text: text, Charset: cs, URL: final, Raw: len(raw)}, nil
}

// decodeBody undoes Content-Encoding (gzip, br, deflate as zlib or raw; stacked codings in reverse order).
func decodeBody(b []byte, enc string) ([]byte, error) {
	codings := strings.Split(strings.ToLower(enc), ",")
	for i := len(codings) - 1; i >= 0; i-- {
		var rd io.Reader
		switch strings.TrimSpace(codings[i]) {
		case "", "identity":
			continue
		case "gzip", "x-gzip":
			z, err := gzip.NewReader(bytes.NewReader(b))
			if err != nil {
				return b, err
			}
			rd = z
		case "br":
			rd = brotli.NewReader(bytes.NewReader(b))
		case "deflate":
			if z, err := zlib.NewReader(bytes.NewReader(b)); err == nil {
				rd = z
			} else {
				rd = flate.NewReader(bytes.NewReader(b))
			}
		default:
			return b, fmt.Errorf("unsupported encoding %q", codings[i])
		}
		out, err := io.ReadAll(io.LimitReader(rd, articleBodyMax))
		if err != nil && len(out) == 0 {
			return b, err
		}
		b = out
	}
	return b, nil
}

var (
	reCTCharset   = regexp.MustCompile(`(?i)charset\s*=\s*["']?([\w.:\-]+)`)
	reMetaCharset = regexp.MustCompile(`(?i)<meta[^>]+charset\s*=\s*["']?\s*([\w.:\-]+)`)
	reXMLEnc      = regexp.MustCompile(`(?i)<\?xml[^>]+encoding\s*=\s*["']([\w.:\-]+)`)
)

// charsetLabel finds the declared charset: BOM, Content-Type, <meta charset> / http-equiv, XML declaration.
func charsetLabel(b []byte, contentType string) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		return "utf-8"
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return "utf-16le"
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return "utf-16be"
	}
	if m := reCTCharset.FindStringSubmatch(contentType); m != nil {
		return strings.ToLower(m[1])
	}
	head := b
	if len(head) > 8192 {
		head = head[:8192]
	}
	if m := reMetaCharset.FindSubmatch(head); m != nil {
		return strings.ToLower(string(m[1]))
	}
	if m := reXMLEnc.FindSubmatch(head); m != nil {
		return strings.ToLower(string(m[1]))
	}
	return ""
}

func encodingFor(label string) encoding.Encoding {
	switch label {
	case "gb2312", "gbk", "x-gbk", "gb_2312-80", "cp936", "windows-936", "euc-cn", "gb18030":
		return simplifiedchinese.GB18030 // superset of GBK / GB2312
	}
	e, err := htmlindex.Get(label)
	if err != nil {
		return nil
	}
	return e
}

// toUTF8 converts a page to UTF-8 and returns the charset that was used. Undeclared or wrongly declared UTF-8
// that isn't valid UTF-8 is tried as GB18030 (common on Chinese sites) before falling back to windows-1252.
func toUTF8(b []byte, contentType, host string) (string, string) {
	label := charsetLabel(b, contentType)
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if label == "" || label == "utf-8" || label == "utf8" {
		if utf8.Valid(b) {
			return string(b), "utf-8"
		}
		if s, ok := gbGuess(b); ok {
			return s, "gb18030"
		}
		if label != "" { // declared UTF-8 with a few broken bytes: keep it, replacing the bad ones
			return strings.ToValidUTF8(string(b), "\uFFFD"), "utf-8"
		}
		label = "windows-1252"
	}
	enc := encodingFor(label)
	if enc == nil {
		return strings.ToValidUTF8(string(b), "\uFFFD"), "utf-8"
	}
	out, err := enc.NewDecoder().Bytes(b)
	if err != nil {
		return strings.ToValidUTF8(string(b), "\uFFFD"), "utf-8"
	}
	name := label
	if enc == simplifiedchinese.GB18030 {
		name = "gb18030"
	}
	return string(out), name
}

// gbGuess decodes b as GB18030 and accepts it when the result is mostly clean CJK text.
func gbGuess(b []byte) (string, bool) {
	out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if err != nil {
		return "", false
	}
	s := string(out)
	cjk := len(cjkRe.FindAllStringIndex(s, 4000))
	bad := strings.Count(s, "\uFFFD")
	return s, cjk >= 20 && bad*20 < cjk
}
