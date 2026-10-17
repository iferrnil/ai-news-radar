package radar

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// siblingFinder returns a lookup for "another item of the same story": a tweet merged into the item, or an item
// of the 48 h window with the same URL / a near-identical title (briefpick.go sameStory). It yields the longest
// stored text of those siblings and their source name.
func (s *Store) siblingFinder() func(it *briefItem) (string, string) {
	var rows []*Row
	var keys []briefKey
	loaded := false
	load := func() {
		loaded = true
		rows, _ = all(s.DB, `SELECT i.id, i.title, i.title_zh, i.url, i.summary, i.content, i.merged_into, s.name AS source
			FROM items i JOIN sources s ON s.id=i.source_id WHERE i.hidden=0 ORDER BY i.published DESC LIMIT 2000`)
		keys = make([]briefKey, len(rows))
		for i, r := range rows {
			keys[i] = keyOf(r)
		}
	}
	return func(it *briefItem) (string, string) {
		if !loaded {
			load()
		}
		me := &Row{}
		me.Set("title", it.origTitle)
		me.Set("title_zh", it.titleZh)
		me.Set("url", it.URL)
		k := keyOf(me)
		best, src := "", ""
		for i, r := range rows {
			if int64(r.Num("id")) == it.id {
				continue
			}
			if int64(r.Num("merged_into")) != it.id && !sameStory(k, keys[i]) {
				continue
			}
			t := strings.TrimSpace(r.Str("content"))
			if sm := strings.TrimSpace(r.Str("summary")); utf8.RuneCountInString(sm) > utf8.RuneCountInString(t) {
				t = sm
			}
			if utf8.RuneCountInString(t) > utf8.RuneCountInString(best) {
				best, src = t, r.Str("source")
			}
		}
		return best, src
	}
}

var methodLabel = map[string]string{"self": "self", "jina": "jina", "firecrawl": "firecrawl", "crawl4ai": "crawl4ai", "feed": "feed"}

// saveOriginals (desktop): every original text a brief / article was written from goes to
// <制品目录>/原文/<date>/<time>-<title>.md with its source, URL and fetch method.
func (s *Store) saveOriginals(items []*briefItem, content, methods []string) {
	if !s.Cfg.Desktop {
		return
	}
	cst := time.FixedZone("CST", 8*3600)
	for i, it := range items {
		text := strings.TrimSpace(content[i])
		if text == "" {
			continue
		}
		title := firstNonEmpty(strings.TrimSpace(it.origTitle), it.Title)
		method := firstNonEmpty(methodLabel[methods[i]], "self")
		status := "full"
		if it.Fetch != nil && it.Fetch.Status != "" && method != "feed" {
			status = it.Fetch.Status
		}
		var b strings.Builder
		fmt.Fprintf(&b, "---\ntitle: %s\nsource: %s\nurl: %s\nmethod: %s\nstatus: %s\n", yamlStr(title), yamlStr(it.Source), it.URL, method, status)
		if it.Published > 0 {
			fmt.Fprintf(&b, "published: %s\n", time.Unix(int64(it.Published), 0).In(cst).Format("2006-01-02 15:04"))
		}
		fmt.Fprintf(&b, "fetched: %s\n---\n\n# %s\n\n%s\n", time.Now().In(cst).Format("2006-01-02 15:04:05"), title, text)
		s.saveArtifact("原文", title, b.String())
	}
}

func yamlStr(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
