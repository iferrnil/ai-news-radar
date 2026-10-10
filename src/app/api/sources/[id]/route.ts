import { NextRequest } from "next/server";
import { getDb } from "@/lib/db";
export const dynamic = "force-dynamic";

export async function PATCH(req: NextRequest, ctx: { params: Promise<{ id: string }> }) {
  const id = Number((await ctx.params).id);
  const b = await req.json();
  const db = getDb();
  if (typeof b.enabled === "boolean") db.prepare("UPDATE sources SET enabled=? WHERE id=?").run(b.enabled ? 1 : 0, id);
  if (b.poll_interval != null) db.prepare("UPDATE sources SET poll_interval=? WHERE id=?").run(Math.max(30, Number(b.poll_interval)), id);
  if (["P0", "P1", "P2"].includes(b.priority)) db.prepare("UPDATE sources SET priority=? WHERE id=?").run(b.priority, id);
  return Response.json({ ok: true });
}

export async function DELETE(_req: NextRequest, ctx: { params: Promise<{ id: string }> }) {
  const id = Number((await ctx.params).id);
  const db = getDb();
  const s = db.prepare("SELECT origin FROM sources WHERE id=?").get(id) as { origin: string } | undefined;
  if (!s) return Response.json({ detail: "not found" }, { status: 404 });
  if (s.origin !== "user") return Response.json({ detail: "内置来源只能停用，不能删除" }, { status: 400 });
  db.transaction(() => {
    db.prepare("DELETE FROM post_links WHERE item_id IN (SELECT id FROM items WHERE source_id=?)").run(id);
    db.prepare("DELETE FROM items WHERE source_id=?").run(id);
    db.prepare("DELETE FROM sources WHERE id=?").run(id);
  })();
  return Response.json({ ok: true });
}
