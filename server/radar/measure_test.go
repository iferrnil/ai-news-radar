package radar

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// TestMeasureFetch measures the self-written fetcher against a real news.db (network!). Skipped unless
// MEASURE_DB=<path to news.db> is set. MEASURE_PER (default 10) items per non-X source, fetched in batches of 10
// (like a brief), plus one mixed batch of 10 different sources.
//
//	MEASURE_DB=…/news.db go test ./radar -run TestMeasureFetch -v -timeout 30m
func TestMeasureFetch(t *testing.T) {
	path := os.Getenv("MEASURE_DB")
	if path == "" {
		t.Skip("MEASURE_DB not set")
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	per := envIntDef("MEASURE_PER", 10)
	rows, err := db.Query(`SELECT s.name, i.url, i.title FROM items i JOIN sources s ON s.id=i.source_id
		WHERE i.is_x=0 AND i.merged_into IS NULL AND i.hidden=0 AND s.enabled=1 ORDER BY s.name, i.published DESC`)
	if err != nil {
		t.Fatal(err)
	}
	bySrc := map[string][]string{}
	var names []string
	for rows.Next() {
		var n, u, ti string
		rows.Scan(&n, &u, &ti)
		if len(bySrc[n]) == 0 {
			names = append(names, n)
		}
		if len(bySrc[n]) < per {
			bySrc[n] = append(bySrc[n], u)
		}
	}
	rows.Close()
	sort.Strings(names)
	type agg struct {
		full, partial, failed, skipped, runes int
		reasons                               map[string]int
	}
	total := agg{reasons: map[string]int{}}
	fmt.Printf("%-14s %4s %4s %4s %4s %4s %8s %s\n", "source", "n", "full", "part", "fail", "skip", "batch_s", "reasons")
	for _, n := range names {
		if os.Getenv("MEASURE_MIXED_ONLY") != "" { // cold process: only the mixed batch below
			break
		}
		urls := bySrc[n]
		a := agg{reasons: map[string]int{}}
		var took []string
		for i := 0; i < len(urls); i += 10 {
			batch := urls[i:min(i+10, len(urls))]
			t0 := time.Now()
			res := fetchArticlesRes(batch)
			took = append(took, fmt.Sprintf("%.1f", time.Since(t0).Seconds()))
			for _, u := range batch {
				r := res[u]
				switch r.Status {
				case "full":
					a.full++
					a.runes += utf8.RuneCountInString(r.Text)
				case "partial":
					a.partial++
				case "skipped":
					a.skipped++
				default:
					a.failed++
				}
				if r.Status != "full" {
					a.reasons[jsSlice(r.Reason, 40)]++
					if os.Getenv("MEASURE_V") != "" {
						fmt.Printf("  %s %s %s\n", r.Status, u, r.Reason)
					}
				}
			}
		}
		var rs []string
		for k, v := range a.reasons {
			rs = append(rs, fmt.Sprintf("%s×%d", k, v))
			total.reasons[k] += v
		}
		sort.Strings(rs)
		fmt.Printf("%-14s %4d %4d %4d %4d %4d %8s %s\n", jsSlice(n, 14), len(urls), a.full, a.partial, a.failed, a.skipped, strings.Join(took, "/"), strings.Join(rs, "; "))
		total.full += a.full
		total.partial += a.partial
		total.failed += a.failed
		total.skipped += a.skipped
	}
	fmt.Printf("TOTAL full=%d partial=%d failed=%d skipped=%d\n", total.full, total.partial, total.failed, total.skipped)
	// one mixed batch: the newest item of each source (cache cleared so it really fetches)
	gArticles.mu.Lock()
	gArticles.m = map[string]articleCached{}
	gArticles.mu.Unlock()
	var mixed []string
	for k := 0; len(mixed) < 10 && k < per; k++ {
		for _, n := range names {
			if len(mixed) < 10 && k < len(bySrc[n]) {
				if _, skip := articleRoute(bySrc[n][k]); skip != "" {
					continue // no page to fetch (抖音热搜 / 煎蛋)
				}
				mixed = append(mixed, bySrc[n][k])
			}
		}
	}
	t0 := time.Now()
	res := fetchArticlesRes(mixed)
	full := 0
	for _, u := range mixed {
		if res[u].Status == "full" {
			full++
		}
	}
	fmt.Printf("MIXED batch of %d: %.1fs, full %d/%d\n", len(mixed), time.Since(t0).Seconds(), full, len(mixed))
}

// TestProbeURL prints what the fetcher gets for PROBE_URL (status, charset, extract method, text head).
func TestProbeURL(t *testing.T) {
	u := os.Getenv("PROBE_URL")
	if u == "" {
		t.Skip("PROBE_URL not set")
	}
	r, err := gFetch.get(t.Context(), u, articleTimeout)
	if err != nil {
		t.Fatal(err)
	}
	ex := extractPage(r.Text)
	fmt.Printf("status=%d charset=%s raw=%d len=%d method=%s full=%v title=%q\n%s\n", r.Status, r.Charset, r.Raw, len(r.Text), ex.Method, ex.Full, ex.Title, jsSlice(ex.Text, 1500))
	if os.Getenv("PROBE_SAVE") != "" {
		os.WriteFile(os.Getenv("PROBE_SAVE"), []byte(r.Text), 0o644)
	}
}

// TestProbeBatch fetches PROBE_URLS (comma separated) together and prints status / extract / length per url.
func TestProbeBatch(t *testing.T) {
	us := os.Getenv("PROBE_URLS")
	if us == "" {
		t.Skip("PROBE_URLS not set")
	}
	urls := strings.Split(us, ",")
	t0 := time.Now()
	res := fetchArticlesRes(urls)
	for _, u := range urls {
		r := res[u]
		fmt.Printf("%-8s %-9s %5d %5.1fs %s %s | %s\n", r.Status, r.Extract, utf8.RuneCountInString(r.Text), r.Took.Seconds(), jsSlice(u, 70), r.Reason, strings.ReplaceAll(jsSlice(r.Text, 90), "\n", " "))
	}
	fmt.Printf("batch %d urls: %.1fs\n", len(urls), time.Since(t0).Seconds())
}
