package radar

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// 简报 Skill (desktop only) — Agent Skills format (agentskills.io): a folder whose SKILL.md starts with YAML front
// matter (name, description; optional license, compatibility, metadata, allowed-tools), followed by the
// instructions, plus optional scripts/, references/ and assets/. Installed skills keep their whole tree under
// <data dir>/skills/<name>/ (install metadata in .install.json). The active one is the "brief_skill" setting;
// without one the built-in brief-polish skill (embedded below) is used.
//
// 「AI 润色」 sends the SKILL.md body as the system prompt, plus the references/ (and assets/) text files that
// SKILL.md names (progressive disclosure, size-capped). scripts/ are never executed: they are listed to the
// model and in the UI as unavailable.

const (
	skillMaxDownload = 8 << 20   // archive / file as downloaded or uploaded
	skillMaxTotal    = 16 << 20  // extracted tree
	skillMaxFile     = 2 << 20   // one file
	skillMaxMD       = 256 << 10 // SKILL.md
	skillMaxEntries  = 500
	skillRefMax      = 24 << 10 // one referenced file in the prompt
	skillRefTotal    = 64 << 10 // all referenced files in the prompt
	DefaultSkillName = "brief-polish"
)

//go:embed defaultskill
var defaultSkillFS embed.FS

// skillKind: what a Skill is used for. Installed skills are shared; each kind remembers its active one in a
// setting and falls back to its own built-in default.
type skillKind struct{ Key, Setting, Builtin, Label string }

var skillKinds = []skillKind{
	{"brief", "brief_skill", DefaultSkillName, "简报"},
	{"twitter", "twitter_skill", "twitter-article", "推特"},
	{"wechat", "wechat_skill", "wechat-article", "公众号"},
	{"xhs", "xhs_skill", "xhs-article", "小红书"},
}

func skillKindOf(k string) (skillKind, bool) {
	if k == "" {
		k = "brief"
	}
	for _, x := range skillKinds {
		if x.Key == k {
			return x, true
		}
	}
	return skillKind{}, false
}

func isBuiltinSkill(name string) bool {
	for _, x := range skillKinds {
		if x.Builtin == name {
			return true
		}
	}
	return false
}

type skillFile struct {
	Path string `json:"path"`
	Size int    `json:"size"`
	Kind string `json:"kind"` // skill | reference | script | asset | other
}

type skillMeta struct {
	Name          string      `json:"name"`
	Slug          string      `json:"slug"`
	Description   string      `json:"description"`
	License       string      `json:"license,omitempty"`
	Compatibility string      `json:"compatibility,omitempty"`
	Source        string      `json:"source,omitempty"`
	InstalledAt   float64     `json:"installedAt,omitempty"`
	Size          int         `json:"size"`
	Files         []skillFile `json:"files"`
	Builtin       bool        `json:"builtin,omitempty"`
	Active        bool        `json:"active"`
}

// ---- front matter (the subset of YAML SKILL.md files use: scalars, quoted scalars, | and > blocks, nested maps)

type frontMatterT struct {
	Name, Description, License, Compatibility string
	Body                                      string
}

func unquoteYAML(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"') {
		if u, err := strconv.Unquote(v); err == nil {
			return u
		}
		return v[1 : len(v)-1]
	}
	if len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'' {
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return v
}

func parseFrontMatter(md string) (*frontMatterT, error) {
	md = strings.TrimPrefix(strings.ReplaceAll(md, "\r\n", "\n"), "\ufeff")
	if !strings.HasPrefix(md, "---\n") {
		return nil, errors.New("SKILL.md 缺少 YAML front matter（文件需以 --- 开头）")
	}
	rest := md[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, errors.New("SKILL.md 的 front matter 没有结束的 ---")
	}
	head, body := rest[:end], rest[end+4:]
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		body = body[i+1:]
	} else {
		body = ""
	}
	fm := &frontMatterT{Body: strings.TrimSpace(body)}
	lines := strings.Split(head, "\n")
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		if strings.TrimSpace(ln) == "" || strings.HasPrefix(strings.TrimSpace(ln), "#") || ln[0] == ' ' || ln[0] == '\t' {
			continue // nested keys (metadata, allowed-tools lists) are not needed here
		}
		k, v, ok := strings.Cut(ln, ":")
		if !ok {
			return nil, fmt.Errorf("front matter 第 %d 行不是 key: value", i+1)
		}
		v = strings.TrimSpace(v)
		if v == "|" || v == ">" || v == "|-" || v == ">-" {
			var block []string
			for i+1 < len(lines) && (strings.TrimSpace(lines[i+1]) == "" || lines[i+1][0] == ' ' || lines[i+1][0] == '\t') {
				i++
				block = append(block, strings.TrimSpace(lines[i]))
			}
			sep := "\n"
			if v[0] == '>' {
				sep = " "
			}
			v = strings.TrimSpace(strings.Join(block, sep))
		} else {
			v = unquoteYAML(v)
		}
		switch strings.TrimSpace(k) {
		case "name":
			fm.Name = v
		case "description":
			fm.Description = v
		case "license":
			fm.License = v
		case "compatibility":
			fm.Compatibility = v
		}
	}
	return fm, nil
}

var skillNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// validateSkill checks SKILL.md per the spec: UTF-8 text, front matter with a valid name and a description.
func validateSkill(b []byte) (*frontMatterT, error) {
	if len(b) > skillMaxMD {
		return nil, errors.New("SKILL.md 过大（上限 256 KB）")
	}
	if !utf8.Valid(b) || bytes.IndexByte(b, 0) >= 0 {
		return nil, errors.New("SKILL.md 不是 UTF-8 文本")
	}
	if lt := strings.ToLower(strings.TrimSpace(string(b))); strings.HasPrefix(lt, "<!doctype") || strings.HasPrefix(lt, "<html") {
		return nil, errors.New("得到的是网页而不是 SKILL.md（请使用 raw 文件、压缩包或 GitHub 目录地址）")
	}
	fm, err := parseFrontMatter(string(b))
	if err != nil {
		return nil, err
	}
	if fm.Name == "" {
		return nil, errors.New("front matter 缺少 name")
	}
	if len(fm.Name) > 64 || !skillNameRe.MatchString(fm.Name) {
		return nil, fmt.Errorf("name「%s」不合规范：只能是小写字母、数字和连字符，最长 64 个字符", fm.Name)
	}
	if fm.Description == "" {
		return nil, errors.New("front matter 缺少 description")
	}
	if utf8.RuneCountInString(fm.Description) > 1024 {
		return nil, errors.New("description 超过 1024 个字符")
	}
	if fm.Body == "" {
		return nil, errors.New("SKILL.md 没有正文（指令）")
	}
	return fm, nil
}

// ---- trees

type treeFile struct {
	path string
	data []byte
}

func cleanRel(p string) (string, bool) {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimLeft(p, "/")
	c := path.Clean(p)
	if c == "." || c == ".." || strings.HasPrefix(c, "../") || strings.Contains(c, "/../") {
		return "", false
	}
	for _, seg := range strings.Split(c, "/") {
		if seg == "__MACOSX" || seg == ".git" || seg == ".DS_Store" || strings.HasPrefix(seg, "._") {
			return "", false
		}
	}
	return c, true
}

func depth(p string) int { return strings.Count(p, "/") }

// rootTree picks the folder of the shallowest SKILL.md (under prefix, if given) and returns the files under it.
func rootTree(files []treeFile, prefix string) ([]treeFile, []byte, error) {
	prefix = strings.Trim(prefix, "/")
	best := ""
	found := false
	for _, f := range files {
		if path.Base(f.path) != "SKILL.md" {
			continue
		}
		if prefix != "" && !(f.path == prefix+"/SKILL.md" || strings.HasPrefix(f.path, prefix+"/")) {
			continue
		}
		if !found || depth(f.path) < depth(best) {
			best, found = f.path, true
		}
	}
	if !found {
		if prefix != "" {
			return nil, nil, fmt.Errorf("「%s」下没有 SKILL.md", prefix)
		}
		return nil, nil, errors.New("没有找到 SKILL.md（Skill 是包含 SKILL.md 的文件夹）")
	}
	root := path.Dir(best)
	var out []treeFile
	var md []byte
	for _, f := range files {
		rel := f.path
		if root != "." {
			if !strings.HasPrefix(f.path, root+"/") {
				continue
			}
			rel = strings.TrimPrefix(f.path, root+"/")
		}
		if rel == "SKILL.md" {
			md = f.data
		}
		out = append(out, treeFile{rel, f.data})
	}
	return out, md, nil
}

type treeLimiter struct{ n, total int }

func (l *treeLimiter) add(size int) error {
	l.n++
	l.total += size
	if l.n > skillMaxEntries {
		return errors.New("文件过多（上限 500 个）")
	}
	if size > skillMaxFile {
		return errors.New("单个文件过大（上限 2 MB）")
	}
	if l.total > skillMaxTotal {
		return errors.New("解压后过大（上限 16 MB）")
	}
	return nil
}

// readArchive returns the regular files of a zip / tar(.gz); ok=false when data is not an archive.
func readArchive(data []byte) ([]treeFile, bool, error) {
	var lim treeLimiter
	readOne := func(r io.Reader) ([]byte, error) {
		b, err := io.ReadAll(io.LimitReader(r, skillMaxFile+1))
		if err == nil && len(b) > skillMaxFile {
			return nil, errors.New("单个文件过大（上限 2 MB）")
		}
		return b, err
	}
	switch {
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, true, errors.New("zip 无法解析")
		}
		var out []treeFile
		for _, f := range zr.File {
			if f.FileInfo().IsDir() || !f.Mode().IsRegular() {
				continue // directories, symlinks
			}
			p, ok := cleanRel(f.Name)
			if !ok {
				continue
			}
			if err := lim.add(int(f.UncompressedSize64)); err != nil {
				return nil, true, err
			}
			rc, err := f.Open()
			if err != nil {
				return nil, true, err
			}
			b, err := readOne(rc)
			rc.Close()
			if err != nil {
				return nil, true, err
			}
			out = append(out, treeFile{p, b})
		}
		return out, true, nil
	case bytes.HasPrefix(data, []byte{0x1f, 0x8b}) || (len(data) > 262 && string(data[257:262]) == "ustar"):
		var r io.Reader = bytes.NewReader(data)
		if data[0] == 0x1f {
			gz, err := gzip.NewReader(r)
			if err != nil {
				return nil, true, errors.New("tar.gz 无法解析")
			}
			r = io.LimitReader(gz, skillMaxTotal*4)
		}
		tr := tar.NewReader(r)
		var out []treeFile
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, true, errors.New("tar 无法解析")
			}
			if h.Typeflag != tar.TypeReg {
				continue
			}
			p, ok := cleanRel(h.Name)
			if !ok {
				continue
			}
			if err := lim.add(int(h.Size)); err != nil {
				return nil, true, err
			}
			b, err := readOne(tr)
			if err != nil {
				return nil, true, err
			}
			out = append(out, treeFile{p, b})
		}
		return out, true, nil
	}
	return nil, false, nil
}

// stripTop drops the first path component (GitHub tarballs wrap everything in <repo>-<ref>/).
func stripTop(files []treeFile) []treeFile {
	out := files[:0:0]
	for _, f := range files {
		if _, rest, ok := strings.Cut(f.path, "/"); ok && rest != "" {
			out = append(out, treeFile{rest, f.data})
		}
	}
	return out
}

// githubSource maps github.com repo / tree / blob URLs to a codeload tarball + the folder inside it.
func githubSource(u *url.URL) (tarball, prefix string, ok bool) {
	if u.Host != "github.com" && u.Host != "www.github.com" {
		return "", "", false
	}
	seg := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(seg) < 2 {
		return "", "", false
	}
	owner, repo := seg[0], strings.TrimSuffix(seg[1], ".git")
	ref := "HEAD"
	if len(seg) >= 4 && (seg[2] == "tree" || seg[2] == "blob") {
		ref = seg[3]
		prefix = strings.Join(seg[4:], "/")
		if seg[2] == "blob" {
			prefix = path.Dir(prefix)
			if prefix == "." {
				prefix = ""
			}
		}
	} else if len(seg) > 2 {
		return "", "", false
	}
	return "https://codeload.github.com/" + owner + "/" + repo + "/tar.gz/" + url.PathEscape(ref), prefix, true
}

func fileKind(rel string) string {
	switch {
	case rel == "SKILL.md":
		return "skill"
	case strings.HasPrefix(rel, "references/"):
		return "reference"
	case strings.HasPrefix(rel, "scripts/"):
		return "script"
	case strings.HasPrefix(rel, "assets/"):
		return "asset"
	}
	return "other"
}

// buildSkillTree validates a set of files (already rooted or not) and returns the rooted tree + front matter.
func buildSkillTree(files []treeFile, prefix string) ([]treeFile, *frontMatterT, error) {
	tree, md, err := rootTree(files, prefix)
	if err != nil {
		return nil, nil, err
	}
	fm, err := validateSkill(md)
	if err != nil {
		return nil, nil, err
	}
	return tree, fm, nil
}

// ---- storage

func (s *Store) skillsDir() string { return filepath.Join(filepath.Dir(s.Cfg.DBPath), "skills") }

func (s *Store) saveSkill(tree []treeFile, fm *frontMatterT, source string) (*skillMeta, error) {
	if isBuiltinSkill(fm.Name) {
		return nil, fmt.Errorf("「%s」是内置 Skill 的名称，请换一个 name", fm.Name)
	}
	dir := filepath.Join(s.skillsDir(), fm.Name)
	tmp := dir + ".tmp-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	for _, f := range tree {
		p := filepath.Join(tmp, filepath.FromSlash(f.path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, f.data, 0o644); err != nil {
			os.RemoveAll(tmp)
			return nil, err
		}
	}
	mb, _ := json.Marshal(map[string]any{"source": source, "installedAt": now()})
	os.WriteFile(filepath.Join(tmp, ".install.json"), mb, 0o644)
	os.RemoveAll(dir) // reinstalling the same name replaces it
	if err := os.Rename(tmp, dir); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	m, _, ok := s.loadSkill(fm.Name)
	if !ok {
		return nil, errors.New("保存失败")
	}
	return m, nil
}

// skillFS returns the tree of an installed skill, or a built-in one ("" = brief-polish).
func (s *Store) skillFS(slug string) (fs.FS, bool) {
	if slug == "" {
		slug = DefaultSkillName
	}
	if isBuiltinSkill(slug) {
		sub, _ := fs.Sub(defaultSkillFS, "defaultskill/"+slug)
		return sub, true
	}
	if !skillNameRe.MatchString(slug) {
		return nil, false
	}
	d := filepath.Join(s.skillsDir(), slug)
	if st, err := os.Stat(filepath.Join(d, "SKILL.md")); err != nil || st.IsDir() {
		return nil, false
	}
	return os.DirFS(d), true
}

func (s *Store) loadSkill(slug string) (*skillMeta, *frontMatterT, bool) {
	fsys, ok := s.skillFS(slug)
	if !ok {
		return nil, nil, false
	}
	md, err := fs.ReadFile(fsys, "SKILL.md")
	if err != nil {
		return nil, nil, false
	}
	fm, err := parseFrontMatter(string(md))
	if err != nil {
		fm = &frontMatterT{Body: string(md)}
	}
	m := &skillMeta{Name: fm.Name, Slug: slug, Description: fm.Description, License: fm.License, Compatibility: fm.Compatibility, Builtin: slug == "" || isBuiltinSkill(slug)}
	if m.Builtin {
		if slug == "" {
			slug = DefaultSkillName
		}
		m.Slug, m.Name = "", slug
	}
	if m.Name == "" {
		m.Name = slug
	}
	if b, err := fs.ReadFile(fsys, ".install.json"); err == nil {
		var x struct {
			Source      string  `json:"source"`
			InstalledAt float64 `json:"installedAt"`
		}
		json.Unmarshal(b, &x)
		m.Source, m.InstalledAt = x.Source, x.InstalledAt
	}
	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(path.Base(p), ".") {
			return nil
		}
		info, _ := d.Info()
		sz := 0
		if info != nil {
			sz = int(info.Size())
		}
		m.Size += sz
		m.Files = append(m.Files, skillFile{Path: p, Size: sz, Kind: fileKind(p)})
		return nil
	})
	return m, fm, true
}

func (s *Store) listSkills(k skillKind) []*skillMeta {
	act, _ := s.GetSetting(k.Setting)
	out := []*skillMeta{}
	ents, _ := os.ReadDir(s.skillsDir())
	for _, e := range ents {
		if !e.IsDir() || strings.Contains(e.Name(), ".tmp-") {
			continue
		}
		if m, _, ok := s.loadSkill(e.Name()); ok {
			m.Active = act == m.Slug
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstalledAt > out[j].InstalledAt })
	def, _, _ := s.loadSkill(k.Builtin)
	def.Active = true
	for _, m := range out {
		if m.Active {
			def.Active = false
		}
	}
	return append([]*skillMeta{def}, out...)
}

// activeSkillPrompt is what 「AI 润色」 sends: the SKILL.md body, the text files it references, and the scripts
// it ships (not executed).
type skillPrompt struct {
	Name, Body string
	Refs       []treeFile
	Scripts    []string
	Vision     bool // front matter "vision: true" (e.g. under metadata): wants the items' images as input
}

var visionRe = regexp.MustCompile(`(?m)^\s*vision:\s*["']?true["']?\s*$`)

var refPathRe = regexp.MustCompile(`(?:\]\(|` + "`" + `|\s|^)((?:\./)?(?:references|assets)/[A-Za-z0-9._/\-]+\.(?:md|markdown|txt|json|ya?ml|csv))`)

func (s *Store) activeSkill(kind string) *skillPrompt {
	k, _ := skillKindOf(kind)
	if k.Key == "" {
		k = skillKinds[0]
	}
	slug := ""
	if s.Cfg.Desktop {
		slug, _ = s.GetSetting(k.Setting)
		if _, ok := s.skillFS(slug); !ok || isBuiltinSkill(slug) {
			slug = ""
		}
	}
	if slug == "" {
		slug = k.Builtin
	}
	fsys, _ := s.skillFS(slug)
	m, fm, _ := s.loadSkill(slug)
	sp := &skillPrompt{Name: m.Name, Body: fm.Body}
	if raw, err := fs.ReadFile(fsys, "SKILL.md"); err == nil {
		if t := strings.TrimLeft(string(raw), "\ufeff \r\n"); strings.HasPrefix(t, "---") {
			if i := strings.Index(t[3:], "\n---"); i > 0 {
				sp.Vision = visionRe.MatchString(t[3 : 3+i])
			}
		}
	}
	seen, total := map[string]bool{}, 0
	for _, mt := range refPathRe.FindAllStringSubmatch(fm.Body, -1) {
		p := strings.TrimPrefix(mt[1], "./")
		if b := strings.ToLower(path.Base(p)); seen[p] || strings.HasPrefix(b, "credits") || strings.HasPrefix(b, "license") {
			continue
		}
		seen[p] = true
		b, err := fs.ReadFile(fsys, p)
		if err != nil || !utf8.Valid(b) {
			continue
		}
		if len(b) > skillRefMax {
			b = append(b[:skillRefMax:skillRefMax], []byte("\n…（已截断）")...)
		}
		if total+len(b) > skillRefTotal {
			break
		}
		total += len(b)
		sp.Refs = append(sp.Refs, treeFile{p, b})
	}
	for _, f := range m.Files {
		if f.Kind == "script" {
			sp.Scripts = append(sp.Scripts, f.Path)
		}
	}
	return sp
}

// ---- HTTP

func desktopOnly(s *Store, w http.ResponseWriter) bool {
	if !s.Cfg.Desktop {
		detail(w, 404, "Skill 仅桌面版可用")
		return false
	}
	return true
}

// reqKind: ?kind= (or the JSON body's "kind"); brief by default.
func reqKind(w http.ResponseWriter, k string) (skillKind, bool) {
	x, ok := skillKindOf(k)
	if !ok {
		detail(w, 400, "未知的 Skill 类型")
	}
	return x, ok
}

func (s *Store) skillsOut(w http.ResponseWriter, k skillKind, extra ...any) {
	w.Header().Set("Cache-Control", "no-store")
	o := obj("skills", s.listSkills(k), "kind", k.Key)
	for i := 0; i+1 < len(extra); i += 2 {
		o.Set(extra[i].(string), extra[i+1])
	}
	writeJSON(w, 200, o)
}

func (s *Store) hSkillsList(w http.ResponseWriter, r *http.Request) {
	if desktopOnly(s, w) {
		if k, ok := reqKind(w, r.URL.Query().Get("kind")); ok {
			s.skillsOut(w, k)
		}
	}
}

// GET /api/skills/{name}?file=references/x.md — SKILL.md (or one text file) for the preview.
func (s *Store) hSkillGet(w http.ResponseWriter, r *http.Request) {
	if !desktopOnly(s, w) {
		return
	}
	slug := r.PathValue("name")
	if slug == "default" {
		k, ok := reqKind(w, r.URL.Query().Get("kind"))
		if !ok {
			return
		}
		slug = k.Builtin
	}
	fsys, ok := s.skillFS(slug)
	if !ok {
		detail(w, 404, "Skill 不存在")
		return
	}
	m, _, _ := s.loadSkill(slug)
	file := r.URL.Query().Get("file")
	if file == "" {
		file = "SKILL.md"
	}
	p, ok := cleanRel(file)
	b, err := fs.ReadFile(fsys, p)
	if !ok || err != nil {
		detail(w, 404, "文件不存在")
		return
	}
	content := string(b)
	if !utf8.Valid(b) {
		content = fmt.Sprintf("（二进制文件，%d 字节）", len(b))
	} else if len(content) > skillMaxMD {
		content = content[:skillMaxMD] + "\n…"
	}
	writeJSON(w, 200, obj("name", m.Name, "slug", m.Slug, "file", p, "content", content, "files", m.Files, "builtin", m.Builtin))
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
	get, prefix, gh := u, "", false
	if tb, pre, ok := githubSource(pu); ok {
		get, prefix, gh = tb, pre, true
	}
	resp, err := netFetch(PublicOnly(context.Background()), "GET", get, map[string]string{"Accept": "*/*"}, nil, 60*time.Second)
	if err != nil {
		detail(w, 502, "下载失败："+err.Error())
		return
	}
	if !resp.OK() {
		detail(w, 502, "下载失败：HTTP "+strconv.Itoa(resp.Status))
		return
	}
	if len(resp.Body) >= skillMaxDownload {
		detail(w, 413, "文件过大（上限 8 MB）")
		return
	}
	files, isArchive, err := readArchive(resp.Body)
	if err != nil {
		detail(w, 422, err.Error())
		return
	}
	if gh {
		files = stripTop(files)
	} else if !isArchive {
		files = []treeFile{{"SKILL.md", resp.Body}} // a single SKILL.md
	}
	tree, fm, err := buildSkillTree(files, prefix)
	if err != nil {
		detail(w, 422, err.Error())
		return
	}
	m, err := s.saveSkill(tree, fm, u)
	if err != nil {
		detail(w, 422, err.Error())
		return
	}
	k, _ := skillKindOf(r.URL.Query().Get("kind"))
	if k.Key == "" {
		k = skillKinds[0]
	}
	s.skillsOut(w, k, "installed", m.Slug)
}

// POST /api/skills/upload: "file" = SKILL.md or a zip / tar.gz; or several "files" whose file names are the
// paths inside a chosen folder (webkitRelativePath).
func (s *Store) hSkillUpload(w http.ResponseWriter, r *http.Request) {
	if !desktopOnly(s, w) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, skillMaxTotal+1<<20)
	if err := r.ParseMultipartForm(skillMaxTotal + 1<<20); err != nil {
		detail(w, 413, "上传过大（上限 16 MB）或格式不对")
		return
	}
	var files []treeFile
	source := "上传"
	var lim treeLimiter
	if hs := r.MultipartForm.File["files"]; len(hs) > 0 {
		// Go's multipart reader drops directory parts of file names, so the client sends each relative path
		// in a parallel "paths" field (same order)
		paths := r.MultipartForm.Value["paths"]
		for i, h := range hs {
			name := h.Filename
			if i < len(paths) && paths[i] != "" {
				name = paths[i]
			}
			p, ok := cleanRel(name)
			if !ok {
				continue
			}
			if err := lim.add(int(h.Size)); err != nil {
				detail(w, 413, err.Error())
				return
			}
			f, err := h.Open()
			if err != nil {
				continue
			}
			b, _ := io.ReadAll(io.LimitReader(f, skillMaxFile+1))
			f.Close()
			files = append(files, treeFile{p, b})
		}
		if len(hs) > 0 {
			first := hs[0].Filename
			if len(paths) > 0 && paths[0] != "" {
				first = paths[0]
			}
			source = "上传文件夹：" + strings.SplitN(first, "/", 2)[0]
		}
	} else {
		f, h, err := r.FormFile("file")
		if err != nil {
			detail(w, 400, "缺少文件")
			return
		}
		data, _ := io.ReadAll(io.LimitReader(f, skillMaxDownload+1))
		f.Close()
		if len(data) > skillMaxDownload {
			detail(w, 413, "文件过大（上限 8 MB）")
			return
		}
		source = "上传：" + h.Filename
		arch, isArchive, err := readArchive(data)
		if err != nil {
			detail(w, 422, err.Error())
			return
		}
		if isArchive {
			files = arch
		} else {
			files = []treeFile{{"SKILL.md", data}}
		}
	}
	tree, fm, err := buildSkillTree(files, "")
	if err != nil {
		detail(w, 422, err.Error())
		return
	}
	m, err := s.saveSkill(tree, fm, source)
	if err != nil {
		detail(w, 422, err.Error())
		return
	}
	k, _ := skillKindOf(r.URL.Query().Get("kind"))
	if k.Key == "" {
		k = skillKinds[0]
	}
	s.skillsOut(w, k, "installed", m.Slug)
}

func (s *Store) hSkillActive(w http.ResponseWriter, r *http.Request) {
	if !desktopOnly(s, w) {
		return
	}
	body := readBody(r)
	k, ok := reqKind(w, str(body["kind"]))
	if !ok {
		return
	}
	slug := str(body["slug"])
	if slug != "" {
		if _, ok := s.skillFS(slug); !ok || isBuiltinSkill(slug) {
			detail(w, 404, "Skill 不存在")
			return
		}
	}
	s.SetSetting(k.Setting, slug)
	s.skillsOut(w, k)
}

func (s *Store) hSkillDelete(w http.ResponseWriter, r *http.Request) {
	if !desktopOnly(s, w) {
		return
	}
	slug := r.PathValue("name")
	if slug == "" || isBuiltinSkill(slug) || !skillNameRe.MatchString(slug) {
		detail(w, 400, "内置 Skill 不能删除")
		return
	}
	if _, ok := s.skillFS(slug); !ok {
		detail(w, 404, "Skill 不存在")
		return
	}
	os.RemoveAll(filepath.Join(s.skillsDir(), slug))
	for _, k := range skillKinds {
		if a, _ := s.GetSetting(k.Setting); a == slug {
			s.SetSetting(k.Setting, "")
		}
	}
	k, _ := skillKindOf(r.URL.Query().Get("kind"))
	if k.Key == "" {
		k = skillKinds[0]
	}
	s.skillsOut(w, k)
}

func str(v any) string { s, _ := v.(string); return s }
