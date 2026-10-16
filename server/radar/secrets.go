package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Runtime DeepSeek configuration (secrets.ts). Precedence: in-app setting (0600 JSON next to the DB) > env vars.
// The key never leaves the server; SETTINGS_LOCKED=1 makes the in-app setting read-only.
const (
	DefaultBase  = "https://api.deepseek.com"
	DefaultModel = "deepseek-chat"
)

type Stored struct {
	DeepseekKey     string `json:"deepseekKey,omitempty"`
	DeepseekBaseURL string `json:"deepseekBaseUrl,omitempty"`
	DeepseekModel   string `json:"deepseekModel,omitempty"`
	// 多模态模型 (desktop: covers / videos / vision); see multimodal.go
	MM MMConfig `json:"multimodal,omitempty"`
}

func (s *Store) loadSecrets() Stored {
	s.secMu.Lock()
	defer s.secMu.Unlock()
	if s.sec == nil {
		var st Stored
		if b, err := os.ReadFile(s.Cfg.SecretsPath); err == nil {
			json.Unmarshal(b, &st)
		}
		s.sec = &st
	}
	return *s.sec
}

func (s *Store) saveSecrets(st Stored) error {
	os.MkdirAll(filepath.Dir(s.Cfg.SecretsPath), 0o755)
	tmp := fmt.Sprintf("%s.%d.tmp", s.Cfg.SecretsPath, os.Getpid())
	b, _ := json.Marshal(st)
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	os.Chmod(tmp, 0o600)
	if err := os.Rename(tmp, s.Cfg.SecretsPath); err != nil {
		return err
	}
	s.secMu.Lock()
	s.sec = &st
	s.secMu.Unlock()
	return nil
}

func SettingsLocked() bool { return os.Getenv("SETTINGS_LOCKED") == "1" }

func (s *Store) deepseekKey() string {
	if k := s.loadSecrets().DeepseekKey; k != "" {
		return k
	}
	return os.Getenv("DEEPSEEK_API_KEY")
}

func (s *Store) keySource() any {
	if s.loadSecrets().DeepseekKey != "" {
		return "app"
	}
	if os.Getenv("DEEPSEEK_API_KEY") != "" {
		return "env"
	}
	return nil
}

var trailSlash = regexp.MustCompile(`/+$`)

func stripBase(b string) string {
	return strings.TrimSuffix(trailSlash.ReplaceAllString(b, ""), "/chat/completions")
}

func (s *Store) deepseekBase() string {
	b := s.loadSecrets().DeepseekBaseURL
	if b == "" {
		b = os.Getenv("DEEPSEEK_BASE_URL")
	}
	if b == "" {
		b = DefaultBase
	}
	return stripBase(b)
}

func (s *Store) deepseekModel() string {
	if m := s.loadSecrets().DeepseekModel; m != "" {
		return m
	}
	if m := os.Getenv("DEEPSEEK_MODEL"); m != "" {
		return m
	}
	return DefaultModel
}

// deepseekPublic: no secret material at all.
func (s *Store) deepseekPublic() *Row {
	r := &Row{}
	st := s.loadSecrets()
	r.Set("configured", s.deepseekKey() != "")
	r.Set("source", s.keySource())
	r.Set("baseUrl", s.deepseekBase())
	r.Set("model", s.deepseekModel())
	r.Set("baseUrlCustom", st.DeepseekBaseURL != "")
	r.Set("modelCustom", st.DeepseekModel != "")
	r.Set("locked", SettingsLocked())
	return r
}

var keyRe = regexp.MustCompile(`^[\x21-\x7e]{16,200}$`)

func cleanKey(v any) (string, bool) {
	k := strings.TrimSpace(jsString(v))
	return k, keyRe.MatchString(k)
}

// cleanBase returns ("", true) for empty, (url, true) when valid, ("", false) when invalid.
func cleanBase(v any) (string, bool) {
	str := strings.TrimSpace(jsString(v))
	if str == "" {
		return "", true
	}
	u, err := url.Parse(str)
	if err != nil || u.Host == "" {
		return "", false
	}
	h := u.Hostname()
	local := h == "localhost" || h == "127.0.0.1" || h == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", false
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Contains(str, "?") || strings.Contains(str, "#") {
		return "", false
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Host = strings.ToLower(u.Host)
	return stripBase(u.String()), true
}

var modelRe = regexp.MustCompile(`^[\w.:\-/]{1,64}$`)

func cleanModel(v any) (string, bool) {
	str := strings.TrimSpace(jsString(v))
	if str == "" {
		return "", true
	}
	return str, modelRe.MatchString(str)
}

type dsUpdate struct {
	Key            string
	ClearKey       bool
	BaseURL, Model *string
}

func (s *Store) updateDeepseek(p dsUpdate) error {
	st := s.loadSecrets()
	if p.ClearKey {
		st.DeepseekKey = ""
	}
	if p.Key != "" {
		st.DeepseekKey = p.Key
	}
	if p.BaseURL != nil {
		st.DeepseekBaseURL = *p.BaseURL
	}
	if p.Model != nil {
		st.DeepseekModel = *p.Model
	}
	return s.saveSecrets(st)
}

// testDeepseek: cheap authenticated call (GET /models); only the status is reported.
func (s *Store) testDeepseek(key, base string) *Row {
	if key == "" {
		key = s.deepseekKey()
	}
	if base == "" {
		base = s.deepseekBase()
	}
	r := &Row{}
	if key == "" {
		r.Set("ok", false)
		r.Set("error", "未配置 API key")
		return r
	}
	resp, err := netFetch(context.Background(), "GET", base+"/models", map[string]string{"Authorization": "Bearer " + key}, nil, 15*time.Second)
	if err != nil {
		r.Set("ok", false)
		msg := err.Error()
		if je, ok := err.(*JSError); ok && je.Name == "TimeoutError" {
			msg = "超时"
		}
		r.Set("error", "连接失败："+strings.ReplaceAll(msg, key, "***"))
		return r
	}
	if resp.OK() {
		r.Set("ok", true)
		r.Set("status", resp.Status)
		return r
	}
	r.Set("ok", false)
	r.Set("status", resp.Status)
	switch resp.Status {
	case 401:
		r.Set("error", "API key 无效（401）")
	case 402:
		r.Set("error", "余额不足（402）")
	default:
		r.Set("error", fmt.Sprintf("HTTP %d", resp.Status))
	}
	return r
}
