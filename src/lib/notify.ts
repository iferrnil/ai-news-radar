// P0 notifications: new P0 items are collected into batches and published on the bus as `notify`
// (SSE → web Notification API fallback in the browser, native notifications in the desktop shell).
// No spam: the first successful poll of every source after start-up is treated as backlog, items older than
// NOTIFY_MAX_AGE are skipped, a batch waits NOTIFY_BATCH_MS for more items and batches are at least
// NOTIFY_MIN_GAP_MS apart (items keep collecting meanwhile, the next batch says "N 条").
import { publish } from "./bus";
import { getSetting, setSetting } from "./translate";
import type { ItemOut } from "./types";

export interface NotifyItem { id: number; title: string; source: string; section: string | null; url: string; published: number }
export interface NotifyBatch { at: number; count: number; items: NotifyItem[]; sections: string[] }
/** Server-side prefs, used by the desktop shell (the web keeps its own in localStorage). sections: [] = all. */
export interface NotifyPrefs { enabled: boolean; sections: string[] }

const MAX_AGE = Number(process.env.NOTIFY_MAX_AGE ?? 3 * 3600);
const BATCH_MS = Number(process.env.NOTIFY_BATCH_MS ?? 20_000);
const MIN_GAP_MS = Number(process.env.NOTIFY_MIN_GAP_MS ?? 60_000);

type St = { polled: Set<number>; queue: NotifyItem[]; timer: NodeJS.Timeout | null; last: number; sent: number; recent: NotifyBatch[] };
const g = globalThis as unknown as { __notify?: St };
const st: St = (g.__notify ??= { polled: new Set(), queue: [], timer: null, last: 0, sent: 0, recent: [] });

export function notifyPrefs(): NotifyPrefs {
  try { const p = JSON.parse(getSetting("notify") ?? "") as NotifyPrefs; return { enabled: !!p.enabled, sections: Array.isArray(p.sections) ? p.sections.map(String) : [] }; }
  catch { return { enabled: process.env.AI_RADAR_DESKTOP === "1", sections: [] }; } // desktop: on by default
}
export function setNotifyPrefs(p: Partial<NotifyPrefs>) {
  const cur = notifyPrefs();
  const next = { enabled: p.enabled ?? cur.enabled, sections: (p.sections ?? cur.sections).filter((s) => typeof s === "string" && s.length < 64).slice(0, 50) };
  setSetting("notify", JSON.stringify(next));
  return next;
}

function flush() {
  st.timer = null;
  if (!st.queue.length) return;
  const wait = st.last + MIN_GAP_MS - Date.now();
  if (wait > 0) { st.timer = setTimeout(flush, wait); return; } // throttled: keep collecting
  const all = st.queue.sort((a, b) => b.published - a.published); st.queue = [];
  const batch: NotifyBatch = { at: Date.now() / 1000, count: all.length, items: all.slice(0, 5), sections: [...new Set(all.map((i) => i.section ?? ""))] };
  st.last = Date.now(); st.sent++;
  st.recent = [batch, ...st.recent].slice(0, 10);
  publish({ event: "notify", batch });
}

/** Called by the poller after each successful poll with the items it stored. */
export function onPolled(sourceId: number, fresh: ItemOut[]) {
  const first = !st.polled.has(sourceId);
  st.polled.add(sourceId);
  if (first) return; // start-up backlog
  const now = Date.now() / 1000;
  const p0 = fresh.filter((i) => i.priority === "P0" && now - i.published <= MAX_AGE);
  if (!p0.length) return;
  for (const i of p0) if (!st.queue.some((q) => q.id === i.id))
    st.queue.push({ id: i.id, title: (i.title_zh || i.title).slice(0, 200), source: i.source, section: i.section, url: i.url, published: i.published });
  if (!st.timer) st.timer = setTimeout(flush, BATCH_MS);
}

export const notifyState = () => ({ queued: st.queue.length, sent: st.sent, recent: st.recent, polled: st.polled.size });
/** test hook: send a fake batch now (not throttled) */
export function notifyTest(items: NotifyItem[]) {
  const batch: NotifyBatch = { at: Date.now() / 1000, count: items.length, items: items.slice(0, 5), sections: [...new Set(items.map((i) => i.section ?? ""))] };
  publish({ event: "notify", batch });
  return batch;
}
