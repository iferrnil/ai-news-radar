package core

import (
	"context"
	"log"
	"math"
	"sync"
	"time"
)

// Scheduler polls due sources every 10s (X concurrency 3, feeds 8), prunes every 10 min,
// and keeps a per-source circuit breaker in source_health (same table as the web app).
type Scheduler struct {
	s       *Store
	mu      sync.Mutex
	running map[int64]bool
	x, feed chan struct{}
	cancel  context.CancelFunc
}

func (s *Store) NewScheduler() *Scheduler {
	return &Scheduler{s: s, running: map[int64]bool{}, x: make(chan struct{}, 3), feed: make(chan struct{}, 8)}
}

func (sc *Scheduler) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	sc.cancel = cancel
	go func() {
		time.AfterFunc(5*time.Second, sc.s.BackfillTranslations)
		var lastPrune time.Time
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			sc.tick(ctx)
			if time.Since(lastPrune) > 10*time.Minute {
				if n, err := sc.s.Prune(); err == nil && n > 0 {
					log.Printf("[poller] pruned %d items", n)
					sc.s.emit(Event{Kind: "items"})
				}
				if sc.s.Thumbs != nil {
					sc.s.Thumbs.Prune(sc.s.ImageURLs())
				}
				lastPrune = time.Now()
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

func (sc *Scheduler) Stop() {
	if sc.cancel != nil {
		sc.cancel()
	}
}

func (sc *Scheduler) tick(ctx context.Context) {
	rows, err := sc.s.DB.Query(`SELECT `+sourceCols+` FROM sources s LEFT JOIN source_health h ON h.source_id=s.id
		WHERE s.enabled=1 AND s.supported=1 AND (s.last_fetched IS NULL OR s.last_fetched + s.poll_interval <= ?) AND (h.open_until IS NULL OR h.open_until <= ?)`, now(), now())
	if err != nil {
		log.Println("[poller]", err)
		return
	}
	var due []Source
	for rows.Next() {
		if x, err := scanSource(rows); err == nil {
			due = append(due, x)
		}
	}
	rows.Close()
	for _, src := range due {
		go sc.PollOne(ctx, src)
	}
}

// PollOne fetches + stores one source now (also used by "立即刷新").
func (sc *Scheduler) PollOne(ctx context.Context, src Source) {
	sc.mu.Lock()
	if sc.running[src.ID] {
		sc.mu.Unlock()
		return
	}
	sc.running[src.ID] = true
	sc.mu.Unlock()
	defer func() { sc.mu.Lock(); delete(sc.running, src.ID); sc.mu.Unlock() }()
	lim := sc.feed
	if src.Type == "x" || src.Type == "x_list" {
		lim = sc.x
	}
	select {
	case lim <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-lim }()
	t0 := time.Now()
	items, err := FetchSource(ctx, src)
	s := sc.s
	if err != nil {
		msg := err.Error()
		s.recordRun(src, false, time.Since(t0), msg)
		s.DB.Exec("UPDATE sources SET last_fetched=?, last_error=? WHERE id=?", now(), msg, src.ID)
		return
	}
	s.recordRun(src, true, time.Since(t0), "")
	ids, err := s.StoreItems(src, items)
	if err != nil {
		log.Println("[store]", src.Name, err)
	}
	s.DB.Exec("UPDATE sources SET last_fetched=?, last_error=NULL, last_new=? WHERE id=?", now(), len(ids), src.ID)
	var fresh []Item
	if len(ids) > 0 {
		fresh, _ = s.itemsByID(ids)
		s.EnqueueTranslation(ids)
		s.emit(Event{Kind: "items"})
	}
	s.notifier.onPolled(src.ID, fresh)
}

func (s *Store) itemsByID(ids []int64) ([]Item, error) {
	all, err := s.Items(Query{Limit: 2000})
	if err != nil {
		return nil, err
	}
	want := map[int64]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []Item
	for _, it := range all {
		if want[it.ID] {
			out = append(out, it)
		}
	}
	return out, nil
}

// recordRun keeps the last 20 outcomes and opens the breaker after 5 consecutive failures (5 min … 1 h cool-down).
func (s *Store) recordRun(src Source, ok bool, d time.Duration, errMsg string) {
	var runs string
	var consec int
	s.DB.QueryRow("SELECT runs, consec FROM source_health WHERE source_id=?", src.ID).Scan(&runs, &consec)
	t := now()
	if ok {
		runs += "1"
		consec = 0
	} else {
		runs += "0"
		consec++
	}
	if len(runs) > 20 {
		runs = runs[len(runs)-20:]
	}
	var open any
	if consec >= 5 {
		cool := math.Min(3600, math.Max(300, float64(src.PollInterval))*math.Pow(2, math.Min(float64(consec-5), 4)))
		open = t + cool
	}
	ms := d.Milliseconds()
	if ok {
		s.DB.Exec(`INSERT INTO source_health(source_id,runs,last_ok,consec,last_ms,lat_ms,open_until) VALUES(?,?,?,?,?,?,NULL)
			ON CONFLICT(source_id) DO UPDATE SET runs=excluded.runs,last_ok=excluded.last_ok,consec=0,last_ms=excluded.last_ms,
			lat_ms=CAST(COALESCE(lat_ms*0.7+excluded.last_ms*0.3, excluded.last_ms) AS INTEGER),open_until=NULL`, src.ID, runs, t, 0, ms, ms)
	} else {
		s.DB.Exec(`INSERT INTO source_health(source_id,runs,last_err,last_err_at,consec,last_ms,open_until) VALUES(?,?,?,?,?,?,?)
			ON CONFLICT(source_id) DO UPDATE SET runs=excluded.runs,last_err=excluded.last_err,last_err_at=excluded.last_err_at,
			consec=excluded.consec,last_ms=excluded.last_ms,open_until=excluded.open_until`, src.ID, runs, errMsg, t, consec, ms, open)
	}
}
