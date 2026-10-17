package radar

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var tinyPNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

// mmMock: DeepSeek chat (summaries / writing / verify) + an OpenAI-compatible image / async video provider.
type mmMock struct {
	mu        sync.Mutex
	sys       map[string]string // last writing system prompt per platform marker
	lastWrite string
	sumIn     []string // user messages of the summarize stage
	revised   int
	images    []map[string]any
	polls     int
	vision    []any
}

func (m *mmMock) handler(w http.ResponseWriter, r *http.Request) {
	u, _ := url.Parse(r.URL.Query().Get("u"))
	if u == nil {
		u = r.URL
	}
	body, _ := io.ReadAll(r.Body)
	var j map[string]any
	json.Unmarshal(body, &j)
	reply := func(v any) { json.NewEncoder(w).Encode(v) }
	chat := func(c string) {
		reply(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": c}}}})
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case u.Host == "mm.test" && u.Path == "/v1/images/generations":
		m.images = append(m.images, j)
		reply(map[string]any{"data": []any{map[string]any{"b64_json": base64.StdEncoding.EncodeToString(tinyPNG)}}})
	case u.Host == "mm.test" && u.Path == "/v1/videos/generations" && r.Method == "POST":
		reply(map[string]any{"id": "task-9"})
	case u.Host == "mm.test" && u.Path == "/v1/videos/generations/task-9":
		m.polls++
		if m.polls < 2 {
			reply(map[string]any{"status": "running"})
		} else {
			reply(map[string]any{"status": "succeeded", "video_url": "https://cdn.mm.test/v.mp4"})
		}
	case u.Host == "cdn.mm.test":
		w.Header().Set("Content-Type", "video/mp4")
		w.Write([]byte("MP4DATA"))
	case u.Host == "mm.test" && u.Path == "/v1/chat/completions":
		msgs := j["messages"].([]any)
		m.vision = msgs[1].(map[string]any)["content"].([]any)
		chat("# VISION-ARTICLE\n\n正文 [[1]](https://e.com/1)")
	case strings.HasSuffix(u.Path, "/chat/completions"):
		msgs := j["messages"].([]any)
		sys := msgs[0].(map[string]any)["content"].(string)
		user := msgs[1].(map[string]any)["content"].(string)
		switch {
		case strings.Contains(sys, "<stage>revise</stage>"):
			m.revised++
			chat(regexp.MustCompile(`(?s)<draft>\n(.*)\n</draft>`).FindStringSubmatch(user)[1] + "\n#话题一 #话题二 #话题三")
		case strings.Contains(sys, "<stage>summarize</stage>"):
			m.sumIn = append(m.sumIn, user)
			chat(`{"items":[{"n":1,"summary":"SUM-ONE 事实摘要"}]}`)
		case strings.Contains(sys, "<stage>verify</stage>"):
			chat(regexp.MustCompile(`(?s)<brief>\n(.*)\n</brief>`).FindStringSubmatch(user)[1] + "\nVERIFIED")
		default:
			m.lastWrite = sys
			name := regexp.MustCompile(`<skill name="([^"]+)">`).FindStringSubmatch(sys)[1]
			out := "# 标题 " + name + "\n\n正文 [[1]](https://e.com/1)\n"
			if strings.Contains(sys, "cover-image size=") && name != "twitter-article" {
				out += "\n<!-- cover-image size=1024x1536: a calm robot reading news -->\n"
			}
			chat(out)
		}
	default:
		w.WriteHeader(404)
	}
}

func composeStore(t *testing.T, desktop bool) (*Store, *httptest.Server, *mmMock, string) {
	m := &mmMock{sys: map[string]string{}}
	mock := httptest.NewServer(http.HandlerFunc(m.handler))
	t.Cleanup(mock.Close)
	t.Setenv("DEEPSEEK_API_KEY", "sk-"+"unit"+"test0000")
	t.Setenv("BRIEF_CACHE_MS", "1")
	d := t.TempDir()
	t.Setenv("ARTIFACTS_DIR", filepath.Join(d, "out"))
	st, err := Open(Config{DBPath: filepath.Join(d, "news.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "secrets.json"), Desktop: desktop})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	t.Setenv("UPSTREAM_MOCK", mock.URL)
	srv := httptest.NewServer(st.Server(nil))
	t.Cleanup(srv.Close)
	var sid int
	st.DB.QueryRow("SELECT id FROM sources WHERE enabled=1 AND type NOT LIKE 'x%' ORDER BY id LIMIT 1").Scan(&sid)
	for i := 1; i <= 12; i++ {
		st.DB.Exec(`INSERT INTO items(uid,source_id,title,url,summary,published,fetched,image) VALUES(?,?,?,?,?,?,?,?)`,
			fmt.Sprint("u", i), sid, fmt.Sprintf("Story %c%c about thing %d", 'A'+i, 'Z'-i, i*997), fmt.Sprint("https://e.com/", i), "Feed summary.", now()-float64(i*60), now(), "https://img.test/"+fmt.Sprint(i)+".jpg")
	}
	return st, srv, m, filepath.Join(d, "out")
}

func ids(st *Store, n int) []int64 {
	rs, _ := all(st.DB, "SELECT id FROM items ORDER BY id LIMIT ?", n)
	var out []int64
	for _, r := range rs {
		out = append(out, int64(r.Num("id")))
	}
	return out
}

func postJSON(t *testing.T, u string, method string, v any, out any) int {
	b, _ := json.Marshal(v)
	req, _ := http.NewRequest(method, u, strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if out != nil {
		json.NewDecoder(r.Body).Decode(out)
	}
	return r.StatusCode
}

func TestComposeSkillPerPlatform(t *testing.T) {
	st, srv, m, outDir := composeStore(t, true)
	sel := ids(st, 2)
	for p, want := range map[string]string{"twitter": "twitter-article", "wechat": "wechat-article", "xhs": "xhs-article"} {
		var a articleOut
		if c := postJSON(t, srv.URL+"/api/compose", "POST", map[string]any{"platform": p, "ids": sel, "section": "all"}, &a); c != 200 {
			t.Fatalf("%s: HTTP %d", p, c)
		}
		if a.Skill != want || !strings.Contains(m.lastWrite, `<skill name="`+want+`">`) || !strings.Contains(a.Markdown, want) || a.Count != 2 || !a.Verified {
			t.Fatalf("%s used skill %q (prompt has %v): %+v", p, a.Skill, m.lastWrite[:80], a)
		}
		if !strings.Contains(m.lastWrite, "逐字照抄") {
			t.Fatalf("%s: factual rules missing", p)
		}
		// no multimodal model: the cover is skipped with a note, the marker never reaches the text
		if p != "twitter" && (len(a.Covers) != 1 || a.Covers[0].Status != "skipped" || a.Covers[0].Note == "" || strings.Contains(a.Markdown, "cover-image")) {
			t.Fatalf("%s covers: %+v", p, a.Covers)
		}
		if p == "twitter" && len(a.Covers) != 0 {
			t.Fatalf("twitter asked for a cover: %+v", a.Covers)
		}
	}
	// nothing is written until 导出
	if fs, _ := filepath.Glob(filepath.Join(outDir, "文章", "*", "*")); len(fs) != 0 {
		t.Fatalf("article auto-saved: %v", fs)
	}
	// an installed skill set active for 小红书 only
	skDir := filepath.Join(filepath.Dir(st.Cfg.DBPath), "skills", "my-xhs")
	os.MkdirAll(skDir, 0o755)
	os.WriteFile(filepath.Join(skDir, "SKILL.md"), []byte("---\nname: my-xhs\ndescription: d\n---\nMY-XHS"), 0o644)
	if c := postJSON(t, srv.URL+"/api/skills/active", "PATCH", map[string]any{"kind": "xhs", "slug": "my-xhs"}, nil); c != 200 {
		t.Fatal("activate", c)
	}
	var a articleOut
	postJSON(t, srv.URL+"/api/compose", "POST", map[string]any{"platform": "xhs", "ids": sel}, &a)
	if a.Skill != "my-xhs" {
		t.Fatalf("xhs active skill not used: %q", a.Skill)
	}
	postJSON(t, srv.URL+"/api/compose", "POST", map[string]any{"platform": "wechat", "ids": sel}, &a)
	if a.Skill != "wechat-article" || st.activeSkill("brief").Name != DefaultSkillName {
		t.Fatalf("other kinds changed: %q", a.Skill)
	}
	var list struct {
		Skills []skillMeta
	}
	r, _ := http.Get(srv.URL + "/api/skills?kind=xhs")
	json.NewDecoder(r.Body).Decode(&list)
	r.Body.Close()
	if len(list.Skills) != 2 || list.Skills[0].Name != "xhs-article" || !list.Skills[1].Active || list.Skills[0].Active {
		t.Fatalf("xhs list: %+v", list.Skills)
	}
	if c := postJSON(t, srv.URL+"/api/compose", "POST", map[string]any{"platform": "xhs", "ids": ids(st, 3)}, nil); c != 400 {
		t.Fatal("more than 2 accepted", c)
	}
	if c := postJSON(t, srv.URL+"/api/compose", "POST", map[string]any{"platform": "weibo", "ids": sel}, nil); c != 400 {
		t.Fatal("unknown platform accepted", c)
	}
}

func TestComposeCoverWithMultimodal(t *testing.T) {
	st, srv, m, outDir := composeStore(t, true)
	var pub map[string]any
	key := "sk-" + "mm" + "unittestkey0000"
	if c := postJSON(t, srv.URL+"/api/multimodal", "PATCH", map[string]any{"baseUrl": "https://mm.test/v1", "key": key, "imageModel": "img-1"}, &pub); c != 200 || pub["image"] != true || pub["video"] != false {
		t.Fatalf("mm patch %d %v", c, pub)
	}
	r, _ := http.Get(srv.URL + "/api/multimodal")
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if strings.Contains(string(b), key) {
		t.Fatal("key leaked to the UI")
	}
	var a articleOut
	postJSON(t, srv.URL+"/api/compose", "POST", map[string]any{"platform": "xhs", "ids": ids(st, 2)}, &a)
	if len(a.Covers) != 1 || a.Covers[0].Status != "ok" || !strings.HasPrefix(a.Covers[0].Path, filepath.Join(outDir, "缓存", "封面")) || !strings.HasSuffix(a.Covers[0].Path, ".png") {
		t.Fatalf("cover: %+v", a.Covers)
	}
	if len(m.images) != 1 || m.images[0]["model"] != "img-1" || m.images[0]["size"] != "1024x1536" || m.images[0]["prompt"] != "a calm robot reading news" {
		t.Fatalf("image request: %+v", m.images)
	}
	// 导出: the Markdown goes to 文章/<date>/, the cover is copied next to it and linked
	var ex struct{ Path string }
	if c := postJSON(t, srv.URL+"/api/artifacts/export", "POST", map[string]any{"kind": "article", "title": "标题/一", "markdown": a.Markdown, "covers": []string{a.Covers[0].Path, st.Cfg.SecretsPath}}, &ex); c != 200 || !strings.HasPrefix(ex.Path, filepath.Join(outDir, "文章")) || !strings.HasSuffix(ex.Path, "-标题-一.md") {
		t.Fatalf("export %d %q", c, ex.Path)
	}
	saved, _ := os.ReadFile(ex.Path)
	stem := strings.TrimSuffix(filepath.Base(ex.Path), ".md")
	if !strings.HasPrefix(string(saved), "![封面]("+stem+"-封面.png)") || strings.Count(string(saved), "![封面]") != 1 {
		t.Fatalf("exported article does not link the cover (only): %s", saved)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(ex.Path), stem+"-封面.png")); err != nil {
		t.Fatal("cover not copied", err)
	}
	if c := postJSON(t, srv.URL+"/api/artifacts/export", "POST", map[string]any{"kind": "x", "markdown": "a"}, nil); c != 400 {
		t.Fatal("bad kind accepted", c)
	}
	// served to the dialog from inside the artifacts dir only
	if r, _ := http.Get(srv.URL + "/api/artifacts/file?path=" + url.QueryEscape(a.Covers[0].Path)); r.StatusCode != 200 {
		t.Fatal("cover not served", r.Status)
	}
	if r, _ := http.Get(srv.URL + "/api/artifacts/file?path=" + url.QueryEscape(st.Cfg.SecretsPath)); r.StatusCode != 404 {
		t.Fatal("file outside the artifacts dir served")
	}
	// the brief is not saved until 导出
	r, _ = http.Get(srv.URL + "/api/brief?section=all")
	r.Body.Close()
	if fs, _ := filepath.Glob(filepath.Join(outDir, "简报", "*", "*")); len(fs) != 0 {
		t.Fatalf("brief auto-saved: %v", fs)
	}
	if c := postJSON(t, srv.URL+"/api/artifacts/export", "POST", map[string]any{"kind": "brief", "title": "AI 简报", "markdown": "# B\n"}, &ex); c != 200 || !strings.HasPrefix(ex.Path, filepath.Join(outDir, "简报")) {
		t.Fatalf("brief export %d %q", c, ex.Path)
	}
}

func TestMMVideoAndVision(t *testing.T) {
	_, _, m, outDir := composeStore(t, true)
	mmPollEvery = 10 * time.Millisecond
	c := MMConfig{BaseURL: "https://mm.test/v1", Key: "sk-" + "x0000000000000000", VideoModel: "vid-1", VisionModel: "vl-1"}
	p, err := c.mmVideo("waves", "", filepath.Join(outDir, "v"))
	if err != nil || !strings.HasSuffix(p, ".mp4") || m.polls < 2 {
		t.Fatalf("video: %v %q polls=%d", err, p, m.polls)
	}
	if b, _ := os.ReadFile(p); string(b) != "MP4DATA" {
		t.Fatal("video not downloaded")
	}
	out, err := c.mmVisionChat("sys", "user", []string{"https://img.test/1.jpg"}, 0, 100)
	if err != nil || !strings.Contains(out, "VISION") || len(m.vision) != 2 || m.vision[1].(map[string]any)["type"] != "image_url" {
		t.Fatalf("vision: %v %v", err, m.vision)
	}
}

func TestExtractCovers(t *testing.T) {
	md, cs := extractCovers("# T\n\nbody\n<!-- cover-image size=1792x1024: sunrise city -->\n`<!-- cover-video: waves -->`\n<!-- cover-image: second -->\n")
	if strings.Contains(md, "cover") || len(cs) != 2 || cs[0].Size != "1792x1024" || cs[0].Prompt != "sunrise city" || cs[1].Kind != "video" {
		t.Fatalf("%q %+v", md, cs)
	}
}

func TestManualBriefAndWebGating(t *testing.T) {
	st, srv, _, _ := composeStore(t, false)
	sel := ids(st, 4)
	var br briefOut
	r, _ := http.Get(srv.URL + "/api/brief?section=all&ids=" + joinIDs(sel))
	json.NewDecoder(r.Body).Decode(&br)
	r.Body.Close()
	if r.StatusCode != 200 || br.Count != 4 || !br.Manual || !strings.Contains(br.Markdown, "手动选择") {
		t.Fatalf("manual brief on web: %d %+v", r.StatusCode, br.Count)
	}
	if r, _ := http.Get(srv.URL + "/api/brief?ids=" + joinIDs(ids(st, 11))); r.StatusCode != 400 {
		t.Fatal(">10 ids accepted", r.Status)
	}
	if c := postJSON(t, srv.URL+"/api/artifacts/export", "POST", map[string]any{"kind": "brief", "markdown": "# B"}, nil); c != 404 {
		t.Fatal("export on web", c)
	}
	for _, p := range []string{"/api/compose", "/api/multimodal", "/api/artifacts"} {
		meth := "GET"
		if p == "/api/compose" {
			meth = "POST"
		}
		if c := postJSON(t, srv.URL+p, meth, map[string]any{}, nil); c != 404 {
			t.Fatalf("%s on web: %d", p, c)
		}
	}
}
