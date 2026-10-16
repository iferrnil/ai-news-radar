package radar

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// 简报 Skill (desktop only): the instructions used by 「AI 润色」. A skill is a SKILL.md (optionally with YAML-ish
// front matter: name / description), installed from an https URL or uploaded (SKILL.md, or a zip / tar.gz that
// contains one). Stored under <data dir>/skills/<slug>/{SKILL.md,meta.json}; the active one is the "brief_skill" setting.

const (
	skillMaxDownload = 4 << 20   // archive / file as downloaded or uploaded
	skillMaxMD       = 256 << 10 // SKILL.md itself
	skillMaxEntries  = 2000
	DefaultSkillName = "默认简报"
)

const defaultSkillMD = `---
name: 默认简报
description: 内置：简洁的中文分区简报（总览 + 分组要点）
---
# 默认简报 Skill

- 开头用 2–4 句话总览本期最重要的动态。
- 按来源分组保留条目；每条：加粗标题，一句话要点，单独一行「来源 · [原文链接](url)」。
- 合并重复或同类的表述，去掉营销话术；保持客观，不编造原文没有的事实。
- 中文输出，专有名词与产品名保留英文原文。
`

type skillMeta struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	Source      string  `json:"source"`
	InstalledAt float64 `json:"installedAt"`
	Size        int     `json:"size"`
	Builtin     bool    `json:"builtin,omitempty"`
	Active      bool    `json:"active"`
}

func (s *Store) skillsDir() string { return filepath.Join(filepath.Dir(s.Cfg.DBPath), "skills") }

var slugBad = regexp.MustCompile(`[^\p{L}\p{N}_-]+`)

func skillSlug(name string) string {
	v := strings.Trim(slugBad.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
	if r := []rune(v); len(r) > 48 {
		v = string(r[:48])
	}
	if v == "" || v == "." || v == ".." {
		v = "skill"
	}
	return v
}

// frontMatter reads name/description from a leading --- block (simple "key: value" lines).
func frontMatter(md string) (name, desc string) {
	md = strings.TrimPrefix(md, "\ufeff")
	if !strings.HasPrefix(md, "---") {
		return
	}
	rest := md[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return
	}
	for _, ln := range strings.Split(rest[:end], "\n") {
		k, v, ok := strings.Cut(ln, ":")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "name":
			name = v
		case "description":
			desc = v
		}
	}
	return
}

func firstHeading(md string) string {
	for _, ln := range strings.Split(md, "\n") {
		if t := strings.TrimSpace(ln); strings.HasPrefix(t, "# ") {
			return strings.TrimSpace(t[2:])
		}
	}
	return ""
}

// validateSkillMD: UTF-8 text, not HTML, not empty, size-limited.
func validateSkillMD(b []byte) (string, error) {
	if len(b) > skillMaxMD {
		return "", errors.New("SKILL.md 过大（上限 256 KB）")
	}
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return "", errors.New("SKILL.md 不是 UTF-8 文本")
	}
	md := strings.TrimPrefix(string(b), "\ufeff")
	lt := strings.ToLower(strings.TrimSpace(md))
	if strings.HasPrefix(lt, "<!doctype") || strings.HasPrefix(lt, "<html") {
		return "", errors.New("得到的是网页而不是 SKILL.md（请使用 raw 文件地址）")
	}
	if len(strings.TrimSpace(md)) < 10 {
		return "", errors.New("SKILL.md 内容为空")
	}
	return md, nil
}

func isSkillFile(p string) bool { return strings.EqualFold(path.Base(p), "SKILL.md") }

// pickShallowest keeps the SKILL.md closest to the archive root.
func depth(p string) int { return strings.Count(strings.Trim(p, "/"), "/") }

// extractSkillMD finds SKILL.md in a zip / tar(.gz) or treats data as the file itself.
func extractSkillMD(data []byte, hint string) (string, error) {
	if len(data) > skillMaxDownload {
		return "", errors.New("文件过大（上限 4 MB）")
	}
	readLimited := func(r io.Reader) ([]byte, error) {
		b, err := io.ReadAll(io.LimitReader(r, skillMaxMD+1))
		if err == nil && len(b) > skillMaxMD {
			return nil, errors.New("SKILL.md 过大（上限 256 KB）")
		}
		return b, err
	}
	switch {
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", errors.New("zip 无法解析")
		}
		if len(zr.File) > skillMaxEntries {
			return "", errors.New("压缩包文件过多")
		}
		var best *zip.File
		for _, f := range zr.File {
			if !f.FileInfo().IsDir() && isSkillFile(f.Name) && !strings.Contains(f.Name, "__MACOSX") && (best == nil || depth(f.Name) < depth(best.Name)) {
				best = f
			}
		}
		if best == nil {
			return "", errors.New("压缩包里没有 SKILL.md")
		}
		rc, err := best.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		b, err := readLimited(rc)
		if err != nil {
			return "", err
		}
		return validateSkillMD(b)
	case bytes.HasPrefix(data, []byte{0x1f, 0x8b}) || (len(data) > 262 && string(data[257:262]) == "ustar"):
		var r io.Reader = bytes.NewReader(data)
		if data[0] == 0x1f {
			gz, err := gzip.NewReader(r)
			if err != nil {
				return "", errors.New("tar.gz 无法解析")
			}
			r = io.LimitReader(gz, 64<<20)
		}
		tr := tar.NewReader(r)
		var best []byte
		bestD := 1 << 30
		for n := 0; ; n++ {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", errors.New("tar 无法解析")
			}
			if n > skillMaxEntries {
				return "", errors.New("压缩包文件过多")
			}
			if h.Typeflag == tar.TypeReg && isSkillFile(h.Name) && depth(h.Name) < bestD {
				b, err := readLimited(tr)
				if err != nil {
					return "", err
				}
				best, bestD = b, depth(h.Name)
			}
		}
		if best == nil {
			return "", errors.New("压缩包里没有 SKILL.md")
		}
		return validateSkillMD(best)
	}
	if ext := strings.ToLower(path.Ext(hint)); ext == ".zip" || ext == ".gz" || ext == ".tgz" || ext == ".tar" {
		return "", errors.New("压缩包无法解析")
	}
	return validateSkillMD(data)
}

func (s *Store) readSkillMeta(slug string) (*skillMeta, string, bool) {
	d := filepath.Join(s.skillsDir(), slug)
	md, err := os.ReadFile(filepath.Join(d, "SKILL.md"))
	if err != nil {
		return nil, "", false
	}
	m := &skillMeta{Slug: slug}
	if b, err := os.ReadFile(filepath.Join(d, "meta.json")); err == nil {
		json.Unmarshal(b, m)
	}
	m.Slug, m.Size = slug, len(md)
	if m.Name == "" {
		m.Name = slug
	}
	return m, string(md), true
}

func (s *Store) listSkills() []*skillMeta {
	act, _ := s.GetSetting("brief_skill")
	out := []*skillMeta{}
	ents, _ := os.ReadDir(s.skillsDir())
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if m, _, ok := s.readSkillMeta(e.Name()); ok {
			m.Active = act == m.Slug
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstalledAt > out[j].InstalledAt })
	def := &skillMeta{Name: DefaultSkillName, Slug: "", Description: "内置：简洁的中文分区简报（总览 + 分组要点）", Builtin: true, Size: len(defaultSkillMD)}
	def.Active = true
	for _, m := range out {
		if m.Active {
			def.Active = false
		}
	}
	return append([]*skillMeta{def}, out...)
}

// activeSkill: (name, SKILL.md) used by 「AI 润色」 — the selected installed skill on desktop, else the built-in one.
func (s *Store) activeSkill() (string, string) {
	if s.Cfg.Desktop {
		if slug, _ := s.GetSetting("brief_skill"); slug != "" {
			if m, md, ok := s.readSkillMeta(slug); ok {
				return m.Name, md
			}
		}
	}
	return DefaultSkillName, defaultSkillMD
}

func (s *Store) saveSkill(md, source, hint string) (*skillMeta, error) {
	name, desc := frontMatter(md)
	if name == "" {
		name = firstHeading(md)
	}
	if name == "" {
		name = strings.TrimSuffix(path.Base(hint), path.Ext(hint))
		if strings.EqualFold(name, "SKILL") || name == "" || name == "." || name == "/" {
			name = "skill"
		}
	}
	name = string([]rune(name)[:min(len([]rune(name)), 60)])
	base := skillSlug(name)
	slug := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(s.skillsDir(), slug)); os.IsNotExist(err) {
			break
		}
		slug = base + "-" + skItoa(i)
	}
	d := filepath.Join(s.skillsDir(), slug)
	if err := os.MkdirAll(d, 0o755); err != nil {
		return nil, err
	}
	m := &skillMeta{Name: name, Slug: slug, Description: desc, Source: source, InstalledAt: now(), Size: len(md)}
	mb, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(md), 0o644); err != nil {
		return nil, err
	}
	os.WriteFile(filepath.Join(d, "meta.json"), mb, 0o644)
	return m, nil
}

func skItoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func desktopOnly(s *Store, w http.ResponseWriter) bool {
	if !s.Cfg.Desktop {
		detail(w, 404, "简报 Skill 仅桌面版可用")
		return false
	}
	return true
}

func (s *Store) skillsOut(w http.ResponseWriter, extra ...any) {
	w.Header().Set("Cache-Control", "no-store")
	o := obj("skills", s.listSkills())
	for i := 0; i+1 < len(extra); i += 2 {
		o.Set(extra[i].(string), extra[i+1])
	}
	writeJSON(w, 200, o)
}

func (s *Store) hSkillsList(w http.ResponseWriter, r *http.Request) {
	if desktopOnly(s, w) {
		s.skillsOut(w)
	}
}

func (s *Store) hSkillGet(w http.ResponseWriter, r *http.Request) {
	if !desktopOnly(s, w) {
		return
	}
	slug := r.PathValue("name")
	if slug == "default" || slug == "" {
		writeJSON(w, 200, obj("name", DefaultSkillName, "slug", "", "content", defaultSkillMD, "builtin", true))
		return
	}
	m, md, ok := s.readSkillMeta(filepath.Base(slug))
	if !ok || filepath.Base(slug) != slug {
		detail(w, 404, "Skill 不存在")
		return
	}
	writeJSON(w, 200, obj("name", m.Name, "slug", m.Slug, "description", m.Description, "source", m.Source, "content", md))
}

func (s *Store) hSkillInstall(w http.ResponseWriter, r *http.Request) {
	if !desktopOnly(s, w) {
		return
	}
	u := strings.TrimSpace(str(readBody(r)["url"]))
	pu, err := url.Parse(u)
	if err != nil || pu.Scheme != "https" || pu.Host == "" {
		detail(w, 400, "只支持 https:// 地址")
		return
	}
	resp, err := netFetch(context.Background(), "GET", u, map[string]string{"Accept": "text/markdown, text/plain, application/zip, application/gzip, */*"}, nil, 30*time.Second)
	if err != nil {
		detail(w, 502, "下载失败："+err.Error())
		return
	}
	if !resp.OK() {
		detail(w, 502, "下载失败：HTTP "+skItoa(resp.Status))
		return
	}
	if len(resp.Body) > skillMaxDownload {
		detail(w, 413, "文件过大（上限 4 MB）")
		return
	}
	md, err := extractSkillMD(resp.Body, pu.Path)
	if err != nil {
		detail(w, 422, err.Error())
		return
	}
	m, err := s.saveSkill(md, u, pu.Path)
	if err != nil {
		detail(w, 500, err.Error())
		return
	}
	s.skillsOut(w, "installed", m.Slug)
}

func (s *Store) hSkillUpload(w http.ResponseWriter, r *http.Request) {
	if !desktopOnly(s, w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, skillMaxDownload+64<<10)
	if err := r.ParseMultipartForm(skillMaxDownload + 64<<10); err != nil {
		detail(w, 413, "文件过大（上限 4 MB）或格式不对")
		return
	}
	f, h, err := r.FormFile("file")
	if err != nil {
		detail(w, 400, "缺少文件")
		return
	}
	defer f.Close()
	data, _ := io.ReadAll(io.LimitReader(f, skillMaxDownload+1))
	md, err := extractSkillMD(data, h.Filename)
	if err != nil {
		detail(w, 422, err.Error())
		return
	}
	m, err := s.saveSkill(md, "上传："+h.Filename, h.Filename)
	if err != nil {
		detail(w, 500, err.Error())
		return
	}
	s.skillsOut(w, "installed", m.Slug)
}

func (s *Store) hSkillActive(w http.ResponseWriter, r *http.Request) {
	if !desktopOnly(s, w) {
		return
	}
	slug := str(readBody(r)["slug"])
	if slug != "" {
		if _, _, ok := s.readSkillMeta(filepath.Base(slug)); !ok || filepath.Base(slug) != slug {
			detail(w, 404, "Skill 不存在")
			return
		}
	}
	s.SetSetting("brief_skill", slug)
	s.skillsOut(w)
}

func (s *Store) hSkillDelete(w http.ResponseWriter, r *http.Request) {
	if !desktopOnly(s, w) {
		return
	}
	slug := r.PathValue("name")
	if slug == "" || filepath.Base(slug) != slug || slug == "." || slug == ".." {
		detail(w, 400, "名称不合法")
		return
	}
	if _, _, ok := s.readSkillMeta(slug); !ok {
		detail(w, 404, "Skill 不存在")
		return
	}
	os.RemoveAll(filepath.Join(s.skillsDir(), slug))
	if a, _ := s.GetSetting("brief_skill"); a == slug {
		s.SetSetting("brief_skill", "")
	}
	s.skillsOut(w)
}

func str(v any) string { s, _ := v.(string); return s }
