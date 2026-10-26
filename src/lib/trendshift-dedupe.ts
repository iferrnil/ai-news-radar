import type { ItemOut } from "./types";

const isTrendshift = (s: string) => /^Trendshift/.test(s);
/** Lower is stronger: 日榜 beats 周榜; then newer published; then higher id. */
const strength = (it: ItemOut) => {
  const kind = /日榜/.test(it.source) ? 0 : /周榜/.test(it.source) ? 1 : 2;
  return kind * 1e15 + (1e12 - (it.published || 0)) * 1e3 - it.id;
};

/** One card per repo when daily+weekly Trendshift sources share a section. Non-Trendshift rows pass through. */
export function dedupeTrendshift(items: ItemOut[]): ItemOut[] {
  const best = new Map<string, ItemOut>();
  const drop = new Set<number>();
  for (const it of items) {
    if (!isTrendshift(it.source)) continue;
    const title = (it.orig?.title ?? it.title).toLowerCase();
    const key = `${it.section ?? ""}\0${title}`;
    const prev = best.get(key);
    if (!prev) {
      best.set(key, it);
      continue;
    }
    if (strength(it) < strength(prev)) {
      drop.add(prev.id);
      best.set(key, it);
    } else {
      drop.add(it.id);
    }
  }
  if (!drop.size) return items;
  return items.filter((it) => !drop.has(it.id));
}
