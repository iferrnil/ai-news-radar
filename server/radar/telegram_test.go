package radar

import (
	"context"
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

// TestTelegramSendBypassesPublicOnly: tgSend must reach the Bot API even when the dial
// target is loopback (httptest). PublicOnly would refuse 127.0.0.1 — that was the false
// positive users hit when Clash/Surge fake-ip resolved api.telegram.org to 198.18.0.0/15.
func TestTelegramSendBypassesPublicOnly(t *testing.T) {
	guardOn(t)
	var gotPath, gotChat, gotText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotPath = r.URL.Path
		gotChat = r.Form.Get("chat_id")
		gotText = r.Form.Get("text")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()

	old := tgAPIBase
	tgAPIBase = srv.URL
	defer func() { tgAPIBase = old }()

	s := testStore(t)
	// testStore pins UPSTREAM_MOCK to a dead port; clear it so tgSend hits our httptest.
	t.Setenv("UPSTREAM_MOCK", "")
	s.Cfg.Desktop = true
	sec := s.loadSecrets()
	sec.TelegramToken = "999000111:BBBfakeTokenForTestsOnly_xxYYzz_0123456789"
	if err := s.saveSecrets(sec); err != nil {
		t.Fatal(err)
	}
	if err := s.tgSend("@mychannel", "<b>hi</b>"); err != nil {
		t.Fatalf("tgSend via loopback (must not hit SSRF): %v", err)
	}
	if !strings.Contains(gotPath, "/bot999000111:BBBfakeTokenForTestsOnly_xxYYzz_0123456789/sendMessage") {
		t.Fatalf("path: %s", gotPath)
	}
	if gotChat != "@mychannel" || gotText != "<b>hi</b>" {
		t.Fatalf("form chat=%q text=%q", gotChat, gotText)
	}

	// Contrast: the same URL under PublicOnly is still refused (SSRF intact for general fetch).
	if _, err := netFetch(PublicOnly(context.Background()), "GET", srv.URL, nil, nil, 0); err == nil || !strings.Contains(err.Error(), "内网") {
		t.Fatalf("PublicOnly must still block loopback: %v", err)
	}
}

// TestSSRFStillBlocksFakeIPAndLoopback: general PublicOnly fetches keep refusing private /
// benchmark (Clash fake-ip) ranges — only the fixed Telegram host path is exempt.
func TestSSRFStillBlocksFakeIPAndLoopback(t *testing.T) {
	guardOn(t)
	ctx := PublicOnly(context.Background())
	for _, u := range []string{
		"http://127.0.0.1/",
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://198.18.0.1/", // Clash/Surge fake-ip / benchmark range
		"http://169.254.169.254/",
	} {
		if err := checkTarget(ctx, u); err == nil {
			t.Fatalf("checkTarget allowed %s", u)
		}
	}
	// Public unicast still OK (no DNS — literal IP).
	if err := checkTarget(ctx, "https://8.8.8.8/"); err != nil {
		t.Fatalf("public IP blocked: %v", err)
	}
}
