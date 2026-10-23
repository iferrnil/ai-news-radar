package radar

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelegramPublicNeverLeaksToken(t *testing.T) {
	s := testStore(t)
	s.Cfg.Desktop = true
	sec := s.loadSecrets()
	sec.TelegramToken = "999000111:BBBfakeTokenForTestsOnly_xxYYzz_0123456789"
	if err := s.saveSecrets(sec); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTelegramPrefs(map[string]any{
		"enabled": true, "chatId": "@mychannel", "schedule": "9:05", "section": "all",
	}); err != nil {
		t.Fatal(err)
	}
	pub := s.TelegramPublic()
	b, _ := json.Marshal(pub)
	if strings.Contains(string(b), "BBBfake") || strings.Contains(string(b), "TelegramToken") || strings.Contains(string(b), "telegramToken") {
		t.Fatalf("token leaked in public: %s", b)
	}
	if pub.Get("hasToken") != true || pub.Str("schedule") != "09:05" || pub.Str("chatId") != "@mychannel" {
		t.Fatalf("public fields: %s", b)
	}

	// GET /api/settings
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/settings", nil)
	s.hSettingsGet(rr, req)
	body := rr.Body.String()
	if strings.Contains(body, "BBBfake") || strings.Contains(body, "123456789:AA") {
		t.Fatalf("token in settings GET: %s", body)
	}
	if !strings.Contains(body, `"hasToken":true`) {
		t.Fatalf("missing hasToken: %s", body)
	}
}

func TestTelegramWeb404(t *testing.T) {
	s := testStore(t)
	s.Cfg.Desktop = false
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/settings", strings.NewReader(`{"telegram":{"enabled":true}}`))
	req.Header.Set("Content-Type", "application/json")
	s.hSettingsPatch(rr, req)
	if rr.Code != 404 {
		t.Fatalf("web patch telegram: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	s.hTelegramTest(rr, httptest.NewRequest("POST", "/api/settings/telegram-test", nil))
	if rr.Code != 404 {
		t.Fatalf("web test: %d", rr.Code)
	}
}

func TestBriefHTMLAndSplit(t *testing.T) {
	b := &briefOut{Title: "「实验室」简报 · 10/23 08:00", Count: 2, Sources: 2, Groups: []briefGroup{
		{Source: "OpenAI", Priority: "P0", Items: []briefItem{{Title: "GPT <6>", Summary: "a & b", URL: "https://openai.com/x"}}},
		{Source: "Media", Items: []briefItem{{Title: "Other", URL: "https://ex.com/y"}}},
	}}
	h := briefHTML(b)
	if !strings.Contains(h, "<b>") || !strings.Contains(h, "GPT &lt;6&gt;") || !strings.Contains(h, "a &amp; b") {
		t.Fatalf("html escape: %s", h)
	}
	big := strings.Repeat("段落内容测试 ", 500)
	parts := splitTG(big, 400)
	if len(parts) < 2 {
		t.Fatalf("expected split, got %d", len(parts))
	}
	for _, p := range parts {
		if len([]rune(p)) > 400 {
			t.Fatalf("part too long: %d", len([]rune(p)))
		}
	}
}

func TestCleanTGFields(t *testing.T) {
	if _, ok := cleanTGToken("short"); ok {
		t.Fatal("short token")
	}
	tok, ok := cleanTGToken("999000111:BBBfakeTokenForTestsOnly_xxYYzz_0123456789")
	if !ok || tok == "" {
		t.Fatal("valid token")
	}
	if _, ok := cleanChatID("nope"); ok {
		t.Fatal("bad chat")
	}
	if c, ok := cleanChatID("-1001234567890"); !ok || c == "" {
		t.Fatal("group chat")
	}
	if s, ok := cleanSchedule("8:5"); !ok || s != "08:05" {
		t.Fatalf("schedule pad: %q %v", s, ok)
	}
}

// silence unused import if http only used via httptest
var _ = http.StatusOK
