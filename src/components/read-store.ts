// Per-item read state: a Set in memory, persisted to localStorage (debounced) and synced across tabs.
// When the server has read sync on (desktop app / READ_SYNC=1) it is merged from and mirrored to /api/read.
// Components subscribe with useReadVersion() (useSyncExternalStore), so marking re-renders only what reads it.
import { useSyncExternalStore } from "react";

const KEY = "read.ids", CAP = 5000, HOVER_KEY = "read.onHover";
let ids: Set<number> | null = null;
let version = 0;
const subs = new Set<() => void>();
let saveT: ReturnType<typeof setTimeout> | null = null;
let sync = false, desktop: boolean | null = null;
const pend = { add: new Set<number>(), remove: new Set<number>() };
let syncT: ReturnType<typeof setTimeout> | null = null;

function load(): Set<number> {
  if (ids) return ids;
  ids = new Set();
  if (typeof window === "undefined") return ids;
  try { for (const n of JSON.parse(localStorage.getItem(KEY) ?? "[]")) if (typeof n === "number") ids.add(n); } catch {}
  window.addEventListener("storage", (e) => {
    if (e.key !== KEY) return;
    ids = null; load(); emit(false);
  });
  return ids;
}
function emit(save = true) {
  version++; subs.forEach((f) => f());
  if (save && !saveT) saveT = setTimeout(() => {
    saveT = null;
    try { localStorage.setItem(KEY, JSON.stringify([...load()].slice(-CAP))); } catch {}
  }, 250);
}
function push() {
  if (!sync || syncT) return;
  syncT = setTimeout(() => {
    syncT = null;
    const body = { add: [...pend.add], remove: [...pend.remove] }; pend.add.clear(); pend.remove.clear();
    if (body.add.length || body.remove.length) fetch("/api/read", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), keepalive: true }).catch(() => {});
  }, 800);
}

export const isRead = (id: number) => load().has(id);
export function markRead(list: number[]) {
  const s = load(); let ch = false;
  for (const id of list) if (!s.has(id)) { s.add(id); ch = true; pend.add.add(id); pend.remove.delete(id); }
  if (ch) { emit(); push(); }
  return ch;
}
export function markUnread(list: number[]) {
  const s = load(); let ch = false;
  for (const id of list) if (s.delete(id)) { ch = true; pend.remove.add(id); pend.add.delete(id); }
  if (ch) { emit(); push(); }
}
const subscribe = (f: () => void) => { subs.add(f); return () => { subs.delete(f); }; };
/** re-render when read state changes; returns a counter usable as a memo dependency */
export const useReadVersion = () => useSyncExternalStore(subscribe, () => version, () => 0);

/** idle-time: learn whether we run in the desktop app and merge server-side read state (when on) */
export async function initReadSync() {
  try {
    const j = await fetch("/api/read", { cache: "no-store" }).then((r) => r.json()) as { sync: boolean; desktop: boolean; ids?: number[] };
    desktop = !!j.desktop; sync = !!j.sync;
    if (sync && j.ids?.length) {
      const s = load(); const local = [...s].filter((id) => !j.ids!.includes(id));
      let ch = false; for (const id of j.ids) if (!s.has(id)) { s.add(id); ch = true; }
      if (ch) emit();
      if (local.length) { local.forEach((id) => pend.add.add(id)); push(); } // upload what only this webview knew
    }
  } catch {}
}
export const isDesktop = () => desktop;

export const markOnHover = () => typeof window !== "undefined" && localStorage.getItem(HOVER_KEY) === "1";
export const setMarkOnHover = (b: boolean) => { try { localStorage.setItem(HOVER_KEY, b ? "1" : "0"); } catch {} };
