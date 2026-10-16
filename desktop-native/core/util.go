// Package core is the Go port of the web app's src/lib backend: fetchers, SQLite storage,
// sections, filter rules, notifications, translation and import/export.
package core

import (
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const UA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36 ai-news-crawler/1.0"

var trackRe = regexp.MustCompile(`^(utm_|ref$|ref_src$|source$|fbclid$|gclid$|s$|t$)`)

// NormURL mirrors util.ts normUrl: host(without www)+path(no trailing slash)+filtered query, lowercased.
func NormURL(u string) string {
	s := strings.TrimSpace(u)
	p, err := url.Parse(s)
	if err != nil || p.Host == "" {
		return strings.ToLower(s)
	}
	type kv struct{ k, v string }
	var q []kv
	// keep original order: parse raw query manually
	for _, part := range strings.Split(p.RawQuery, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		kk, _ := url.QueryUnescape(strings.ReplaceAll(k, "+", " "))
		vv, _ := url.QueryUnescape(strings.ReplaceAll(v, "+", " "))
		if trackRe.MatchString(kk) {
			continue
		}
		q = append(q, kv{kk, vv})
	}
	qs := ""
	if len(q) > 0 {
		parts := make([]string, len(q))
		for i, x := range q {
			parts[i] = url.QueryEscape(x.k) + "=" + url.QueryEscape(x.v)
		}
		qs = "?" + strings.Join(parts, "&")
	}
	path := strings.TrimRight(p.EscapedPath(), "/")
	if path == "" {
		path = "/"
	}
	host := strings.TrimPrefix(strings.ToLower(p.Host), "www.")
	return strings.ToLower(host + path + qs)
}

var (
	scriptRe = regexp.MustCompile(`(?is)<script.*?</script>|<style.*?</style>`)
	tagRe    = regexp.MustCompile(`<[^>]+>`)
	wsRe     = regexp.MustCompile(`[\s\p{Zs}\x{feff}]+`) // JS \s also matches NBSP etc.
)

// Decode HTML entities (twice-escaped safe: html.UnescapeString handles named + numeric).
func Decode(s string) string { return html.UnescapeString(s) }

// Clean strips tags/entities, collapses whitespace and caps to n runes (+ "…"), like util.ts clean.
func Clean(s string, n int) string {
	t := Decode(s)
	t = scriptRe.ReplaceAllString(t, " ")
	t = tagRe.ReplaceAllString(t, " ")
	t = Decode(t)
	t = strings.TrimSpace(wsRe.ReplaceAllString(t, " "))
	return Trunc(t, n, "…")
}

// Trunc caps s to n runes (JS slice counts UTF-16 units; runes are close enough for CJK/latin).
func Trunc(s string, n int, suffix string) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + suffix
}

var dateLayouts = []string{
	time.RFC1123Z, time.RFC1123, time.RFC3339Nano, time.RFC3339,
	"Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST", "Mon, 02 Jan 2006 15:04 -0700",
	"2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 06 15:04:05 -0700", "Mon Jan 2 15:04:05 -0700 2006", // X created_at
	"2006-01-02T15:04:05-0700", "2006-01-02T15:04:05Z0700", "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04Z07:00", "2006-01-02",
}

// ParseDate returns unix seconds or 0 when unparseable (JS returned null).
func ParseDate(v string) int64 {
	s := strings.TrimSpace(v)
	if s == "" {
		return 0
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil && !strings.ContainsAny(s, "-:") {
		if n > 1e12 {
			return int64(n / 1000)
		}
		return int64(n)
	}
	s = strings.Replace(s, " GMT", " +0000", 1)
	s = strings.Replace(s, " UT", " +0000", 1)
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}

func uniq(xs []string) []string {
	seen := map[string]bool{}
	out := xs[:0:0]
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func now() float64 { return float64(time.Now().UnixMilli()) / 1000 }
