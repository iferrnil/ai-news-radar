package core

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNormURL(t *testing.T) {
	cases := map[string]string{
		"https://www.Example.com/a/b/?utm_source=x&id=3": "example.com/a/b?id=3",
		"http://example.com":                             "example.com/",
		"https://x.com/foo/status/1?s=20&t=abc":          "x.com/foo/status/1",
	}
	for in, want := range cases {
		if got := NormURL(in); got != want {
			t.Errorf("NormURL(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCleanAndDate(t *testing.T) {
	if got := Clean("<p>Hello&nbsp;<b>world</b> &amp;mdash; ok</p>", 280); got != "Hello world — ok" {
		t.Errorf("Clean=%q", got)
	}
	if got := Clean(strings.Repeat("字", 10), 4); got != "字字字字…" {
		t.Errorf("trunc=%q", got)
	}
	for _, s := range []string{"Tue, 10 Mar 2026 08:00:00 GMT", "2026-03-10T08:00:00Z", "2026-03-10T16:00:00+08:00", "Tue Mar 10 08:00:00 +0000 2026", "1773129600"} {
		if got := ParseDate(s); got != 1773129600 {
			t.Errorf("ParseDate(%q)=%d", s, got)
		}
	}
}

const rss = `<?xml version="1.0"?><rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel>
<title>Demo &amp; Feed</title><link>https://demo.test/</link>
<item><title><![CDATA[First <i>post</i>]]></title><link>https://demo.test/1?utm_source=rss</link><pubDate>Tue, 10 Mar 2026 08:00:00 GMT</pubDate>
<dc:creator>Ann</dc:creator><content:encoded><![CDATA[<p>Body &mdash; one</p>]]></content:encoded></item>
<item><title>Guid only</title><guid>https://demo.test/2</guid></item>
<item><title>No link</title></item></channel></rss>`

const atom = `<?xml version="1.0" encoding="utf-8"?><feed xmlns="http://www.w3.org/2005/Atom"><title>Atom T</title>
<link rel="self" href="https://a.test/feed"/><link href="https://a.test/"/>
<entry><title type="html">E1</title><link rel="alternate" href="https://a.test/e1"/><updated>2026-03-10T08:00:00Z</updated><author><name>Bo</name></author><summary>S1</summary></entry></feed>`

const rdf = `<?xml version="1.0"?><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns="http://purl.org/rss/1.0/" xmlns:dc="http://purl.org/dc/elements/1.1/">
<channel><title>R</title><link>https://r.test</link></channel><item><title>R1</title><link>https://r.test/1</link><dc:date>2026-03-10T08:00:00Z</dc:date></item></rdf:RDF>`

func TestParseFeed(t *testing.T) {
	f := ParseFeed([]byte(rss))
	if f == nil || f.Title != "Demo & Feed" || len(f.Items) != 2 {
		t.Fatalf("rss: %+v", f)
	}
	it := f.Items[0]
	if it.Title != "First post" || it.UID != "demo.test/1" || it.Author != "Ann" || it.Summary != "Body — one" || it.Published != 1773129600 {
		t.Errorf("rss item: %+v", it)
	}
	if f.Items[1].URL != "https://demo.test/2" {
		t.Errorf("guid link: %+v", f.Items[1])
	}
	a := ParseFeed([]byte(atom))
	if a == nil || a.Link != "https://a.test/" || len(a.Items) != 1 || a.Items[0].URL != "https://a.test/e1" || a.Items[0].Author != "Bo" || a.Items[0].Published != 1773129600 {
		t.Fatalf("atom: %+v", a)
	}
	r := ParseFeed([]byte(rdf))
	if r == nil || len(r.Items) != 1 || r.Items[0].Published != 1773129600 {
		t.Fatalf("rdf: %+v", r)
	}
	if ParseFeed([]byte("<html><body>nope</body></html>")) != nil {
		t.Error("html should not parse")
	}
}

func TestDiscover(t *testing.T) {
	got := DiscoverFeeds(`<link rel="alternate" type="application/rss+xml" href="/feed.xml?a=1&amp;b=2">`, "https://s.test/blog/")
	if len(got) != 1 || got[0] != "https://s.test/feed.xml?a=1&b=2" {
		t.Errorf("%v", got)
	}
}

func TestAPIAdapters(t *testing.T) {
	hn := `{"hits":[{"title":"Show HN","url":"","objectID":"42","points":10,"num_comments":3,"author":"pg","created_at_i":1773129600}]}`
	items, err := ParseAPI("https://hn.algolia.com/api/v1/search", []byte(hn))
	if err != nil || len(items) != 1 || items[0].URL != "https://news.ycombinator.com/item?id=42" || !strings.HasPrefix(items[0].Summary, "HN 10 points · 3 comments") {
		t.Fatalf("hn: %v %+v", err, items)
	}
	ne := `data_callback([{"docurl":"https://ent.163.com/a.html","title":"标题","time":"03/10/2026 16:00:00","keywords":[{"keyname":"甲"},{"keyname":"乙"}]}])`
	items, err = ParseAPI("https://ent.163.com/special/xx", []byte(ne))
	if err != nil || len(items) != 1 || items[0].Published != 1773129600 || items[0].Summary != "甲 · 乙" {
		t.Fatalf("163: %v %+v", err, items)
	}
	if _, err := ParseAPI("https://unknown.test/", []byte(`{}`)); err == nil {
		t.Error("unknown api should error")
	}
}

func TestXParsers(t *testing.T) {
	body := `{"results":[{"id":"1","url":"https://x.com/a/status/1","text":"Hello https://t.co/x\nmore","author":{"screen_name":"A"},"created_timestamp":1773129600,
	"raw_text":{"facets":[{"type":"url","replacement":"https://blog.test/p"}]}},
	{"id":"2","text":"rt","author":{"screen_name":"other"}},{"id":"3","text":"reply","author":{"screen_name":"a"},"replying_to":{"screen_name":"z"}}]}`
	items, err := ParseXStatuses("a", []byte(body))
	if err != nil || len(items) != 1 || items[0].Title != "Hello" || items[0].UID != "x:1" || items[0].Links[0] != "https://blog.test/p" {
		t.Fatalf("%v %+v", err, items)
	}
	html := `<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"headerProps":{"listName":"L"},"timeline":{"entries":[
	{"type":"tweet","content":{"tweet":{"id_str":"9","full_text":"Hi","user":{"screen_name":"u"},"created_at":"Tue Mar 10 08:00:00 +0000 2026"}}},
	{"type":"tweet","content":{"tweet":{"id_str":"8","full_text":"RT @x: y","user":{"screen_name":"u"}}}}]}}}}</script>`
	h, entries, err := ParseSyndication([]byte(html))
	if err != nil || js(h["listName"]) != "L" {
		t.Fatal(err)
	}
	li := XListItems(entries)
	if len(li) != 1 || li[0].URL != "https://x.com/u/status/9" || li[0].Published != 1773129600 {
		t.Fatalf("%+v", li)
	}
}

func TestRules(t *testing.T) {
	r := Rule{Include: "gpt|claude", Exclude: "rumou?r"}
	k := r.Keeps([]string{"New GPT model", "Claude rumor", "Gemini"})
	if fmt.Sprint(k) != "[true false false]" {
		t.Errorf("keeps=%v", k)
	}
	cs := Rule{Include: "GPT", CS: true}
	if cs.Keeps([]string{"gpt"})[0] {
		t.Error("case-sensitive")
	}
	for p, bad := range map[string]bool{"(a+)+": true, `(x)\1`: true, "foo(?=bar)": true, "[": true, "ok|fine": false, `\u4e2d`: false, "(?<n>a)": false} {
		if got := StaticCheck(p) != ""; got != bad {
			t.Errorf("StaticCheck(%q) bad=%v want %v (%s)", p, got, bad, StaticCheck(p))
		}
	}
	if RuleText("all", "T", "S") != "T\nS" || RuleText("title", "T", "S") != "T" {
		t.Error("RuleText")
	}
}

func TestNeedsTranslation(t *testing.T) {
	if !NeedsTranslation("OpenAI releases a new model today", "") || NeedsTranslation("OpenAI 发布新模型 GPT-6", "") {
		t.Error("heuristic")
	}
}

func newStore(t *testing.T) *Store {
	t.Helper()
	seed := `{"sources":[{"name":"Seed A","type":"rss","url":"https://a.test/feed","group":"官方实验室","priority":"P0"},
	{"name":"Seed X","type":"x","url":"https://api.fxtwitter.com/2/profile/sx/statuses","username":"sx","group":"X 快讯"},
	{"name":"HTML","type":"html","url":"https://h.test/"}]}`
	s, err := Open(t.TempDir(), []byte(seed))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func srcByName(t *testing.T, s *Store, name string) Source {
	list, _ := s.Sources()
	for _, x := range list {
		if x.Name == name {
			return x
		}
	}
	t.Fatalf("no source %s", name)
	return Source{}
}

func TestStorage(t *testing.T) {
	s := newStore(t)
	secs, _ := s.Sections()
	if len(secs) != 6 || secs[0].Key != "lab" || secs[4].InAll { // 5 defaults + 未分类 (HTML source has no mapping? -> media)
		names := []string{}
		for _, x := range secs {
			names = append(names, x.Key+":"+x.Name)
		}
		if len(secs) < 5 {
			t.Fatalf("sections: %v", names)
		}
	}
	a := srcByName(t, s, "Seed A")
	if a.Section != "lab" || a.PollInterval != 300 || !a.Supported {
		t.Errorf("seed A: %+v", a)
	}
	if h := srcByName(t, s, "HTML"); h.Supported {
		t.Error("html should be unsupported")
	}
	nowS := time.Now().Unix()
	items := []FetchedItem{
		{UID: "a.test/1", Title: "Fresh GPT news", URL: "https://a.test/1", Published: nowS - 60},
		{UID: "a.test/old", Title: "Too old", URL: "https://a.test/old", Published: nowS - 72*3600},
		{UID: "a.test/nodate", Title: "Undated backlog", URL: "https://a.test/nd"},
		{UID: "a.test/spam", Title: "Sponsored stuff", URL: "https://a.test/spam", Published: nowS - 30},
	}
	if _, err := s.SetRule(a.ID, Rule{Exclude: "sponsored"}); err != nil {
		t.Fatal(err)
	}
	a = srcByName(t, s, "Seed A")
	ids, err := s.StoreItems(a, items)
	if err != nil || len(ids) != 1 {
		t.Fatalf("store: %v %v", err, ids)
	}
	// second time: all seen
	if ids2, _ := s.StoreItems(a, items); len(ids2) != 0 {
		t.Error("dedupe failed")
	}
	// X post linking the article merges into it
	x := srcByName(t, s, "Seed X")
	xi := []FetchedItem{{UID: "x:1", Title: "Look", URL: "https://x.com/sx/status/1", IsX: true, Author: "sx", Published: nowS, Links: []string{"https://a.test/1?utm_source=x"}}}
	if ids, _ := s.StoreItems(x, xi); len(ids) != 0 {
		t.Error("x post should merge")
	}
	all, _ := s.Items(Query{})
	if len(all) != 1 || len(all[0].AlsoOnX) != 1 || all[0].AlsoOnX[0].URL != "https://x.com/sx/status/1" {
		t.Fatalf("items: %+v", all)
	}
	// read / unread
	if s.UnreadCounts()[""] != 1 {
		t.Error("unread count")
	}
	s.SetRead([]int64{all[0].ID}, true)
	if s.UnreadCounts()[""] != 0 {
		t.Error("read")
	}
	if u, _ := s.Items(Query{UnreadOnly: true}); len(u) != 0 {
		t.Error("unread filter")
	}
	// retro rule hides
	if n, _ := s.SetRule(a.ID, Rule{Include: "claude"}); n != 1 {
		t.Errorf("hidden=%d", n)
	}
	if v, _ := s.Items(Query{}); len(v) != 0 {
		t.Error("rule should hide")
	}
	s.SetRule(a.ID, Rule{})
	// prune drops items published before cutoff
	s.DB.Exec("UPDATE items SET published=published-49*3600")
	if n, _ := s.Prune(); n == 0 {
		t.Error("prune")
	}
	// delete seed -> tombstone, reseed does not re-add
	if n, _ := s.DeleteSources([]int64{a.ID}); n != 1 {
		t.Error("delete")
	}
	s.Seed([]byte(`{"sources":[{"name":"Seed A","type":"rss","url":"https://a.test/feed"}]}`))
	list, _ := s.Sources()
	for _, l := range list {
		if l.Name == "Seed A" {
			t.Error("tombstone ignored")
		}
	}
	// rename keeps seed_name
	if err := s.RenameSource(x.ID, "我的 X"); err != nil {
		t.Fatal(err)
	}
	s.Seed([]byte(`{"sources":[{"name":"Seed X","type":"x","url":"u","username":"sx"}]}`))
	if n := len(s.strings("SELECT name FROM sources WHERE name IN ('Seed X','我的 X')")); n != 1 {
		t.Errorf("rename + reseed duplicated: %d", n)
	}
}

func TestSections(t *testing.T) {
	s := newStore(t)
	k, err := s.CreateSection("工具", "🔧", true)
	if err != nil || !strings.HasPrefix(k, "c") {
		t.Fatal(k, err)
	}
	if k2, _ := s.CreateSection("工具", "", true); k2 != k {
		t.Error("reuse by name")
	}
	if _, err := s.CreateSection("全部", "", true); err == nil {
		t.Error("reserved")
	}
	x := srcByName(t, s, "Seed X")
	s.SetSection(x.ID, k)
	if err := s.DeleteSection(k, "media"); err != nil {
		t.Fatal(err)
	}
	if srcByName(t, s, "Seed X").Section != "media" {
		t.Error("move on delete")
	}
}

func TestImportExportRoundTrip(t *testing.T) {
	s := newStore(t)
	k, _ := s.CreateSection("工具", "🔧", false)
	a := srcByName(t, s, "Seed A")
	s.SetSection(a.ID, k)
	s.SetRule(a.ID, Rule{Include: "gpt", Scope: "all", CS: true})
	j, err := s.ExportJSON(time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC))
	if err != nil || !strings.Contains(string(j), `"version": 2`) || !strings.Contains(string(j), `"exportedAt": "2026-03-10T08:00:00+08:00"`) {
		t.Fatalf("%v %s", err, j)
	}
	o, _ := s.ExportOPML(time.Now())
	if !strings.Contains(string(o), `sectionCustom="1" sectionKey="`+k+`"`) || !strings.Contains(string(o), `filterInclude="gpt" filterScope="all" filterCase="true"`) {
		t.Fatalf("opml: %s", o)
	}
	// import both into a fresh store (no seed)
	for _, c := range []struct {
		name string
		data []byte
	}{{"x.json", j}, {"x.opml", o}} {
		d, _ := Open(t.TempDir(), nil)
		format, rows, err := d.ParseImport(c.data, c.name, "media")
		if err != nil {
			t.Fatal(err)
		}
		p, _ := d.PlanImport(rows, "merge")
		wantNew := 2 // JSON: html sources export but are not importable (same as web)
		if format == "opml" {
			wantNew = 3 // OPML writes html sources as type="rss" (same as web)
		}
		if p.New != wantNew {
			t.Fatalf("%s plan: %+v", format, p)
		}
		if _, err := d.ApplyImport(p, "merge"); err != nil {
			t.Fatal(err)
		}
		got := srcByName(t, d, "Seed A")
		secs, _ := d.Sections()
		var sec Section
		for _, x := range secs {
			if x.Key == got.Section {
				sec = x
			}
		}
		if sec.Name != "工具" || sec.InAll || got.Rule.Include != "gpt" || !got.Rule.CS || got.Rule.Scope != "all" || got.Priority != "P0" {
			t.Errorf("%s: %+v sec %+v", format, got, sec)
		}
		// re-import merge => all dup; replace with subset removes others
		p2, _ := d.PlanImport(rows, "merge")
		if p2.Dup != wantNew {
			t.Errorf("dup: %+v", p2)
		}
		var one []ParsedRow
		for _, r := range rows {
			if r.Name == "Seed A" {
				one = append(one, r)
			}
		}
		p3, _ := d.PlanImport(one, "replace")
		if len(p3.Remove) != wantNew-1 {
			t.Errorf("replace: %+v", p3.Remove)
		}
		d.Close()
	}
	// v1 file + bad rows
	v1 := `{"app":"ai-news-radar","version":1,"sources":[{"name":"N","type":"rss","url":"https://n.test/f","section":"cn"},{"name":"","type":"rss"},{"name":"bad","type":"x"},{"name":"F","type":"rss","url":"ftp://x"}]}`
	_, rows, err := s.ParseImport([]byte(v1), "a.json", "media")
	if err != nil || rows[0].Entry == nil || rows[1].Error != "名称为空" || rows[2].Error != "X 账号缺少 username" || rows[3].Error != "只允许 http(s) URL" {
		t.Fatalf("%v %+v", err, rows)
	}
	if _, _, err := s.ParseImport([]byte(`{"app":"other","version":1,"sources":[]}`), "a.json", "media"); err == nil {
		t.Error("foreign json")
	}
}

func TestNotifier(t *testing.T) {
	s := newStore(t)
	got := make(chan *NotifyBatch, 1)
	s.OnEvent = func(e Event) {
		if e.Kind == "notify" {
			got <- e.Batch
		}
	}
	s.notifier.batchWait, s.notifier.minGap = 10*time.Millisecond, 0
	p0 := Item{ID: 1, Title: "Big", Priority: "P0", Published: now(), Section: "lab"}
	s.notifier.onPolled(7, []Item{p0}) // first poll = backlog
	s.notifier.onPolled(7, []Item{p0, {ID: 2, Priority: "P1", Published: now()}, {ID: 3, Priority: "P0", Published: now() - 4*3600}})
	select {
	case b := <-got:
		if b.Count != 1 || b.Items[0].ID != 1 {
			t.Errorf("%+v", b)
		}
	case <-time.After(time.Second):
		t.Fatal("no batch")
	}
	s.SetNotifyPrefs(NotifyPrefs{Enabled: false})
	s.notifier.onPolled(7, []Item{{ID: 9, Priority: "P0", Published: now()}})
	select {
	case <-got:
		t.Error("disabled prefs still notified")
	case <-time.After(50 * time.Millisecond):
	}
}
