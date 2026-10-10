import { backfillTranslations } from "./translate";
import { getDb } from "./db";
import { FETCHERS } from "./fetchers";
import { prune, storeItems } from "./store";
import { errMsg } from "./util";
import type { SourceRow } from "./types";

class Limiter {
  private q: (() => void)[] = []; private n = 0;
  constructor(private max: number) {}
  async run<T>(f: () => Promise<T>): Promise<T> {
    if (this.n >= this.max) await new Promise<void>((r) => this.q.push(r));
    this.n++;
    try { return await f(); } finally { this.n--; this.q.shift()?.(); }
  }
}

type State = { started: boolean; running: Set<number>; timers: NodeJS.Timeout[]; x: Limiter; feed: Limiter; lastPrune?: number; pruned?: number };
const g = globalThis as unknown as { __newsSched?: State };
const st: State = g.__newsSched ?? (g.__newsSched = {
  started: false, running: new Set(), timers: [],
  x: new Limiter(Number(process.env.X_CONCURRENCY ?? 3)), feed: new Limiter(Number(process.env.FEED_CONCURRENCY ?? 8)),
});

export async function pollOne(src: SourceRow) {
  if (st.running.has(src.id)) return { skipped: true };
  st.running.add(src.id);
  const db = getDb();
  try {
    return await (src.type === "x" || src.type === "x_list" ? st.x : st.feed).run(async () => {
      try {
        const items = await FETCHERS[src.type](src);
        const { newIds, merged } = storeItems(src, items);
        db.prepare("UPDATE sources SET last_fetched=?, last_error=NULL, last_new=? WHERE id=?").run(Date.now() / 1000, newIds.length, src.id);
        return { fetched: items.length, new: newIds.length, merged };
      } catch (e) {
        const msg = errMsg(e);
        db.prepare("UPDATE sources SET last_fetched=?, last_error=? WHERE id=?").run(Date.now() / 1000, msg, src.id);
        console.warn(`[poller] ${src.name}: ${msg}`);
        return { error: msg };
      }
    });
  } finally {
    st.running.delete(src.id);
  }
}

function tick() {
  const now = Date.now() / 1000;
  const due = getDb().prepare(`SELECT * FROM sources WHERE enabled=1 AND supported=1
    AND (last_fetched IS NULL OR last_fetched + poll_interval <= ?)`).all(now) as SourceRow[];
  for (const s of due) if (!st.running.has(s.id)) void pollOne(s);
  if (!st.lastPrune || now - st.lastPrune > 600) {
    st.pruned = prune(); st.lastPrune = now;
    if (st.pruned) console.log(`[poller] pruned ${st.pruned} items older than retention`);
  }
}

/** Start once per process (guarded on globalThis; safe with HMR / multiple imports). */
export function startScheduler() {
  if (st.started || process.env.DISABLE_POLLER === "1") return;
  st.started = true;
  getDb();
  console.log("[poller] scheduler started");
  setTimeout(() => backfillTranslations(), 5000);
  tick();
  st.timers.push(setInterval(() => { try { tick(); } catch (e) { console.error("[poller] tick failed", e); } }, 10_000));
}

export const schedulerState = () => ({ started: st.started, running: st.running.size, lastPrune: st.lastPrune ?? null });
