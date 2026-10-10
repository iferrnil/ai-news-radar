// Feed filter model (deliberately simple): ONE sidebar view at a time.
//   view = "" (全部) | an exclusive section (lab/cn/dev/media/x — every item is in exactly one, all X posts are "x") | a single source.
// Plus an optional P0-only toggle. State lives in the URL: /?section=cn | /?section=x | /?source=12 [&p0=1].
import type { ItemOut } from "@/lib/types";
import { type SectionKey, SECTIONS, inAll, sectionOf } from "./common";

export type View = "" | SectionKey;
export type Filters = { section: View; source: number | null; p0: boolean };
export const EMPTY: Filters = { section: "", source: null, p0: false };

export function passes(it: ItemOut, f: Filters) {
  if (f.source != null) { if (it.source_id !== f.source) return false; }
  else if (f.section ? sectionOf(it) !== f.section : !inAll(sectionOf(it))) return false; // 全部 skips includeInAll:false sections
  if (f.p0 && it.priority !== "P0") return false;
  return true;
}
export const countOf = (items: ItemOut[], f: Filters) => items.reduce((n, it) => n + (passes(it, f) ? 1 : 0), 0);

export function parseFilters(sp: URLSearchParams): Filters {
  const sec = sp.get("section") ?? "", src = Number(sp.get("source"));
  return {
    section: (SECTIONS.some((s) => s.key === sec) ? sec : "") as View,
    source: Number.isFinite(src) && src > 0 ? src : null,
    p0: sp.get("p0") === "1",
  };
}
export function toQuery(f: Filters) {
  const p = new URLSearchParams();
  if (f.source != null) p.set("source", String(f.source)); else if (f.section) p.set("section", f.section);
  if (f.p0) p.set("p0", "1");
  const s = p.toString();
  return s ? `/?${s}` : "/";
}
