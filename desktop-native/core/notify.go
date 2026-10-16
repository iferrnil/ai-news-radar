package core

import (
	"encoding/json"
	"slices"
	"sort"
	"sync"
	"time"
)

type NotifyItem struct {
	ID        int64
	Title     string
	Source    string
	Section   string
	URL       string
	Published float64
}
type NotifyBatch struct {
	At       time.Time
	Count    int
	Items    []NotifyItem
	Sections []string
}

// NotifyPrefs: sections empty = all. Same JSON as the web app's settings row "notify".
type NotifyPrefs struct {
	Enabled  bool     `json:"enabled"`
	Sections []string `json:"sections"`
}

const (
	notifyMaxAge = 3 * 3600
	notifyBatch  = 20 * time.Second
	notifyGap    = 60 * time.Second
)

type notifier struct {
	s      *Store
	mu     sync.Mutex
	polled map[int64]bool
	queue  []NotifyItem
	timer  *time.Timer
	last   time.Time
	// overridable in tests
	batchWait, minGap time.Duration
}

func newNotifier(s *Store) *notifier {
	return &notifier{s: s, polled: map[int64]bool{}, batchWait: notifyBatch, minGap: notifyGap}
}

func (s *Store) NotifyPrefs() NotifyPrefs {
	p := NotifyPrefs{Enabled: true} // desktop: on by default
	if v, ok := s.GetSetting("notify"); ok {
		json.Unmarshal([]byte(v), &p)
	}
	return p
}
func (s *Store) SetNotifyPrefs(p NotifyPrefs) error {
	b, _ := json.Marshal(p)
	return s.SetSetting("notify", string(b))
}

// onPolled is called after each successful poll with the newly stored items.
func (n *notifier) onPolled(sourceID int64, fresh []Item) {
	n.mu.Lock()
	defer n.mu.Unlock()
	first := !n.polled[sourceID]
	n.polled[sourceID] = true
	if first {
		return // start-up backlog
	}
	prefs := n.s.NotifyPrefs()
	if !prefs.Enabled {
		return
	}
	t := now()
	added := false
	for _, i := range fresh {
		if i.Priority != "P0" || t-i.Published > notifyMaxAge {
			continue
		}
		if len(prefs.Sections) > 0 && !slices.Contains(prefs.Sections, i.Section) {
			continue
		}
		if slices.ContainsFunc(n.queue, func(q NotifyItem) bool { return q.ID == i.ID }) {
			continue
		}
		title := i.TitleZh
		if title == "" {
			title = i.Title
		}
		n.queue = append(n.queue, NotifyItem{i.ID, Trunc(title, 200, ""), i.Source, i.Section, i.URL, i.Published})
		added = true
	}
	if added && n.timer == nil {
		n.timer = time.AfterFunc(n.batchWait, n.flush)
	}
}

func (n *notifier) flush() {
	n.mu.Lock()
	n.timer = nil
	if len(n.queue) == 0 {
		n.mu.Unlock()
		return
	}
	if wait := time.Until(n.last.Add(n.minGap)); wait > 0 {
		n.timer = time.AfterFunc(wait, n.flush)
		n.mu.Unlock()
		return
	}
	all := n.queue
	n.queue = nil
	sort.Slice(all, func(a, b int) bool { return all[a].Published > all[b].Published })
	secs := map[string]bool{}
	for _, i := range all {
		secs[i.Section] = true
	}
	b := &NotifyBatch{At: time.Now(), Count: len(all), Items: all[:min(5, len(all))], Sections: sortedKeys(secs)}
	n.last = time.Now()
	n.mu.Unlock()
	n.s.emit(Event{Kind: "notify", Batch: b})
}
