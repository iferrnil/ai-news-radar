package radar

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestDiskCacheTTL(t *testing.T) {
	d := t.TempDir()
	t.Setenv("ARTIFACTS_DIR", filepath.Join(d, "work"))
	st, err := Open(Config{DBPath: filepath.Join(d, "news.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "secrets.json"), Desktop: true})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reset := func() {
		gArticles.mu.Lock()
		gArticles.m = map[string]articleCached{}
		gArticles.mu.Unlock()
	}
	reset()
	gDisk.mu.Lock()
	gDisk.lastClean = time.Now() // no background clean racing the test
	gDisk.mu.Unlock()
	u := "https://news.example.com/a?id=1"
	text := strings.Repeat("Full text of the story. ", 40)
	gArticles.put(u, articleRes{Text: text, Title: `He said "hi"`, Status: "full", Method: "firecrawl"})
	gArticles.put("https://news.example.com/b", articleRes{Text: "teaser", Status: "partial", Method: "self"})
	f := diskCacheFile(st.cacheDir(), u)
	if !strings.HasPrefix(f, filepath.Join(d, "work", "缓存", "原文", "news.example.com")) {
		t.Fatalf("cache path %s", f)
	}
	if fs, _ := filepath.Glob(filepath.Join(st.cacheDir(), "原文", "*", "*.md")); len(fs) != 1 {
		t.Fatalf("only full texts are cached: %v", fs)
	}
	reset()
	r, ok := gArticles.get(u)
	if !ok || r.Method != "firecrawl" || r.Title != `He said "hi"` || strings.TrimSpace(r.Text) != strings.TrimSpace(text) {
		t.Fatalf("disk hit: %v %+v", ok, r)
	}
	// older than a day: a miss, and the cleaner deletes it (and the empty folders)
	old := time.Now().Add(-25 * time.Hour)
	os.Chtimes(f, old, old)
	reset()
	if _, ok := gArticles.get(u); ok {
		t.Fatal("stale entry used")
	}
	fresh := filepath.Join(st.cacheDir(), "封面", "2026-10-17", "x.png")
	os.MkdirAll(filepath.Dir(fresh), 0o755)
	os.WriteFile(fresh, []byte("png"), 0o644)
	if n := cleanCache(st.cacheDir(), cacheTTL); n != 1 {
		t.Fatalf("cleaned %d", n)
	}
	if _, err := os.Stat(filepath.Dir(f)); err == nil {
		t.Fatal("empty folder left")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("fresh file removed")
	}
	if cleanCache(filepath.Join(d, "work"), 0) != 0 {
		t.Fatal("cleaned outside 缓存/")
	}
	// web server: no disk cache
	st.Close()
	st2, _ := Open(Config{DBPath: filepath.Join(d, "web.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "s2.json")})
	defer st2.Close()
	reset()
	gArticles.put("https://news.example.com/c", articleRes{Text: text, Status: "full"})
	if fs, _ := filepath.Glob(filepath.Join(st.cacheDir(), "原文", "*", "*.md")); len(fs) != 0 {
		t.Fatalf("web server wrote the cache: %v", fs)
	}
}

func TestCondense(t *testing.T) {
	var b strings.Builder
	b.WriteString("OpenAI on Tuesday released GPT-9, a model it says is 40% cheaper than GPT-8 and handles 2 million tokens.\n")
	b.WriteString("Share on X\nSubscribe to our newsletter\n![chart](https://x/c.png)\n[Home](https://x) | [AI](https://x/ai)\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "Filler paragraph %c number words that go on without much in them, background and colour for the reader here.\n", 'a'+i%26)
	}
	b.WriteString("“We expect revenue of $12 billion next year,” Sam Altman said.\n")
	b.WriteString("Copyright 2026 Example Media. All rights reserved.\n")
	src := b.String()
	out := condense(src, 600)
	n := utf8.RuneCountInString(strings.NewReplacer("\n", "", "…", "").Replace(out))
	if n > 600 || !strings.HasPrefix(out, "OpenAI on Tuesday") || !strings.Contains(out, "$12 billion") {
		t.Fatalf("condensed (%d):\n%s", n, out)
	}
	for _, bad := range []string{"Share on X", "Subscribe", "![chart]", "[Home]", "Copyright"} {
		if strings.Contains(out, bad) {
			t.Fatalf("boilerplate %q kept:\n%s", bad, out)
		}
	}
	if !strings.Contains(out, "…") {
		t.Fatal("gap not marked")
	}
	// within budget: only the boilerplate goes; repeated paragraphs once
	small := condense("Lead paragraph with 3 facts.\nShare on X\nLead paragraph with 3 facts.\nSecond point here.", 1000)
	if small != "Lead paragraph with 3 facts.\n\nSecond point here." {
		t.Fatalf("small: %q", small)
	}
	// Chinese: a long single paragraph is cut by sentences, the lead and the numbers first
	zh := "阿里巴巴周三发布通义千问 4，参数量 1.2 万亿。" + strings.Repeat("这是一段没有信息量的背景描述，用来填充篇幅。", 60) + "公司说，新模型的价格比上一代低 60%。"
	zo := condense(zh, 200)
	if !strings.HasPrefix(zo, "阿里巴巴周三发布通义千问 4") || !strings.Contains(zo, "低 60%") || utf8.RuneCountInString(zo) > 210 {
		t.Fatalf("zh: %q", zo)
	}
	// total budget across items; short texts leave their share to the long ones
	texts := []string{strings.Repeat("Short one. ", 5), src, src, src}
	all := condenseAll(texts, 1400, 1500)
	sum := 0
	for _, x := range all {
		sum += utf8.RuneCountInString(x)
	}
	if sum > 1500+30 || all[0] != strings.TrimSpace(texts[0]) {
		t.Fatalf("total %d: %q", sum, all[0])
	}
}

func TestSlopLint(t *testing.T) {
	if is := slopLint("# 标题\n\n综上所述，这是重磅消息。值得注意的是，它很快。\n\n## 原文链接\n\n1. 综上所述", "brief"); len(is) != 1 || !strings.Contains(is[0], "综上所述") || !strings.Contains(is[0], "重磅") || !strings.Contains(is[0], "值得注意的是") {
		t.Fatalf("banned: %v", is)
	}
	clean := "# OpenAI 把 GPT-9 的价格降了 40%\n\nOpenAI 周二发布 GPT-9，价格比 GPT-8 低 40% [[1]](https://e.com/1)。\n\n## 价格\n\n每百万 token 2 美元。\n"
	if is := slopLint(clean, "wechat"); len(is) != 0 {
		t.Fatalf("clean wechat flagged: %v", is)
	}
	long := "1/ " + strings.Repeat("汉", 135) + " [[1]](https://e.com/1)\n\n2/ 短的一条。\n"
	if is := slopLint(long, "twitter"); len(is) != 1 || !strings.Contains(is[0], "第 1 条") {
		t.Fatalf("tweet weight: %v", is)
	}
	if w := xWeight("abc 汉字 https://e.com/very/long/path"); w != 4+4+1+23 {
		t.Fatalf("xWeight %d", w)
	}
	xhs := "这是一个超过二十个字的小红书标题会被提醒修改的那种标题\n\n正文。\n\n#AI #模型 #工具"
	if is := slopLint(xhs, "xhs"); len(is) != 1 || !strings.Contains(is[0], "标题") {
		t.Fatalf("xhs: %v", is)
	}
	if is := slopLint("短标题\n\n正文没有话题。", "xhs"); len(is) != 1 || !strings.Contains(is[0], "话题") {
		t.Fatalf("xhs tags: %v", is)
	}
	if is := slopLint("这不是 A 而是 B。那不是 C 而是 D。此外，然而，同时，另外，并且。他高兴地说。", "brief"); len(is) != 3 {
		t.Fatalf("patterns: %v", is)
	}
}

// A brief's originals the self-written fetcher cannot get go to the 抓取 API (keys configured); an item left with
// nothing is dropped and listed.
func TestBriefProviderFallbackAndDrop(t *testing.T) {
	g := newProvRig(t)
	st := g.st
	ds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"items\":[]}"}}]}`)
	}))
	defer ds.Close()
	t.Setenv("DEEPSEEK_API_KEY", "sk-"+"unit"+"test0000")
	t.Setenv("DEEPSEEK_BASE_URL", ds.URL)
	selfFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer selfFail.Close()
	var sid int
	st.DB.QueryRow("SELECT id FROM sources WHERE enabled=1 AND type NOT LIKE 'x%' ORDER BY id LIMIT 1").Scan(&sid)
	add := func(uid, title, u, summary string) int64 {
		res, _ := st.DB.Exec(`INSERT INTO items(uid,source_id,title,url,summary,published,fetched) VALUES(?,?,?,?,?,?,?)`, uid, sid, title, u, summary, now(), now())
		id, _ := res.LastInsertId()
		return id
	}
	a := add("a", "Alpha story one", selfFail.URL+"/a", "")
	b := add("b", "Bravo story two", g.selfURL, "")
	c := add("c", "Charlie story three", selfFail.URL+"/c", "")
	gArticles.mu.Lock()
	gArticles.m = map[string]articleCached{}
	gArticles.mu.Unlock()

	// jina works: the failed originals come from it
	rule, err := st.buildRuleBrief("all", false, a, b)
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.gatherFacts(rule)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, it := range f.items {
		got[it.Title] = it.Fetch.Method + "/" + it.Fetch.Status
	}
	if got["Alpha story one"] != "jina/full" || got["Bravo story two"] != "self/full" || len(f.out.NoFullText) != 0 {
		t.Fatalf("fallback: %v %+v", got, f.out.NoFullText)
	}
	// every provider down: the item with nothing is dropped and listed, the brief goes on
	for _, p := range []*fakeProv{g.jina, g.fc, g.c4} {
		p.mode.Store("500")
	}
	rule, _ = st.buildRuleBrief("all", false, b, c)
	f, err = st.gatherFacts(rule)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.items) != 1 || f.items[0].Title != "Bravo story two" || f.out.Count != 1 || strings.Contains(f.out.Markdown, "Charlie") {
		t.Fatalf("drop: %d items, %s", len(f.items), f.out.Markdown)
	}
	if len(f.out.NoFullText) != 1 || !f.out.NoFullText[0].Dropped || f.out.NoFullText[0].Title != "Charlie story three" || !strings.Contains(f.out.NoFullText[0].Reason, "服务错误") {
		t.Fatalf("dropped list: %+v", f.out.NoFullText)
	}
	// no key configured: no provider call at all
	sec := st.loadSecrets()
	sec.Fetch = FetchKeys{}
	st.saveSecrets(sec)
	for _, p := range []*fakeProv{g.jina, g.fc, g.c4} {
		p.mode.Store("ok")
	}
	gArticles.mu.Lock()
	gArticles.m = map[string]articleCached{}
	gArticles.mu.Unlock()
	os.RemoveAll(st.cacheDir())
	h := g.jina.hits.Load()
	rule, _ = st.buildRuleBrief("all", false, a, b)
	if f, _ = st.gatherFacts(rule); g.jina.hits.Load() != h || len(f.items) != 1 {
		t.Fatalf("providers used without keys (%d items)", len(f.items))
	}
}

// The built-in article Skills v2: their prompt carries the checklist, the platform rules and the template (not the
// credits), keeps the factual rules and each platform's cover marker, and notes that the script is not run.
func TestArticleSkillsV2Prompt(t *testing.T) {
	d := t.TempDir()
	st, err := Open(Config{DBPath: filepath.Join(d, "news.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "secrets.json"), Desktop: true})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for kind, cover := range map[string]string{"twitter": "", "wechat": "cover-image size=1792x1024", "xhs": "cover-image size=1024x1536"} {
		sys, name := st.skillSystemPrompt(kind)
		for _, want := range []string{`version: "2.0"`, "逐字照抄", "原文未说明", `<reference path="references/checklist.md">`, `<reference path="references/platform.md">`, `<reference path="assets/template.md">`, "scripts/check.py", "## 自检", "不是……而是"} {
			if want == `version: "2.0"` {
				if b, err := defaultSkillFS.ReadFile("defaultskill/" + name + "/SKILL.md"); err != nil || !strings.Contains(string(b), want) {
					t.Fatalf("%s version: %v", name, err)
				}
				continue
			}
			if !strings.Contains(sys, want) {
				t.Fatalf("%s prompt lacks %q", name, want)
			}
		}
		if strings.Contains(sys, "MIT License\n") || strings.Contains(sys, "Permission is hereby granted") {
			t.Fatalf("%s: credits sent to the model", name)
		}
		if cover == "" && strings.Contains(sys, "cover-image size=") || cover != "" && !strings.Contains(sys, cover) {
			t.Fatalf("%s cover marker", name)
		}
		if n := len(sys); n > 40<<10 {
			t.Fatalf("%s prompt is %d bytes", name, n)
		}
	}
}
