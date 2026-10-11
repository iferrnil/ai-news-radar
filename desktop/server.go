package main

// The embedded Node/Next.js standalone server: started on a free 127.0.0.1 port,
// health-checked, and stopped gracefully (SIGTERM, then kill after a grace period).

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// ServerConfig locates the bundled runtime and the user's data.
type ServerConfig struct {
	Node     string // node executable
	AppDir   string // Next standalone dir (server.js, .next, public, sources, node_modules)
	DataDir  string // per-user data dir; the SQLite DB lives here
	Log      io.Writer
	Grace    time.Duration // SIGTERM -> kill
	ExtraEnv []string
}

type Server struct {
	cfg  ServerConfig
	cmd  *exec.Cmd
	Port int
	URL  string
	done chan struct{}
	err  error
	once sync.Once
	stdin io.WriteCloser
}

const preloadJS = `"use strict";
// AI News Radar desktop: exit (closing SQLite via the "exit" handler) when the shell closes our stdin.
const bye = () => process.exit(0);
process.stdin.on("end", bye); process.stdin.on("close", bye); process.stdin.on("error", bye);
process.stdin.resume();
`

// FreePort asks the OS for an unused TCP port on 127.0.0.1.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// serverEnv is the child's environment: the parent's minus anything secret or
// conflicting, plus the desktop settings. The DeepSeek key is never passed and
// translation stays off.
func serverEnv(parent []string, port int, dataDir string, appDir string, extra []string) []string {
	drop := map[string]bool{"DEEPSEEK_API_KEY": true, "TRANSLATE_ENABLED": true, "PORT": true, "HOSTNAME": true, "NEWS_DB": true, "SOURCES_JSON": true, "NODE_OPTIONS": true}
	var env []string
	for _, kv := range parent {
		k := kv
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				k = kv[:i]
				break
			}
		}
		if !drop[k] {
			env = append(env, kv)
		}
	}
	env = append(env,
		"NODE_ENV=production", "NEXT_TELEMETRY_DISABLED=1", "TRANSLATE_ENABLED=0",
		fmt.Sprintf("PORT=%d", port), "HOSTNAME=127.0.0.1",
		"NEWS_DB="+filepath.Join(dataDir, "news.db"),
		"SOURCES_JSON="+filepath.Join(appDir, "sources", "sources.json"),
		"AI_RADAR_DESKTOP=1",
	)
	return append(env, extra...)
}

// StartServer spawns node server.js and returns once the process started (not yet healthy).
func StartServer(cfg ServerConfig) (*Server, error) {
	if cfg.Grace == 0 {
		cfg.Grace = 5 * time.Second
	}
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	port, err := FreePort()
	if err != nil {
		return nil, err
	}
	// Shutdown goes through a stdin pipe, not only signals: GTK/WebKit (and possibly AppKit) threads can
	// leave SIGTERM blocked in the mask the child inherits. The preload exits node when stdin closes, which
	// also covers a crashed shell (no orphaned server). process "exit" closes SQLite (src/lib/db.ts).
	pre := filepath.Join(cfg.DataDir, "desktop-preload.cjs")
	if err := os.WriteFile(pre, []byte(preloadJS), 0o644); err != nil {
		return nil, err
	}
	cmd := exec.Command(cfg.Node, "--require", pre, filepath.Join(cfg.AppDir, "server.js"))
	cmd.Dir = cfg.AppDir
	cmd.Env = serverEnv(os.Environ(), port, cfg.DataDir, cfg.AppDir, cfg.ExtraEnv)
	cmd.Stdout, cmd.Stderr = cfg.Log, cfg.Log
	setProcAttrs(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start node: %w", err)
	}
	s := &Server{cfg: cfg, cmd: cmd, Port: port, URL: fmt.Sprintf("http://127.0.0.1:%d", port), done: make(chan struct{}), stdin: stdin}
	go func() { s.err = cmd.Wait(); close(s.done) }()
	return s, nil
}

// WaitHealthy polls /api/health until it answers 200, the process exits, or ctx ends.
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
			return fmt.Errorf("server exited before it was healthy: %v", s.err)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// Done is closed when the node process exits.
func (s *Server) Done() <-chan struct{} { return s.done }

// Stop sends SIGTERM (node closes SQLite on exit), waits up to Grace, then kills.
func (s *Server) Stop() error {
	s.once.Do(func() {
		select {
		case <-s.done:
			return
		default:
		}
		_ = s.stdin.Close() // 1) graceful: the preload exits node
		select {
		case <-s.done:
			return
		case <-time.After(s.cfg.Grace / 2):
		}
		if runtime.GOOS != "windows" { // 2) SIGTERM (Next's own handler, then "exit")
			_ = s.cmd.Process.Signal(syscall.SIGTERM)
		}
		select {
		case <-s.done:
		case <-time.After(s.cfg.Grace / 2): // 3) kill
			_ = s.cmd.Process.Kill()
			<-s.done
		}
	})
	var ee *exec.ExitError
	if s.err != nil && !errors.As(s.err, &ee) {
		return s.err
	}
	return nil
}
