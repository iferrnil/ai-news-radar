package radar

import (
	"math"
	"time"
)

// Per-source fetch health + circuit breaker, persisted in source_health (health.ts).
const healthN = 20

// HealthCols are appended to /api/sources rows (h_*).
const HealthCols = `h.runs h_runs, h.last_ok h_last_ok, h.last_err_at h_err_at, h.last_kind h_kind, h.consec h_consec, h.lat_ms h_lat, h.open_until h_open`

func coolDown(consec, interval int) float64 {
	breakAfter := envInt("BREAK_AFTER", 5)
	if consec < breakAfter {
		return 0
	}
	return math.Min(envNum("BREAK_MAX_COOL", 3600), math.Max(300, float64(interval))*math.Pow(2, math.Min(float64(consec-breakAfter), 4)))
}

func (s *Store) recordRun(src *Row, ok bool, d time.Duration, errText string, kind FailKind) {
	id := src.Get("id")
	t := now()
	h, _ := one(s.DB, "SELECT * FROM source_health WHERE source_id=?", id)
	if h == nil {
		h = &Row{}
		for _, k := range []string{"source_id", "runs", "last_ok", "last_err", "last_err_at", "last_kind", "consec", "lat_ms", "last_ms", "open_until"} {
			h.Set(k, nil)
		}
		h.Set("source_id", id)
		h.Set("runs", "")
		h.Set("consec", int64(0))
	}
	runs := h.Str("runs")
	if ok {
		runs += "1"
	} else {
		runs += "0"
	}
	if len(runs) > healthN {
		runs = runs[len(runs)-healthN:]
	}
	h.Set("runs", runs)
	ms := float64(d.Microseconds()) / 1000
	h.Set("last_ms", int64(math.Round(ms)))
	if ok {
		h.Set("last_ok", t)
		h.Set("consec", int64(0))
		h.Set("open_until", nil)
		lat := ms
		if h.Has("lat_ms") {
			lat = h.Num("lat_ms")*0.7 + ms*0.3
		}
		h.Set("lat_ms", int64(math.Round(lat)))
	} else {
		c := int(h.Num("consec")) + 1
		h.Set("consec", int64(c))
		h.Set("last_err", errText)
		h.Set("last_err_at", t)
		h.Set("last_kind", string(kind))
		if cool := coolDown(c, int(src.Num("poll_interval"))); cool > 0 {
			h.Set("open_until", t+cool)
		} else {
			h.Set("open_until", nil)
		}
	}
	s.DB.Exec(`INSERT INTO source_health(source_id,runs,last_ok,last_err,last_err_at,last_kind,consec,lat_ms,last_ms,open_until) VALUES(?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(source_id) DO UPDATE SET runs=excluded.runs,last_ok=excluded.last_ok,last_err=excluded.last_err,last_err_at=excluded.last_err_at,
		last_kind=excluded.last_kind,consec=excluded.consec,lat_ms=excluded.lat_ms,last_ms=excluded.last_ms,open_until=excluded.open_until`,
		id, h.Get("runs"), h.Get("last_ok"), h.Get("last_err"), h.Get("last_err_at"), h.Get("last_kind"), h.Get("consec"), h.Get("lat_ms"), h.Get("last_ms"), h.Get("open_until"))
}
