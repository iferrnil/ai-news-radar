package radar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FailKind is the coarse failure class kept in source_health.last_kind (net.ts FailKind).
type FailKind string

// JSError carries a JavaScript-style error name so messages read like the Node build ("Error: …", "TimeoutError: …").
type JSError struct {
	Name, Msg string
	Kind      FailKind // set for FetchError
}

func (e *JSError) Error() string { return e.Msg }

func fetchErr(kind FailKind, format string, a ...any) error {
	return &JSError{Name: "FetchError", Msg: fmt.Sprintf(format, a...), Kind: kind}
}
func plainErr(format string, a ...any) error {
	return &JSError{Name: "Error", Msg: fmt.Sprintf(format, a...)}
}

// errMsg mirrors util.ts errMsg: "<name>: <message>" capped at 300 chars.
func errMsg(e error) string {
	var je *JSError
	s := ""
	if errors.As(e, &je) {
		s = je.Name + ": " + je.Msg
	} else {
		s = "Error: " + e.Error()
	}
	return jsSlice(s, 300)
}

var (
	reTimeout = regexp.MustCompile(`(?i)Timeout|aborted`)
	re429     = regexp.MustCompile(`HTTP 429`)
	reHTTP    = regexp.MustCompile(`HTTP \d`)
	reNet     = regexp.MustCompile(`(?i)fetch failed|ECONN|ENOTFOUND|EAI_AGAIN|socket`)
	reParse   = regexp.MustCompile(`JSON|RSS|Unexpected token|有效`)
)

// kindOf mirrors net.ts kindOf.
func kindOf(e error) FailKind {
	var je *JSError
	if errors.As(e, &je) && je.Kind != "" {
		return je.Kind
	}
	n := e.Error()
	if errors.As(e, &je) {
		n = je.Name + " " + je.Msg
	}
	switch {
	case reTimeout.MatchString(n):
		return "timeout"
	case re429.MatchString(n):
		return "rate_limit"
	case reHTTP.MatchString(n):
		return "http"
	case reNet.MatchString(n):
		return "network"
	case reParse.MatchString(n):
		return "parse"
	}
	return "other"
}

// wrapNetErr turns a Go transport error into the shape fetch() errors have in Node.
func wrapNetErr(err error) error {
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return &JSError{Name: "TimeoutError", Msg: "The operation was aborted due to timeout"}
	}
	if errors.Is(err, context.Canceled) {
		return &JSError{Name: "AbortError", Msg: "This operation was aborted"}
	}
	cause := err.Error()
	var ue *url.Error
	if errors.As(err, &ue) {
		cause = ue.Err.Error()
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		cause = "getaddrinfo ENOTFOUND " + dns.Name
	} else if strings.Contains(cause, "connection refused") {
		cause = "connect ECONNREFUSED " + cause
	} else if cause == "EOF" || strings.HasSuffix(cause, ": EOF") {
		cause = "other side closed"
	} else if strings.Contains(cause, "connection reset") {
		cause = "read ECONNRESET"
	}
	return &JSError{Name: "TypeError", Msg: "fetch failed (" + cause + ")"}
}

// backoffD: exponential backoff with full jitter in [base·2^i/2, base·2^i], capped.
func backoffD(i int, base, cap time.Duration) time.Duration {
	d := base << i
	if d > cap || d <= 0 {
		d = cap
	}
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}

var httpClient = &http.Client{}

// Resp is a fully read HTTP response (bodies are small: feeds / JSON, capped at 8 MB).
type Resp struct {
	Status int
	Header http.Header
	Body   []byte
	URL    string // final URL (the real one, also under UPSTREAM_MOCK)
}

func (r *Resp) OK() bool { return r.Status >= 200 && r.Status < 300 }

// netFetch does one request; UPSTREAM_MOCK=http://127.0.0.1:4010 (tests only) sends it to the mock as /?u=<url>.
func netFetch(ctx context.Context, method, u string, hdr map[string]string, body io.Reader, timeout time.Duration) (*Resp, error) {
	target := u
	mock := os.Getenv("UPSTREAM_MOCK")
	if mock != "" {
		target = strings.TrimRight(mock, "/") + "/?u=" + url.QueryEscape(u)
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, &JSError{Name: "TypeError", Msg: "Failed to parse URL from " + u}
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	c := httpClient
	if isPublicOnly(ctx) {
		if err := checkTarget(ctx, u); err != nil {
			return nil, &JSError{Name: "TypeError", Msg: "fetch failed (" + err.Error() + ")"}
		}
		c = publicClient
	}
	r, err := c.Do(req)
	if err != nil {
		if errors.Is(err, errPrivateAddr) {
			return nil, &JSError{Name: "TypeError", Msg: "fetch failed (" + errPrivateAddr.Error() + ")"}
		}
		return nil, wrapNetErr(err)
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		return nil, wrapNetErr(err)
	}
	final := r.Request.URL.String()
	if mock != "" {
		final = u
	}
	return &Resp{Status: r.StatusCode, Header: r.Header, Body: b, URL: final}, nil
}

// retryAfter: Retry-After (seconds or HTTP date), capped at 60 s.
func retryAfter(r *Resp) (time.Duration, bool) {
	v := r.Header.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if n, err := strconv.ParseFloat(v, 64); err == nil {
		return min(60*time.Second, time.Duration(n*float64(time.Second))), true
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(0, min(60*time.Second, time.Until(t))), true
	}
	return 0, false
}

func sleepCtx(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// httpGet mirrors util.ts httpGet: per-attempt timeout, retries 429/5xx/network errors with jittered backoff.
func httpGet(ctx context.Context, u string, timeout time.Duration, tries int, headers map[string]string) (*Resp, error) {
	if tries <= 0 {
		tries = 3
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	h := map[string]string{
		"User-Agent":      UA,
		"Accept":          "application/rss+xml, application/atom+xml, application/xml;q=0.9, text/html;q=0.8, application/json;q=0.8, */*;q=0.5",
		"Accept-Language": "en-US,en;q=0.9,zh-CN;q=0.8",
	}
	for k, v := range headers {
		h[k] = v
	}
	var last error
	for i := 0; i < tries; i++ {
		r, err := netFetch(ctx, "GET", u, h, nil, timeout)
		if err == nil {
			switch r.Status {
			case 429, 500, 502, 503, 504:
				if i < tries-1 {
					d, ok := retryAfter(r)
					if !ok {
						d = backoffD(i, 1500*time.Millisecond, 15*time.Second)
					}
					sleepCtx(ctx, d)
					continue
				}
			}
			return r, nil
		}
		last = err
		if i < tries-1 {
			sleepCtx(ctx, backoffD(i, 1500*time.Millisecond, 15*time.Second))
		}
	}
	return nil, last
}
