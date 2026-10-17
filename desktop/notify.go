package main

// Native notifications: the shell listens to the bundled server's SSE stream (/api/stream) for `notify` (new P0
// items) or `digest` (every new subscription item) batches — already batched, throttled and free of start-up
// backlog, see src/lib/notify.ts — reads the user's prefs from /api/settings and shows one system notification
// per batch ("新增 N 条" + top titles). Clicking it focuses the app and opens the item.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

type notifyItem struct {
	ID      int64   `json:"id"`
	Title   string  `json:"title"`
	Source  string  `json:"source"`
	Section *string `json:"section"`
}

type notifyBatch struct {
	Count  int          `json:"count"`
	Items  []notifyItem `json:"items"`
	Digest bool         `json:"-"` // every new item, not only P0
}

type notifyPrefs struct {
	Enabled  bool     `json:"enabled"`
	Sections []string `json:"sections"` // empty = all
	System   *bool    `json:"system"`   // 桌面系统通知 (nil = on, older servers)
	Scope    string   `json:"scope"`    // "p0" (default) | "all"
}

// wants reports whether an SSE event should become a system notification under these prefs.
func (p notifyPrefs) wants(event string) bool {
	if !p.Enabled || (p.System != nil && !*p.System) {
		return false
	}
	if p.Scope == "all" {
		return event == "digest"
	}
	return event == "notify"
}

// filterBatch keeps the items of the chosen sections; nil when nothing is left.
func filterBatch(b notifyBatch, sections []string) *notifyBatch {
	if len(sections) == 0 {
		return &b
	}
	var items []notifyItem
	for _, it := range b.Items {
		sec := ""
		if it.Section != nil {
			sec = *it.Section
		}
		if slices.Contains(sections, sec) {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return nil
	}
	n := b.Count
	if len(items) < len(b.Items) {
		n = len(items)
	}
	return &notifyBatch{Count: n, Items: items, Digest: b.Digest}
}

// batchText is the notification's title and body.
func batchText(b notifyBatch) (title, body string) {
	if b.Count == 1 && len(b.Items) == 1 {
		if b.Digest {
			return b.Items[0].Source, b.Items[0].Title
		}
		return "P0 · " + b.Items[0].Source, b.Items[0].Title
	}
	var lines []string
	for i, it := range b.Items {
		if i == 3 {
			break
		}
		lines = append(lines, "· "+it.Source+"："+it.Title)
	}
	if b.Count > 3 {
		lines = append(lines, fmt.Sprintf("…还有 %d 条", b.Count-3))
	}
	if b.Digest {
		return fmt.Sprintf("新增 %d 条", b.Count), strings.Join(lines, "\n")
	}
	return fmt.Sprintf("%d 条新 P0 动态", b.Count), strings.Join(lines, "\n")
}

// readSSE calls fn(event, data) for every event of an SSE stream until it ends.
func readSSE(r io.Reader, fn func(event, data string)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	ev, data := "", []string{}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if len(data) > 0 {
				fn(cmpOr(ev, "message"), strings.Join(data, "\n"))
			}
			ev, data = "", data[:0]
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			ev = strings.TrimSpace(line[6:])
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(line[5:], " "))
		}
	}
	return sc.Err()
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func fetchPrefs(ctx context.Context, base string) (notifyPrefs, error) {
	var out struct {
		Notify notifyPrefs `json:"notify"`
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/settings", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return out.Notify, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return out.Notify, fmt.Errorf("settings: HTTP %d", res.StatusCode)
	}
	return out.Notify, json.NewDecoder(res.Body).Decode(&out)
}

// watchNotifications follows /api/stream (reconnecting with backoff) and calls show for every batch the prefs allow.
func watchNotifications(ctx context.Context, base string, show func(notifyBatch)) {
	wait := time.Second
	for ctx.Err() == nil {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/stream", nil)
		req.Header.Set("Accept", "text/event-stream")
		res, err := (&http.Client{}).Do(req)
		if err == nil {
			wait = time.Second
			_ = readSSE(res.Body, func(event, data string) {
				if event != "notify" && event != "digest" {
					return
				}
				var msg struct {
					Batch notifyBatch `json:"batch"`
				}
				if json.Unmarshal([]byte(data), &msg) != nil || len(msg.Batch.Items) == 0 {
					return
				}
				p, err := fetchPrefs(ctx, base)
				if err != nil || !p.wants(event) {
					return
				}
				msg.Batch.Digest = event == "digest"
				if b := filterBatch(msg.Batch, p.Sections); b != nil {
					show(*b)
				}
			})
			res.Body.Close()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, 30*time.Second)
	}
}
