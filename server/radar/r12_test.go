package radar

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// ---- 关键词提醒

func TestWatchMatching(t *testing.T) {
	for _, c := range []struct {
		text, kw string
		want     bool
	}{
		{"OpenAI ships GPT-6 today", "GPT-6", true},
		{"GPT-60 is not a thing", "GPT-6", false},
		{"Claudette", "Claude", false},
		{"Claude's new model", "Claude", true},
		{"深度求索发布DeepSeek新模型", "DeepSeek", true},
		{"深度求索发布新模型", "求索", true},
		{"xDeepSeek", "DeepSeek", false},
	} {
		if got := containsWord(c.text, c.kw); got != c.want {
			t.Errorf("containsWord(%q, %q) = %v", c.text, c.kw, got)
		}
	}
	kws := cleanKeywords([]any{" GPT-6 ", "gpt-6", "", "Deep\tSeek", strings.Repeat("长", 41), 3}, false)
	if strings.Join(kws, "|") != "GPT-6|Deep Seek" {
		t.Fatalf("clean: %q", kws)
	}
	w := WatchPrefs{Keywords: []string{"DeepSeek", "Claude"}, Scope: "all"}.matcher()
	if m := w.match("deepseek v4", "about claude"); strings.Join(m, ",") != "DeepSeek,Claude" {
		t.Fatalf("ci all: %v", m)
	}
	w = WatchPrefs{Keywords: []string{"DeepSeek", "Claude"}, Scope: "title", CaseSensitive: true}.matcher()
	if m := w.match("deepseek v4 and Claude", "DeepSeek in body"); strings.Join(m, ",") != "Claude" {
		t.Fatalf("cs title: %v", m)
	}
	if (WatchPrefs{}).matcher() != nil {
		t.Fatal("empty list should match nothing")
	}
}

func r12Items(st *Store, sid int64, items ...FetchedItem) []int64 {
	src, _ := all(st.DB, "SELECT * FROM sources WHERE id=?", sid)
	src[0].Set("last_fetched", now())
	ids, _, fresh, _ := st.StoreItems(src[0], items)
	st.notif.onPolled(sid, fresh) // what PollOne does
	return ids
}

func firstSources(st *Store, n int) []int64 {
	rs, _ := all(st.DB, "SELECT id FROM sources WHERE enabled=1 AND type NOT LIKE 'x%' ORDER BY id LIMIT ?", n)
	var out []int64
	for _, r := range rs {
		out = append(out, int64(r.Num("id")))
	}
	return out
}

func TestWatchStoreRematchAndNotify(t *testing.T) {
	t.Setenv("KW_NOTIFY_BATCH_MS", "600000") // keep the batch queued so the test can look at it
	t.Setenv("NOTIFY_BATCH_MS", "600000")
	st := testStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	var out map[string]any
	if c := postJSON(t, srv.URL+"/api/settings", "PATCH", map[string]any{"watch": map[string]any{"keywords": []string{"GPT-6", "Kling"}}}, &out); c != 200 {
		t.Fatalf("patch %d %v", c, out)
	}
	sid := firstSources(st, 1)[0]
	st.notif.onPolled(sid, nil) // the first poll of a source is its start-up backlog
	p := int64(now()) - 60
	ids := r12Items(st, sid,
		FetchedItem{UID: "https://a.test/1", Title: "OpenAI teases GPT-6", URL: "https://a.test/1", Summary: "x", Published: p},
		FetchedItem{UID: "https://a.test/2", Title: "Something else", URL: "https://a.test/2", Summary: "kling 3 video model", Published: p},
		FetchedItem{UID: "https://a.test/3", Title: "Nothing here", URL: "https://a.test/3", Summary: "GPT-60", Published: p})
	if len(ids) != 3 {
		t.Fatalf("stored %v", ids)
	}
	var items []map[string]any
	r, _ := http.Get(srv.URL + "/api/items?limit=10")
	json.NewDecoder(r.Body).Decode(&items)
	r.Body.Close()
	got := map[string]string{}
	for _, it := range items {
		if k, ok := it["kw"].([]any); ok {
			got[it["title"].(string)] = fmt.Sprint(k...)
		}
	}
	if got["OpenAI teases GPT-6"] != "GPT-6" || got["Something else"] != "Kling" || got["Nothing here"] != "" {
		t.Fatalf("kw: %v", got)
	}
	st.notif.mu.Lock()
	nkw := len(st.notif.kw.queue)
	st.notif.mu.Unlock()
	if nkw != 2 {
		t.Fatalf("keyword queue %d", nkw)
	}
	// the same story again (another URL, same title) is not alerted twice
	src2 := firstSources(st, 2)[1]
	st.notif.onPolled(src2, nil)
	r12Items(st, src2, FetchedItem{UID: "https://b.test/1", Title: "OpenAI teases GPT-6", URL: "https://b.test/1", Summary: "y", Published: p})
	st.notif.mu.Lock()
	nkw = len(st.notif.kw.queue)
	st.notif.mu.Unlock()
	if nkw != 2 {
		t.Fatalf("duplicate story alerted: %d", nkw)
	}
	// a different headline of the same event (a 热点 topic with an alerted item) is not alerted either
	src3 := firstSources(st, 3)[2]
	st.notif.onPolled(src3, nil)
	r12Items(st, src3, FetchedItem{UID: "https://c.test/1", Title: "GPT-6 teased by OpenAI's Altman in a new video", URL: "https://c.test/1", Summary: "z", Published: p})
	st.notif.mu.Lock()
	nkw = len(st.notif.kw.queue)
	st.notif.mu.Unlock()
	if nkw != 2 {
		t.Fatalf("same 热点 topic alerted again: %d (topics %v)", nkw, len(st.hotTopics(now())))
	}
	// changing the list re-matches every retained item
	postJSON(t, srv.URL+"/api/settings", "PATCH", map[string]any{"watch": map[string]any{"keywords": []string{"nothing"}}}, nil)
	var kw1, kw3 string
	st.DB.QueryRow("SELECT COALESCE(kw,'?') FROM items WHERE uid='https://a.test/1'").Scan(&kw1)
	st.DB.QueryRow("SELECT COALESCE(kw,'?') FROM items WHERE uid='https://a.test/3'").Scan(&kw3)
	if kw1 != "" || kw3 != "nothing" {
		t.Fatalf("rematch: %q %q", kw1, kw3)
	}
	// editable under SETTINGS_LOCKED (like the translation scopes)
	t.Setenv("SETTINGS_LOCKED", "1")
	if c := postJSON(t, srv.URL+"/api/settings", "PATCH", map[string]any{"watch": map[string]any{"keywords": []string{"Kling"}}}, &out); c != 200 {
		t.Fatalf("locked watch patch: %d %v", c, out)
	}
}

// ---- 选题收藏夹

func TestFavoritesPruneFilterExportImport(t *testing.T) {
	st := testStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	sid := firstSources(st, 1)[0]
	old := now() - 100*3600
	st.DB.Exec(`INSERT INTO items(uid,source_id,title,url,summary,published,fetched) VALUES('https://f.test/old',?,'Old favorite','https://f.test/old','s',?,?)`, sid, old, old)
	st.DB.Exec(`INSERT INTO items(uid,source_id,title,url,summary,published,fetched) VALUES('https://f.test/gone',?,'Old other','https://f.test/gone','s',?,?)`, sid, old, old)
	var fid int64
	st.DB.QueryRow("SELECT id FROM items WHERE uid='https://f.test/old'").Scan(&fid)
	if c := postJSON(t, srv.URL+"/api/favorites", "POST", map[string]any{"id": fid, "on": true}, nil); c != 200 {
		t.Fatalf("fav %d", c)
	}
	if c := postJSON(t, srv.URL+"/api/favorites", "POST", map[string]any{"id": 999999, "on": true}, nil); c != 404 {
		t.Fatalf("missing item: %d", c)
	}
	st.Prune()
	var n int
	st.DB.QueryRow("SELECT COUNT(*) FROM items WHERE uid LIKE 'https://f.test/%'").Scan(&n)
	if n != 1 {
		t.Fatalf("prune kept %d (want only the favorite)", n)
	}
	var items []map[string]any
	r, _ := http.Get(srv.URL + "/api/items?fav=1")
	json.NewDecoder(r.Body).Decode(&items)
	r.Body.Close()
	if len(items) != 1 || items[0]["fav"] != true || items[0]["title"] != "Old favorite" {
		t.Fatalf("fav view: %v", items)
	}
	r, _ = http.Get(srv.URL + "/api/items?limit=50")
	items = nil
	json.NewDecoder(r.Body).Decode(&items)
	r.Body.Close()
	for _, it := range items {
		if it["title"] == "Old favorite" {
			t.Fatal("an old favorite leaked into the time-windowed feed")
		}
	}
	var exp []map[string]any
	r, _ = http.Get(srv.URL + "/api/favorites/export")
	json.NewDecoder(r.Body).Decode(&exp)
	r.Body.Close()
	if len(exp) != 1 || exp[0]["uid"] != "https://f.test/old" || exp[0]["source"] == nil {
		t.Fatalf("export: %v", exp)
	}
	// import into a fresh instance: the item is re-created and favorited; a second import changes nothing
	st2 := testStore(t)
	srv2 := httptest.NewServer(st2.Server(nil))
	defer srv2.Close()
	bad := map[string]any{"uid": "javascript:x", "title": "x", "url": "javascript:x", "source": exp[0]["source"]}
	var res map[string]float64
	if c := postJSON(t, srv2.URL+"/api/favorites/import", "POST", map[string]any{"favorites": []any{exp[0], bad}}, &res); c != 200 || res["added"] != 1 || res["skipped"] != 1 {
		t.Fatalf("import %d %v", c, res)
	}
	postJSON(t, srv2.URL+"/api/favorites/import", "POST", map[string]any{"favorites": []any{exp[0]}}, &res)
	if res["added"] != 0 || res["existing"] != 1 {
		t.Fatalf("re-import %v", res)
	}
	t.Setenv("SETTINGS_LOCKED", "1")
	if c := postJSON(t, srv2.URL+"/api/favorites/import", "POST", map[string]any{"favorites": []any{exp[0]}}, nil); c != 403 {
		t.Fatalf("locked import %d", c)
	}
	if c := postJSON(t, srv.URL+"/api/favorites", "POST", map[string]any{"id": fid, "on": false}, nil); c != 200 {
		t.Fatalf("unfav %d", c)
	}
	st.DB.QueryRow("SELECT COUNT(*) FROM favorites").Scan(&n)
	if n != 0 {
		t.Fatal("unfavorite")
	}
}

// ---- 热点追踪

func TestHotTopicsClustering(t *testing.T) {
	t.Setenv("RETENTION_HOURS", "48")
	st := testStore(t)
	srcs := firstSources(st, 4)
	st.DB.Exec("UPDATE items SET published=0") // seed noise out of the window
	t0 := now() - 5*3600
	add := func(sid int64, uid, title, sum string, at float64) {
		if _, err := st.DB.Exec(`INSERT INTO items(uid,source_id,title,url,summary,published,fetched) VALUES(?,?,?,?,?,?,?)`, uid, sid, title, uid, sum, at, at); err != nil {
			t.Fatal(err)
		}
	}
	add(srcs[1], "https://h.test/b", "Zentrix releases Quorra-7, a 300B open model", "The Quorra-7 weights are on Hugging Face.", t0+1800)
	add(srcs[0], "https://h.test/a", "Zentrix 发布 Quorra-7 开源模型", "Quorra-7 参数 300B。", t0)
	add(srcs[2], "https://h.test/c", "Quorra-7 benchmarks: what the 300B model scores", "Zentrix says Quorra-7 beats rivals.", t0+3*3600)
	add(srcs[2], "https://h.test/c2", "OpenAI updates ChatGPT memory", "OpenAI said memory is better.", t0+3*3600)
	add(srcs[3], "https://h.test/d", "OpenAI hires a new CFO", "OpenAI announced a hire.", t0+3600)     // shares only an org name
	add(srcs[3], "https://h.test/e", "This week in AI: Quorra-7, Sora and more", "Roundup.", t0+4*3600) // a roundup never joins
	topics := st.hotTopics(now())
	if len(topics) != 1 {
		for _, x := range topics {
			t.Logf("%s %d", x.Title, len(x.Sources))
		}
		t.Fatalf("topics: %d", len(topics))
	}
	h := topics[0]
	if len(h.Sources) != 3 || h.Reports != 3 || len(h.Items) != 3 {
		t.Fatalf("topic: %+v", h)
	}
	if h.First.Published != t0 || h.First.Source == "" {
		t.Fatalf("first report: %+v", h.First)
	}
	if !strings.Contains(h.Title, "发布") { // a Chinese headline when there is one
		t.Fatalf("title %q", h.Title)
	}
	sum := 0
	for _, v := range h.Spark {
		sum += v
	}
	if len(h.Spark) != hotSparkHours || sum != 3 || h.Heat <= 0 {
		t.Fatalf("spark %v heat %v", h.Spark, h.Heat)
	}
	// /api/hot serves the same
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	var out struct{ Topics []HotTopic }
	r, _ := http.Get(srv.URL + "/api/hot")
	json.NewDecoder(r.Body).Decode(&out)
	r.Body.Close()
	if len(out.Topics) != 1 || len(out.Topics[0].Sources) != 3 {
		t.Fatalf("api: %+v", out)
	}
}

func TestHotTokens(t *testing.T) {
	tk := hotTokens(hotPrep("Build your own agent. Reflection AI releases Beam with GPT-6 and Kling 4.0 https://x.test/Zeta9 @Someone"))
	for _, want := range []string{"reflection", "beam", "gpt6", "kling4.0"} {
		if !tk[want] {
			t.Errorf("missing name %q in %v", want, tk)
		}
	}
	for _, no := range []string{"build", "zeta9", "someone"} {
		if tk[no] {
			t.Errorf("%q should not be a name: %v", no, tk)
		}
	}
}

// ---- 写作风格学习

func TestStyleSamplesProfileAndPrompt(t *testing.T) {
	st, srv, m, _ := composeStore(t, true)
	long := strings.Repeat("我一般先把结论放在第一句，然后再慢慢讲为什么。", 6)
	var s StyleState
	if c := postJSON(t, srv.URL+"/api/style", "PATCH", map[string]any{"kind": "wechat", "add": map[string]any{"text": "太短"}}, nil); c != 400 {
		t.Fatalf("short sample: %d", c)
	}
	if c := postJSON(t, srv.URL+"/api/style", "PATCH", map[string]any{"kind": "wechat", "add": map[string]any{"title": "旧文一", "text": long + "\n\n独家秘闻：某公司去年营收九十九亿元整。", "source": "file"}}, &s); c != 200 || len(s.Samples) != 1 || s.Samples[0].Source != "file" {
		t.Fatalf("add: %d %+v", c, s)
	}
	if c := postJSON(t, srv.URL+"/api/style", "PATCH", map[string]any{"kind": "wechat", "add": map[string]any{"text": long + "\n\n独家秘闻：某公司去年营收九十九亿元整。"}}, nil); c != 400 {
		t.Fatalf("duplicate sample: %d", c)
	}
	if c := postJSON(t, srv.URL+"/api/style/profile", "POST", map[string]any{"kind": "wechat"}, &s); c != 200 || !strings.Contains(s.Profile, "语气与人称") || !strings.Contains(m.styleIn, "<sample n=\"1\">") {
		t.Fatalf("profile: %d %+v", c, s)
	}
	s.Profile += "\n- 用户补充：少用形容词"
	postJSON(t, srv.URL+"/api/style", "PATCH", map[string]any{"kind": "wechat", "profile": s.Profile}, &s)
	if c := postJSON(t, srv.URL+"/api/style", "PATCH", map[string]any{"kind": "nope"}, nil); c != 400 {
		t.Fatalf("bad kind %d", c)
	}
	// compose injects the profile + excerpts with the no-facts rule; other platforms are untouched
	var a articleOut
	postJSON(t, srv.URL+"/api/compose", "POST", map[string]any{"platform": "wechat", "ids": ids(st, 1), "section": "all"}, &a)
	if !a.Style || !strings.Contains(m.lastWrite, "<style_profile>") || !strings.Contains(m.lastWrite, "少用形容词") ||
		!strings.Contains(m.lastWrite, "<excerpt n=\"1\">") || !strings.Contains(m.lastWrite, "一律不得写进新文章") {
		t.Fatalf("style not injected: %v", a.Style)
	}
	var a2 articleOut
	postJSON(t, srv.URL+"/api/compose", "POST", map[string]any{"platform": "twitter", "ids": ids(st, 1), "section": "all"}, &a2)
	if a2.Style || strings.Contains(m.lastWrite, "<style_profile>") {
		t.Fatal("wechat style used for twitter")
	}
	// remove + disable
	postJSON(t, srv.URL+"/api/style", "PATCH", map[string]any{"kind": "wechat", "enabled": false}, &s)
	if p, _ := st.stylePrompt("wechat"); p != "" {
		t.Fatal("disabled style still injected")
	}
	postJSON(t, srv.URL+"/api/style", "PATCH", map[string]any{"kind": "wechat", "remove": s.Samples[0].ID}, &s)
	if len(s.Samples) != 0 {
		t.Fatal("remove")
	}
	t.Setenv("SETTINGS_LOCKED", "1")
	if c := postJSON(t, srv.URL+"/api/style/profile", "POST", map[string]any{"kind": "wechat"}, nil); c != 403 {
		t.Fatalf("locked profile %d", c)
	}
}

func TestStyleWebOnly404AndHelpers(t *testing.T) {
	_, srv, _, _ := composeStore(t, false)
	r, _ := http.Get(srv.URL + "/api/style?kind=wechat")
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Fatalf("web style: %d", r.StatusCode)
	}
	samples := []StyleSample{{Text: strings.Repeat("甲", 500) + "\n\n" + strings.Repeat("乙", 500)}, {Text: "短开头一段。\n\n第二段也不长。"}}
	ex := styleExcerpts(samples, 600)
	if len(ex) != 2 || len([]rune(ex[0])) > 301 || !strings.Contains(ex[1], "第二段") {
		t.Fatalf("excerpts: %q", ex)
	}
	leak := styleLeaks("今天聊聊。独家秘闻某公司去年营收九十九亿元整，另外模型发布了新的版本号是三点零。",
		[]StyleSample{{Text: "独家秘闻：某公司去年营收九十九亿元整。模型发布了新的版本号是三点零"}}, []string{"模型发布了新的版本号是三点零"})
	if len(leak) != 1 || !strings.Contains(leak[0], "九十九亿") {
		t.Fatalf("leaks: %q", leak)
	}
}

// ---- 标题修正 / 推文链接

func TestFixTitleAndTweetLinks(t *testing.T) {
	st, _, m, _ := composeStore(t, true)
	md, ok := st.fixTitle("这是一个远远超过二十个字的小红书标题而且还在继续写下去\n\n正文。\n\n#a #b #c", "xhs")
	if !ok || !strings.HasPrefix(md, "短标题候选标题\n") || m.titles != 1 {
		t.Fatalf("xhs fix: %v %q", ok, md)
	}
	md, ok = st.fixTitle("OpenAI 的编程智能体 Codex，今年 6 月周活超过 500 万，其中约五分之一是知识工作者。\n\n1️⃣ 正文", "xhs")
	if !ok || !strings.HasPrefix(md, "短标题候选标题\n\nOpenAI 的编程智能体") {
		t.Fatalf("missing xhs title not inserted: %q", md)
	}
	if _, ok := st.fixTitle("# 合适长度的公众号标题写在这里\n\n正文", "wechat"); ok {
		t.Fatal("a fitting title was rewritten")
	}
	issues := slopLint("1/ 钩子 🧵\n\n2/ 细节 [[1]](https://e.com/1)\n\n3/ 更多 [[1]](https://e.com/1)\n\n4/ 结尾 [[2]](https://e.com/2)", "twitter")
	if len(issues) != 1 || !strings.Contains(issues[0], "[[1]] 出现在 2 条") {
		t.Fatalf("tweet links: %v", issues)
	}
}

var _ = filepath.Join

// ---- 一键复制: the desktop shell's native rich clipboard (web has no hook → 404)

func TestDesktopClipboardHook(t *testing.T) {
	_, srv, _, _ := composeStore(t, false)
	post := func(body string) (int, map[string]any) {
		r, err := http.Post(srv.URL+"/api/desktop/clipboard", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		var m map[string]any
		json.NewDecoder(r.Body).Decode(&m)
		return r.StatusCode, m
	}
	if c, _ := post(`{"html":"<p>x</p>","text":"x"}`); c != 404 {
		t.Fatalf("web clipboard: %d", c)
	}
	var gotHTML, gotText string
	DesktopHooks.CopyRich = func(h, tx string) error { gotHTML, gotText = h, tx; return nil }
	defer func() { DesktopHooks.CopyRich = nil }()
	if c, m := post(`{"html":"<p style=\"color:red\">标题</p>","text":"标题"}`); c != 200 || m["ok"] != true || gotHTML != `<p style="color:red">标题</p>` || gotText != "标题" {
		t.Fatalf("desktop clipboard: %d %v %q %q", c, m, gotHTML, gotText)
	}
	if c, _ := post(`{"html":"<p>x</p>","text":""}`); c != 400 {
		t.Fatalf("empty text: %d", c)
	}
	DesktopHooks.CopyRich = func(string, string) error { return fmt.Errorf("pasteboard busy") }
	if c, m := post(`{"text":"t"}`); c != 200 || m["ok"] != false {
		t.Fatalf("hook error: %d %v", c, m)
	}
}

// one X account: posts a few seconds apart are one thread; minutes apart only when they share a name
func TestHotSameSourceThread(t *testing.T) {
	doc := func(id int, title string, pub float64) *hotDoc {
		r := &Row{}
		r.Set("id", float64(id))
		r.Set("title", title)
		r.Set("url", fmt.Sprintf("https://x.com/a/status/%d", id))
		r.Set("published", pub)
		r.Set("source_id", float64(1))
		r.Set("is_x", float64(1))
		return newHotDoc(r)
	}
	idx := newHotIdx(nil)
	a := doc(1, "Orbitlab releases Zephyrion-9 with 2M context", 1000)
	if !idx.same(a, doc(2, "the weights are on the hub today", 1030)) {
		t.Error("30 s apart: thread part")
	}
	if idx.same(a, doc(3, "shipping update 3 for our API", 1120)) {
		t.Error("2 min apart, nothing shared: not a thread")
	}
	if !idx.same(a, doc(4, "Zephyrion-9 benchmarks below", 1150)) {
		t.Error("2.5 min apart with a shared name: thread part")
	}
}
