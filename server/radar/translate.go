package radar

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Fetch-time translation of non-Chinese titles/summaries via DeepSeek (translate.ts).
const trSystem = `把每项的 t(标题) 和 s(摘要) 译成简体中文；人名、产品名、代码、链接、@账号保持原样；不要解释。只输出 JSON：{"r":[{"i":序号,"t":"…","s":"…"}]}`

type translator struct {
	s                                    *Store
	mu                                   sync.Mutex
	queue                                []int64
	queued                               map[int64]bool
	running, done, cached, failed, calls int
	tokIn, tokOut                        int
	lastError                            any
	authFailed, backfilled               bool
	progressPending                      bool
}

func newTranslator(s *Store) *translator { return &translator{s: s, queued: map[int64]bool{}} }

func (t *translator) resetAuth() { t.mu.Lock(); t.authFailed = false; t.lastError = nil; t.mu.Unlock() }

// unavailableReason: why translation can't run right now ("" = it can): no key / rejected key.
// Translation is switched on and off by the user per view (the top-right toggle); the server only needs a key.
// TRANSLATE_ENABLED is informational now (reported as `env`), it no longer blocks the user's switches.
func (s *Store) unavailableReason() string {
	if s.deepseekKey() == "" {
		return "未配置 DeepSeek API key，翻译不可用"
	}
	s.tr.mu.Lock()
	af := s.tr.authFailed
	s.tr.mu.Unlock()
	if af {
		return "DeepSeek API key 无效，请在设置中更新"
	}
	return ""
}

func (s *Store) translateEnabled() bool { return s.unavailableReason() == "" }

// Translation is switched on per VIEW: 「全部」 (settings tr_all), a section (sections.tr) or a single source (sources.tr).
// Effective for an item = any of them.
func (s *Store) trScopes() *Row {
	allOn, _ := s.GetSetting("tr_all")
	secs := []any{}
	if rs, _ := all(s.DB, "SELECT key FROM sections WHERE tr=1 ORDER BY pos"); rs != nil {
		for _, r := range rs {
			secs = append(secs, r.Get("key"))
		}
	}
	srcs := []any{}
	if rs, _ := all(s.DB, "SELECT id FROM sources WHERE tr=1 ORDER BY id"); rs != nil {
		for _, r := range rs {
			srcs = append(srcs, r.Get("id"))
		}
	}
	r := &Row{}
	r.Set("all", allOn == "1")
	r.Set("sections", secs)
	r.Set("sources", srcs)
	return r
}

// setTrScope switches one scope; turning on queues that scope's untranslated items. false = unknown section/source.
func (s *Store) setTrScope(scope string, key any, on bool) bool {
	switch scope {
	case "all":
		v := "0"
		if on {
			v = "1"
		}
		s.SetSetting("tr_all", v)
	case "section":
		res, err := s.DB.Exec("UPDATE sections SET tr=? WHERE key=?", b2i(on), jsString(key))
		if n, _ := res.RowsAffected(); err != nil || n == 0 {
			return false
		}
	default:
		id, _ := key.(float64)
		res, err := s.DB.Exec("UPDATE sources SET tr=? WHERE id=?", b2i(on), int64(id))
		if n, _ := res.RowsAffected(); err != nil || n == 0 {
			return false
		}
	}
	if on {
		s.tr.backfill(true)
	}
	s.Bus.Publish(Msg{Event: "translate"})
	return true
}

func (s *Store) translateStatus() *Row {
	t := s.tr
	r := &Row{}
	reason := s.unavailableReason()
	r.Set("available", reason == "")
	r.Set("enabled", reason == "")
	if reason == "" {
		r.Set("reason", nil)
	} else {
		r.Set("reason", reason)
	}
	if v, ok := os.LookupEnv("TRANSLATE_ENABLED"); ok {
		r.Set("env", v)
	} else {
		r.Set("env", nil)
	}
	r.Set("scopes", s.trScopes())
	t.mu.Lock()
	r.Set("queued", len(t.queue))
	r.Set("running", t.running)
	r.Set("done", t.done)
	r.Set("cached", t.cached)
	r.Set("failed", t.failed)
	r.Set("calls", t.calls)
	r.Set("tokensIn", t.tokIn)
	r.Set("tokensOut", t.tokOut)
	r.Set("lastError", t.lastError)
	t.mu.Unlock()
	r.Set("model", s.deepseekModel())
	return r
}

var (
	cjkRe   = regexp.MustCompile(`[\x{3400}-\x{9fff}\x{3040}-\x{30ff}\x{ac00}-\x{d7af}]`)
	latinRe = regexp.MustCompile(`[A-Za-z\x{00C0}-\x{024F}\x{0400}-\x{04FF}]`)
)

// NeedsTranslation: mostly non-CJK (latin/cyrillic letters outnumber CJK 2:1).
func NeedsTranslation(title, summary string) bool {
	if len(cjkRe.FindAllStringIndex(title, -1)) >= 4 {
		return false
	}
	x := jsSlice(title+" "+summary, 500)
	l, c := len(latinRe.FindAllStringIndex(x, -1)), len(cjkRe.FindAllStringIndex(x, -1))
	return l > 8 && l > c*2
}

func hashOf(t, s string) string {
	h := sha1.Sum([]byte(t + "\x00" + s))
	return base64.RawURLEncoding.EncodeToString(h[:])[:22]
}

// effectiveOn: 「全部」 OR the item's section OR its source
func (t *translator) effectiveOn() string {
	if v, _ := t.s.GetSetting("tr_all"); v == "1" {
		return "1=1"
	}
	return "(s.tr=1 OR EXISTS(SELECT 1 FROM sections sec WHERE sec.key=s.section AND sec.tr=1))"
}

// translatableIDs keeps the ids in a switched-on view.
func (t *translator) translatableIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rs, _ := all(t.s.DB, "SELECT i.id FROM items i JOIN sources s ON s.id=i.source_id WHERE "+t.effectiveOn()+" AND i.id IN ("+placeholders(len(ids))+")", args...)
	out := make([]int64, len(rs))
	for i, r := range rs {
		out[i] = int64(r.Num("id"))
	}
	return out
}

func (t *translator) enqueue(ids []int64) {
	if len(ids) == 0 || !t.s.translateEnabled() {
		return
	}
	ids = t.translatableIDs(ids)
	t.mu.Lock()
	for _, id := range ids {
		if !t.queued[id] {
			t.queued[id] = true
			t.queue = append(t.queue, id)
		}
	}
	t.mu.Unlock()
	t.notifyProgress()
	t.pump()
}

// notifyProgress publishes a throttled SSE "translate" event (queue / progress changed), at most every 700 ms.
func (t *translator) notifyProgress() {
	t.mu.Lock()
	if t.progressPending {
		t.mu.Unlock()
		return
	}
	t.progressPending = true
	t.mu.Unlock()
	time.AfterFunc(700*time.Millisecond, func() {
		t.mu.Lock()
		t.progressPending = false
		t.mu.Unlock()
		t.s.Bus.Publish(Msg{Event: "translate"})
	})
}

// backfill queues every retained item that still lacks a translation (once per process / when re-enabled).
func (t *translator) backfill(force bool) {
	if !t.s.translateEnabled() {
		return
	}
	t.mu.Lock()
	if t.backfilled && !force {
		t.mu.Unlock()
		return
	}
	t.backfilled = true
	t.mu.Unlock()
	rs, _ := all(t.s.DB, "SELECT i.id FROM items i JOIN sources s ON s.id=i.source_id WHERE i.title_zh IS NULL AND i.merged_into IS NULL AND "+t.effectiveOn()+" ORDER BY i.published DESC")
	ids := make([]int64, len(rs))
	for i, r := range rs {
		ids[i] = int64(r.Num("id"))
	}
	t.enqueue(ids)
}

func (t *translator) pump() {
	batch, conc := max(1, envInt("TRANSLATE_BATCH", 10)), max(1, envInt("TRANSLATE_CONCURRENCY", 2))
	for {
		t.mu.Lock()
		if t.running >= conc || len(t.queue) == 0 {
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()
		if !t.s.translateEnabled() {
			return
		}
		t.mu.Lock()
		n := min(batch, len(t.queue))
		ids := append([]int64(nil), t.queue[:n]...)
		t.queue = t.queue[n:]
		t.running++
		t.mu.Unlock()
		go func() {
			if err := t.run(ids); err != nil {
				t.mu.Lock()
				t.failed += len(ids)
				t.lastError = err.Error()
				t.mu.Unlock()
				log.Println("[translate]", err)
			}
			t.mu.Lock()
			t.running--
			for _, i := range ids {
				delete(t.queued, i)
			}
			t.mu.Unlock()
			t.notifyProgress()
			t.pump()
		}()
	}
}

type trIn struct {
	I int    `json:"i"`
	T string `json:"t"`
	S string `json:"s"`
}
type trOut struct {
	I int     `json:"i"`
	T *string `json:"t"`
	S *string `json:"s"`
}

func (t *translator) callAPI(batch []trIn) ([]trOut, error) {
	in, _ := marshalNoEscape(batch)
	body, _ := json.Marshal(map[string]any{"model": t.s.deepseekModel(), "temperature": 0.2, "response_format": map[string]string{"type": "json_object"}, "max_tokens": 6000,
		"messages": []map[string]string{{"role": "system", "content": trSystem}, {"role": "user", "content": string(in)}}})
	for attempt := 0; ; attempt++ {
		t.mu.Lock()
		t.calls++
		t.mu.Unlock()
		r, err := netFetch(context.Background(), "POST", t.s.deepseekBase()+"/chat/completions",
			map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + t.s.deepseekKey()}, bytes.NewReader(body), 90*time.Second)
		if err != nil {
			if attempt < 3 {
				time.Sleep(2 * time.Second << attempt)
				continue
			}
			return nil, err
		}
		if r.Status == 401 || r.Status == 402 {
			t.mu.Lock()
			t.authFailed = r.Status == 401
			t.mu.Unlock()
			why := "余额不足"
			if r.Status == 401 {
				why = "API key 无效"
			}
			return nil, fmt.Errorf("DeepSeek HTTP %d（%s）", r.Status, why)
		}
		if (r.Status == 429 || r.Status >= 500) && attempt < 3 {
			ra, _ := strconv.Atoi(r.Header.Get("Retry-After"))
			d := time.Duration(ra) * time.Second
			if d == 0 {
				d = 2 * time.Second << attempt
			}
			time.Sleep(d)
			continue
		}
		if !r.OK() {
			return nil, fmt.Errorf("DeepSeek HTTP %d: %s", r.Status, jsSlice(string(r.Body), 200))
		}
		var j struct {
			Usage struct {
				P int `json:"prompt_tokens"`
				C int `json:"completion_tokens"`
			} `json:"usage"`
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(r.Body, &j); err != nil {
			return nil, &JSError{Name: "SyntaxError", Msg: "Unexpected token in JSON"}
		}
		t.mu.Lock()
		t.tokIn += j.Usage.P
		t.tokOut += j.Usage.C
		t.mu.Unlock()
		txt := ""
		if len(j.Choices) > 0 {
			txt = strings.TrimSpace(j.Choices[0].Message.Content)
		}
		var out struct {
			R []trOut `json:"r"`
		}
		if json.Unmarshal([]byte(txt), &out) != nil {
			if attempt < 1 {
				continue
			}
			return nil, fmt.Errorf("DeepSeek 返回的不是合法 JSON")
		}
		return out.R, nil
	}
}

func (t *translator) run(ids []int64) error {
	if ids = t.translatableIDs(ids); len(ids) == 0 { // the switch may have been turned off while queued
		return nil
	}
	return t.runIDs(ids)
}

// runIDs translates the given items regardless of the view switches (the brief's 「翻译」 option uses it directly).
func (t *translator) runIDs(ids []int64) error {
	db := t.s.DB
	capN := envInt("TRANSLATE_SUMMARY_CHARS", 1500)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, _ := all(db, "SELECT id,title,summary FROM items WHERE title_zh IS NULL AND id IN ("+placeholders(len(ids))+")", args...)
	type todoT struct {
		id      any
		h, t, s string
	}
	var doneIDs []int64
	var todo []todoT
	for _, r := range rows {
		ti, su := r.Str("title"), jsSlice(r.Str("summary"), capN)
		if !NeedsTranslation(ti, su) {
			db.Exec("UPDATE items SET title_zh=?, summary_zh=? WHERE id=?", "", "", r.Get("id"))
			continue
		}
		h := hashOf(ti, su)
		var ct, cs string
		if db.QueryRow("SELECT t,s FROM tcache WHERE h=?", h).Scan(&ct, &cs) == nil {
			db.Exec("UPDATE items SET title_zh=?, summary_zh=? WHERE id=?", ct, cs, r.Get("id"))
			t.mu.Lock()
			t.cached++
			t.mu.Unlock()
			doneIDs = append(doneIDs, int64(r.Num("id")))
		} else {
			todo = append(todo, todoT{r.Get("id"), h, ti, su})
		}
	}
	if len(todo) > 0 {
		in := make([]trIn, len(todo))
		for i, x := range todo {
			in[i] = trIn{i, x.t, x.s}
		}
		out, err := t.callAPI(in)
		if err != nil {
			return err
		}
		ts := now()
		ok := 0
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		for _, o := range out {
			if o.I < 0 || o.I >= len(todo) || o.T == nil || *o.T == "" {
				continue
			}
			x := todo[o.I]
			sv := ""
			if x.s != "" && o.S != nil {
				sv = *o.S
			}
			tt := jsSlice(*o.T, 400)
			tx.Exec("UPDATE items SET title_zh=?, summary_zh=? WHERE id=?", tt, jsSlice(sv, capN*2), x.id)
			tx.Exec("INSERT OR REPLACE INTO tcache(h,t,s,ts) VALUES(?,?,?,?)", x.h, tt, sv, ts)
			ok++
			if id, isInt := x.id.(int64); isInt {
				doneIDs = append(doneIDs, id)
			}
		}
		tx.Commit()
		t.mu.Lock()
		t.done += ok
		t.failed += len(todo) - ok
		t.lastError = nil
		t.mu.Unlock()
	}
	if len(doneIDs) > 0 {
		w, a := idsWhere(doneIDs)
		for _, it := range t.s.QueryItems(w, a, len(doneIDs), false, true) {
			t.s.Bus.Publish(Msg{Event: "update", Item: it})
		}
	}
	return nil
}
