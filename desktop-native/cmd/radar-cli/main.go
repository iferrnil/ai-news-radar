// radar-cli is a headless smoke tool for the Go backend: radar-cli -dir DIR [-seed sources.json] [-poll] [-export json|opml]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

func main() {
	dir := flag.String("dir", "", "data dir (news.db)")
	seed := flag.String("seed", "", "sources.json to sync")
	poll := flag.Bool("poll", false, "poll every enabled source once")
	export := flag.String("export", "", "print export (json|opml)")
	flag.Parse()
	var seedData []byte
	if *seed != "" {
		b, err := os.ReadFile(*seed)
		if err != nil {
			log.Fatal(err)
		}
		seedData = b
	}
	s, err := core.Open(*dir, seedData)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	if *poll {
		list, _ := s.Sources()
		sc := s.NewScheduler()
		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		for _, x := range list {
			if !x.Enabled || !x.Supported {
				continue
			}
			wg.Add(1)
			go func() { defer wg.Done(); sc.PollOne(ctx, x) }()
		}
		wg.Wait()
		list, _ = s.Sources()
		for _, x := range list {
			if x.Enabled && x.Supported {
				fmt.Printf("%-40s %-6s new=%-3d items=%-4d err=%s\n", x.Name, x.Type, x.LastNew, x.Count, x.LastError)
			}
		}
	}
	switch *export {
	case "json":
		b, _ := s.ExportJSON(time.Now())
		os.Stdout.Write(b)
	case "opml":
		b, _ := s.ExportOPML(time.Now())
		os.Stdout.Write(b)
	}
	secs, _ := s.Sections()
	items, _ := s.Items(core.Query{Limit: 2000})
	fmt.Fprintf(os.Stderr, "sections=%d visible items(全部)=%d unread=%v\n", len(secs), len(items), s.UnreadCounts()[""])
}
