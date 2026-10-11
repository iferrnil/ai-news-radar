import { NextRequest } from "next/server";
import { getDb } from "@/lib/db";
import { publish } from "@/lib/bus";
import { cleanIcon, cleanSectionName, isSectionKey, listSections } from "@/lib/sections";
export const dynamic = "force-dynamic";
type Ctx = { params: Promise<{ key: string }> };

/** rename / re-icon / 全部 toggle / reorder (move: -1 up, +1 down) — every section, including the former built-ins */
export async function PATCH(req: NextRequest, ctx: Ctx) {
  const key = (await ctx.params).key; const b = await req.json().catch(() => ({}));
  const db = getDb();
  if (!isSectionKey(db, key)) return Response.json({ detail: "分区不存在" }, { status: 404 });
  if (b.name !== undefined) {
    const name = cleanSectionName(b.name);
    if (!name) return Response.json({ detail: "分区名称不能为空" }, { status: 400 });
    if (name === "全部") return Response.json({ detail: "「全部」是保留名称" }, { status: 400 });
    if (db.prepare("SELECT 1 FROM sections WHERE name=? AND key<>?").get(name, key)) return Response.json({ detail: "已有同名分区" }, { status: 409 });
    db.prepare("UPDATE sections SET name=? WHERE key=?").run(name, key);
  }
  if (b.icon !== undefined) db.prepare("UPDATE sections SET icon=? WHERE key=?").run(cleanIcon(b.icon), key);
  if (typeof b.inAll === "boolean") db.prepare("UPDATE sections SET in_all=? WHERE key=?").run(b.inAll ? 1 : 0, key);
  if (b.move === -1 || b.move === 1) {
    const list = listSections(db); const i = list.findIndex((s) => s.key === key); const j = i + b.move;
    if (j >= 0 && j < list.length) {
      [list[i], list[j]] = [list[j], list[i]];
      const set = db.prepare("UPDATE sections SET pos=? WHERE key=?");
      db.transaction(() => list.forEach((s, n) => set.run(n + 1, s.key)))();
    }
  }
  publish({ event: "sources" });
  return Response.json({ ok: true });
}

/** Delete a section. If sources still use it, `?moveTo=<section key>` is required (they are moved first). */
export async function DELETE(req: NextRequest, ctx: Ctx) {
  const key = (await ctx.params).key; const db = getDb();
  if (!isSectionKey(db, key)) return Response.json({ detail: "分区不存在" }, { status: 404 });
  const n = (db.prepare("SELECT COUNT(*) n FROM sources WHERE section=?").get(key) as { n: number }).n;
  const to = req.nextUrl.searchParams.get("moveTo");
  if (n && (!to || to === key || !isSectionKey(db, to))) return Response.json({ detail: `该分区还有 ${n} 个来源，请选择要移入的分区`, count: n }, { status: 409 });
  db.transaction(() => {
    if (n) db.prepare("UPDATE sources SET section=?, overrides = CASE WHEN instr(overrides,'section') THEN overrides ELSE trim(overrides || ',section', ',') END WHERE section=?").run(to, key);
    db.prepare("DELETE FROM sections WHERE key=?").run(key);
  })();
  publish({ event: "sources" });
  return Response.json({ ok: true, moved: n });
}
