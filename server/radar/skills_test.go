package radar

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const skillText = "---\nname: test-skill\ndescription: >\n  A test skill\n  for briefs.\nlicense: MIT\nmetadata:\n  version: \"1\"\n---\n# 测试\n\nSKILL-MARKER-7f3a：每条用一句话。参考 `references/style.md` 与 [词表](references/words.md)。\n"

func zipOf(t *testing.T, files map[string]string) []byte {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for n, c := range files {
		w, _ := zw.Create(n)
		w.Write([]byte(c))
	}
	zw.Close()
	return b.Bytes()
}

func tgzOf(files map[string]string) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for n, c := range files {
		tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(c)), Typeflag: tar.TypeReg})
		tw.Write([]byte(c))
	}
	tw.Close()
	gz.Close()
	return b.Bytes()
}

var multiSkill = map[string]string{
	"pkg/test-skill/SKILL.md":             skillText,
	"pkg/test-skill/references/style.md":  "REF-STYLE-MARKER",
	"pkg/test-skill/references/words.md":  "REF-WORDS-MARKER",
	"pkg/test-skill/references/unused.md": "REF-UNUSED-MARKER",
	"pkg/test-skill/scripts/run.sh":       "rm -rf /",
	"pkg/test-skill/assets/logo.txt":      "logo",
	"pkg/README.md":                       "outside the skill",
}

func TestSkillFormat(t *testing.T) {
	fm, err := validateSkill([]byte(skillText))
	if err != nil || fm.Name != "test-skill" || fm.Description != "A test skill for briefs." || fm.License != "MIT" || !strings.Contains(fm.Body, "SKILL-MARKER") {
		t.Fatalf("front matter: %+v %v", fm, err)
	}
	for name, md := range map[string]string{
		"no front matter": "# hi\n\ntext text text",
		"no name":         "---\ndescription: d\n---\nbody",
		"bad name":        "---\nname: Bad_Name\ndescription: d\n---\nbody",
		"no description":  "---\nname: ok\n---\nbody",
		"no body":         "---\nname: ok\ndescription: d\n---\n",
		"html":            "<!DOCTYPE html><html></html>",
	} {
		if _, err := validateSkill([]byte(md)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	for _, data := range [][]byte{zipOf(t, multiSkill), tgzOf(multiSkill)} {
		files, isArch, err := readArchive(data)
		if err != nil || !isArch {
			t.Fatal(err)
		}
		tree, fm, err := buildSkillTree(files, "")
		if err != nil || fm.Name != "test-skill" || len(tree) != 6 {
			t.Fatalf("tree: %d %v", len(tree), err)
		}
	}
	// path traversal / junk is dropped
	files, _, _ := readArchive(zipOf(t, map[string]string{"../evil/SKILL.md": skillText, "__MACOSX/x/SKILL.md": skillText}))
	if len(files) != 0 {
		t.Fatalf("unsafe paths kept: %+v", files)
	}
	if tb, pre, ok := githubSource(mustURL("https://github.com/o/r/tree/main/skills/x")); !ok || tb != "https://codeload.github.com/o/r/tar.gz/main" || pre != "skills/x" {
		t.Fatal("github tree", tb, pre)
	}
	if tb, pre, ok := githubSource(mustURL("https://github.com/o/r")); !ok || tb != "https://codeload.github.com/o/r/tar.gz/HEAD" || pre != "" {
		t.Fatal("github repo", tb, pre)
	}
	if _, pre, _ := githubSource(mustURL("https://github.com/o/r/blob/v1/a/SKILL.md")); pre != "a" {
		t.Fatal("github blob", pre)
	}
}

func mustURL(s string) *url.URL { u, _ := url.Parse(s); return u }

func TestDefaultSkill(t *testing.T) {
	st := testStore(t)
	sys, name := st.briefSystemPrompt()
	if name != DefaultSkillName {
		t.Fatal(name)
	}
	if _, err := validateSkill(mustRead(t, "defaultskill/brief-polish/SKILL.md")); err != nil {
		t.Fatal("built-in skill invalid:", err)
	}
	for _, want := range []string{"始终用简体中文写", "用简体中文写", `<reference path="references/style-rules.md">`, `<reference path="references/banned-phrases.md">`, "值得注意的是"} {
		if !strings.Contains(sys, want) {
			t.Errorf("system prompt lacks %q", want)
		}
	}
	if strings.Contains(sys, "Permission is hereby granted") {
		t.Error("credits/license should not be sent to the model")
	}
}

func mustRead(t *testing.T, p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExtractArticle(t *testing.T) {
	html := `<html><head><meta property="og:description" content="desc"></head><body><nav><p>Menu menu menu menu menu menu menu menu menu menu</p></nav>
<article><p>By Jane</p><p>OpenAI released a new model on Tuesday with a one million token context window and lower prices.</p>
<script>var x = "<p>nope nope nope nope nope nope nope nope nope nope</p>"</script><p>它在多项测试中的成绩比上一代高出不少，开发者可以立即通过接口使用。</p></article><footer><p>Copyright footer text that is long enough to count here</p></footer></body></html>`
	got := extractArticle(html)
	if !strings.Contains(got, "one million token") || !strings.Contains(got, "开发者可以立即") || strings.Contains(got, "Menu") || strings.Contains(got, "nope") || strings.Contains(got, "By Jane") || strings.Contains(got, "Copyright") {
		t.Fatalf("extract: %q", got)
	}
	if !skipArticleHost("https://x.com/a/status/1") || skipArticleHost("https://techcrunch.com/a") {
		t.Fatal("skip hosts")
	}
}

// AI 润色 must send the active skill in the system prompt (built-in default when none is selected).
func TestPolishUsesActiveSkill(t *testing.T) {
	var mu sync.Mutex
	var lastSys, lastURL string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query().Get("u")
		if strings.HasSuffix(u, "/SKILL.md") {
			io.WriteString(w, skillText)
			return
		}
		if strings.HasPrefix(u, "https://codeload.github.com/") {
			w.Write(tgzOf(map[string]string{"repo-main/pkg/test-skill/SKILL.md": skillText, "repo-main/pkg/test-skill/references/style.md": "REF-STYLE-MARKER",
				"repo-main/pkg/test-skill/references/words.md": "REF-WORDS-MARKER", "repo-main/pkg/test-skill/references/unused.md": "REF-UNUSED-MARKER", "repo-main/pkg/test-skill/scripts/run.sh": "x"}))
			return
		}
		var body struct {
			Messages []struct{ Role, Content string } `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		lastURL = u
		if len(body.Messages) > 0 {
			lastSys = body.Messages[0].Content
		}
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": "# 润色结果"}}}})
	}))
	defer mock.Close()
	t.Setenv("DEEPSEEK_API_KEY", "sk-"+"unit"+"test0000")
	t.Setenv("BRIEF_CACHE_MS", "1")
	d := t.TempDir()
	st, err := Open(Config{DBPath: filepath.Join(d, "news.db"), SeedPath: "../../sources/sources.json", SecretsPath: filepath.Join(d, "secrets.json"), Desktop: true})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	t.Setenv("UPSTREAM_MOCK", mock.URL)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()

	brief := func() {
		r, err := http.Get(srv.URL + "/api/brief?section=all&ai=1")
		if err != nil || r.StatusCode != 200 {
			t.Fatalf("brief: %v %v", err, r.Status)
		}
		r.Body.Close()
	}
	brief()
	if strings.Contains(lastSys, "SKILL-MARKER") || !strings.Contains(lastSys, `<skill name="brief-polish">`) || !strings.Contains(lastSys, "简体中文") || !strings.Contains(lastURL, "/chat/completions") {
		t.Fatalf("default skill missing from system prompt: %q", lastSys)
	}
	// install from https URL, then it is used once active
	r, _ := http.Post(srv.URL+"/api/skills/install", "application/json", strings.NewReader(`{"url":"https://github.com/o/repo/tree/main/pkg/test-skill"}`))
	var res struct {
		Installed string `json:"installed"`
		Skills    []skillMeta
	}
	json.NewDecoder(r.Body).Decode(&res)
	r.Body.Close()
	if res.Installed == "" || len(res.Skills) != 2 {
		t.Fatalf("install: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(d, "skills", res.Installed, "SKILL.md")); err != nil {
		t.Fatal("not stored in the data dir")
	}
	req, _ := http.NewRequest("PATCH", srv.URL+"/api/skills/active", strings.NewReader(`{"slug":"`+res.Installed+`"}`))
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != 200 {
		t.Fatal("activate", r.Status)
	}
	brief()
	if !strings.Contains(lastSys, "SKILL-MARKER-7f3a") || !strings.Contains(lastSys, "REF-STYLE-MARKER") || !strings.Contains(lastSys, "REF-WORDS-MARKER") || strings.Contains(lastSys, "REF-UNUSED-MARKER") || !strings.Contains(lastSys, "scripts/run.sh") || strings.Contains(lastSys, "rm -rf") {
		t.Fatalf("active skill / referenced files / unavailable scripts wrong in system prompt: %q", lastSys)
	}
	if _, err := os.Stat(filepath.Join(d, "skills", "test-skill", "references", "unused.md")); err != nil {
		t.Fatal("whole tree not stored")
	}
	// http (not https) refused
	if r, _ := http.Post(srv.URL+"/api/skills/install", "application/json", strings.NewReader(`{"url":"http://example.com/SKILL.md"}`)); r.StatusCode != 400 {
		t.Fatal("http accepted")
	}
	// upload a zip
	var mb bytes.Buffer
	mw := multipart.NewWriter(&mb)
	fw, _ := mw.CreateFormFile("file", "pkg.zip")
	fw.Write(zipOf(t, map[string]string{"pkg/SKILL.md": "---\nname: uploaded\ndescription: d\n---\nUPLOADED-SKILL"}))
	mw.Close()
	if r, _ := http.Post(srv.URL+"/api/skills/upload", mw.FormDataContentType(), &mb); r.StatusCode != 200 {
		t.Fatal("upload", r.Status)
	}
	// delete the active one → falls back to default
	req, _ = http.NewRequest("DELETE", srv.URL+"/api/skills/"+res.Installed, nil)
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != 200 {
		t.Fatal("delete", r.Status)
	}
	if n := st.activeSkill().Name; n != DefaultSkillName {
		t.Fatalf("after delete active=%q", n)
	}
}

func TestSkillsWebDisabled(t *testing.T) {
	st := testStore(t)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	if r, _ := http.Get(srv.URL + "/api/skills"); r.StatusCode != 404 {
		t.Fatal("skills on web", r.Status)
	}
}

func TestBriefTimeAndTruncate(t *testing.T) {
	if got := fmtBriefTime(1792149123); got != "10/16 19:12" { // published is in seconds (Beijing time)
		t.Fatalf("fmtBriefTime = %q", got)
	}
	if got := oneLine(strings.Repeat("词", 300)); len([]rune(got)) != briefSumLine || !strings.HasSuffix(got, "…") {
		t.Fatalf("oneLine truncation: %d", len([]rune(got)))
	}
}
