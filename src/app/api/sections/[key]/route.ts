import { NextRequest } from "next/server";
import { getDb } from "@/lib/db";
import { publish } from "@/lib/bus";
import { BUILTIN_ZH, cleanIcon, cleanSectionName, grpFor, isSectionKey } from "@/lib/sections";
export const dynamic = "force-dynamic";
type Ctx = { params: Promise<{ key: string }> };

export async function PATCH(req: NextRequest, ctx: Ctx) {
  const key = (await ctx.params).key; const b = await req.json().catch(() => ({}));
  const db = getDb();
  if (!db.prepare("SELECT 1 FROM sections WHERE key=?").get(key)) return Response.json({ detail: "分区不存在（内置分区不可修改）" }, { status: 404 });
  if (b.name !== undefined) {
    const name = cleanSectionName(b.name);
    if (!name) return Response.json({ detail: "分区名称不能为空" }, { status: 400 });
    if (Object.values(BUILTIN_ZH).includes(name) || db.prepare("SELECT 1 FROM sections WHERE name=? AND key<>?").get(name, key)) return Response.json({ detail: "已有同名分区" }, { status: 409 });
    db.prepare("UPDATE sections SET name=? WHERE key=?").run(name, key);
  }
  if (b.icon !== undefined) db.prepare("UPDATE sections SET icon=? WHERE key=?").run(cleanIcon(b.icon), key);
  if (typeof b.inAll === "boolean") db.prepare("UPDATE sections SET in_all=? WHERE key=?").run(b.inAll ? 1 : 0, key);
  publish({ event: "sources" });
  return Response.json({ ok: true });
}

/** Delete a custom section. If sources still use it, `?moveTo=<section key>` is required (they are moved first). */
export function DELETE(req: NextRequest, ctx: Ctx) {
  return (async () => {
    const key = (await ctx.params).key; const db = getDb();
    if (!db.prepare("SELECT 1 FROM sections WHERE key=?").get(key)) return Response.json({ detail: "分区不存在（内置分区不可删除）" }, { status: 404 });
    const n = (db.prepare("SELECT COUNT(*) n FROM sources WHERE grp=?").get(`sec:${key}`) as { n: number }).n;
    const to = req.nextUrl.searchParams.get("moveTo");
    if (n && (!to || to === key || !isSectionKey(db, to))) return Response.json({ detail: `该分区还有 ${n} 个来源，请选择要移入的分区`, count: n }, { status: 409 });
    db.transaction(() => {
      if (n) db.prepare("UPDATE sources SET grp=?, overrides = CASE WHEN instr(overrides,'grp') THEN overrides ELSE trim(overrides || ',grp', ',') END WHERE grp=?").run(grpFor(to!), `sec:${key}`);
      db.prepare("DELETE FROM sections WHERE key=?").run(key);
    })();
    publish({ event: "sources" });
    return Response.json({ ok: true, moved: n });
  })();
}
