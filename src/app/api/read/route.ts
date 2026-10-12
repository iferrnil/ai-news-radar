import { NextRequest } from "next/server";
import { getDb } from "@/lib/db";
export const dynamic = "force-dynamic";

// Read state lives in the browser (localStorage). The desktop app (AI_RADAR_DESKTOP=1) — or a single-user
// deployment with READ_SYNC=1 — also mirrors it here, so it survives a new webview origin / another browser.
const syncOn = () => process.env.AI_RADAR_DESKTOP === "1" || process.env.READ_SYNC === "1";
const ids = (v: unknown) => (Array.isArray(v) ? v.map(Number).filter((n) => Number.isInteger(n) && n > 0).slice(0, 5000) : []);

export function GET() {
  const desktop = process.env.AI_RADAR_DESKTOP === "1";
  if (!syncOn()) return Response.json({ sync: false, desktop }, { headers: { "Cache-Control": "no-store" } });
  const rows = getDb().prepare("SELECT item_id FROM read_items ORDER BY ts DESC LIMIT 5000").all() as { item_id: number }[];
  return Response.json({ sync: true, desktop, ids: rows.map((r) => r.item_id) }, { headers: { "Cache-Control": "no-store" } });
}

/** { add?: number[], remove?: number[] } */
export async function POST(req: NextRequest) {
  if (!syncOn()) return Response.json({ detail: "read sync is off" }, { status: 404 });
  const b = await req.json().catch(() => ({}));
  const db = getDb(); const now = Date.now() / 1000;
  const add = db.prepare("INSERT INTO read_items(item_id, ts) SELECT ?, ? WHERE EXISTS (SELECT 1 FROM items WHERE id=?) ON CONFLICT(item_id) DO NOTHING");
  const del = db.prepare("DELETE FROM read_items WHERE item_id=?");
  db.transaction(() => { for (const id of ids(b.add)) add.run(id, now, id); for (const id of ids(b.remove)) del.run(id); })();
  return Response.json({ ok: true });
}
