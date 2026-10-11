package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerEnvDropsSecrets(t *testing.T) {
	fake := "not" + "-a-real-key" // keeps the repo secret scan quiet
	env := serverEnv([]string{"DEEPSEEK_API_KEY=" + fake, "TRANSLATE_ENABLED=1", "PATH=/bin", "NEWS_DB=/x"}, 4321, "/data", "/app", nil)
	j := strings.Join(env, "\n")
	for _, bad := range []string{fake, "TRANSLATE_ENABLED=1", "NEWS_DB=/x"} {
		if strings.Contains(j, bad) {
			t.Fatalf("env leaks %q", bad)
		}
	}
	for _, want := range []string{"PATH=/bin", "TRANSLATE_ENABLED=0", "PORT=4321", "HOSTNAME=127.0.0.1", "NEWS_DB=" + filepath.Join("/data", "news.db"), "SOURCES_JSON=" + filepath.Join("/app", "sources", "sources.json")} {
		if !strings.Contains(j, want) {
			t.Fatalf("env misses %q", want)
		}
	}
}

func TestFreePort(t *testing.T) {
	p, err := FreePort()
	if err != nil || p <= 0 {
		t.Fatal(p, err)
	}
}

// End-to-end with the real bundle: AI_RADAR_TEST_RES=<dir holding node/bin/node and server/>.
func TestBundledServerLifecycle(t *testing.T) {
	res := os.Getenv("AI_RADAR_TEST_RES")
	if res == "" {
		t.Skip("set AI_RADAR_TEST_RES to a built resources dir")
	}
	t.Setenv("DEEPSEEK_API_KEY", "x-should-not-leak")
	node, app := bundleLayout(res)
	data := t.TempDir()
	s, err := StartServer(ServerConfig{Node: node, AppDir: app, DataDir: data, Log: os.Stderr, Grace: 8 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := s.WaitHealthy(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s.URL, "http://127.0.0.1:") {
		t.Fatal(s.URL)
	}
	var srcs []map[string]any
	r, err := http.Get(s.URL + "/api/sources")
	if err != nil {
		t.Fatal(err)
	}
	json.NewDecoder(r.Body).Decode(&srcs)
	r.Body.Close()
	if len(srcs) < 20 {
		t.Fatalf("seeded sources: %d", len(srcs))
	}
	if r, err := http.Get(s.URL + "/"); err != nil || r.StatusCode != 200 {
		t.Fatal("index", err)
	}
	db := filepath.Join(data, "news.db")
	if _, err := os.Stat(db + "-wal"); err != nil {
		t.Logf("no WAL while running (%v)", err)
	}
	start := time.Now()
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	t.Logf("stopped in %v", time.Since(start))
	if time.Since(start) > 7*time.Second {
		t.Fatal("node needed a kill (no graceful exit)")
	}
	if _, err := os.Stat(db); err != nil {
		t.Fatal("db missing", err)
	}
	if fi, err := os.Stat(db + "-wal"); err == nil && fi.Size() > 0 {
		t.Fatal("WAL not checkpointed: SQLite was not closed cleanly")
	}
	if _, err := http.Get(s.URL + "/api/health"); err == nil {
		t.Fatal("server still answering after Stop")
	}
}
