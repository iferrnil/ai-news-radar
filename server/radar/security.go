package radar

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ---- request guards (0.14.2 security review)
//
// crossSite: a browser request to /api/* that another site started — a form post, fetch, <img> or a DNS-rebound
// page on a different origin. Browsers mark every request with Sec-Fetch-Site; older ones only send Origin on
// unsafe methods, so that is compared with the Host (or X-Forwarded-Host behind a proxy). Requests without either
// header (curl, the desktop shell's own Go client) are not browser-driven and pass.
func crossSite(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return false
	case "":
	default: // cross-site, same-site
		return true
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return false
	}
	u, err := url.Parse(o)
	if err != nil || u.Host == "" {
		return true // "null" (sandboxed frame, file://) or garbage
	}
	if strings.EqualFold(u.Host, r.Host) {
		return false
	}
	for _, h := range strings.Split(r.Header.Get("X-Forwarded-Host"), ",") {
		if h = strings.TrimSpace(h); h != "" && strings.EqualFold(u.Host, h) {
			return false
		}
	}
	return true
}

// hostAllowed: ALLOWED_HOSTS (comma-separated host or host:port, optional) pins the Host header so a DNS-rebound
// name cannot reach a self-hosted server; unset = any Host (the desktop shell pins 127.0.0.1 / localhost itself).
func hostAllowed(r *http.Request) bool {
	v := strings.TrimSpace(os.Getenv("ALLOWED_HOSTS"))
	if v == "" {
		return true
	}
	h := strings.ToLower(r.Host)
	hn := h
	if x, _, err := net.SplitHostPort(h); err == nil {
		hn = x
	}
	for _, a := range strings.Split(strings.ToLower(v), ",") {
		if a = strings.TrimSpace(a); a != "" && (a == h || a == hn) {
			return true
		}
	}
	return false
}

// securityHeaders: no framing (clickjacking), no MIME sniffing, no full Referer to other sites.
func securityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

// SafeLink keeps http(s) links only: item / homepage URLs come from feeds, and a file://, smb:// or javascript:
// link would be handed to the OS (or the page) when clicked.
func SafeLink(u string) string {
	s := strings.TrimSpace(u)
	p, err := url.Parse(s)
	if err != nil || p.Host == "" || (p.Scheme != "http" && p.Scheme != "https") || p.User != nil {
		return ""
	}
	return s
}

// Detect probes an add-source input (detectRaw) and keeps only http(s) links in the result.
func Detect(ctx context.Context, input string) (*Detected, error) {
	d, err := detectRaw(ctx, input)
	if d != nil {
		if d.Homepage != "" {
			d.Homepage = SafeLink(d.Homepage)
		}
		d.Sample = safeItems(d.Sample)
	}
	return d, err
}

// safeItems drops items whose link has another scheme (relative links become "").
func safeItems(items []FetchedItem) []FetchedItem {
	out := items[:0:0]
	for _, it := range items {
		if it.URL != "" {
			raw := it.URL
			if it.URL = SafeLink(raw); it.URL == "" {
				if p, err := url.Parse(strings.TrimSpace(raw)); err != nil || p.Scheme != "" || p.Host != "" {
					continue // file:, smb:, javascript:, userinfo tricks: the item goes
				}
				// a relative link (never resolvable here) just loses its link
			}
		}
		out = append(out, it)
	}
	return out
}

// ---- 工作目录 guards

var coverExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true}

// coverFile: p is a generated cover — an image file under <工作目录>/缓存/封面/, symlinks resolved — so the cover
// routes can never serve or copy another file (secrets, ~/.ssh, …) even if the 工作目录 setting points somewhere odd.
func (s *Store) coverFile(p string) (string, bool) {
	if p == "" || !filepath.IsAbs(p) || !coverExts[strings.ToLower(filepath.Ext(p))] {
		return "", false
	}
	root, err := filepath.EvalSymlinks(filepath.Join(s.cacheDir(), "封面"))
	if err != nil {
		return "", false
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(p))
	if err != nil || !strings.HasPrefix(real, root+string(filepath.Separator)) {
		return "", false
	}
	if st, err := os.Stat(real); err != nil || !st.Mode().IsRegular() {
		return "", false
	}
	return real, true
}

// isPackage: a macOS bundle name (Foo.app, x.pkg, y.workflow …): Finder / NSWorkspace would launch or install it
// instead of showing a folder, so the 工作目录 is never one and open never opens one.
func isPackage(p string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
		switch strings.ToLower(filepath.Ext(seg)) {
		case ".app", ".pkg", ".mpkg", ".bundle", ".plugin", ".prefpane", ".workflow", ".action", ".framework",
			".kext", ".appex", ".xpc", ".saver", ".qlgenerator", ".mdimporter", ".component", ".scpt", ".scptd",
			".command", ".terminal", ".tool", ".webloc", ".inetloc", ".fileloc":
			return true
		}
	}
	return false
}

// badArtifactsDir: the root, the home folder itself and system folders are refused as 工作目录 (the cache janitor and
// exports write there); so are bundles.
func badArtifactsDir(d string) bool {
	if isPackage(d) {
		return true
	}
	c := filepath.Clean(d)
	if home, err := os.UserHomeDir(); err == nil && c == filepath.Clean(home) {
		return true
	}
	if filepath.Dir(c) == c { // "/"
		return true
	}
	for _, sys := range []string{"/System", "/Library", "/Applications", "/Users", "/private", "/etc", "/bin", "/sbin", "/usr", "/var", "/tmp", "/dev", "/Volumes", "/opt", "/cores"} {
		if c == sys {
			return true
		}
	}
	return false
}
