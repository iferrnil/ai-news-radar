package radar

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ---- 关键词 (tags + filter; no notifications since 0.13.0)

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

func TestWatchStoreRematchFilter(t *testing.T) {
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
	// filter: one keyword / any keyword
	kwTitles := func(q string) string {
		var l []map[string]any
		r, _ := http.Get(srv.URL + "/api/items?limit=10&kw=" + q)
		json.NewDecoder(r.Body).Decode(&l)
		r.Body.Close()
		var ts []string
		for _, it := range l {
			ts = append(ts, it["title"].(string))
		}
		sort.Strings(ts)
		return strings.Join(ts, "|")
	}
	if g := kwTitles("GPT-6"); g != "OpenAI teases GPT-6" {
		t.Fatalf("kw=GPT-6: %q", g)
	}
	if g := kwTitles("*"); g != "OpenAI teases GPT-6|Something else" {
		t.Fatalf("kw=*: %q", g)
	}
	if g := kwTitles("GPT"); g != "" {
		t.Fatalf("kw=GPT (not a keyword): %q", g)
	}
	// no keyword notification channel any more (0.13.0): the setting no longer stores a notify flag
	var stored string
	st.DB.QueryRow("SELECT v FROM settings WHERE k='watch'").Scan(&stored)
	if strings.Contains(stored, "notify") {
		t.Fatalf("watch setting still carries notify: %s", stored)
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
