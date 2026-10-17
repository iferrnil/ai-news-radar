package radar

import (
	"encoding/json"
	"os"
	"slices"
	"sort"
	"sync"
	"time"
)

// P0 notifications (notify.ts): new P0 items are batched and published on the bus as `notify`.
// The first successful poll of each source after start-up is backlog; items older than NOTIFY_MAX_AGE are skipped;
// a batch waits NOTIFY_BATCH_MS for more and batches are >= NOTIFY_MIN_GAP_MS apart.

type NotifyItem struct {
	ID        any      `json:"id"`
	Title     string   `json:"title"`
	Source    string   `json:"source"`
	Section   any      `json:"section"`
	URL       string   `json:"url"`
	Published float64  `json:"published"`
	Keywords  []string `json:"keywords,omitempty"` // 关键词提醒: the matched watch keywords
}
type NotifyBatch struct {
	At       float64      `json:"at"`
	Count    int          `json:"count"`
	Items    []NotifyItem `json:"items"`
	Sections []string     `json:"sections"`
}

// NotifyPrefs: server-side prefs used by the desktop shell (the web keeps its own in localStorage). sections: [] = all.
// System: show system notifications (desktop; in-app toasts unaffected). Scope: "p0" = `notify` batches (P0 only),
// "all" = `digest` batches (every new subscription item, one "新增 N 条" summary per batch).
type NotifyPrefs struct {
	Enabled  bool     `json:"enabled"`
	Sections []string `json:"sections"`
	System   bool     `json:"system"`
	Scope    string   `json:"scope"`
}

// nchan is one batching channel (P0 `notify` or all-items `digest`).
type nchan struct {
	event  string
	queue  []NotifyItem
	timer  *time.Timer
	last   time.Time
	sent   int
	recent []NotifyBatch
}

type notifier struct {
	s          *Store
	mu         sync.Mutex
	polled     map[int64]bool
	p0, dg, kw nchan
	kwSeen     []kwSeen            // stories already alerted (watch.go)
	kwIDs      map[int64]time.Time // items already alerted, for the 热点 topic dedupe
}

func newNotifier(s *Store) *notifier {
	return &notifier{s: s, polled: map[int64]bool{}, p0: nchan{event: "notify"}, dg: nchan{event: "digest"}, kw: nchan{event: "keyword"}}
}

func (s *Store) NotifyPrefs() NotifyPrefs {
	v, ok := s.GetSetting("notify")
	var p struct {
		Enabled  any   `json:"enabled"`
		Sections []any `json:"sections"`
		System   any   `json:"system"`
		Scope    any   `json:"scope"`
	}
	if !ok || json.Unmarshal([]byte(v), &p) != nil {
		return NotifyPrefs{Enabled: s.Cfg.Desktop, Sections: []string{}, System: true, Scope: "p0"} // desktop: on by default
	}
	out := NotifyPrefs{Enabled: truthy(p.Enabled), Sections: []string{}, System: true, Scope: "p0"}
	for _, x := range p.Sections {
		out.Sections = append(out.Sections, jsString(x))
	}
	if b, ok := p.System.(bool); ok {
		out.System = b
	}
	if p.Scope == "all" {
		out.Scope = "all"
	}
	return out
}

// SetNotifyPrefs merges a PATCH; nil / invalid fields keep the current value.
func (s *Store) SetNotifyPrefs(enabled *bool, sections []any, system *bool, scope string) NotifyPrefs {
	cur := s.NotifyPrefs()
	next := NotifyPrefs{Enabled: cur.Enabled, Sections: []string{}, System: cur.System, Scope: cur.Scope}
	if enabled != nil {
		next.Enabled = *enabled
	}
	if system != nil {
		next.System = *system
	}
	if scope == "p0" || scope == "all" {
		next.Scope = scope
	}
	src := make([]any, 0)
	if sections != nil {
		src = sections
	} else {
		for _, x := range cur.Sections {
			src = append(src, x)
		}
	}
	for _, x := range src {
		if str, ok := x.(string); ok && jsLen(str) < 64 && len(next.Sections) < 50 {
			next.Sections = append(next.Sections, str)
		}
	}
	b, _ := json.Marshal(next)
	s.SetSetting("notify", string(b))
	return next
}

func notifyGap() time.Duration   { return envMs("NOTIFY_MIN_GAP_MS", 60000) }
func notifyBatch() time.Duration { return envMs("NOTIFY_BATCH_MS", 20000) }

// keyword alerts are what the user asked for by name: shorter batching
func (c *nchan) gap() time.Duration {
	if c.event == "keyword" {
		return envMs("KW_NOTIFY_MIN_GAP_MS", 15000)
	}
	return notifyGap()
}
func (c *nchan) batch() time.Duration {
	if c.event == "keyword" {
		return envMs("KW_NOTIFY_BATCH_MS", 5000)
	}
	return notifyBatch()
}

func makeBatch(items []NotifyItem) *NotifyBatch {
	secs := []string{}
	seen := map[string]bool{}
	for _, i := range items {
		k, _ := i.Section.(string)
		if !seen[k] {
			seen[k] = true
			secs = append(secs, k)
		}
	}
	return &NotifyBatch{At: now(), Count: len(items), Items: items[:min(5, len(items))], Sections: secs}
}

func (n *notifier) flush(c *nchan) {
	n.mu.Lock()
	c.timer = nil
	if len(c.queue) == 0 {
		n.mu.Unlock()
		return
	}
	if wait := time.Until(c.last.Add(c.gap())); wait > 0 {
		c.timer = time.AfterFunc(wait, func() { n.flush(c) }) // throttled: keep collecting
		n.mu.Unlock()
		return
	}
	all := c.queue
	c.queue = nil
	sort.SliceStable(all, func(a, b int) bool { return all[a].Published > all[b].Published })
	b := makeBatch(all)
	c.last = time.Now()
	c.sent++
	c.recent = append([]NotifyBatch{*b}, c.recent...)
	if len(c.recent) > 10 {
		c.recent = c.recent[:10]
	}
	n.mu.Unlock()
	n.s.Bus.Publish(Msg{Event: c.event, Batch: b})
}

// enqueue adds items to a channel (caller holds n.mu) and arms its batch timer.
func (n *notifier) enqueue(c *nchan, items []*Row) {
	added := false
	for _, i := range items {
		id := i.Get("id")
		if slices.ContainsFunc(c.queue, func(q NotifyItem) bool { return q.ID == id }) {
			continue
		}
		title := i.Str("title_zh")
		if title == "" {
			title = i.Str("title")
		}
		kw, _ := i.Get("kw").([]string)
		c.queue = append(c.queue, NotifyItem{id, jsSlice(title, 200), i.Str("source"), i.Get("section"), i.Str("url"), i.Num("published"), kw})
		added = true
	}
	if added && c.timer == nil {
		c.timer = time.AfterFunc(c.batch(), func() { n.flush(c) })
	}
}

// onPolled is called by the poller after each successful poll with the items it stored.
func (n *notifier) onPolled(sourceID int64, fresh []*Row) {
	digest := n.s.Cfg.Desktop || n.s.NotifyPrefs().Scope == "all" // the digest only matters for desktop system notifications
	topics := n.kwTopics(fresh)                                   // outside the lock: reads the DB
	n.mu.Lock()
	defer n.mu.Unlock()
	first := !n.polled[sourceID]
	n.polled[sourceID] = true
	if first {
		return // start-up backlog
	}
	t := now()
	maxAge := envNum("NOTIFY_MAX_AGE", 3*3600)
	watchOn := n.s.WatchPrefs().Notify
	var recent, p0, kw []*Row
	for _, i := range fresh {
		if t-i.Num("published") > maxAge {
			continue
		}
		if k, _ := i.Get("kw").([]string); watchOn && len(k) > 0 {
			// one keyword alert per story; an item alerted here is not repeated as a P0 / digest notification
			if !n.kwDuplicate(i, topics) {
				kw = append(kw, i)
			}
			continue
		}
		recent = append(recent, i)
		if i.Str("priority") == "P0" {
			p0 = append(p0, i)
		}
	}
	n.enqueue(&n.kw, kw)
	n.enqueue(&n.p0, p0)
	if digest {
		n.enqueue(&n.dg, recent)
	}
}

func (s *Store) notifyState() *Row {
	n := s.notif
	n.mu.Lock()
	defer n.mu.Unlock()
	r := &Row{}
	r.Set("queued", len(n.p0.queue))
	r.Set("sent", n.p0.sent)
	r.Set("recent", append([]NotifyBatch{}, n.p0.recent...))
	r.Set("polled", len(n.polled))
	r.Set("digest", obj("queued", len(n.dg.queue), "sent", n.dg.sent))
	r.Set("keyword", obj("queued", len(n.kw.queue), "sent", n.kw.sent, "recent", append([]NotifyBatch{}, n.kw.recent...)))
	return r
}

// NotifyTest sends a batch now (not throttled).
func (s *Store) NotifyTest(items []NotifyItem) *NotifyBatch {
	b := makeBatch(items)
	s.Bus.Publish(Msg{Event: "notify", Batch: b})
	if s.NotifyPrefs().Scope == "all" {
		s.Bus.Publish(Msg{Event: "digest", Batch: b}) // desktop in 全部新内容 mode listens to digests
	}
	return b
}

var _ = os.Getenv
