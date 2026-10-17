package radar

import (
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// 关键词提醒: the user's watch keywords (setting `watch`). Every stored item is matched once (items.kw = the matched
// keywords, "\n"-joined; '' = checked, no match) — on insert, after a translation lands and for all retained items
// when the list changes. The feed highlights items with kw; new matches go to the notifier's `keyword` channel
// (native notification on desktop, opt-in web notification / toast on the web), deduped per item and per story.

type WatchPrefs struct {
	Keywords      []string `json:"keywords"`
	Scope         string   `json:"scope"` // "all" (title + content, default) | "title"
	CaseSensitive bool     `json:"caseSensitive"`
	Notify        bool     `json:"notify"`
}

const (
	watchMaxKeywords = 50
	watchMaxLen      = 40
)

func (s *Store) WatchPrefs() WatchPrefs {
	p := WatchPrefs{Keywords: []string{}, Scope: "all", Notify: true}
	if v, ok := s.GetSetting("watch"); ok {
		json.Unmarshal([]byte(v), &p)
	}
	if p.Keywords == nil {
		p.Keywords = []string{}
	}
	if p.Scope != "title" {
		p.Scope = "all"
	}
	return p
}

// cleanKeywords trims, drops empties / control chars / duplicates (case-insensitively unless cs) and caps the list.
func cleanKeywords(in []any, cs bool) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, x := range in {
		k, _ := x.(string)
		k = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, k)), " ")
		if k == "" || utf8.RuneCountInString(k) > watchMaxLen {
			continue
		}
		key := k
		if !cs {
			key = strings.ToLower(k)
		}
		if seen[key] || len(out) >= watchMaxKeywords {
			continue
		}
		seen[key] = true
		out = append(out, k)
	}
	return out
}

// SetWatchPrefs merges a PATCH {keywords?, scope?, caseSensitive?, notify?}; returns whether matching changed.
func (s *Store) SetWatchPrefs(m map[string]any) (WatchPrefs, bool) {
	cur := s.WatchPrefs()
	next := cur
	if v, ok := m["caseSensitive"].(bool); ok {
		next.CaseSensitive = v
	}
	if v, ok := m["notify"].(bool); ok {
		next.Notify = v
	}
	if v, ok := m["scope"].(string); ok && (v == "all" || v == "title") {
		next.Scope = v
	}
	if v, ok := m["keywords"].([]any); ok {
		next.Keywords = cleanKeywords(v, next.CaseSensitive)
	}
	b, _ := json.Marshal(next)
	s.SetSetting("watch", string(b))
	changed := next.Scope != cur.Scope || next.CaseSensitive != cur.CaseSensitive || strings.Join(next.Keywords, "\n") != strings.Join(cur.Keywords, "\n")
	return next, changed
}

type watcher struct {
	kws   []string
	low   []string
	cs    bool
	title bool
}

func (p WatchPrefs) matcher() *watcher {
	if len(p.Keywords) == 0 {
		return nil
	}
	w := &watcher{kws: p.Keywords, cs: p.CaseSensitive, title: p.Scope == "title"}
	for _, k := range p.Keywords {
		w.low = append(w.low, strings.ToLower(k))
	}
	return w
}

func isWordRune(r rune) bool { return r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r)) }

// containsWord: substring match; a keyword that starts / ends with an ASCII letter or digit must not continue
// an ASCII word there ("GPT-6" ≠ "GPT-60", "Claude" ≠ "Claudette"; CJK keywords are plain substrings).
func containsWord(text, kw string) bool {
	if kw == "" {
		return false
	}
	first, _ := utf8.DecodeRuneInString(kw)
	last, _ := utf8.DecodeLastRuneInString(kw)
	for off := 0; off <= len(text)-len(kw); {
		i := strings.Index(text[off:], kw)
		if i < 0 {
			return false
		}
		i += off
		ok := true
		if isWordRune(first) && i > 0 {
			if r, _ := utf8.DecodeLastRuneInString(text[:i]); isWordRune(r) {
				ok = false
			}
		}
		if ok && isWordRune(last) && i+len(kw) < len(text) {
			if r, _ := utf8.DecodeRuneInString(text[i+len(kw):]); isWordRune(r) {
				ok = false
			}
		}
		if ok {
			return true
		}
		_, sz := utf8.DecodeRuneInString(text[i:])
		off = i + sz
	}
	return false
}

// match returns the keywords found in the item's texts (title first; content only for scope "all").
func (w *watcher) match(title string, body ...string) []string {
	text := title
	if !w.title {
		text = strings.Join(append([]string{title}, body...), "\n")
	}
	if !w.cs {
		text = strings.ToLower(text)
	}
	var out []string
	for i, k := range w.kws {
		kk := k
		if !w.cs {
			kk = w.low[i]
		}
		if containsWord(text, kk) {
			out = append(out, k)
		}
	}
	return out
}

const watchCols = "id, title, summary, content, title_zh, summary_zh"

// applyWatch (re)matches the given item ids (nil = every item whose kw is NULL).
func (s *Store) applyWatch(ids []int64) {
	w := s.WatchPrefs().matcher()
	q := "SELECT " + watchCols + " FROM items WHERE kw IS NULL"
	var args []any
	if ids != nil {
		if len(ids) == 0 {
			return
		}
		q = "SELECT " + watchCols + " FROM items WHERE id IN (" + placeholders(len(ids)) + ")"
		for _, id := range ids {
			args = append(args, id)
		}
	}
	rs, err := all(s.DB, q, args...)
	if err != nil || len(rs) == 0 {
		return
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	for _, r := range rs {
		kw := ""
		if w != nil {
			kw = strings.Join(w.match(r.Str("title")+"\n"+r.Str("title_zh"), r.Str("summary"), r.Str("content"), r.Str("summary_zh")), "\n")
		}
		tx.Exec("UPDATE items SET kw=? WHERE id=?", kw, r.Get("id"))
	}
	tx.Commit()
}

// rematchAll: after the keyword list / options change. Publishes `watch` so clients reload their lists.
func (s *Store) rematchAll() {
	s.DB.Exec("UPDATE items SET kw=NULL")
	s.applyWatch(nil)
	s.Bus.Publish(Msg{Event: "watch", Data: obj("at", now())})
}

func kwList(v string) []string {
	if v == "" {
		return nil
	}
	return strings.Split(v, "\n")
}

// ---- notification dedupe: one alert per item, and per story (same URL / near-identical title) for 24 h

type kwSeen struct {
	key briefKey
	at  time.Time
}

// kwTopics maps item id → the ids of its 热点 topic (other sources' reports of the same event), computed only
// when a poll brings keyword matches.
func (n *notifier) kwTopics(fresh []*Row) map[int64][]int64 {
	for _, i := range fresh {
		if k, _ := i.Get("kw").([]string); len(k) > 0 {
			m := map[int64][]int64{}
			for _, tp := range n.s.hotTopics(now()) {
				for _, id := range tp.Items {
					m[id] = tp.Items
				}
			}
			return m
		}
	}
	return nil
}

func (n *notifier) kwDuplicate(r *Row, topics map[int64][]int64) bool {
	k := keyOf(r)
	id := int64(r.Num("id"))
	cut := time.Now().Add(-24 * time.Hour)
	if n.kwIDs == nil {
		n.kwIDs = map[int64]time.Time{}
	}
	for x, at := range n.kwIDs {
		if at.Before(cut) {
			delete(n.kwIDs, x)
		}
	}
	_, seenItem := n.kwIDs[id]
	topicSeen := false
	for _, peer := range topics[id] {
		if _, ok := n.kwIDs[peer]; ok && peer != id {
			topicSeen = true
			break
		}
	}
	n.kwIDs[id] = time.Now()
	if seenItem || topicSeen {
		return true
	}
	keep := n.kwSeen[:0]
	dup := false
	for _, x := range n.kwSeen {
		if x.at.Before(cut) {
			continue
		}
		keep = append(keep, x)
		if !dup && sameStory(k, x.key) {
			dup = true
		}
	}
	n.kwSeen = keep
	if !dup {
		n.kwSeen = append(n.kwSeen, kwSeen{k, time.Now()})
	}
	return dup
}
