package radar

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
)

// golden input: a 48 h window of the prod instance (testdata/hot_prod.json, summaries cut to 300 chars)
func hotGolden(t testing.TB) ([]*Row, float64) {
	b, err := os.ReadFile("testdata/hot_prod.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		At    float64          `json:"at"`
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	var rs []*Row
	for _, it := range g.Items {
		r := &Row{}
		for _, k := range []string{"id", "title", "title_zh", "summary", "url", "published", "is_x", "merged_into", "uid", "source_id", "source", "priority", "section", "username"} {
			if v, ok := it[k]; ok && v != nil {
				r.Set(k, v)
			}
		}
		rs = append(rs, r)
	}
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].Num("published") < rs[j].Num("published") })
	return rs, g.At
}

func topicIDs(tp HotTopic) string {
	ids := append([]int64{}, tp.Items...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return strings.Trim(fmt.Sprint(ids), "[]")
}

func TestHotGoldenDump(t *testing.T) {
	rs, at := hotGolden(t)
	topics, _ := hotCompute(hotLexDefault(), rs, at, nil)
	for _, tp := range topics {
		t.Logf("%d src %d rep %.2f heat v%.1f core=%q | %s | first %s | ids %s", len(tp.Sources), tp.Reports, tp.Heat, tp.Velocity, tp.Core, tp.Title, tp.First.Source, topicIDs(tp))
	}
}

// ---- golden: the prod window (see testdata/hot_prod.json)

func findTopic(topics []HotTopic, id int64) *HotTopic {
	for i := range topics {
		for _, x := range topics[i].Items {
			if x == id {
				return &topics[i]
			}
		}
	}
	return nil
}

func TestHotGoldenProd(t *testing.T) {
	rs, at := hotGolden(t)
	topics, upd := hotCompute(hotLexDefault(), rs, at, nil)
	var got []string
	for _, tp := range topics {
		got = append(got, topicIDs(tp))
	}
	// the exact grouping (regression guard; update deliberately when the rules change)
	want := []string{
		"3348 3349 3350 3487 3504 3519",           // DeepSeek V4.1-Flash-1017: the lab's thread, Techmeme, @dotey, 机器之心
		"3021 3022 3035 3036 3037 3106 3123 3124", // Reflection Beam: arena, Artificial Analysis, Techmeme
		"2607 2608 2649 2650 2651 2661 3144 3145", // Meta Muse Spark Open: arena (twice), Artificial Analysis, 机器之心
		"2707 2708 3143 3157",                     // Tesla Roadster: @dotey, @AiBreakfast
		"2554 2555 2556 3043",                     // RTX Spark laptops: Artificial Analysis, Techmeme
		"2538 2609 2610",                          // GPT-Rosalind: OpenAI News, @OpenAI
		"2569 2570 2627 3114",                     // Claude Opus 5.5 posts (@AiBreakfast, @dotey)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("topics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	ds := findTopic(topics, 3487)
	if ds == nil || len(ds.Sources) < 4 || ds.First.Item != 3348 || ds.Core == "" || ds != &topics[0] {
		t.Fatalf("DeepSeek topic: %+v", ds)
	}
	if ds.Velocity <= 0 { // 机器之心 and @dotey within the last hour of the snapshot
		t.Errorf("velocity %v", ds.Velocity)
	}
	// main subject: 机器之心's RoboDojo piece only compares with GPT-6 Astra; Meta's medical Muse is another story
	if tp := findTopic(topics, 3521); tp != nil {
		t.Errorf("RoboDojo joined %v", tp.Items)
	}
	if tp := findTopic(topics, 2659); tp != nil {
		t.Errorf("Meta Muse 医疗 joined %v", tp.Items)
	}
	// every topic has a first report to remember
	if len(upd) != len(topics) {
		t.Errorf("history rows %d for %d topics", len(upd), len(topics))
	}
}

// ---- rules

func TestHotLexiconChinese(t *testing.T) {
	tk := hotTokens("字节跳动发布豆包1.6，阿里通义千问3.5跟进；可灵4.0杀疯了，智谱、月之暗面、腾讯和混元也在列")
	for _, want := range []string{"bytedance", "doubao1.6", "doubao", "alibaba", "qwen3.5", "qwen", "kling4.0", "kling", "zhipu", "moonshot", "tencent", "hunyuan"} {
		if _, ok := tk[want]; !ok {
			t.Errorf("missing %q in %v", want, tk)
		}
	}
	if !tk["doubao1.6"] || !tk["kling"] {
		t.Errorf("lexicon names are name-like: %v", tk)
	}
	// longest match: 腾讯混元 is hunyuan; 阿里巴巴 is one name, not 阿里 + 巴巴; 通义千问 is qwen, not 通义 + 千问 twice
	x := hotLexDefault()
	if s, hits := x.hanReplace("阿里巴巴和通义千问"); fmt.Sprint(hits) != "[alibaba qwen]" || !strings.Contains(s, "alibaba") {
		t.Errorf("longest match: %q %v", s, hits)
	}
	// Latin next to Chinese text is a name even at a sentence start ("Meta杀回开源")
	if !hotTokens("Meta杀回开源：816B新模型")["meta"] {
		t.Error("Meta in Chinese context")
	}
}

func TestHotAliases(t *testing.T) {
	for _, c := range []struct{ a, b, id string }{
		{"ByteDance unveils a new model", "字节跳动发布新模型", "bytedance"},
		{"Alibaba Cloud cuts prices", "阿里云降价", "alibaba"},
		{"Qwen 3.5 is out", "通义千问3.5 发布", "qwen3.5"},
		{"Moonshot's Kimi K3", "月之暗面发布新版", "moonshot"},
		{"Zhipu GLM-5 tops the board", "智谱新模型登顶", "zhipu"},
		{"Baidu ERNIE 5.0 launches", "百度文心5.0 上线", "ernie5.0"},
		{"Kling 4.0 by Kuaishou", "快手可灵4.0", "kling4.0"},
		{"MiniMax Hailuo 03", "海螺视频新版", "hailuo"},
	} {
		if !hotTokens(c.a)[c.id] && hotTokens(c.a)[c.id] == hotTokens(c.b)[c.id] {
			t.Errorf("%q / %q: no shared %q (%v | %v)", c.a, c.b, c.id, hotTokens(c.a), hotTokens(c.b))
		}
		if _, ok := hotTokens(c.a)[c.id]; !ok {
			t.Errorf("%q lacks %q: %v", c.a, c.id, hotTokens(c.a))
		}
		if _, ok := hotTokens(c.b)[c.id]; !ok {
			t.Errorf("%q lacks %q: %v", c.b, c.id, hotTokens(c.b))
		}
	}
}

func TestHotVersionSpelling(t *testing.T) {
	for _, s := range []string{"GPT-6 is here", "GPT 6 is here", "gpt6 is here", "GPT‑6 is here", "ＧＰＴ－６ is here", "OpenAI 发布 GPT-6"} {
		if _, ok := hotTokens(s)["gpt6"]; !ok {
			t.Errorf("%q: %v", s, hotTokens(s))
		}
	}
	tk := hotTokens("Build your own agent. Reflection AI releases Beam with GPT-6 and Kling 4.0 https://x.test/Zeta9 @Someone")
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
	// a versioned name also carries its base and its plain version ("V4.1F" → v4.1, "Beam-1.0" → beam)
	if tk := hotTokens("DeepSeek V4.1F新版本来了 · Beam-1.0-501B"); !tk["v4.1"] || !tk["beam"] || !tk["beam1.0"] || !tk["501b"] {
		t.Errorf("versions: %v", tk)
	}
}

func TestHotUserLexicon(t *testing.T) {
	g := parseUserLexicon(" 灵光 = Lingguang \n\n星河大模型\n超长" + strings.Repeat("长", 40) + "\n灵光=灵光")
	if userLexiconText(g) != "灵光=Lingguang\n星河大模型" {
		t.Fatalf("parse: %q", userLexiconText(g))
	}
	x := newHotLex(userLexiconText(g))
	if tk := x.scan("灵光上线新功能").toks; !tk["lingguang"] {
		t.Errorf("user name: %v", tk)
	}
	if tk := x.scan("Lingguang 2.0 ships").toks; !tk["lingguang2.0"] {
		t.Errorf("user family version: %v", tk)
	}
	syn := ""
	for tk := range x.scan("星河大模型发布").toks {
		syn = tk
	}
	if !strings.HasPrefix(syn, "u") || len(syn) != 9 || !x.user[syn] {
		t.Errorf("synthetic id %q", syn)
	}
	if tk := hotLexDefault().scan("灵光上线").toks; len(tk) != 0 {
		t.Errorf("built-in lexicon unchanged: %v", tk)
	}
	// API: saved normalized; locked deployments can't change it
	st := testStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	var out map[string]any
	if c := postJSON(t, srv.URL+"/api/settings", "PATCH", map[string]any{"hotLexicon": "灵光 = Lingguang"}, &out); c != 200 || out["hotLexicon"] != "灵光=Lingguang" {
		t.Fatalf("patch: %d %v", c, out)
	}
	if !st.hotLexicon().scan("灵光").toks["lingguang"] {
		t.Error("store lexicon not rebuilt")
	}
	t.Setenv("SETTINGS_LOCKED", "1")
	if c := postJSON(t, srv.URL+"/api/settings", "PATCH", map[string]any{"hotLexicon": "x"}, &out); c != 403 {
		t.Fatalf("locked: %d", c)
	}
}

// hdoc: a synthetic item for rule tests
func hdoc(id, sid int, title, sum string, pub float64, x bool) *Row {
	r := &Row{}
	r.Set("id", float64(id))
	r.Set("title", title)
	r.Set("summary", sum)
	r.Set("url", fmt.Sprintf("https://s%d.test/%d", sid, id))
	r.Set("uid", fmt.Sprintf("https://s%d.test/%d", sid, id))
	r.Set("published", pub)
	r.Set("source_id", float64(sid))
	r.Set("source", fmt.Sprintf("Source %d", sid))
	r.Set("priority", "P0")
	if x {
		r.Set("is_x", float64(1))
	} else {
		r.Set("is_x", float64(0))
	}
	return r
}

// a window with enough unrelated sources that the story's names are rare
func hotNoise(n int) []*Row {
	var rs []*Row
	for i := 0; i < n; i++ {
		w := []string{"Accenture memo on expense cards", "Volvo picks a driving platform", "Netflix content chief departs", "Proofpoint to buy Varonis",
			"Waymo plans Toronto robotaxis", "PayPal voices concerns to DOJ", "Intuit acquires a billing startup", "Kioxia loses market cap",
			"Scarlet raises a Series B", "Duvo automates ERPs", "Workday buys C1.ai", "CoreWeave founders sell stock", "Taiwan plans US parks"}[i%13]
		rs = append(rs, hdoc(9000+i, 100+i, w, "OpenAI and Google were not involved.", 500+float64(i)*7000, false))
	}
	return rs
}

func groupsOf(rs []*Row) []string {
	topics, _ := hotCompute(hotLexDefault(), rs, 100000, nil)
	var out []string
	for _, tp := range topics {
		out = append(out, topicIDs(tp))
	}
	sort.Strings(out)
	return out
}

func TestHotMainSubject(t *testing.T) {
	base := hotNoise(12)
	rel := hdoc(1, 1, "Orbitlab releases Zephyrion-9 Pro with a 2M context", "Orbitlab says Zephyrion-9 Pro is its best model.", 90000, false)
	// compared with, not about: names after 超过 / beats don't count
	cmp := hdoc(2, 2, "登顶RoboBench智能体超过Zephyrion-9 Pro：Kestrel推出K2-Agent", "Kestrel 的 K2-Agent 超越 Zephyrion-9 Pro 等模型。", 90100, false)
	cmpEn := hdoc(3, 3, "Kestrel's K2-Agent beats Zephyrion-9 Pro on RoboBench", "Kestrel K2-Agent outperforms Zephyrion-9 Pro.", 90200, false)
	// a passing mention in the text only
	pass := hdoc(4, 4, "Kestrel ships a new robotics toolkit", "The toolkit also supports Zephyrion-9 Pro as a planner.", 90300, false)
	// (the two K2-Agent pieces are one story: that's what they are about)
	if g := groupsOf(append(append([]*Row{}, base...), rel, cmp, cmpEn, pass)); strings.Join(g, "|") != "2 3" {
		t.Errorf("comparison / passing mention merged: %v", g)
	}
	// about it: in both headlines, or one headline + repeated in the other's lead
	rep := hdoc(5, 5, "Orbitlab 发布 Zephyrion-9 Pro", "", 90400, false)
	lead := hdoc(6, 6, "A closer look at Orbitlab's new flagship", "Zephyrion-9 Pro has a 2M context. Zephyrion-9 Pro is priced at $2.", 90500, false)
	if g := groupsOf(append(append([]*Row{}, base...), rel, cmp, rep, lead)); strings.Join(g, "|") != "1 5 6" {
		t.Errorf("subject: %v", g)
	}
}

func TestHotNoChainingDrift(t *testing.T) {
	base := hotNoise(12)
	// A and B share Quorra-7; B and C share Vexel-3; nothing is shared by all three
	a := hdoc(1, 1, "Zentrix releases Quorra-7", "Quorra-7 is open.", 90000, false)
	b := hdoc(2, 2, "Zentrix Quorra-7 and Vexel-3 benchmarks", "Quorra-7 vs. Vexel-3 compared side by side; Quorra-7 Vexel-3 Quorra-7 Vexel-3", 90100, false)
	c := hdoc(3, 3, "Lumina ships Vexel-3", "Vexel-3 is fast.", 90200, false)
	g := groupsOf(append(append([]*Row{}, base...), a, b, c))
	for _, x := range g {
		if x == "1 2 3" {
			t.Fatalf("A~B~C drifted into one topic: %v", g)
		}
	}
	if len(g) != 1 {
		t.Errorf("one of the two pairs should still form a topic: %v", g)
	}
	// with a core name shared by all, the chain is one story
	c2 := hdoc(3, 3, "Lumina tests Quorra-7 against Vexel-3", "Quorra-7 wins on code; Quorra-7 is cheaper.", 90200, false)
	if g := groupsOf(append(append([]*Row{}, base...), a, b, c2)); strings.Join(g, "|") != "1 2 3" {
		t.Errorf("shared core: %v", g)
	}
}

func TestHotFragmentReconnect(t *testing.T) {
	base := hotNoise(12)
	// one story in two fragments: the Atlas-2 chip pieces and the Pulse pricing pieces all mention Nova-5, but only
	// one piece of each fragment is about it — too few for a greedy merge (half the units), enough to reconnect
	rs := append(append([]*Row{}, base...),
		hdoc(1, 1, "Helio Atlas-2 Forge-3 line enters production", "Atlas-2 will power Nova-5 next month.", 90000, false),
		hdoc(2, 2, "Atlas-2 Forge-3 line ramps at Helio fabs", "Atlas-2 yields are up; Nova-5 runs on Atlas-2.", 90100, false),
		hdoc(3, 3, "Atlas-2 Forge-3 chips power Nova-5 launch", "Atlas-2 serves Nova-5 fast.", 90200, false),
		hdoc(4, 4, "Helio Pulse Tier-9 plan doubles in price", "Pulse Tier-9 covers Nova-5 usage.", 90300, false),
		hdoc(5, 5, "Pulse Tier-9 pricing hits Nova-5 users", "Pulse Tier-9 costs more.", 90400, false))
	if g := groupsOf(rs); strings.Join(g, "|") != "1 2 3 4 5" {
		t.Errorf("fragments: %v", g)
	}
}

func TestHotHeatWeightsVelocity(t *testing.T) {
	x := hotLexDefault()
	names := map[string]bool{"deepseek": true, "v4.1": true}
	lab := hdoc(1, 1, "t", "", 0, false)
	lab.Set("section", "lab")
	fp := hdoc(2, 2, "t", "", 0, true)
	fp.Set("source", "X @deepseek_ai (DeepSeek)")
	art := hdoc(3, 3, "t", "", 0, false)
	rp := hdoc(4, 4, "t", "", 0, true)
	p1 := hdoc(5, 5, "t", "", 0, false)
	p1.Set("priority", "P1")
	for _, c := range []struct {
		r *Row
		w float64
	}{{lab, 1.5}, {fp, 1.5}, {art, 1}, {rp, 0.7}, {p1, 0.6}} {
		if w := hotWeight(x, c.r, names); math.Abs(w-c.w) > 1e-9 {
			t.Errorf("%s: %v want %v", c.r.Str("source"), w, c.w)
		}
	}
	// same coverage, but one story's reports are all in the last hour: it ranks first and has velocity
	at := 200000.0
	base := hotNoise(12)
	old := []*Row{hdoc(1, 1, "Zentrix releases Quorra-7", "", at-5*3600, false), hdoc(2, 2, "Zentrix Quorra-7 is out", "", at-5*3600+60, false)}
	fresh := []*Row{hdoc(3, 1, "Lumina releases Vexel-3", "", at-1200, false), hdoc(4, 2, "Lumina Vexel-3 is out", "", at-600, false)}
	topics, _ := hotCompute(x, append(append(append([]*Row{}, base...), old...), fresh...), at, nil)
	if len(topics) != 2 || topics[0].Items[0] != 3 || topics[0].Velocity != 2 || topics[1].Velocity != 0 {
		t.Fatalf("velocity: %+v", topics)
	}
}

func TestHotHistoryKeepsFirstReporter(t *testing.T) {
	t.Setenv("RETENTION_HOURS", "48")
	st := testStore(t)
	srcs := firstSources(st, 3)
	st.DB.Exec("UPDATE items SET published=0")
	t0 := now() - 40*3600
	add := func(sid int64, uid, title string, at float64) {
		if _, err := st.DB.Exec(`INSERT INTO items(uid,source_id,title,url,summary,published,fetched) VALUES(?,?,?,?,?,?,?)`, uid, sid, title, uid, "", at, at); err != nil {
			t.Fatal(err)
		}
	}
	add(srcs[0], "https://h.test/first", "Zentrix releases Quorra-7, a 300B model", t0)
	add(srcs[1], "https://h.test/b", "Zentrix Quorra-7 benchmarks", t0+3600)
	add(srcs[2], "https://h.test/c", "Quorra-7 by Zentrix: hands-on", t0+30*3600)
	tp := st.hotTopics(now())
	if len(tp) != 1 || tp[0].First.Published != t0 || tp[0].First.Archived {
		t.Fatalf("topics %+v", tp)
	}
	var n int
	st.DB.QueryRow("SELECT COUNT(*) FROM hot_history").Scan(&n)
	if n != 1 {
		t.Fatalf("history rows %d", n)
	}
	// the first report leaves the 48 h window (retention cleanup): the topic still names it, archived
	st.DB.Exec("DELETE FROM items WHERE uid='https://h.test/first'")
	tp = st.hotTopics(now())
	if len(tp) != 1 || tp[0].First.Published != t0 || !tp[0].First.Archived || tp[0].First.Item != 0 ||
		tp[0].First.URL != "https://h.test/first" || !strings.Contains(tp[0].First.Title, "Quorra-7") {
		t.Fatalf("archived first: %+v", tp[0].First)
	}
	// history has its own 7 days
	st.hotTopics(now() + 8*24*3600)
	st.DB.QueryRow("SELECT COUNT(*) FROM hot_history").Scan(&n)
	if n != 0 {
		t.Fatalf("history not pruned: %d", n)
	}
}

// one X account: posts a few seconds apart are one thread; minutes apart only when they share a name; its
// look-alike boilerplate posts are not one report
func TestHotSameSourceThread(t *testing.T) {
	doc := func(id int, title string, pub float64) *hotDoc { return newHotDoc(hdoc(id, 1, title, "", pub, true)) }
	a := doc(1, "Orbitlab releases Zephyrion-9 with 2M context", 1000)
	if !sameUnit(a, doc(2, "the weights are on the hub today", 1030)) {
		t.Error("30 s apart: thread part")
	}
	if sameUnit(a, doc(3, "shipping update 3 for our API", 1120)) {
		t.Error("2 min apart, nothing shared: not a thread")
	}
	if !sameUnit(a, doc(4, "Zephyrion-9 benchmarks below", 1150)) {
		t.Error("2.5 min apart with a shared name: thread part")
	}
	if sameUnit(doc(5, "Dive into the Agent Arena and Code Arena leaderboards at", 1000), doc(6, "Dive into the Code Arena: WebDev at", 9000)) {
		t.Error("boilerplate posts of one account")
	}
}

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

// BenchmarkHotCompute: the pure 热点 pass on the 48 h prod snapshot (go test ./radar -run x -bench HotCompute -benchmem);
// warm = the scan memo filled by an earlier recompute (the usual case), cold = an empty memo.
func BenchmarkHotCompute(b *testing.B) {
	rs, at := hotGolden(b)
	for _, warm := range []bool{true, false} {
		b.Run(map[bool]string{true: "warm", false: "cold"}[warm], func(b *testing.B) {
			x := newHotLex("")
			hotCompute(x, rs, at, nil)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if !warm {
					x.memo = nil
				}
				hotCompute(x, rs, at, nil)
			}
		})
	}
}
