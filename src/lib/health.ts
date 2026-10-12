// Per-source fetch health + circuit breaker, persisted in `source_health`.
// runs = outcome of the last HEALTH_N polls as "1"/"0" (newest last); lat_ms = EMA of successful poll latency.
// Breaker: after BREAK_AFTER consecutive failures the source is not polled until open_until
// (cool-down doubles per further failure, 5 min … 1 h); then one probe poll decides (half-open).
import type Database from "better-sqlite3";
import type { FailKind } from "./net";

export const HEALTH_N = 20;
export const BREAK_AFTER = Number(process.env.BREAK_AFTER ?? 5);
const MAX_COOL = Number(process.env.BREAK_MAX_COOL ?? 3600);

export interface HealthRow {
  source_id: number; runs: string; last_ok: number | null; last_err: string | null; last_err_at: number | null; last_kind: FailKind | null;
  consec: number; lat_ms: number | null; last_ms: number | null; open_until: number | null;
}

export const HEALTH_SCHEMA = `CREATE TABLE IF NOT EXISTS source_health(source_id INTEGER PRIMARY KEY, runs TEXT NOT NULL DEFAULT '', last_ok REAL, last_err TEXT,
  last_err_at REAL, last_kind TEXT, consec INTEGER NOT NULL DEFAULT 0, lat_ms INTEGER, last_ms INTEGER, open_until REAL);`;

export function coolDown(consec: number, interval: number) {
  if (consec < BREAK_AFTER) return 0;
  return Math.min(MAX_COOL, Math.max(300, interval) * 2 ** Math.min(consec - BREAK_AFTER, 4));
}

export function recordRun(db: Database.Database, src: { id: number; poll_interval: number }, r: { ok: true; ms: number } | { ok: false; ms: number; err: string; kind: FailKind }) {
  const now = Date.now() / 1000;
  const h = (db.prepare("SELECT * FROM source_health WHERE source_id=?").get(src.id) as HealthRow | undefined) ?? {
    source_id: src.id, runs: "", last_ok: null, last_err: null, last_err_at: null, last_kind: null, consec: 0, lat_ms: null, last_ms: null, open_until: null };
  h.runs = (h.runs + (r.ok ? "1" : "0")).slice(-HEALTH_N);
  h.last_ms = Math.round(r.ms);
  if (r.ok) {
    h.last_ok = now; h.consec = 0; h.open_until = null;
    h.lat_ms = Math.round(h.lat_ms == null ? r.ms : h.lat_ms * 0.7 + r.ms * 0.3);
  } else {
    h.consec += 1; h.last_err = r.err; h.last_err_at = now; h.last_kind = r.kind;
    const cool = coolDown(h.consec, src.poll_interval);
    h.open_until = cool ? now + cool : null;
  }
  db.prepare(`INSERT INTO source_health(source_id,runs,last_ok,last_err,last_err_at,last_kind,consec,lat_ms,last_ms,open_until)
    VALUES(@source_id,@runs,@last_ok,@last_err,@last_err_at,@last_kind,@consec,@lat_ms,@last_ms,@open_until)
    ON CONFLICT(source_id) DO UPDATE SET runs=@runs,last_ok=@last_ok,last_err=@last_err,last_err_at=@last_err_at,last_kind=@last_kind,
    consec=@consec,lat_ms=@lat_ms,last_ms=@last_ms,open_until=@open_until`).run(h);
  return h;
}

/** Columns appended to /api/sources rows (h_*). */
export const HEALTH_COLS = `h.runs h_runs, h.last_ok h_last_ok, h.last_err_at h_err_at, h.last_kind h_kind, h.consec h_consec, h.lat_ms h_lat, h.open_until h_open`;
