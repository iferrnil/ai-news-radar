package radar

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 成果目录 (desktop only): every generated brief / article (Markdown) and cover image / video is saved under
//   <dir>/简报|文章|封面|视频/<YYYY-MM-DD>/<HHMMSS>-<label>.<ext>
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
	t := time.Now().In(time.FixedZone("CST", 8*3600))
	label = strings.TrimSpace(unsafeName.ReplaceAllString(label, "-"))
	if r := []rune(label); len(r) > 40 {
		label = string(r[:40])
	}
	name := t.Format("150405")
	if label != "" {
		name += "-" + label
	}
	return filepath.Join(s.artifactsDir(), typ, t.Format("2006-01-02"), name)
}

// saveArtifact writes a Markdown artifact; "" on the web server or when it fails (generation never fails on it).
func (s *Store) saveArtifact(typ, label, md string) string {
	if !s.Cfg.Desktop {
		return ""
	}
	p := s.artifactPath(typ, label) + ".md"
	base := p
	if os.MkdirAll(filepath.Dir(p), 0o755) != nil {
		return ""
	}
	for i := 2; ; i++ { // two in one second
		if _, err := os.Stat(p); err != nil {
			break
		}
		p = strings.TrimSuffix(base, ".md") + "-" + strconv.Itoa(i) + ".md"
	}
	if os.WriteFile(p, []byte(md), 0o644) != nil {
		return ""
	}
	return p
}

func (s *Store) artifactsOut(w http.ResponseWriter) {
	v, _ := s.GetSetting("artifacts_dir")
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, obj("dir", s.artifactsDir(), "default", s.defaultArtifactsDir(), "custom", strings.TrimSpace(v) != "", "canOpen", DesktopHooks.OpenPath != nil))
}

func (s *Store) hArtifactsGet(w http.ResponseWriter, r *http.Request) {
	if desktopOnlyMsg(s, w, "成果目录仅桌面版可用") {
		s.artifactsOut(w)
	}
}

func (s *Store) hArtifactsPatch(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "成果目录仅桌面版可用") {
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
		if err := os.MkdirAll(d, 0o755); err != nil {
			detail(w, 400, "无法创建目录："+err.Error())
			return
		}
	}
	s.SetSetting("artifacts_dir", d)
	s.artifactsOut(w)
}

func (s *Store) hArtifactsOpen(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "成果目录仅桌面版可用") {
		return
	}
	if DesktopHooks.OpenPath == nil {
		detail(w, 404, "仅桌面版可用")
		return
	}
	d := s.artifactsDir()
	os.MkdirAll(d, 0o755)
	if err := DesktopHooks.OpenPath(d); err != nil {
		writeJSON(w, 200, obj("ok", false, "error", err.Error()))
		return
	}
	writeJSON(w, 200, obj("ok", true))
}

// GET /api/artifacts/file?path=… — a generated cover for the article dialog (only files inside the 成果目录).
func (s *Store) hArtifactFile(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "成果目录仅桌面版可用") {
		return
	}
	p := filepath.Clean(r.URL.Query().Get("path"))
	root := filepath.Clean(s.artifactsDir()) + string(filepath.Separator)
	if !strings.HasPrefix(p, root) {
		detail(w, 404, "文件不存在")
		return
	}
	if st, err := os.Stat(p); err != nil || st.IsDir() {
		detail(w, 404, "文件不存在")
		return
	}
	http.ServeFile(w, r, p)
}
