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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const skillText = "---\nname: 测试 Skill\ndescription: d\n---\n# 测试\n\nSKILL-MARKER-7f3a：每条用一句话。\n"

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

func TestExtractSkillMD(t *testing.T) {
	if md, err := extractSkillMD([]byte(skillText), "SKILL.md"); err != nil || !strings.Contains(md, "SKILL-MARKER") {
		t.Fatalf("plain: %v", err)
	}
	if md, err := extractSkillMD(zipOf(t, map[string]string{"x/deep/SKILL.md": "# deep one here", "x/SKILL.md": skillText, "x/README.md": "r"}), "a.zip"); err != nil || !strings.Contains(md, "SKILL-MARKER") {
		t.Fatalf("zip (shallowest): %q %v", md, err)
	}
	if md, err := extractSkillMD(tgzOf(map[string]string{"pkg/SKILL.md": skillText}), "a.tgz"); err != nil || !strings.Contains(md, "SKILL-MARKER") {
		t.Fatalf("tgz: %v", err)
	}
	for name, data := range map[string][]byte{
		"no skill in zip": zipOf(t, map[string]string{"README.md": "hello world!!"}),
		"html":            []byte("<!DOCTYPE html><html>nope</html>"),
		"empty":           []byte("   "),
		"binary":          {0xff, 0xfe, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		"too big":         bytes.Repeat([]byte("a"), skillMaxMD+1),
	} {
		if _, err := extractSkillMD(data, "SKILL.md"); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if n, d := frontMatter(skillText); n != "测试 Skill" || d != "d" {
		t.Errorf("front matter %q %q", n, d)
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
	if strings.Contains(lastSys, "SKILL-MARKER") || !strings.Contains(lastSys, "默认简报") || !strings.Contains(lastURL, "/chat/completions") {
		t.Fatalf("default skill missing from system prompt: %q", lastSys)
	}
	// install from https URL, then it is used once active
	r, _ := http.Post(srv.URL+"/api/skills/install", "application/json", strings.NewReader(`{"url":"https://example.com/s/SKILL.md"}`))
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
	if !strings.Contains(lastSys, "SKILL-MARKER-7f3a") {
		t.Fatalf("active skill missing from system prompt: %q", lastSys)
	}
	// http (not https) refused
	if r, _ := http.Post(srv.URL+"/api/skills/install", "application/json", strings.NewReader(`{"url":"http://example.com/SKILL.md"}`)); r.StatusCode != 400 {
		t.Fatal("http accepted")
	}
	// upload a zip
	var mb bytes.Buffer
	mw := multipart.NewWriter(&mb)
	fw, _ := mw.CreateFormFile("file", "pkg.zip")
	fw.Write(zipOf(t, map[string]string{"pkg/SKILL.md": "# 上传的\n\nUPLOADED-SKILL"}))
	mw.Close()
	if r, _ := http.Post(srv.URL+"/api/skills/upload", mw.FormDataContentType(), &mb); r.StatusCode != 200 {
		t.Fatal("upload", r.Status)
	}
	// delete the active one → falls back to default
	req, _ = http.NewRequest("DELETE", srv.URL+"/api/skills/"+res.Installed, nil)
	if r, _ := http.DefaultClient.Do(req); r.StatusCode != 200 {
		t.Fatal("delete", r.Status)
	}
	if n, _ := st.activeSkill(); n != DefaultSkillName {
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
