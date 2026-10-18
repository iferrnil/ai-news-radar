package main

import (
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGate(t *testing.T) {
	data := t.TempDir()
	seed, _ := os.ReadFile("../sources/sources.json")
	ui := os.DirFS(t.TempDir())
	s, err := StartServer(ServerConfig{DataDir: filepath.Join(data, "d"), UI: ui, Seed: seed, Version: "9.9.9"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	if st, _ := os.Stat(filepath.Join(data, "d")); st.Mode().Perm() != 0o700 {
		t.Errorf("data dir mode %v", st.Mode().Perm())
	}
	get := func(c *http.Client, u string, hdr map[string]string) int {
		t.Helper()
		rq, _ := http.NewRequest("GET", u, nil)
		for k, v := range hdr {
			if k == "Host" {
				rq.Host = v
			} else {
				rq.Header.Set(k, v)
			}
		}
		r, err := c.Do(rq)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	plain := &http.Client{}
	// another local process / user / a website: no token → nothing but /api/health
	for _, p := range []string{"/api/settings", "/api/fetchapi", "/api/sources", "/api/artifacts"} {
		if c := get(plain, s.URL+p, nil); c != 401 {
			t.Errorf("%s without token: %d", p, c)
		}
	}
	if c := get(plain, s.URL+"/api/health", nil); c != 200 {
		t.Errorf("health: %d", c)
	}
	if c := get(plain, s.URL+"/api/settings", map[string]string{"X-Radar-Token": strings.Repeat("0", 64)}); c != 401 {
		t.Errorf("wrong token: %d", c)
	}
	// DNS rebinding: an attacker name pointing at 127.0.0.1
	if c := get(localClient, s.URL+"/api/settings", map[string]string{"Host": "rebind.attacker.example:" + strconv.Itoa(s.Port)}); c != http.StatusMisdirectedRequest {
		t.Errorf("rebound Host: %d", c)
	}
	// the shell's client (header) and the window (cookie from the start URL)
	if c := get(localClient, s.URL+"/api/settings", nil); c != 200 {
		t.Errorf("localClient: %d", c)
	}
	jar, _ := cookiejar.New(nil)
	win := &http.Client{Jar: jar}
	r, err := win.Get(s.StartURL())
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if strings.Contains(r.Request.URL.RawQuery, "_k=") {
		t.Errorf("token left in the URL: %s", r.Request.URL)
	}
	if c := get(win, s.URL+"/api/settings", nil); c != 200 {
		t.Errorf("window cookie: %d", c)
	}
	// a wrong _k sets nothing
	jar2, _ := cookiejar.New(nil)
	bad := &http.Client{Jar: jar2}
	r, _ = bad.Get(s.URL + "/?_k=nope")
	r.Body.Close()
	if c := get(bad, s.URL+"/api/settings", nil); c != 401 {
		t.Errorf("bad _k got a session: %d", c)
	}
	// a cross-site browser request with the cookie (CSRF) is refused by the radar guard
	if c := get(win, s.URL+"/api/settings", map[string]string{"Sec-Fetch-Site": "cross-site"}); c != 403 {
		t.Errorf("cross-site with cookie: %d", c)
	}
}

func TestExternalURL(t *testing.T) {
	for u, want := range map[string]bool{
		"https://example.com/a": true, "http://example.com": true, "mailto:a@b.c": true,
		"file:///System/Applications/Calculator.app": false, "smb://attacker/share": false, "javascript:alert(1)": false,
		"x-apple.systempreferences:com.apple.preference": false, "ssh://host": false, "http://u:p@example.com/": false,
		"vnc://host": false, "": false, "https://": false,
	} {
		if got := externalURL(u); got != want {
			t.Errorf("externalURL(%q) = %v", u, got)
		}
	}
	if !sameOrigin("http://127.0.0.1:47821/?view=hot", "http://127.0.0.1:47821") || sameOrigin("http://127.0.0.1:47821@evil.example/", "http://127.0.0.1:47821") || sameOrigin("http://127.0.0.1:478210/", "http://127.0.0.1:47821") {
		t.Error("sameOrigin")
	}
}
