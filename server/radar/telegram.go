package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Telegram scheduled brief (desktop only): settings hold enable/chat/schedule/section; bot token in secrets.json
// (0600). The in-process scheduler fires when the local Asia/Shanghai clock matches HH:MM; the app must be running.

const (
	tgMaxMsg     = 4000 // under Telegram's 4096; leave room for entities
	tgSettingKey = "telegram"
)

// tgAPIBase is the Bot API origin (var so tests can point at httptest).
// Not user-controlled — never subject to the PublicOnly SSRF gate (see tgSend).
var tgAPIBase = "https://api.telegram.org"

// TelegramPrefs is the non-secret side of the Telegram card (token never appears here).
type TelegramPrefs struct {
	Enabled   bool    `json:"enabled"`
	ChatID    string  `json:"chatId"`
	Schedule  string  `json:"schedule"` // HH:MM Asia/Shanghai
	Section   string  `json:"section"`  // "all" or section key
	HasToken  bool    `json:"hasToken"` // token configured (masked)
	LastAt    float64 `json:"lastAt,omitempty"`
	LastError string  `json:"lastError,omitempty"`
	LastOK    bool    `json:"lastOk,omitempty"`
}

type tgState struct {
	Enabled  bool    `json:"enabled"`
	ChatID   string  `json:"chatId"`
	Schedule string  `json:"schedule"`
	Section  string  `json:"section"`
	LastAt   float64 `json:"lastAt,omitempty"`
	LastErr  string  `json:"lastError,omitempty"`
	LastOK   bool    `json:"lastOk,omitempty"`
	LastFire string  `json:"lastFire,omitempty"` // YYYY-MM-DD HH:MM fired (avoid double-send)
}

var tgHHMM = regexp.MustCompile(`^([01]?\d|2[0-3]):([0-5]?\d)$`)

func (s *Store) loadTG() tgState {
	raw, ok := s.GetSetting(tgSettingKey)
	var st tgState
	if ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &st)
	}
	if st.Schedule == "" {
		st.Schedule = "08:00"
	}
	if st.Section == "" {
		st.Section = "all"
	}
	return st
}

func (s *Store) saveTG(st tgState) error {
	b, _ := json.Marshal(st)
	return s.SetSetting(tgSettingKey, string(b))
}

func (s *Store) telegramToken() string {
	return strings.TrimSpace(s.loadSecrets().TelegramToken)
}

// TelegramPublic: never includes the bot token.
func (s *Store) TelegramPublic() *Row {
	st := s.loadTG()
	r := &Row{}
	r.Set("enabled", st.Enabled)
	r.Set("chatId", st.ChatID)
	r.Set("schedule", st.Schedule)
	r.Set("section", st.Section)
	r.Set("hasToken", s.telegramToken() != "")
	r.Set("desktop", s.Cfg.Desktop)
	if st.LastAt > 0 {
		r.Set("lastAt", st.LastAt)
	}
	if st.LastErr != "" {
		r.Set("lastError", st.LastErr)
	}
	if st.LastOK {
		r.Set("lastOk", true)
	}
	return r
}

func cleanTGToken(v any) (string, bool) {
	k := strings.TrimSpace(jsString(v))
	if k == "" {
		return "", true
	}
	// BotFather tokens look like 123456:AA...
	if len(k) < 20 || len(k) > 200 || !regexp.MustCompile(`^\d{6,}:[A-Za-z0-9_-]+$`).MatchString(k) {
		return "", false
	}
	return k, true
}

func cleanChatID(v any) (string, bool) {
	s := strings.TrimSpace(jsString(v))
	if s == "" {
		return "", true
	}
	if len(s) > 64 {
		return "", false
	}
	// numeric chat id (optionally negative for groups) or @username
	if regexp.MustCompile(`^-?\d{5,20}$`).MatchString(s) {
		return s, true
	}
	if regexp.MustCompile(`^@[A-Za-z][A-Za-z0-9_]{3,31}$`).MatchString(s) {
		return s, true
	}
	return "", false
}

func cleanSchedule(v any) (string, bool) {
	s := strings.TrimSpace(jsString(v))
	if s == "" {
		return "08:00", true
	}
	m := tgHHMM.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	return fmt.Sprintf("%02d:%02d", h, mi), true
}

// SetTelegramPrefs updates prefs + optional token in secrets. Token is never echoed back.
func (s *Store) SetTelegramPrefs(p map[string]any) error {
	st := s.loadTG()
	if v, ok := p["enabled"].(bool); ok {
		st.Enabled = v
	}
	if v, has := p["chatId"]; has {
		c, ok := cleanChatID(v)
		if !ok {
			return &httpError{400, "Chat ID 格式不正确（数字或 @username）"}
		}
		st.ChatID = c
	}
	if v, has := p["schedule"]; has {
		sc, ok := cleanSchedule(v)
		if !ok {
			return &httpError{400, "时间格式为 HH:MM（北京时间）"}
		}
		st.Schedule = sc
	}
	if v, has := p["section"]; has {
		sec := strings.TrimSpace(jsString(v))
		if sec == "" {
			sec = "all"
		}
		if sec != "all" && !isSectionKey(s.DB, sec) {
			return &httpError{404, "分区不存在"}
		}
		st.Section = sec
	}
	if err := s.saveTG(st); err != nil {
		return err
	}
	sec := s.loadSecrets()
	changed := false
	if p["clearToken"] == true {
		sec.TelegramToken = ""
		changed = true
	}
	if v, has := p["token"]; has && jsString(v) != "" {
		tok, ok := cleanTGToken(v)
		if !ok {
			return &httpError{400, "Bot Token 格式不正确"}
		}
		sec.TelegramToken = tok
		changed = true
	}
	if changed {
		if err := s.saveSecrets(sec); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) recordTG(ok bool, errMsg string) {
	st := s.loadTG()
	st.LastAt = now()
	st.LastOK = ok
	st.LastErr = errMsg
	_ = s.saveTG(st)
}

// briefHTML formats a rule brief for Telegram HTML parse_mode.
func briefHTML(b *briefOut) string {
	var w strings.Builder
	fmt.Fprintf(&w, "<b>%s</b>\n", html.EscapeString(b.Title))
	fmt.Fprintf(&w, "<i>共 %d 条 · %d 个来源 · 近 24 小时</i>\n\n", b.Count, b.Sources)
	if b.Count == 0 {
		w.WriteString("近 24 小时暂无内容。")
		return w.String()
	}
	n := 0
	for _, g := range b.Groups {
		tag := ""
		if g.Priority == "P0" {
			tag = " · P0"
		}
		fmt.Fprintf(&w, "<b>%s%s</b>\n\n", html.EscapeString(g.Source), tag)
		for _, it := range g.Items {
			n++
			title := html.EscapeString(it.Title)
			if it.Title == "" {
				title = "(无标题)"
			}
			sum := html.EscapeString(it.Summary)
			link := html.EscapeString(it.URL)
			if sum != "" {
				fmt.Fprintf(&w, "<b>%d. %s</b>\n%s\n<a href=\"%s\">原文</a>\n\n", n, title, sum, link)
			} else {
				fmt.Fprintf(&w, "<b>%d. %s</b>\n<a href=\"%s\">原文</a>\n\n", n, title, link)
			}
		}
	}
	return strings.TrimSpace(w.String())
}

func splitTG(s string, max int) []string {
	if utf8.RuneCountInString(s) <= max {
		return []string{s}
	}
	var out []string
	paras := strings.Split(s, "\n\n")
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	for _, p := range paras {
		cand := p
		if cur.Len() > 0 {
			cand = cur.String() + "\n\n" + p
		}
		if utf8.RuneCountInString(cand) <= max {
			if cur.Len() > 0 {
				cur.WriteString("\n\n")
			}
			cur.WriteString(p)
			continue
		}
		flush()
		if utf8.RuneCountInString(p) <= max {
			cur.WriteString(p)
			continue
		}
		// hard split by runes
		r := []rune(p)
		for len(r) > 0 {
			n := max
			if n > len(r) {
				n = len(r)
			}
			out = append(out, string(r[:n]))
			r = r[n:]
		}
	}
	flush()
	return out
}

func (s *Store) tgSend(chat, text string) error {
	tok := s.telegramToken()
	if tok == "" {
		return fmt.Errorf("未配置 Bot Token")
	}
	if chat == "" {
		return fmt.Errorf("未配置 Chat ID")
	}
	parts := splitTG(text, tgMaxMsg)
	// Fixed Bot API host — not a user/content URL. PublicOnly would false-positive under
	// Clash/Surge fake-ip DNS (api.telegram.org → 198.18.0.0/15, blocked by blockedIP).
	ctx := context.Background()
	for i, part := range parts {
		form := url.Values{}
		form.Set("chat_id", chat)
		form.Set("text", part)
		form.Set("parse_mode", "HTML")
		form.Set("disable_web_page_preview", "true")
		api := fmt.Sprintf("%s/bot%s/sendMessage", tgAPIBase, tok)
		resp, err := netFetch(ctx, "POST", api, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, strings.NewReader(form.Encode()), 30*time.Second)
		if err != nil {
			return fmt.Errorf("发送失败：%s", strings.ReplaceAll(err.Error(), tok, "***"))
		}
		if !resp.OK() {
			body := string(resp.Body)
			body = strings.ReplaceAll(body, tok, "***")
			if len(body) > 200 {
				body = body[:200]
			}
			return fmt.Errorf("Telegram HTTP %d: %s", resp.Status, body)
		}
		if i+1 < len(parts) {
			time.Sleep(350 * time.Millisecond)
		}
	}
	return nil
}

// RunTelegramBrief builds the auto brief for the configured section and sends it.
func (s *Store) RunTelegramBrief() error {
	if !s.Cfg.Desktop {
		return fmt.Errorf("仅桌面版可用")
	}
	st := s.loadTG()
	if s.telegramToken() == "" || st.ChatID == "" {
		return fmt.Errorf("请先填写 Bot Token 与 Chat ID")
	}
	sec := st.Section
	if sec == "" {
		sec = "all"
	}
	out, err := s.getBrief(sec, false, true)
	if err != nil {
		s.recordTG(false, err.Error())
		return err
	}
	htmlBody := briefHTML(out)
	if err := s.tgSend(st.ChatID, htmlBody); err != nil {
		s.recordTG(false, err.Error())
		return err
	}
	s.recordTG(true, "")
	return nil
}

// startTelegramScheduler: desktop only; ticks every 20s, fires once per HH:MM match (Asia/Shanghai).
func (s *Store) startTelegramScheduler(done <-chan struct{}) {
	if !s.Cfg.Desktop {
		return
	}
	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		var running sync.Mutex
		for {
			select {
			case <-done:
				return
			case <-t.C:
				st := s.loadTG()
				if !st.Enabled || s.telegramToken() == "" || st.ChatID == "" {
					continue
				}
				nowSH := time.Now().In(cst)
				want := st.Schedule
				cur := nowSH.Format("15:04")
				fireKey := nowSH.Format("2006-01-02") + " " + want
				if cur != want || st.LastFire == fireKey {
					continue
				}
				if !running.TryLock() {
					continue
				}
				st.LastFire = fireKey
				_ = s.saveTG(st)
				go func() {
					defer running.Unlock()
					log.Printf("[telegram] scheduled brief section=%s", st.Section)
					if err := s.RunTelegramBrief(); err != nil {
						log.Printf("[telegram] send failed: %v", err)
					} else {
						log.Printf("[telegram] sent ok")
					}
				}()
			}
		}
	}()
}

func (s *Store) hTelegramTest(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.Desktop {
		detail(w, 404, "Telegram 推送仅桌面版可用")
		return
	}
	if SettingsLocked() {
		detail(w, 403, "设置已锁定（SETTINGS_LOCKED=1）")
		return
	}
	if err := s.RunTelegramBrief(); err != nil {
		if he, ok := err.(*httpError); ok {
			detail(w, he.code, he.msg)
			return
		}
		// Always include telegram so the UI can refresh lastError / lastOk and stay retryable.
		writeJSON(w, 200, obj("ok", false, "error", err.Error(), "telegram", s.TelegramPublic()))
		return
	}
	writeJSON(w, 200, obj("ok", true, "telegram", s.TelegramPublic()))
}

// redact helpers used by tests
func tgTokenRedacted(body string, tok string) bool {
	return tok == "" || !strings.Contains(body, tok)
}
