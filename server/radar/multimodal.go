package radar

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// 多模态模型 (desktop only): one OpenAI-compatible provider for cover images, cover videos and vision input.
//   image:  POST {base}/images/generations {model, prompt, n, size} → data[0].b64_json | data[0].url
//   video:  POST {base}{videoSubmit} {model, prompt[, size]} → a URL right away, or a task id that is polled with
//           GET {base}{videoPoll with {id}} until a URL appears (or a failed status / the timeout)
//   vision: chat/completions on the same provider with image_url content parts (VisionModel)
// The key lives in secrets.json (0600) like the DeepSeek key and is never sent to the UI.

type MMConfig struct {
	Provider    string `json:"provider,omitempty"` // label only ("openai-compatible")
	BaseURL     string `json:"baseUrl,omitempty"`
	Key         string `json:"key,omitempty"`
	ImageModel  string `json:"imageModel,omitempty"`
	VideoModel  string `json:"videoModel,omitempty"`
	VisionModel string `json:"visionModel,omitempty"`
	VideoSubmit string `json:"videoSubmit,omitempty"` // path, default /videos/generations
	VideoPoll   string `json:"videoPoll,omitempty"`   // path with {id}, default /videos/generations/{id}
}

const (
	mmVideoSubmit = "/videos/generations"
	mmVideoPoll   = "/videos/generations/{id}"
	mmMediaMax    = 512 << 20
)

func (c MMConfig) can(kind string) bool {
	if c.BaseURL == "" || c.Key == "" {
		return false
	}
	switch kind {
	case "image":
		return c.ImageModel != ""
	case "video":
		return c.VideoModel != ""
	case "vision":
		return c.VisionModel != ""
	}
	return false
}

func (s *Store) mmConfig() MMConfig { return s.loadSecrets().MM }

func (s *Store) mmPublic() *Row {
	c := s.mmConfig()
	return obj("provider", firstNonEmpty(c.Provider, "openai-compatible"), "baseUrl", c.BaseURL, "configured", c.Key != "",
		"imageModel", c.ImageModel, "videoModel", c.VideoModel, "visionModel", c.VisionModel,
		"videoSubmit", firstNonEmpty(c.VideoSubmit, mmVideoSubmit), "videoPoll", firstNonEmpty(c.VideoPoll, mmVideoPoll),
		"image", c.can("image"), "video", c.can("video"), "vision", c.can("vision"), "locked", SettingsLocked())
}

var mmPathRe = regexp.MustCompile(`^/[\w./{}\-]{0,120}$`)

// GET/PATCH /api/multimodal (desktop only)
func (s *Store) hMMGet(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "多模态模型仅桌面版可用") {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, s.mmPublic())
}

func (s *Store) hMMPatch(w http.ResponseWriter, r *http.Request) {
	if !desktopOnlyMsg(s, w, "多模态模型仅桌面版可用") {
		return
	}
	if SettingsLocked() {
		detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1）")
		return
	}
	b := readBody(r)
	st := s.loadSecrets()
	c := st.MM
	if v, ok := b["baseUrl"]; ok {
		u, ok := cleanBase(v)
		if !ok {
			detail(w, 400, "Base URL 无效（需 https://）")
			return
		}
		c.BaseURL = u
	}
	for _, f := range []struct {
		k string
		p *string
	}{{"imageModel", &c.ImageModel}, {"videoModel", &c.VideoModel}, {"visionModel", &c.VisionModel}, {"provider", &c.Provider}} {
		if v, ok := b[f.k]; ok {
			m, ok := cleanModel(v)
			if !ok {
				detail(w, 400, f.k+" 无效")
				return
			}
			*f.p = m
		}
	}
	for _, f := range []struct {
		k string
		p *string
	}{{"videoSubmit", &c.VideoSubmit}, {"videoPoll", &c.VideoPoll}} {
		if v, ok := b[f.k]; ok {
			x := strings.TrimSpace(jsString(v))
			if x != "" && !mmPathRe.MatchString(x) {
				detail(w, 400, f.k+" 应是以 / 开头的路径")
				return
			}
			*f.p = x
		}
	}
	if b["clearKey"] == true {
		c.Key = ""
	}
	if v, ok := b["key"]; ok && strings.TrimSpace(jsString(v)) != "" {
		k, ok := cleanKey(v)
		if !ok {
			detail(w, 400, "API key 格式不对")
			return
		}
		c.Key = k
	}
	st.MM = c
	if err := s.saveSecrets(st); err != nil {
		detail(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, s.mmPublic())
}

func desktopOnlyMsg(s *Store, w http.ResponseWriter, msg string) bool {
	if !s.Cfg.Desktop {
		detail(w, 404, msg)
		return false
	}
	return true
}

func (c MMConfig) post(path string, body any, timeout time.Duration) (map[string]any, error) {
	b, _ := json.Marshal(body)
	r, err := netFetch(context.Background(), "POST", strings.TrimRight(c.BaseURL, "/")+path,
		map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + c.Key}, strings.NewReader(string(b)), timeout)
	return c.decode(r, err)
}

func (c MMConfig) get(path string) (map[string]any, error) {
	r, err := netFetch(context.Background(), "GET", strings.TrimRight(c.BaseURL, "/")+path, map[string]string{"Authorization": "Bearer " + c.Key}, nil, 30*time.Second)
	return c.decode(r, err)
}

func (c MMConfig) decode(r *Resp, err error) (map[string]any, error) {
	if err != nil {
		return nil, fmt.Errorf("连接失败：%s", strings.ReplaceAll(err.Error(), c.Key, "***"))
	}
	if !r.OK() {
		return nil, fmt.Errorf("HTTP %d: %s", r.Status, strings.ReplaceAll(jsSlice(string(r.Body), 200), c.Key, "***"))
	}
	var j map[string]any
	if json.Unmarshal(r.Body, &j) != nil {
		return nil, fmt.Errorf("返回不是 JSON")
	}
	return j, nil
}

// dig finds the first string at one of the dotted paths ("data.0.url").
func dig(j any, paths ...string) string {
	for _, p := range paths {
		cur := j
		for _, k := range strings.Split(p, ".") {
			switch v := cur.(type) {
			case map[string]any:
				cur = v[k]
			case []any:
				if k == "0" && len(v) > 0 {
					cur = v[0]
				} else {
					cur = nil
				}
			default:
				cur = nil
			}
		}
		if s, ok := cur.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// mmImage generates one image and writes it to dst (extension added from the content). Returns the path.
func (c MMConfig) mmImage(prompt, size, dst string) (string, error) {
	if size == "" {
		size = "1024x1024"
	}
	j, err := c.post("/images/generations", map[string]any{"model": c.ImageModel, "prompt": prompt, "n": 1, "size": size}, 180*time.Second)
	if err != nil {
		return "", err
	}
	if b64 := dig(j, "data.0.b64_json", "data.0.b64"); b64 != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return "", fmt.Errorf("图片数据无效")
		}
		p := dst + mediaExt(http.DetectContentType(data), ".png")
		os.MkdirAll(filepath.Dir(p), 0o755)
		return p, os.WriteFile(p, data, 0o644)
	}
	if u := dig(j, "data.0.url", "url"); u != "" {
		return mediaDownload(u, dst, ".png")
	}
	return "", fmt.Errorf("返回里没有图片")
}

var (
	mmPollEvery   = 5 * time.Second
	mmPollTimeout = 10 * time.Minute
)

// mmVideo: submit a job, poll its task id until a video URL appears, download it to dst.
func (c MMConfig) mmVideo(prompt, size, dst string) (string, error) {
	body := map[string]any{"model": c.VideoModel, "prompt": prompt}
	if size != "" {
		body["size"] = size
	}
	j, err := c.post(firstNonEmpty(c.VideoSubmit, mmVideoSubmit), body, 120*time.Second)
	if err != nil {
		return "", err
	}
	urlKeys := []string{"video_url", "url", "data.0.url", "output.video_url", "output.url", "result.video_url", "result.url", "content.video_url"}
	if u := dig(j, urlKeys...); u != "" {
		return mediaDownload(u, dst, ".mp4")
	}
	id := dig(j, "id", "task_id", "data.task_id", "output.task_id", "data.0.id")
	if id == "" {
		return "", fmt.Errorf("返回里没有视频地址或任务 id")
	}
	poll := strings.ReplaceAll(firstNonEmpty(c.VideoPoll, mmVideoPoll), "{id}", url.PathEscape(id))
	deadline := time.Now().Add(mmPollTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(mmPollEvery)
		j, err := c.get(poll)
		if err != nil {
			return "", err
		}
		st := strings.ToLower(dig(j, "status", "task_status", "data.status", "output.task_status", "state"))
		if u := dig(j, urlKeys...); u != "" && (st == "" || strings.Contains("succeeded success completed done finished", st)) {
			return mediaDownload(u, dst, ".mp4")
		}
		if strings.Contains("failed failure error cancelled canceled expired", st) && st != "" {
			return "", fmt.Errorf("视频任务失败（%s）", st)
		}
	}
	return "", fmt.Errorf("视频任务超时")
}

func mediaExt(ct, def string) string {
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"):
		return ".jpg"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "mp4"):
		return ".mp4"
	case strings.Contains(ct, "webm"):
		return ".webm"
	case strings.Contains(ct, "quicktime"):
		return ".mov"
	}
	return def
}

// mediaDownload streams a generated file to dst+ext (no 8 MB cap; UPSTREAM_MOCK is honoured like netFetch).
func mediaDownload(u, dst, def string) (string, error) {
	target := u
	if mock := os.Getenv("UPSTREAM_MOCK"); mock != "" {
		target = strings.TrimRight(mock, "/") + "/?u=" + url.QueryEscape(u)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return "", fmt.Errorf("下载地址无效")
	}
	r, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载失败：%v", err)
	}
	defer r.Body.Close()
	if r.StatusCode/100 != 2 {
		return "", fmt.Errorf("下载失败：HTTP %d", r.StatusCode)
	}
	p := dst + mediaExt(r.Header.Get("Content-Type"), def)
	os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	n, err := io.Copy(f, io.LimitReader(r.Body, mmMediaMax+1))
	f.Close()
	if err == nil && n > mmMediaMax {
		err = fmt.Errorf("文件过大")
	}
	if err != nil {
		os.Remove(p)
		return "", err
	}
	return p, nil
}

// mmVisionChat: one chat call with image_url parts on the multimodal provider.
func (c MMConfig) mmVisionChat(sys, user string, images []string, temp float64, maxTok int) (string, error) {
	parts := []map[string]any{{"type": "text", "text": user}}
	for _, u := range images {
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]string{"url": u}})
	}
	j, err := c.post("/chat/completions", map[string]any{"model": c.VisionModel, "temperature": temp, "max_tokens": maxTok,
		"messages": []map[string]any{{"role": "system", "content": sys}, {"role": "user", "content": parts}}}, 180*time.Second)
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(dig(j, "choices.0.message.content"))
	if out == "" {
		return "", fmt.Errorf("多模态模型返回为空")
	}
	return out, nil
}
