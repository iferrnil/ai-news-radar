// Feed filter model (deliberately simple): ONE sidebar view at a time.
//   view = "" (全部) | an exclusive section (lab/cn/dev/media/x — every item is in exactly one, all X posts are "x") | a single source.
// Plus an optional P0-only toggle and 只看未读. State lives in the URL: /?section=cn | /?section=x | /?source=12 [&p0=1][&unread=1].
import type { ItemOut } from "@/lib/types";
import { type SectionKey, inAll, isSectionKeyLike, sectionOf } from "./common";

export type View = "" | SectionKey;
export type Filters = { section: View; source: number | null; p0: boolean; unread?: boolean };
export const EMPTY: Filters = { section: "", source: null, p0: false, unread: false };

/** `hide(id)` = treat as read for 只看未读 (the Feed passes a snapshot, so items read while looking stay visible). */
export function passes(it: ItemOut, f: Filters, hide?: (id: number) => boolean) {
  if (f.source != null) { if (it.source_id !== f.source) return false; }
  else if (f.section ? sectionOf(it) !== f.section : !inAll(sectionOf(it))) return false; // 全部 skips includeInAll:false sections
  if (f.p0 && it.priority !== "P0") return false;
  if (f.unread && hide?.(it.id)) return false;
  return true;
}
export const countOf = (items: ItemOut[], f: Filters, hide?: (id: number) => boolean) => items.reduce((n, it) => n + (passes(it, f, hide) ? 1 : 0), 0);

export function parseFilters(sp: URLSearchParams): Filters {
  const sec = sp.get("section") ?? "", src = Number(sp.get("source"));
  return {
    section: (isSectionKeyLike(sec) ? sec : "") as View, // any section key; validated against the list once it loads
    source: Number.isFinite(src) && src > 0 ? src : null,
    p0: sp.get("p0") === "1",
    unread: sp.get("unread") === "1",
  };
}
export function toQuery(f: Filters) {
  const p = new URLSearchParams();
  if (f.source != null) p.set("source", String(f.source)); else if (f.section) p.set("section", f.section);
  if (f.p0) p.set("p0", "1");
  if (f.unread) p.set("unread", "1");
  const s = p.toString();
  return s ? `/?${s}` : "/";
}

// 仅 P0 is remembered PER VIEW (全部 / each section / each source) in localStorage; the URL carries the current view's value.
export const viewKey = (f: Pick<Filters, "section" | "source">) => (f.source != null ? `src:${f.source}` : f.section || "all");
const P0_KEY = "p0ByView";
const readP0 = (): Record<string, boolean> => { try { return JSON.parse(localStorage.getItem(P0_KEY) || "{}"); } catch { return {}; } };
export const p0For = (key: string) => typeof window !== "undefined" && readP0()[key] === true;
export function setP0For(key: string, on: boolean) {
  const m = readP0(); if (on) m[key] = true; else delete m[key];
  try { localStorage.setItem(P0_KEY, JSON.stringify(m)); } catch {}
}

// 只看未读 is a reading mode: kept when switching views and remembered across visits
const UNREAD_KEY = "unreadOnly";
export const unreadPref = () => typeof window !== "undefined" && localStorage.getItem(UNREAD_KEY) === "1";
export const setUnreadPref = (on: boolean) => { try { localStorage.setItem(UNREAD_KEY, on ? "1" : "0"); } catch {} };
