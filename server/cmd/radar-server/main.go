// radar-server: the AI News Radar backend + static UI in one pure-Go binary (no Node, no cgo).
//
//	radar-server [-addr :3000] [-ui ./out]        serve (env: NEWS_DB, SOURCES_JSON, SECRETS_FILE, PORT, HOSTNAME, …)
//	radar-server vacuum-into <out.db>             compact copy of NEWS_DB (e2e fixture)
//	radar-server healthcheck                      exit 0 when the server on $PORT answers /api/health
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iferrnil/ai-news-radar/server/radar"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" { // for the container HEALTHCHECK (no shell/curl in the image)
		port := envOr("PORT", "3000")
		c := http.Client{Timeout: 5 * time.Second}
		r, err := c.Get("http://127.0.0.1:" + port + "/api/health")
		if err != nil || r.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "vacuum-into" {
		st, err := radar.Open(radar.ConfigFromEnv())
		if err != nil {
			log.Fatal(err)
		}
		os.Remove(os.Args[2])
		if err := st.VacuumInto(os.Args[2]); err != nil {
			log.Fatal(err)
		}
		var n int
		st.DB.QueryRow("SELECT COUNT(*) FROM items").Scan(&n)
		st.Close()
		fmt.Println("fixture items:", n)
		return
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	host := os.Getenv("HOSTNAME")
	if net.ParseIP(host) == nil && host != "localhost" {
		host = "" // HOSTNAME is often the machine name (docker): listen on all interfaces
	}
	addr := flag.String("addr", net.JoinHostPort(host, port), "listen address")
	ui := flag.String("ui", envOr("UI_DIR", "out"), "directory with the static UI (Next.js export)")
	flag.Parse()

	st, err := radar.Open(radar.ConfigFromEnv())
	if err != nil {
		log.Fatal(err)
	}
	var uiFS = os.DirFS(*ui)
	if _, err := os.Stat(*ui); err != nil {
		log.Printf("[server] no UI at %s (API only)", *ui)
		uiFS = nil
	}
	st.StartScheduler()
	srv := &http.Server{Addr: *addr, Handler: st.Server(uiFS), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("[server] listening on http://%s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	st.StopScheduler()
	srv.Shutdown(ctx)
	st.Close()
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
