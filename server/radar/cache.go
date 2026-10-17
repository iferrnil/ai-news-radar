package radar

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 原文缓存 (desktop): every original text fetched in full (self-written fetcher or a 抓取 API) is kept as
//   <工作目录>/缓存/原文/<host>/<sha1(url)[:16]>.md
// for cacheTTL, so a brief or article written again within a day does not fetch (or pay for) it again. The
// in-memory cache (article.go, 30 min) sits in front of it. Everything under 缓存/ older than cacheTTL is deleted
// at start-up and every cacheTTL (generated covers wait there too until they are exported).

const cacheTTL = 24 * time.Hour

type diskCacheT struct {
	mu        sync.Mutex
	owner     *Store
	dir       func() string // <工作目录>/缓存; nil = no disk cache (web server, tests)
	lastClean time.Time
}

var gDisk = &diskCacheT{}

// cacheDir: <工作目录>/缓存.
func (s *Store) cacheDir() string { return filepath.Join(s.artifactsDir(), "缓存") }

// useDiskCache makes s the owner of the disk cache (desktop only).
func (s *Store) useDiskCache() {
	if !s.Cfg.Desktop {
		return
	}
	gDisk.mu.Lock()
	gDisk.owner, gDisk.dir, gDisk.lastClean = s, s.cacheDir, time.Time{}
	gDisk.mu.Unlock()
}

func (s *Store) dropDiskCache() {
	gDisk.mu.Lock()
	if gDisk.owner == s {
		gDisk.owner, gDisk.dir = nil, nil
	}
	gDisk.mu.Unlock()
}

func (d *diskCacheT) root() string {
	d.mu.Lock()
	f := d.dir
	d.mu.Unlock()
	if f == nil {
		return ""
	}
	return f()
}

func diskCacheFile(root, u string) string {
	h := sha1.Sum([]byte(u))
	host := strings.TrimSpace(unsafeName.ReplaceAllString(artHost(u), "-"))
	if host == "" {
		host = "other"
	}
	return filepath.Join(root, "原文", host, hex.EncodeToString(h[:8])+".md")
}

// load returns a cached full text younger than cacheTTL.
func (d *diskCacheT) load(u string) (articleRes, bool) {
	root := d.root()
	if root == "" {
		return articleRes{}, false
	}
	p := diskCacheFile(root, u)
	st, err := os.Stat(p)
	if err != nil || time.Since(st.ModTime()) >= cacheTTL {
		return articleRes{}, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return articleRes{}, false
	}
	head, body, ok := strings.Cut(string(b), "\n---\n\n")
	if !ok || !strings.HasPrefix(head, "---\n") {
		return articleRes{}, false
	}
	res := articleRes{Status: "full"}
	for _, ln := range strings.Split(head, "\n") {
		k, v, _ := strings.Cut(ln, ": ")
		switch k {
		case "url":
			if v != u { // hash collision or a hand-edited file
				return articleRes{}, false
			}
		case "title":
			res.Title = unYamlStr(v)
		case "method":
			res.Method = v
		case "extract":
			res.Extract = v
		}
	}
	res.Text = strings.TrimSpace(body)
	if res.Text == "" {
		return articleRes{}, false
	}
	return res, true
}

func (d *diskCacheT) save(u string, r articleRes) {
	root := d.root()
	if root == "" || r.Status != "full" || strings.TrimSpace(r.Text) == "" {
		return
	}
	p := diskCacheFile(root, u)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\nurl: %s\ntitle: %s\nmethod: %s\n", u, yamlStr(r.Title), firstNonEmpty(r.Method, "self"))
	if r.Extract != "" {
		fmt.Fprintf(&b, "extract: %s\n", r.Extract)
	}
	fmt.Fprintf(&b, "fetched: %s\n---\n\n%s\n", time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"), strings.TrimSpace(r.Text))
	tmp := p + ".tmp"
	if os.WriteFile(tmp, []byte(b.String()), 0o644) == nil {
		os.Rename(tmp, p)
	}
	d.maybeClean(root)
}

// maybeClean runs cleanCache when the last run is cacheTTL old (covers the periods the ticker sleeps through).
func (d *diskCacheT) maybeClean(root string) {
	d.mu.Lock()
	due := time.Since(d.lastClean) >= cacheTTL
	if due {
		d.lastClean = time.Now()
	}
	d.mu.Unlock()
	if due {
		go cleanCache(root, cacheTTL)
	}
}

// cleanCache deletes the files under root older than ttl, then the directories left empty. Returns the count.
func cleanCache(root string, ttl time.Duration) int {
	if root == "" || filepath.Base(root) != "缓存" { // never walk anything but a 缓存 directory
		return 0
	}
	n := 0
	var dirs []string
	filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() {
			if p != root {
				dirs = append(dirs, p)
			}
			return nil
		}
		if info, err := e.Info(); err == nil && time.Since(info.ModTime()) >= ttl {
			if os.Remove(p) == nil {
				n++
			}
		}
		return nil
	})
	for i := len(dirs) - 1; i >= 0; i-- { // deepest first
		os.Remove(dirs[i]) // fails (keeps it) unless empty
	}
	if n > 0 {
		log.Printf("[cache] removed %d files older than %s", n, ttl)
	}
	return n
}

// startCacheJanitor (desktop): clean at start-up, then every cacheTTL.
func (s *Store) startCacheJanitor(done <-chan struct{}) {
	if !s.Cfg.Desktop {
		return
	}
	go func() {
		run := func() {
			gDisk.mu.Lock()
			gDisk.lastClean = time.Now()
			gDisk.mu.Unlock()
			cleanCache(s.cacheDir(), cacheTTL)
		}
		run()
		t := time.NewTicker(cacheTTL)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				run()
			}
		}
	}()
}

func unYamlStr(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		if out, err := strconv.Unquote(v); err == nil {
			return out
		}
	}
	return v
}
