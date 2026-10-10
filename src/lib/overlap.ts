import { getDb } from "./db";
/** Share of sample titles already stored (same content under another feed URL). */
export function overlap(sample: { title: string }[]) {
  const ts = sample.map((i) => i.title).filter(Boolean);
  if (!ts.length) return 0;
  const q = getDb().prepare("SELECT 1 FROM items WHERE title=? LIMIT 1");
  return ts.filter((t) => q.get(t)).length / ts.length;
}
