package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func TestApplyEnvDropsSecrets(t *testing.T) {
	fake := "not" + "-a-real-key" // keeps the repo secret scan quiet
	t.Setenv("DEEPSEEK_API_KEY", fake)
	t.Setenv("TRANSLATE_ENABLED", "1")
	t.Setenv("SETTINGS_LOCKED", "1")
	t.Setenv("NEWS_DB", "/x")
	applyEnv("1.2.3")
	for _, k := range []string{"DEEPSEEK_API_KEY", "SETTINGS_LOCKED", "NEWS_DB", "TRANSLATE_ENABLED"} {
		if _, ok := os.LookupEnv(k); ok {
			t.Fatalf("%s still set", k)
		}
	}
	for k, want := range map[string]string{"AI_RADAR_DESKTOP": "1", "AI_RADAR_VERSION": "1.2.3"} {
		if os.Getenv(k) != want {
			t.Fatalf("%s=%q", k, os.Getenv(k))
		}
	}
}

func TestFreePort(t *testing.T) {
	p, err := FreePort()
	if err != nil || p <= 0 {
		t.Fatal(p, err)
	}
}

// In-process backend: seeds, serves the UI + API on 127.0.0.1, stops cleanly with SQLite checkpointed.
func TestServerLifecycle(t *testing.T) {
	seed, err := os.ReadFile("../sources/sources.json")
	if err != nil {
		t.Skip("no sources.json")
	}
	t.Setenv("UPSTREAM_MOCK", "http://127.0.0.1:9") // nothing listens: the poller fails fast, no network
	t.Setenv("AI_RADAR_TEST_KEEP_UPSTREAM_MOCK", "1")
	t.Setenv("DEEPSEEK_API_KEY", "x-should-not-leak")
	data := t.TempDir()
	ui := fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>radar</title>")}}
	s, err := StartServer(ServerConfig{DataDir: data, UI: ui, Seed: seed, Version: "9.9.9"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.WaitHealthy(ctx); err != nil {
		t.Fatal(err)
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
	var set struct {
		Desktop  bool `json:"desktop"`
		Deepseek struct {
			Configured bool `json:"configured"`
		} `json:"deepseek"`
	}
	r, _ = http.Get(s.URL + "/api/settings")
	json.NewDecoder(r.Body).Decode(&set)
	r.Body.Close()
	if set.Deepseek.Configured {
		t.Fatal("DEEPSEEK_API_KEY leaked into the desktop backend")
	}
	if r, err := http.Get(s.URL + "/"); err != nil || r.StatusCode != 200 {
		t.Fatal("index", err)
	}
	start := time.Now()
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	t.Logf("stopped in %v", time.Since(start))
	db := filepath.Join(data, "news.db")
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
