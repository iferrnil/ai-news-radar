package core

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ---- secrets.json (0600, next to the DB; same format as src/lib/secrets.ts)
type Secrets struct {
	DeepseekKey     string `json:"deepseekKey,omitempty"`
	DeepseekBaseURL string `json:"deepseekBaseUrl,omitempty"`
	DeepseekModel   string `json:"deepseekModel,omitempty"`
}

func (s *Store) secretsPath() string { return filepath.Join(s.Dir, "secrets.json") }

func (s *Store) Secrets() Secrets {
	var x Secrets
	if b, err := os.ReadFile(s.secretsPath()); err == nil {
		json.Unmarshal(b, &x)
	}
	return x
}

var keyRe = regexp.MustCompile(`^[\x21-\x7e]{16,200}$`)

// SaveSecrets writes atomically with mode 0600. An empty key keeps the old one unless clearKey.
func (s *Store) SaveSecrets(key string, clearKey bool, base, model string) error {
	cur := s.Secrets()
	if clearKey {
		cur.DeepseekKey = ""
	}
	if k := strings.TrimSpace(key); k != "" {
		if !keyRe.MatchString(k) {
			return errors.New("API key 格式不正确")
		}
		cur.DeepseekKey = k
	}
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if b != "" && !strings.HasPrefix(b, "https://") && !strings.HasPrefix(b, "http://localhost") && !strings.HasPrefix(b, "http://127.0.0.1") {
		return errors.New("Base URL 必须是 https")
	}
	cur.DeepseekBaseURL = strings.TrimSuffix(b, "/chat/completions")
	cur.DeepseekModel = strings.TrimSpace(model)
	data, _ := json.Marshal(cur)
	tmp := s.secretsPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	os.Chmod(tmp, 0o600)
	if err := os.Rename(tmp, s.secretsPath()); err != nil {
		return err
	}
	s.translating.resetAuth()
	return nil
}

func (x Secrets) base() string {
	if x.DeepseekBaseURL != "" {
		return x.DeepseekBaseURL
	}
	return "https://api.deepseek.com"
}
func (x Secrets) model() string {
	if x.DeepseekModel != "" {
		return x.DeepseekModel
	}
	return "deepseek-chat"
}

// TestDeepseek does GET /models with the stored key.
func (s *Store) TestDeepseek(ctx context.Context) error {
	x := s.Secrets()
	if x.DeepseekKey == "" {
		return errors.New("未配置 API key")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", x.base()+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+x.DeepseekKey)
	r, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败：%s", strings.ReplaceAll(err.Error(), x.DeepseekKey, "***"))
	}
	r.Body.Close()
	switch r.StatusCode {
	case 200:
		return nil
	case 401:
		return errors.New("API key 无效（401）")
	case 402:
		return errors.New("余额不足（402）")
	}
	return fmt.Errorf("HTTP %d", r.StatusCode)
}

// ---- translation (off by default on desktop: settings "translate" must be "1" and a key must exist)
type translator struct {
	s          *Store
	mu         sync.Mutex
	queue      []int64
	queued     map[int64]bool
	running    int
	authFailed bool
	LastError  string
	Done       int
}

func newTranslator(s *Store) *translator { return &translator{s: s, queued: map[int64]bool{}} }
func (t *translator) resetAuth()         { t.mu.Lock(); t.authFailed = false; t.LastError = ""; t.mu.Unlock() }

func (s *Store) TranslateEnabled() bool {
	if s.Secrets().DeepseekKey == "" {
		return false
	}
	s.translating.mu.Lock()
	af := s.translating.authFailed
	s.translating.mu.Unlock()
	v, _ := s.GetSetting("translate")
	return !af && v == "1"
}

func (s *Store) TranslateStatus() (done int, queued int, lastErr string) {
	t := s.translating
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Done, len(t.queue), t.LastError
}

var (
	cjkRe   = regexp.MustCompile(`[\x{3400}-\x{9fff}\x{3040}-\x{30ff}\x{ac00}-\x{d7af}]`)
	latinRe = regexp.MustCompile(`[A-Za-z\x{00C0}-\x{024F}\x{0400}-\x{04FF}]`)
)

// NeedsTranslation: mostly non-CJK text (latin outnumbers CJK 2:1), same heuristic as translate.ts.
func NeedsTranslation(title, summary string) bool {
	if len(cjkRe.FindAllStringIndex(title, -1)) >= 4 {
		return false
	}
	x := Trunc(title+" "+summary, 500, "")
	l, c := len(latinRe.FindAllStringIndex(x, -1)), len(cjkRe.FindAllStringIndex(x, -1))
	return l > 8 && l > c*2
}

func hashOf(t, s string) string {
	h := sha1.Sum([]byte(t + "\x00" + s))
	return base64.RawURLEncoding.EncodeToString(h[:])[:22]
}

func (s *Store) EnqueueTranslation(ids []int64) {
	if len(ids) == 0 || !s.TranslateEnabled() {
		return
	}
	t := s.translating
	t.mu.Lock()
	for _, id := range ids {
		if !t.queued[id] {
			t.queued[id] = true
			t.queue = append(t.queue, id)
		}
	}
	t.mu.Unlock()
	t.pump()
}

// BackfillTranslations queues every retained item lacking a translation.
func (s *Store) BackfillTranslations() {
	if !s.TranslateEnabled() {
		return
	}
	rows, err := s.DB.Query("SELECT id FROM items WHERE title_zh IS NULL AND merged_into IS NULL ORDER BY published DESC")
	if err != nil {
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	s.EnqueueTranslation(ids)
}

const trSystem = `把每项的 t(标题) 和 s(摘要) 译成简体中文；人名、产品名、代码、链接、@账号保持原样；不要解释。只输出 JSON：{"r":[{"i":序号,"t":"…","s":"…"}]}`

func (t *translator) pump() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for t.running < 2 && len(t.queue) > 0 && !t.authFailed {
		n := min(10, len(t.queue))
		ids := append([]int64(nil), t.queue[:n]...)
		t.queue = t.queue[n:]
		t.running++
		go func() {
			err := t.run(ids)
			t.mu.Lock()
			t.running--
			for _, i := range ids {
				delete(t.queued, i)
			}
			if err != nil {
				t.LastError = err.Error()
			}
			t.mu.Unlock()
			t.pump()
		}()
	}
}

type trIn struct {
	I int    `json:"i"`
	T string `json:"t"`
	S string `json:"s"`
}

func (t *translator) run(ids []int64) error {
	s := t.s
	rows, err := s.DB.Query("SELECT id, COALESCE(title,''), COALESCE(summary,'') FROM items WHERE title_zh IS NULL AND id IN ("+placeholders(len(ids))+")", anys(ids)...)
	if err != nil {
		return err
	}
	type todoT struct {
		id      int64
		h, t, s string
	}
	var todo []todoT
	var all []todoT
	for rows.Next() {
		var x todoT
		rows.Scan(&x.id, &x.t, &x.s)
		x.s = Trunc(x.s, 1500, "")
		all = append(all, x)
	}
	rows.Close()
	changed := false
	for _, x := range all {
		if !NeedsTranslation(x.t, x.s) {
			s.DB.Exec("UPDATE items SET title_zh='', summary_zh='' WHERE id=?", x.id)
			continue
		}
		x.h = hashOf(x.t, x.s)
		var ct, cs string
		if s.DB.QueryRow("SELECT t, s FROM tcache WHERE h=?", x.h).Scan(&ct, &cs) == nil {
			s.DB.Exec("UPDATE items SET title_zh=?, summary_zh=? WHERE id=?", ct, cs, x.id)
			changed = true
			continue
		}
		todo = append(todo, x)
	}
	if len(todo) > 0 {
		in := make([]trIn, len(todo))
		for i, x := range todo {
			in[i] = trIn{i, x.t, x.s}
		}
		out, err := t.call(in)
		if err != nil {
			return err
		}
		for _, o := range out {
			if o.I < 0 || o.I >= len(todo) || o.T == "" {
				continue
			}
			x := todo[o.I]
			sm := ""
			if x.s != "" {
				sm = o.S
			}
			s.DB.Exec("UPDATE items SET title_zh=?, summary_zh=? WHERE id=?", Trunc(o.T, 400, ""), Trunc(sm, 3000, ""), x.id)
			s.DB.Exec("INSERT OR REPLACE INTO tcache(h,t,s,ts) VALUES(?,?,?,?)", x.h, Trunc(o.T, 400, ""), sm, now())
			t.mu.Lock()
			t.Done++
			t.mu.Unlock()
			changed = true
		}
	}
	if changed {
		s.emit(Event{Kind: "translate"})
	}
	return nil
}

func (t *translator) call(batch []trIn) ([]trIn, error) {
	x := t.s.Secrets()
	payload, _ := json.Marshal(batch)
	body, _ := json.Marshal(map[string]any{"model": x.model(), "temperature": 0.2, "response_format": map[string]string{"type": "json_object"}, "max_tokens": 6000,
		"messages": []map[string]string{{"role": "system", "content": trSystem}, {"role": "user", "content": string(payload)}}})
	for attempt := 0; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "POST", x.base()+"/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+x.DeepseekKey)
		r, err := (&http.Client{}).Do(req)
		if err != nil {
			cancel()
			if attempt < 3 {
				time.Sleep(time.Duration(2000<<attempt) * time.Millisecond)
				continue
			}
			return nil, err
		}
		rb, _ := io.ReadAll(r.Body)
		r.Body.Close()
		cancel()
		if r.StatusCode == 401 || r.StatusCode == 402 {
			if r.StatusCode == 401 {
				t.mu.Lock()
				t.authFailed = true
				t.mu.Unlock()
				return nil, errors.New("DeepSeek HTTP 401（API key 无效）")
			}
			return nil, errors.New("DeepSeek HTTP 402（余额不足）")
		}
		if (r.StatusCode == 429 || r.StatusCode >= 500) && attempt < 3 {
			time.Sleep(time.Duration(2000<<attempt) * time.Millisecond)
			continue
		}
		if r.StatusCode != 200 {
			return nil, fmt.Errorf("DeepSeek HTTP %d: %s", r.StatusCode, Trunc(string(rb), 200, ""))
		}
		var j struct {
			Choices []struct {
				Message struct{ Content string } `json:"message"`
			} `json:"choices"`
		}
		json.Unmarshal(rb, &j)
		if len(j.Choices) == 0 {
			return nil, errors.New("DeepSeek 返回为空")
		}
		var res struct{ R []trIn }
		if err := json.Unmarshal([]byte(strings.TrimSpace(j.Choices[0].Message.Content)), &res); err != nil {
			if attempt < 1 {
				continue
			}
			return nil, errors.New("DeepSeek 返回的不是合法 JSON")
		}
		return res.R, nil
	}
}

func anys(ids []int64) []any {
	out := make([]any, len(ids))
	for i, v := range ids {
		out[i] = v
	}
	return out
}
