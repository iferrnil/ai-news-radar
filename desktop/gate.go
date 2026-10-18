package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ---- local API gate (0.14.2 security review)
//
// The in-process server listens on 127.0.0.1, which every local process, every other user of the Mac and (with
// DNS rebinding) any website can reach. So:
//   - the Host header must be 127.0.0.1:<port> or localhost:<port> (a rebound attacker name is refused);
//   - every /api/* request except /api/health needs the per-launch session token, as the HttpOnly SameSite=Strict
//     cookie the window gets from its first URL (/?_k=<token>, redirected away at once) or as the X-Radar-Token
//     header (the shell's own Go clients);
//   - the radar package itself refuses cross-site browser requests (Sec-Fetch-Site / Origin).

const sessionCookie = "radar_session"

// apiToken is this launch's token (set by StartServer); localClient sends it.
var apiToken string

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func tokenOK(got, want string) bool {
	return want != "" && len(got) == len(want) && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func gate(next http.Handler, port int, token string) http.Handler {
	p := strconv.Itoa(port)
	hosts := map[string]bool{"127.0.0.1:" + p: true, "localhost:" + p: true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hosts[strings.ToLower(r.Host)] {
			http.Error(w, "unknown host", http.StatusMisdirectedRequest)
			return
		}
		if r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		if k := r.URL.Query().Get("_k"); k != "" && r.Method == http.MethodGet && !strings.HasPrefix(r.URL.Path, "/api/") {
			if tokenOK(k, token) {
				http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			}
			q := r.URL.Query()
			q.Del("_k")
			u := *r.URL
			u.RawQuery = q.Encode()
			http.Redirect(w, r, u.RequestURI(), http.StatusFound)
			return
		}
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			got := r.Header.Get("X-Radar-Token")
			if c, err := r.Cookie(sessionCookie); err == nil && got == "" {
				got = c.Value
			}
			if !tokenOK(got, token) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"detail":"unauthorized"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// tokenRT adds the session token to the shell's own requests to the local server.
type tokenRT struct{}

func (tokenRT) RoundTrip(r *http.Request) (*http.Response, error) {
	if h, _, err := net.SplitHostPort(r.URL.Host); err == nil && (h == "127.0.0.1" || h == "localhost") {
		r = r.Clone(r.Context())
		r.Header.Set("X-Radar-Token", apiToken)
	}
	return http.DefaultTransport.RoundTrip(r)
}

// localClient is the HTTP client for the shell's own calls to the local API.
var localClient = &http.Client{Transport: tokenRT{}}

// externalURL: a link the default browser / mail app may open — http(s) with a host and no user info, or mailto.
func externalURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return u.Opaque != ""
	}
	return false
}

// sameOrigin compares scheme + host:port (a prefix check let http://127.0.0.1:47821@evil.example through).
func sameOrigin(a, b string) bool {
	x, err1 := url.Parse(a)
	y, err2 := url.Parse(b)
	return err1 == nil && err2 == nil && x.User == nil && x.Scheme == y.Scheme && strings.EqualFold(x.Host, y.Host)
}
