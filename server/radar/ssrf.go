package radar

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

// ---- SSRF guard (0.14.2)
//
// Fetches whose target comes from content or from anyone who can reach the API — item links (原文 for briefs /
// articles), source icons, 写作风格 URLs, Skill downloads, and on the web server also add-source probes and polls —
// may only connect to public addresses: loopback, private, link-local (169.254.169.254 metadata), CGNAT,
// unspecified and multicast are refused at dial time, so redirects and DNS rebinding are covered too.
// The desktop app keeps allowing private addresses for the sources the user adds (a local RSSHub etc.).
// ALLOW_PRIVATE_FETCH=1 lifts the guard (self-hosters with a LAN feed); UPSTREAM_MOCK (tests) does as well.

var errPrivateAddr = errors.New("不允许访问本机或内网地址")

// privateOK is the ALLOW_PRIVATE_FETCH switch (tests flip it).
var privateOK = os.Getenv("ALLOW_PRIVATE_FETCH") == "1"

type publicOnlyKey struct{}

// PublicOnly marks ctx: fetches made with it may only reach public addresses.
func PublicOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, publicOnlyKey{}, true)
}

func guardOff() bool { return privateOK || os.Getenv("UPSTREAM_MOCK") != "" }

func isPublicOnly(ctx context.Context) bool {
	return !guardOff() && ctx.Value(publicOnlyKey{}) != nil
}

var cgnat = &net.IPNet{IP: net.IP{100, 64, 0, 0}, Mask: net.CIDRMask(10, 32)}
var benchNet = &net.IPNet{IP: net.IP{198, 18, 0, 0}, Mask: net.CIDRMask(15, 32)}
var nat64 = &net.IPNet{IP: net.ParseIP("64:ff9b::"), Mask: net.CIDRMask(96, 128)}

// blockedIP: not a public unicast address.
func blockedIP(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		ip = v4
		if v4[0] == 0 || cgnat.Contains(v4) || benchNet.Contains(v4) {
			return true
		}
	} else if nat64.Contains(ip) {
		return blockedIP(ip[12:16])
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

// proxyAddrs: host:port of the configured HTTP(S) proxies — the guarded dialer may connect to those (a local
// Clash / Surge proxy); the target behind a proxy is checked before the request (checkTarget).
func proxyAddrs() map[string]bool {
	out := map[string]bool{}
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if v := os.Getenv(k); v != "" {
			if u, err := url.Parse(v); err == nil && u.Host != "" {
				h, p := u.Hostname(), u.Port()
				if p == "" {
					p = map[string]string{"https": "443", "socks5": "1080"}[u.Scheme]
					if p == "" {
						p = "80"
					}
				}
				out[net.JoinHostPort(h, p)] = true
			}
		}
	}
	return out
}

func guardControl(network, address string, _ syscall.RawConn) error {
	if guardOff() || proxyAddrs()[address] {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errPrivateAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || blockedIP(ip) {
		return errPrivateAddr
	}
	return nil
}

func guardedTransport() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	d := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second, Control: guardControl}
	tr.DialContext = d.DialContext
	return tr
}

var publicClient = &http.Client{Transport: guardedTransport()}

// checkTarget resolves u's host and refuses non-public addresses (used up front, also when a proxy would make
// the connection for us).
func checkTarget(ctx context.Context, u string) error {
	if guardOff() {
		return nil
	}
	p, err := url.Parse(u)
	if err != nil || p.Hostname() == "" {
		return errPrivateAddr
	}
	h := strings.TrimSuffix(p.Hostname(), ".")
	if strings.EqualFold(h, "localhost") || strings.HasSuffix(strings.ToLower(h), ".localhost") {
		return errPrivateAddr
	}
	if ip := net.ParseIP(h); ip != nil {
		if blockedIP(ip) {
			return errPrivateAddr
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, h)
	if err != nil {
		return nil // let the request fail with the usual DNS error
	}
	for _, a := range ips {
		if blockedIP(a.IP) {
			return errPrivateAddr
		}
	}
	return nil
}

// fetchCtx: source probes and polls are public-only on the web server (anyone who can reach it may add a source);
// the desktop app lets its user subscribe to local / LAN feeds.
func (s *Store) fetchCtx(ctx context.Context) context.Context {
	if s.Cfg.Desktop {
		return ctx
	}
	return PublicOnly(ctx)
}
