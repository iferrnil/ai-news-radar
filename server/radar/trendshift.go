package radar

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Trendshift (trendshift.io) daily / weekly GitHub repository rankings. Trendshift has no feed or free API, so:
//  1. primary: the static JSON that the third-party project quantavil/trendshift syncs every day to GitHub Pages
//     (a list of snapshots: period_key, rank, full_name, description, language, stars_gained, …; only the latest
//     period is used, ordered by rank);
//  2. fallback: the ranking page itself (server-rendered; its JSON-LD ItemList is the ranked list — other blocks of
//     the page are ignored).
// Both failing or yielding nothing is an error for the source; stored items are never removed by a failed poll.
// A source is type "api" with the page URL (https://trendshift.io/ = daily, https://trendshift.io/weekly = weekly).
// The uid is per repository and period, so re-polls and the two paths never duplicate a repository.

var (
	trendshiftRe   = regexp.MustCompile(`^(?:https?://)?(?:www\.)?trendshift\.io/?(weekly)?/?(?:[?#].*)?$`)
	trendshiftLDRe = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
	// overridable in tests
	trendshiftJSON = map[string]string{
		"daily":  "https://quantavil.github.io/trendshift/data/daily/daily-all.json",
		"weekly": "https://quantavil.github.io/trendshift/data/weekly/weekly-all.json",
	}
)

const trendshiftBrowserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// trendshiftKind: "daily" / "weekly" for a Trendshift ranking URL.
func trendshiftKind(u string) (string, bool) {
	m := trendshiftRe.FindStringSubmatch(strings.TrimSpace(u))
	if m == nil {
		return "", false
	}
	if m[1] != "" {
		return "weekly", true
	}
	return "daily", true
}

func trendshiftPage(kind string) string {
	if kind == "weekly" {
		return "https://trendshift.io/weekly"
	}
	return "https://trendshift.io/"
}

// sourceInterval: the poll interval of a new source — the priority tier, except rankings that change daily
// (Trendshift: every 3 h / 6 h; the JSON is synced once a day).
func sourceInterval(d *Detected, priority string) int {
	if kind, ok := trendshiftKind(d.URL); ok && d.Type == "api" {
		return map[string]int{"daily": 10800, "weekly": 21600}[kind]
	}
	return IntervalFor(d.Type, priority)
}

var trendshiftName = map[string]string{"daily": "Trendshift 日榜", "weekly": "Trendshift 周榜"}

type tsRepo struct {
	Period      string   `json:"period_key"`
	Rank        int      `json:"rank"`
	FullName    string   `json:"full_name"`
	URL         string   `json:"github_url"`
	Description string   `json:"description"`
	Language    string   `json:"language"`
	Stars       int      `json:"stars_total"`
	StarsGained int      `json:"stars_gained"`
	Forks       int      `json:"forks_total"`
	Created     string   `json:"created_at"`
	Tags        []string `json:"tags"`
	Social      []string `json:"social_mention_platforms"`
	ID          string   `json:"-"` // trendshift repository id (HTML only)
}

// cond: conditional GET (polls); the add-source probe reads the file in full so the first poll isn't a 304.
func fetchTrendshift(ctx context.Context, kind string, cond bool) ([]FetchedItem, error) {
	items, perr := trendshiftPrimary(ctx, kind, cond)
	if perr == nil && items != nil {
		return items, nil
	}
	if perr == nil { // 304: nothing new
		return nil, nil
	}
	log.Printf("[trendshift] %s: JSON failed (%v), trying trendshift.io", kind, perr)
	r, err := httpGet(ctx, trendshiftPage(kind), 20*time.Second, 2, map[string]string{"User-Agent": trendshiftBrowserUA, "Accept": "text/html,application/xhtml+xml"})
	if err == nil && !r.OK() {
		err = fetchErr("http", "HTTP %d", r.Status)
	}
	if err == nil {
		if items, err = ParseTrendshiftHTML(kind, r.Body, now()); err == nil {
			return items, nil
		}
	}
	log.Printf("[trendshift] %s: page fallback failed too (%v); keeping stored items", kind, err)
	return nil, fetchErr(kindOf(err), "Trendshift：JSON %s；页面 %s", perr.Error(), err.Error())
}

// trendshiftPrimary: (nil, nil) = not modified.
func trendshiftPrimary(ctx context.Context, kind string, cond bool) ([]FetchedItem, error) {
	h := map[string]string{"Accept": "application/json"}
	var r *Resp
	var err error
	if cond {
		r, err = condGet(ctx, trendshiftJSON[kind], h)
	} else {
		r, err = httpGet(ctx, trendshiftJSON[kind], 30*time.Second, 2, h)
	}
	if err != nil {
		return nil, err
	}
	if r.Status == 304 {
		return nil, nil
	}
	if !r.OK() {
		return nil, fetchErr("http", "HTTP %d", r.Status)
	}
	at := now()
	if t, err := http.ParseTime(r.Header.Get("Last-Modified")); err == nil && float64(t.Unix()) < at {
		at = float64(t.Unix()) // when the snapshot was synced
	}
	items, err := ParseTrendshiftJSON(kind, r.Body, at)
	if err != nil {
		valMu.Lock()
		delete(validators, trendshiftJSON[kind]) // a broken file must not turn into 304s later
		valMu.Unlock()
	}
	return items, err
}

// ParseTrendshiftJSON: the latest period of a quantavil/trendshift *-all.json, by rank. at = snapshot time.
func ParseTrendshiftJSON(kind string, raw []byte, at float64) ([]FetchedItem, error) {
	var all []tsRepo
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, fetchErr("parse", "Trendshift JSON 无法解析")
	}
	latest := ""
	for _, x := range all {
		if x.Period > latest {
			latest = x.Period
		}
	}
	var cur []tsRepo
	seen := map[string]bool{}
	for _, x := range all {
		k := strings.ToLower(x.FullName)
		if x.Period == latest && x.FullName != "" && !seen[k] {
			seen[k] = true
			cur = append(cur, x)
		}
	}
	if len(cur) == 0 {
		return nil, fetchErr("parse", "Trendshift JSON 没有条目")
	}
	sort.SliceStable(cur, func(i, j int) bool { return cur[i].Rank < cur[j].Rank })
	return trendshiftItems(kind, cur, at), nil
}

type tsLD struct {
	Type     string `json:"@type"`
	Name     string `json:"name"`
	Elements []struct {
		Position int    `json:"position"`
		URL      string `json:"url"`
		Item     struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			URL         string   `json:"url"`
			Repo        string   `json:"codeRepository"`
			Language    string   `json:"programmingLanguage"`
			Created     string   `json:"dateCreated"`
			Modified    string   `json:"dateModified"`
			Keywords    []string `json:"keywords"`
		} `json:"item"`
	} `json:"itemListElement"`
}

var tsRepoName = regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)

// ParseTrendshiftHTML: the ranked list of a trendshift.io page (its JSON-LD ItemList of repositories).
func ParseTrendshiftHTML(kind string, body []byte, at float64) ([]FetchedItem, error) {
	var best *tsLD
	for _, m := range trendshiftLDRe.FindAllSubmatch(body, -1) {
		var d tsLD
		if json.Unmarshal(m[1], &d) != nil || d.Type != "ItemList" || len(d.Elements) == 0 {
			continue
		}
		if best == nil || len(d.Elements) > len(best.Elements) {
			best = &d
		}
	}
	if best == nil {
		return nil, fetchErr("parse", "页面里没有排行榜")
	}
	var cur []tsRepo
	seen := map[string]bool{}
	for _, e := range best.Elements {
		name := strings.TrimSpace(e.Item.Name)
		if !tsRepoName.MatchString(name) || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		u := firstNonEmpty(e.Item.Repo, e.Item.URL)
		if !strings.HasPrefix(u, "https://github.com/") {
			u = "https://github.com/" + name
		}
		id := ""
		if i := strings.LastIndex(e.URL, "/repositories/"); i >= 0 {
			id = e.URL[i+len("/repositories/"):]
		}
		cur = append(cur, tsRepo{Period: tsPeriod(kind, e.Item.Modified, at), Rank: e.Position, FullName: name, URL: u, Description: e.Item.Description,
			Language: e.Item.Language, Created: e.Item.Created, Tags: e.Item.Keywords, ID: id})
	}
	if len(cur) == 0 {
		return nil, fetchErr("parse", "排行榜没有条目")
	}
	sort.SliceStable(cur, func(i, j int) bool { return cur[i].Rank < cur[j].Rank })
	return trendshiftItems(kind, cur, at), nil
}

// tsPeriod: the period key the JSON uses ("2026-10-18" / "2026-W42") from the page's dateModified (else at).
func tsPeriod(kind, modified string, at float64) string {
	t, err := time.Parse(time.RFC3339, modified)
	if err != nil {
		t = time.Unix(int64(at), 0).UTC()
	}
	t = t.UTC()
	if kind == "weekly" {
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w)
	}
	return t.Format("2006-01-02")
}

func groupInt(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// trendshiftItems: title owner/repo, summary "#1 · TypeScript · 今日 +634 星 · description", link to GitHub.
// published = at − rank seconds, so the feed (newest first) keeps the ranking order.
func trendshiftItems(kind string, rs []tsRepo, at float64) []FetchedItem {
	if len(rs) > trendshiftMax {
		rs = rs[:trendshiftMax] // rs is in rank order
	}
	label, gained := "日榜", "今日"
	if kind == "weekly" {
		label, gained = "周榜", "本周"
	}
	out := make([]FetchedItem, 0, len(rs))
	for i, r := range rs {
		rank := r.Rank
		if rank <= 0 {
			rank = i + 1
		}
		meta := []string{"#" + strconv.Itoa(rank)}
		if r.Language != "" {
			meta = append(meta, r.Language)
		}
		if r.StarsGained > 0 {
			meta = append(meta, gained+" +"+groupInt(r.StarsGained)+" 星")
		}
		desc := Clean(r.Description, 240)
		sum := strings.Join(meta, " · ")
		if desc != "" {
			sum += " · " + desc
		}
		var c strings.Builder
		fmt.Fprintf(&c, "Trendshift %s第 %d 名（%s）：%s\n\n", label, rank, r.Period, r.FullName)
		if desc != "" {
			c.WriteString(desc + "\n\n")
		}
		if r.Language != "" {
			c.WriteString("语言：" + r.Language + "\n")
		}
		if r.Stars > 0 || r.StarsGained > 0 {
			fmt.Fprintf(&c, "星标：%s（%s +%s）\n", groupInt(r.Stars), gained, groupInt(r.StarsGained))
		}
		if r.Forks > 0 {
			c.WriteString("Fork：" + groupInt(r.Forks) + "\n")
		}
		if len(r.Created) >= 10 {
			c.WriteString("创建于 " + r.Created[:10] + "\n")
		}
		if len(r.Tags) > 0 {
			c.WriteString("标签：" + strings.Join(r.Tags, "、") + "\n")
		}
		if len(r.Social) > 0 {
			c.WriteString("讨论平台：" + strings.Join(r.Social, "、") + "\n")
		}
		c.WriteString(r.URL)
		owner, _, _ := strings.Cut(r.FullName, "/")
		out = append(out, FetchedItem{UID: "trendshift:" + kind + ":" + r.Period + ":" + strings.ToLower(r.FullName), Title: r.FullName, URL: r.URL,
			Summary: sum, Published: int64(at) - int64(rank), Author: owner, Content: strings.TrimSpace(c.String())})
	}
	return out
}

// trendshiftMax: each Trendshift source keeps the top 25 of its list (JSON or page).
const trendshiftMax = 25

// trendshiftDedupSQL (feed / section lists, not 收藏 or ?id=): a repo shows once per section. When 日榜 and 周榜 (or
// two periods of one list) both hold it in the same section, the daily one wins over the weekly one, then the
// newest period; the loser is hidden, so moving a source to another section brings its items back. The twin must
// itself be in the window (arg: the window start). Titles are owner/repo; uid = trendshift:<kind>:<period>:<repo>
// (substr 12..17 = the kind); the uid range keeps the lookup on the uid index.
const trendshiftDedupSQL = `(i.uid NOT BETWEEN 'trendshift:' AND 'trendshift;' OR NOT EXISTS (
  SELECT 1 FROM items j JOIN sources sj ON sj.id=j.source_id
  WHERE j.uid BETWEEN 'trendshift:' AND 'trendshift;' AND j.id<>i.id AND j.hidden=0 AND j.published >= ? AND sj.enabled=1
    AND sj.section=s.section AND lower(j.title) = lower(i.title)
    AND ((substr(j.uid, 12, 6)='daily:' AND substr(i.uid, 12, 6)<>'daily:')
      OR (substr(j.uid, 12, 6)=substr(i.uid, 12, 6) AND (j.published > i.published OR (j.published = i.published AND j.id > i.id))))))`

// trendshiftVisible: true when id would appear in a feed/section list under trendshiftDedupSQL.
func (s *Store) trendshiftVisible(id int64, cut float64) bool {
	var one int
	err := s.DB.QueryRow(`SELECT 1 FROM items i JOIN sources s ON s.id=i.source_id
		WHERE i.id=? AND i.merged_into IS NULL AND i.hidden=0 AND s.enabled=1 AND i.published>=?
		AND `+trendshiftDedupSQL, id, cut, cut).Scan(&one)
	return err == nil
}

// trendshiftHiddenTwins: other same-section Trendshift rows for the same repo that the listing SQL hides
// while winnerID stays visible. Used so SSE "removed" drops the weekly twin when the daily one arrives live.
func (s *Store) trendshiftHiddenTwins(winnerID int64, cut float64) []int64 {
	rs, err := all(s.DB, `SELECT i.id FROM items i JOIN sources s ON s.id=i.source_id
		JOIN items w ON w.id=? JOIN sources sw ON sw.id=w.source_id
		WHERE i.id<>w.id AND i.uid BETWEEN 'trendshift:' AND 'trendshift;' AND i.merged_into IS NULL AND i.hidden=0
			AND i.published>=? AND s.enabled=1 AND s.section=sw.section AND lower(i.title)=lower(w.title)`, winnerID, cut)
	if err != nil || len(rs) == 0 {
		return nil
	}
	var out []int64
	for _, r := range rs {
		id := int64(r.Num("id"))
		if !s.trendshiftVisible(id, cut) {
			out = append(out, id)
		}
	}
	return out
}

// publishFreshItems: SSE "item" for new rows. Trendshift losers (same-section daily/weekly twin) are not
// broadcast — listing SQL already hides them on reload, but live pending/merge used to reintroduce dupes.
// When a winner is published, emit "removed" for twins it displaces so the open feed drops them immediately.
func (s *Store) publishFreshItems(fresh []*Row) {
	if len(fresh) == 0 {
		return
	}
	cut := now() - retentionHours()*3600
	seenRemove := map[int64]bool{}
	for _, r := range fresh {
		id := int64(r.Num("id"))
		var uid string
		_ = s.DB.QueryRow("SELECT uid FROM items WHERE id=?", id).Scan(&uid)
		if strings.HasPrefix(uid, "trendshift:") {
			if !s.trendshiftVisible(id, cut) {
				continue
			}
			for _, lid := range s.trendshiftHiddenTwins(id, cut) {
				if seenRemove[lid] {
					continue
				}
				seenRemove[lid] = true
				s.Bus.Publish(Msg{Event: "removed", ID: lid})
			}
		}
		s.Bus.Publish(Msg{Event: "item", Item: r})
	}
}
