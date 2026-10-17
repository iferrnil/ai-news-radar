package radar

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

// 关键词: the user's watch keywords (setting `watch`). Every stored item is matched once (items.kw = the matched
// keywords, "\n"-joined; '' = checked, no match) — on insert, after a translation lands and for all retained items
// when the list changes. Cards show the matched keywords as a tag; GET /api/items?kw=<keyword>|* filters to them.
// (0.12.0 also sent keyword notifications; 0.13.0 dropped that channel — a stored `notify` flag is ignored.)

type WatchPrefs struct {
	Keywords      []string `json:"keywords"`
	Scope         string   `json:"scope"` // "all" (title + content, default) | "title"
	CaseSensitive bool     `json:"caseSensitive"`
}

const (
	watchMaxKeywords = 50
	watchMaxLen      = 40
)

func (s *Store) WatchPrefs() WatchPrefs {
	p := WatchPrefs{Keywords: []string{}, Scope: "all"}
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

// SetWatchPrefs merges a PATCH {keywords?, scope?, caseSensitive?}; returns whether matching changed.
func (s *Store) SetWatchPrefs(m map[string]any) (WatchPrefs, bool) {
	cur := s.WatchPrefs()
	next := cur
	if v, ok := m["caseSensitive"].(bool); ok {
		next.CaseSensitive = v
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
