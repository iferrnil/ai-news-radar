package radar

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 工作目录 (desktop only; setting key "artifacts_dir" kept from the 制品目录 days): a brief / article is written
// there only when it is exported (导出), as
//   <dir>/简报|文章/<YYYY-MM-DD>/<HHMMSS>-<title>.md  (+ its cover next to it)
// Generated covers and the 原文缓存 live in <dir>/缓存/ and are deleted after a day (cache.go).
// <dir> is the "artifacts_dir" setting, else $ARTIFACTS_DIR, else ~/Documents/AI News Radar.

func (s *Store) defaultArtifactsDir() string {
	if v := strings.TrimSpace(os.Getenv("ARTIFACTS_DIR")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(filepath.Dir(s.Cfg.DBPath), "artifacts")
	}
	return filepath.Join(home, "Documents", "AI News Radar")
}

func (s *Store) artifactsDir() string {
	if v, _ := s.GetSetting("artifacts_dir"); strings.TrimSpace(v) != "" {
		return v
	}
	return s.defaultArtifactsDir()
}

var unsafeName = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]+`)

// artifactPath: <dir>/<typ>/<date>/<time>-<label> (no extension).
func (s *Store) artifactPath(typ, label string) string {
	return datedPath(s.artifactsDir(), typ, label)
}

// cachePath: like artifactPath, under <dir>/缓存.
func (s *Store) cachePath(typ, label string) string { return datedPath(s.cacheDir(), typ, label) }

func datedPath(root, typ, label string) string {
	t := time.Now().In(time.FixedZone("CST", 8*3600))
	label = strings.TrimSpace(unsafeName.ReplaceAllString(label, "-"))
	if r := []rune(label); len(r) > 40 {
		label = string(r[:40])
	}
	name := t.Format("150405")
	if label != "" {
		name += "-" + label
	}
	return filepath.Join(root, typ, t.Format("2006-01-02"), name)
}

func (s *Store) artifactsOut(w http.ResponseWriter) {
	v, _ := s.GetSetting("artifacts_dir")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, obj("dir", s.artifactsDir(), "default", s.defaultArtifactsDir(), "custom", strings.TrimSpace(v) != "", "canOpen", DesktopHooks.OpenPath != nil))
}

func (s *Store) hArtifactsGet(w http.ResponseWriter, r *http.Request) {
	if desktopOnlyMsg(s, w, "工作目录仅桌面版可用") {
		s.artifactsOut(w)
	}
}

func (s *Store) hArtifactsPatch(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "工作目录仅桌面版可用") {
		return
	}
	d := strings.TrimSpace(jsString(readBody(r)["dir"]))
	if strings.HasPrefix(d, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			d = filepath.Join(h, d[2:])
		}
	}
	if d != "" {
		if !filepath.IsAbs(d) {
			detail(w, 400, "请填写绝对路径")
			return
		}
		d = filepath.Clean(d)
		if badArtifactsDir(d) {
			detail(w, 400, "不能用这个目录作为工作目录")
			return
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			detail(w, 400, "无法创建目录："+err.Error())
			return
		}
	}
	s.SetSetting("artifacts_dir", d)
	s.artifactsOut(w)
}

func (s *Store) hArtifactsOpen(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "工作目录仅桌面版可用") {
		return
	}
	if DesktopHooks.OpenPath == nil {
		detail(w, 404, "仅桌面版可用")
		return
	}
	d := s.artifactsDir()
	os.MkdirAll(d, 0o755)
	if p := filepath.Clean(str(readBody(r)["path"])); strings.HasPrefix(p, filepath.Clean(d)+string(filepath.Separator)) { // an exported file: its folder
		if st, err := os.Stat(p); err == nil {
			if !st.IsDir() {
				p = filepath.Dir(p)
			}
			d = p
		}
	}
	if st, err := os.Stat(d); err != nil || !st.IsDir() || isPackage(d) { // folders only, never an app / installer
		writeJSON(w, 200, obj("ok", false, "error", "无法打开"))
		return
	}
	if err := DesktopHooks.OpenPath(d); err != nil {
		writeJSON(w, 200, obj("ok", false, "error", err.Error()))
		return
	}
	writeJSON(w, 200, obj("ok", true))
}

// GET /api/artifacts/file?path=… — a generated cover for the article dialog (only images in <工作目录>/缓存/封面).
func (s *Store) hArtifactFile(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "工作目录仅桌面版可用") {
		return
	}
	p, ok := s.coverFile(r.URL.Query().Get("path"))
	if !ok {
		detail(w, 404, "文件不存在")
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeFile(w, r, p)
}

// POST /api/artifacts/export {kind: brief|article, title, markdown, covers: [path]} (desktop): 导出 writes the
// Markdown to <工作目录>/简报|文章/<date>/; a generated cover (inside the 工作目录) is copied next to it and linked.
func (s *Store) hArtifactsExport(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "工作目录仅桌面版可用") {
		return
	}
	b := readBody(r)
	typ := map[string]string{"brief": "简报", "article": "文章"}[str(b["kind"])]
	md := str(b["markdown"])
	if typ == "" || strings.TrimSpace(md) == "" {
		detail(w, 400, "没有可导出的内容")
		return
	}
	title := strings.TrimSpace(str(b["title"]))
	if title == "" {
		title = typ
	}
	p := s.artifactPath(typ, title) + ".md"
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		detail(w, 500, "无法创建目录："+err.Error())
		return
	}
	for i, base := 2, strings.TrimSuffix(p, ".md"); ; i++ { // two in one second
		if _, err := os.Stat(p); err != nil {
			break
		}
		p = base + "-" + strconv.Itoa(i) + ".md"
	}
	stem := strings.TrimSuffix(filepath.Base(p), ".md")
	var head, tail string
	if xs, ok := b["covers"].([]any); ok {
		for _, x := range xs {
			src, ok := s.coverFile(str(x))
			if !ok {
				continue
			}
			data, err := os.ReadFile(src)
			if err != nil {
				continue
			}
			name := stem + "-封面" + filepath.Ext(src)
			if os.WriteFile(filepath.Join(filepath.Dir(p), name), data, 0o644) != nil {
				continue
			}
			head += fmt.Sprintf("![封面](%s)\n\n", name)
		}
	}
	if err := os.WriteFile(p, []byte(head+strings.TrimRight(md, "\n")+"\n"+tail), 0o644); err != nil {
		detail(w, 500, "写入失败："+err.Error())
		return
	}
	writeJSON(w, 200, obj("path", p, "canOpen", DesktopHooks.OpenPath != nil))
}
