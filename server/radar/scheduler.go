package radar

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

type scheduler struct {
	s         *Store
	mu        sync.Mutex
	started   bool
	running   map[int64]bool
	x, feed   chan struct{}
	lastPrune float64
	cancel    context.CancelFunc
	ctx       context.Context
}

func newScheduler(s *Store) *scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &scheduler{s: s, running: map[int64]bool{}, ctx: ctx, cancel: cancel,
		x: make(chan struct{}, max(1, envInt("X_CONCURRENCY", 3))), feed: make(chan struct{}, max(1, envInt("FEED_CONCURRENCY", 8)))}
}

// PollResult is the JSON of pollOne: {skipped} | {fetched,new,merged} | {error}.
type PollResult = *Row

// PollOne fetches + stores one source now (scheduler.ts pollOne).
func (s *Store) PollOne(src *Row) PollResult {
	sc := s.sched
	id := int64(src.Num("id"))
	sc.mu.Lock()
	if sc.running[id] {
		sc.mu.Unlock()
		return obj("skipped", true)
	}
	sc.running[id] = true
	sc.mu.Unlock()
	defer func() { sc.mu.Lock(); delete(sc.running, id); sc.mu.Unlock() }()
	lim := sc.feed
	if t := src.Str("type"); t == "x" || t == "x_list" {
		lim = sc.x
	}
	lim <- struct{}{}
	defer func() { <-lim }()
	t0 := time.Now()
	items, err := FetchSource(sc.ctx, src)
	if err != nil {
		msg := strings.TrimPrefix(errMsg(err), "FetchError: ")
		s.recordRun(src, false, time.Since(t0), msg, kindOf(err))
		s.DB.Exec("UPDATE sources SET last_fetched=?, last_error=? WHERE id=?", now(), msg, id)
		log.Printf("[poller] %s: %s", src.Str("name"), msg)
		return obj("error", msg)
	}
	s.recordRun(src, true, time.Since(t0), "", "")
	newIDs, merged, fresh, err := s.StoreItems(src, items)
	if err != nil {
		log.Printf("[store] %s: %v", src.Str("name"), err)
	}
	s.notif.onPolled(id, fresh)
	// cheap poll path: only touch last_new when it changed (avoids a write on quiet sources every interval)
	nNew := len(newIDs)
	if nNew == 0 && merged == 0 {
		s.DB.Exec("UPDATE sources SET last_fetched=?, last_error=NULL WHERE id=?", now(), id)
	} else {
		s.DB.Exec("UPDATE sources SET last_fetched=?, last_error=NULL, last_new=? WHERE id=?", now(), nNew, id)
	}
	return obj("fetched", len(items), "new", nNew, "merged", merged)
}

func (sc *scheduler) tick() {
	t := now()
	due, err := all(sc.s.DB, `SELECT s.* FROM sources s LEFT JOIN source_health h ON h.source_id=s.id WHERE s.enabled=1 AND s.supported=1
		AND (s.last_fetched IS NULL OR s.last_fetched + s.poll_interval <= ?) AND (h.open_until IS NULL OR h.open_until <= ?)`, t, t)
	if err != nil {
		log.Println("[poller] tick failed", err)
		return
	}
	for _, src := range due {
		sc.mu.Lock()
		busy := sc.running[int64(src.Num("id"))]
		sc.mu.Unlock()
		if !busy {
			go sc.s.PollOne(src)
		}
	}
	if sc.lastPrune == 0 || t-sc.lastPrune > 600 {
		n := sc.s.Prune()
		sc.lastPrune = t
		if n > 0 {
			log.Printf("[poller] pruned %d items older than retention", n)
		}
	}
}

// StartScheduler starts polling once (DISABLE_POLLER=1 turns it off).
func (s *Store) StartScheduler() {
	sc := s.sched
	sc.mu.Lock()
	if sc.started || os.Getenv("DISABLE_POLLER") == "1" {
		sc.mu.Unlock()
		return
	}
	sc.started = true
	sc.mu.Unlock()
	log.Println("[poller] scheduler started")
	time.AfterFunc(5*time.Second, func() { s.tr.backfill(false) })
	go func() {
		sc.tick()
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-sc.ctx.Done():
				return
			case <-t.C:
				sc.tick()
			}
		}
	}()
}

// StopScheduler cancels the poller and in-flight fetches.
func (s *Store) StopScheduler() { s.sched.cancel() }

func (s *Store) schedulerState() *Row {
	sc := s.sched
	sc.mu.Lock()
	defer sc.mu.Unlock()
	r := &Row{}
	r.Set("started", sc.started)
	r.Set("running", len(sc.running))
	if sc.lastPrune != 0 {
		r.Set("lastPrune", sc.lastPrune)
	} else {
		r.Set("lastPrune", nil)
	}
	return r
}
