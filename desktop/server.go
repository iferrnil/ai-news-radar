package main

// The embedded Go backend (../server, package radar) runs in-process: no Node, no child process.
// It listens on 127.0.0.1 only and serves the static Next.js UI embedded in the binary (webui/).

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/iferrnil/ai-news-radar/server/radar"
)

// webui is filled by scripts/package.sh (Next.js static export + sources.json); the committed README.md keeps
// `go build` / `go test` working without a UI build.
//
//go:embed all:webui
var webui embed.FS

// ServerConfig locates the user's data and the UI.
type ServerConfig struct {
	DataDir string // per-user data dir; the SQLite DB lives here
	Log     io.Writer
	Port    int    // preferred port (0 = any free one)
	Version string // AI_RADAR_VERSION
	UI      fs.FS  // static UI; nil = embedded webui/out
	Seed    []byte // sources.json; nil = embedded webui/sources.json
}

type Server struct {
	st   *radar.Store
	srv  *http.Server
	Port int
	URL  string
	done chan struct{}
	err  error
	once sync.Once
}

// PreferredPort is tried first so the page keeps the same origin (http://127.0.0.1:47821) between launches:
// localStorage (theme, read state, sidebar) belongs to the origin. Taken -> a random free port.
const PreferredPort = 47821

// listen binds preferred on 127.0.0.1 when it is free, else a free port from the OS.
func listen(preferred int) (net.Listener, error) {
	if preferred > 0 {
		if l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred)); err == nil {
			return l, nil
		}
	}
	return net.Listen("tcp", "127.0.0.1:0")
}

// FreePort asks the OS for an unused TCP port on 127.0.0.1.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// desktopEnv is applied to our own process before the backend starts (it reads tunables from the environment):
// anything secret or conflicting is removed, plus the desktop settings. The DeepSeek key is never taken from
// the env and translation defaults to off; the user sets both in 设置, which stores the key in
// <dataDir>/secrets.json (0600). In-app settings are never locked here.
var desktopDrop = []string{"DEEPSEEK_API_KEY", "TRANSLATE_ENABLED", "PORT", "HOSTNAME", "NEWS_DB", "SOURCES_JSON", "SETTINGS_LOCKED", "SECRETS_FILE", "UPSTREAM_MOCK", "DISABLE_POLLER"}

func desktopEnv(version string) map[string]string {
	return map[string]string{"TRANSLATE_ENABLED": "0", "AI_RADAR_DESKTOP": "1", "AI_RADAR_VERSION": version}
}

func applyEnv(version string) {
	for _, k := range desktopDrop {
		if os.Getenv("AI_RADAR_TEST_KEEP_"+k) == "" { // tests may keep e.g. UPSTREAM_MOCK
			os.Unsetenv(k)
		}
	}
	for k, v := range desktopEnv(version) {
		os.Setenv(k, v)
	}
}

func embeddedUI() (ui fs.FS, seed []byte) {
	if sub, err := fs.Sub(webui, "webui/out"); err == nil {
		if _, err := fs.Stat(sub, "index.html"); err == nil {
			ui = sub
		}
	}
	seed, _ = webui.ReadFile("webui/sources.json")
	return
}

// StartServer opens the database, starts the poller and serves on 127.0.0.1; returns once listening.
func StartServer(cfg ServerConfig) (*Server, error) {
	if cfg.Log != nil {
		log.SetOutput(cfg.Log)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	applyEnv(cfg.Version)
	ui, seed := embeddedUI()
	if d := os.Getenv("AI_RADAR_UI"); d != "" { // development: serve ../out instead of the embedded copy
		ui = os.DirFS(d)
	}
	if cfg.UI != nil {
		ui = cfg.UI
	}
	if cfg.Seed != nil {
		seed = cfg.Seed
	}
	if seed == nil {
		seed = []byte(`{"sources":[]}`)
	}
	rc := radar.Config{
		DBPath:      filepath.Join(cfg.DataDir, "news.db"),
		SecretsPath: filepath.Join(cfg.DataDir, "secrets.json"),
		SeedData:    seed,
		Desktop:     true,
		Version:     cfg.Version,
	}
	st, err := radar.Open(rc)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	l, err := listen(cfg.Port)
	if err != nil {
		st.Close()
		return nil, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	s := &Server{st: st, Port: port, URL: fmt.Sprintf("http://127.0.0.1:%d", port), done: make(chan struct{})}
	s.srv = &http.Server{Handler: st.Server(ui), ReadHeaderTimeout: 10 * time.Second}
	st.StartScheduler()
	go func() {
		if err := s.srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.err = err
		}
		close(s.done)
	}()
	return s, nil
}

// WaitHealthy polls /api/health until it answers 200, the server stops, or ctx ends.
func (s *Server) WaitHealthy(ctx context.Context) error {
	c := &http.Client{Timeout: 2 * time.Second}
	for {
		req, _ := http.NewRequestWithContext(ctx, "GET", s.URL+"/api/health", nil)
		if r, err := c.Do(req); err == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				return nil
			}
		}
		select {
		case <-s.done:
			return fmt.Errorf("server stopped before it was healthy: %v", s.err)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Done is closed when the HTTP server stops.
func (s *Server) Done() <-chan struct{} { return s.done }

// Stop: poller off, open requests (SSE) cut after 2 s, then SQLite is checkpointed and closed.
func (s *Server) Stop() error {
	var err error
	s.once.Do(func() {
		s.st.StopScheduler()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if e := s.srv.Shutdown(ctx); e != nil {
			s.srv.Close()
		}
		<-s.done
		err = s.st.Close()
		if err != nil && strings.Contains(err.Error(), "closed") {
			err = nil
		}
	})
	return err
}
