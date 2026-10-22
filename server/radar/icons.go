package radar

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GET /api/icon?host=<source host> | ?x=<handle>: source avatars through the server, so the UI never loads a
// foreign icon service (Google favicons / unavatar) itself. Site icons: the page's <link rel=icon> (largest),
// then /favicon.ico, then the Google favicon service as a last resort — all fetched server-side. Only hosts of
// configured sources are served (no open proxy). Results are kept in <data dir>/icons for 14 days (misses 1 day).

var (
	iconLinkRe   = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	iconRelRe    = regexp.MustCompile(`(?i)\brel\s*=\s*["']?([^"'>]*)`)
	iconHrefRe   = regexp.MustCompile(`(?i)\bhref\s*=\s*["']?([^"'\s>]+)`)
	iconSizesRe  = regexp.MustCompile(`(?i)\bsizes\s*=\s*["']?(\d+)`)
	iconHandleRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,15}$`)
	iconHostRe   = regexp.MustCompile(`^[a-z0-9.-]+\.[a-z]{2,}$`)
	iconLocks    sync.Map
)

const (
	iconKeep    = 14 * 24 * time.Hour
	iconMissTTL = 24 * time.Hour
	iconMax     = 512 << 10
)

func (s *Store) iconDir() string { return filepath.Join(filepath.Dir(s.Cfg.DBPath), "icons") }

func (s *Store) iconHostAllowed(h string) bool {
	rs, _ := all(s.DB, "SELECT homepage, url FROM sources")
	for _, r := range rs {
		for _, u := range []string{r.Str("homepage"), r.Str("url")} {
			if p, err := url.Parse(u); err == nil && strings.EqualFold(p.Hostname(), h) {
				return true
			}
		}
	}
	return false
}

// iconHandleAllowed: /api/icon?x= is not an open avatar proxy — only handles of configured X sources.
func (s *Store) iconHandleAllowed(x string) bool {
	x = strings.ToLower(x)
	rs, _ := all(s.DB, "SELECT username FROM sources WHERE type IN ('x','x_list') AND username IS NOT NULL AND username != ''")
	for _, r := range rs {
		if strings.EqualFold(r.Str("username"), x) {
			return true
		}
	}
	return false
}

func (s *Store) hIcon(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var key string
	var fetch func(context.Context) ([]byte, string)
	if x := q.Get("x"); x != "" {
		if !iconHandleRe.MatchString(x) {
			detail(w, 400, "invalid handle")
			return
		}
		if !s.iconHandleAllowed(x) {
			detail(w, 404, "unknown handle")
			return
		}
		key = "x:" + strings.ToLower(x)
		fetch = func(ctx context.Context) ([]byte, string) {
			return iconGet(ctx, "https://unavatar.io/x/"+x+"?fallback=false")
		}
	} else {
		h := strings.ToLower(q.Get("host"))
		if !iconHostRe.MatchString(h) || !s.iconHostAllowed(h) {
			detail(w, 404, "unknown host")
			return
		}
		key = "h:" + h
		fetch = func(ctx context.Context) ([]byte, string) { return siteIcon(ctx, h) }
	}
	data, ct := s.iconCached(r.Context(), key, fetch)
	hd := w.Header()
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	if data == nil {
		// 204, not 404: the <img> still errors into its fallback, but the browser logs no failed resource
		hd.Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(204)
		return
	}
	hd.Set("Content-Type", ct)
	hd.Set("Cache-Control", "public, max-age=604800")
	w.Write(data)
}

// iconCached: disk cache "<ct>\n<bytes>" (a miss is "-\n"); one fetch per key at a time.
func (s *Store) iconCached(ctx context.Context, key string, fetch func(context.Context) ([]byte, string)) ([]byte, string) {
	sum := sha1.Sum([]byte(key))
	p := filepath.Join(s.iconDir(), hex.EncodeToString(sum[:8]))
	read := func() ([]byte, string, bool) {
		st, err := os.Stat(p)
		if err != nil {
			return nil, "", false
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, "", false
		}
		ct, data, _ := strings.Cut(string(b), "\n")
		age := time.Since(st.ModTime())
		if ct == "-" {
			return nil, "", age < iconMissTTL
		}
		return []byte(data), ct, age < iconKeep
	}
	if data, ct, fresh := read(); fresh {
		return data, ct
	}
	mu, _ := iconLocks.LoadOrStore(key, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	old, oldCT, fresh := read()
	if fresh {
		return old, oldCT
	}
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	data, ct := fetch(fctx)
	if data == nil && old != nil {
		os.Chtimes(p, time.Now(), time.Now()) // keep a stale icon rather than none
		return old, oldCT
	}
	os.MkdirAll(s.iconDir(), 0o755)
	if data == nil {
		os.WriteFile(p, []byte("-\n"), 0o644)
		return nil, ""
	}
	os.WriteFile(p, append([]byte(ct+"\n"), data...), 0o644)
	return data, ct
}

// iconGet: an image (by content sniffing) of at most 512 KB, else nil.
func iconGet(ctx context.Context, u string) ([]byte, string) {
	ctx = PublicOnly(ctx)
	r, err := httpGet(ctx, u, 8*time.Second, 1, map[string]string{"User-Agent": trendshiftBrowserUA, "Accept": "image/avif,image/webp,image/png,image/svg+xml,image/*;q=0.8,*/*;q=0.5"})
	if err != nil || !r.OK() || len(r.Body) == 0 || len(r.Body) > iconMax {
		return nil, ""
	}
	ct := http.DetectContentType(r.Body)
	head := strings.TrimSpace(string(r.Body[:min(len(r.Body), 512)]))
	switch {
	case strings.HasPrefix(ct, "image/"):
	case strings.Contains(head, "<svg"):
		ct = "image/svg+xml"
	case len(r.Body) > 4 && r.Body[0] == 0 && r.Body[1] == 0 && r.Body[2] == 1 && r.Body[3] == 0:
		ct = "image/x-icon"
	default:
		return nil, ""
	}
	return r.Body, ct
}

// siteIcon: best <link rel=icon> of the home page, /favicon.ico, then the Google favicon service.
func siteIcon(ctx context.Context, host string) ([]byte, string) {
	ctx = PublicOnly(ctx)
	home := "https://" + host + "/"
	var cands []string
	if r, err := httpGet(ctx, home, 8*time.Second, 1, map[string]string{"User-Agent": trendshiftBrowserUA, "Accept": "text/html"}); err == nil && r.OK() {
		base, _ := url.Parse(r.URL)
		if base == nil {
			base, _ = url.Parse(home)
		}
		cands = iconLinks(string(r.Body[:min(len(r.Body), 256<<10)]), base)
	}
	cands = append(cands, home+"favicon.ico", "https://www.google.com/s2/favicons?domain="+url.QueryEscape(host)+"&sz=64")
	for _, c := range cands {
		if data, ct := iconGet(ctx, c); data != nil {
			return data, ct
		}
	}
	return nil, ""
}

// iconLinks: icon URLs of a page, best first (apple-touch-icon / the largest sizes; tiny ones last).
func iconLinks(html string, base *url.URL) []string {
	type cand struct {
		u     string
		score int
	}
	var cs []cand
	for _, tag := range iconLinkRe.FindAllString(html, 40) {
		rel := iconRelRe.FindStringSubmatch(tag)
		href := iconHrefRe.FindStringSubmatch(tag)
		if rel == nil || href == nil || !strings.Contains(strings.ToLower(rel[1]), "icon") || strings.Contains(strings.ToLower(rel[1]), "mask") {
			continue
		}
		ref, err := url.Parse(strings.ReplaceAll(href[1], "&amp;", "&"))
		if err != nil {
			continue
		}
		u := base.ResolveReference(ref)
		if u.Scheme != "https" && u.Scheme != "http" {
			continue
		}
		score := 32
		if m := iconSizesRe.FindStringSubmatch(tag); m != nil {
			score, _ = strconv.Atoi(m[1])
		} else if strings.Contains(strings.ToLower(rel[1]), "apple-touch") {
			score = 180
		}
		if score > 256 { // huge PNGs: fine, but prefer ~64–256
			score = 256 - score/100
		}
		cs = append(cs, cand{u.String(), score})
	}
	for i := 1; i < len(cs); i++ { // insertion sort, best first, stable
		for j := i; j > 0 && cs[j].score > cs[j-1].score; j-- {
			cs[j], cs[j-1] = cs[j-1], cs[j]
		}
	}
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.u)
	}
	return out
}
