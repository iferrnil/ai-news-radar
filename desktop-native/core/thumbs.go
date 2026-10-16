package core

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ThumbCache downloads item thumbnails into dir (one file per URL, named by its SHA-1), capped at MaxBytes.
// Files are pruned with the 48h retention (no item references them) and by size (oldest first).
type ThumbCache struct {
	Dir      string
	MaxBytes int64
	mu       sync.Mutex
	inflight map[string]chan struct{}
}

const thumbMaxFile = 4 << 20

func NewThumbCache(dir string, max int64) *ThumbCache {
	os.MkdirAll(dir, 0o755)
	return &ThumbCache{Dir: dir, MaxBytes: max, inflight: map[string]chan struct{}{}}
}

func thumbName(u string) string { h := sha1.Sum([]byte(u)); return hex.EncodeToString(h[:]) }

func (t *ThumbCache) path(u string) string { return filepath.Join(t.Dir, thumbName(u)) }

// Get returns the cached bytes, downloading them first when needed (concurrent callers share one download).
// A failed download leaves a ".fail" marker so it is not retried for an hour.
func (t *ThumbCache) Get(ctx context.Context, u, referer string) ([]byte, error) {
	p := t.path(u)
	if b, err := os.ReadFile(p); err == nil {
		now := time.Now()
		os.Chtimes(p, now, now) // LRU
		return b, nil
	}
	if st, err := os.Stat(p + ".fail"); err == nil && time.Since(st.ModTime()) < time.Hour {
		return nil, errors.New("最近下载失败")
	}
	t.mu.Lock()
	if ch, ok := t.inflight[u]; ok {
		t.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return os.ReadFile(p)
	}
	ch := make(chan struct{})
	t.inflight[u] = ch
	t.mu.Unlock()
	defer func() { t.mu.Lock(); delete(t.inflight, u); t.mu.Unlock(); close(ch) }()
	b, err := t.download(ctx, u, referer)
	if err != nil {
		os.WriteFile(p+".fail", nil, 0o644)
		return nil, err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return nil, err
	}
	return b, os.Rename(tmp, p)
}

func (t *ThumbCache) download(ctx context.Context, u, referer string) ([]byte, error) {
	if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		return nil, errors.New("不是 http(s) 图片")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UA)
	req.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	r, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", r.StatusCode)
	}
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") && !strings.HasPrefix(ct, "application/octet-stream") {
		return nil, fmt.Errorf("不是图片（%s）", ct)
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, thumbMaxFile+1))
	if err != nil {
		return nil, err
	}
	if len(b) > thumbMaxFile {
		return nil, errors.New("图片过大")
	}
	return b, nil
}

// Prune deletes files no longer referenced by keep (image URLs of retained items), stale .fail markers,
// then the least recently used files until the cache fits MaxBytes. Returns files removed.
func (t *ThumbCache) Prune(keep []string) int {
	want := map[string]bool{}
	for _, u := range keep {
		want[thumbName(u)] = true
	}
	ents, err := os.ReadDir(t.Dir)
	if err != nil {
		return 0
	}
	type f struct {
		p    string
		size int64
		mod  time.Time
	}
	var files []f
	var total int64
	n := 0
	for _, e := range ents {
		name := e.Name()
		p := filepath.Join(t.Dir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}
		base := strings.TrimSuffix(strings.TrimSuffix(name, ".fail"), ".tmp")
		if !want[base] || (strings.HasSuffix(name, ".fail") && time.Since(info.ModTime()) > time.Hour) ||
			(strings.HasSuffix(name, ".tmp") && time.Since(info.ModTime()) > time.Hour) {
			if os.Remove(p) == nil {
				n++
			}
			continue
		}
		if strings.Contains(name, ".") {
			continue
		}
		files = append(files, f{p, info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(files, func(a, b int) bool { return files[a].mod.Before(files[b].mod) })
	for _, x := range files {
		if total <= t.MaxBytes {
			break
		}
		if os.Remove(x.p) == nil {
			total -= x.size
			n++
		}
	}
	return n
}

// Size returns the bytes currently cached.
func (t *ThumbCache) Size() (n int64) {
	ents, _ := os.ReadDir(t.Dir)
	for _, e := range ents {
		if i, err := e.Info(); err == nil {
			n += i.Size()
		}
	}
	return
}

// ImageURLs lists the thumbnails referenced by retained items.
func (s *Store) ImageURLs() []string {
	return s.strings("SELECT DISTINCT image FROM items WHERE image IS NOT NULL AND image<>''")
}
