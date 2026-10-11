import { NextRequest } from "next/server";
import { getDb } from "@/lib/db";
import { publish } from "@/lib/bus";
import { deleteSources, reapplyRule } from "@/lib/store";
import { createSection, isSectionKey } from "@/lib/sections";
import { validateRule } from "@/lib/rule-guard";
export const dynamic = "force-dynamic";

export async function PATCH(req: NextRequest, ctx: { params: Promise<{ id: string }> }) {
  const id = Number((await ctx.params).id);
  const b = await req.json();
  const db = getDb();
  if (!db.prepare("SELECT 1 FROM sources WHERE id=?").get(id)) return Response.json({ detail: "not found" }, { status: 404 });
  let name: string | null = null;
  if (b.name !== undefined) {
    name = String(b.name).replace(/[\u0000-\u001f\u007f<>]/g, "").replace(/\s+/g, " ").trim();
    if (!name) return Response.json({ detail: "名称不能为空" }, { status: 400 });
    if ([...name].length > 80) return Response.json({ detail: "名称最多 80 个字符" }, { status: 400 });
    if (db.prepare("SELECT 1 FROM sources WHERE name=? COLLATE NOCASE AND id<>?").get(name, id)) return Response.json({ detail: "已有同名来源" }, { status: 409 });
  }
  let rule = null;
  if (b.rule) { const v = validateRule(b.rule); if ("error" in v) return Response.json({ detail: v.error }, { status: 400 }); rule = v.rule; }
  let sec: string | null = isSectionKey(db, b.section) ? b.section : null;
  if (b.newSection) { const c = createSection(db, b.newSection); if ("error" in c) return Response.json({ detail: c.error }, { status: 400 }); sec = c.key; }
  // record which fields the user changed, so seed sync (sources.json on start) never overwrites them
  const mark = db.prepare("UPDATE sources SET overrides = CASE WHEN instr(overrides, ?) THEN overrides ELSE trim(overrides || ',' || ?, ',') END WHERE id=?");
  db.transaction(() => {
    if (typeof b.enabled === "boolean") { db.prepare("UPDATE sources SET enabled=? WHERE id=?").run(b.enabled ? 1 : 0, id); mark.run("enabled", "enabled", id); }
    if (b.poll_interval != null) { db.prepare("UPDATE sources SET poll_interval=? WHERE id=?").run(Math.max(30, Number(b.poll_interval)), id); mark.run("poll_interval", "poll_interval", id); }
    if (["P0", "P1", "P2"].includes(b.priority)) { db.prepare("UPDATE sources SET priority=? WHERE id=?").run(b.priority, id); mark.run("priority", "priority", id); }
    if (name) { db.prepare("UPDATE sources SET seed_name = CASE WHEN origin='seed' THEN COALESCE(seed_name, name) END, name=? WHERE id=?").run(name, id); mark.run("name", "name", id); }
    if (sec) { db.prepare("UPDATE sources SET section=? WHERE id=?").run(sec, id); mark.run("section", "section", id); }
    if (rule) db.prepare("UPDATE sources SET f_include=?, f_exclude=?, f_scope=?, f_cs=? WHERE id=?").run(rule.include, rule.exclude, rule.scope, rule.cs ? 1 : 0, id);
  })();
  const hidden = rule ? reapplyRule(id) : undefined; // retroactive: hide/unhide stored items under the new rule
  publish({ event: "sources", id }); // other tabs refresh their sidebar / feed
  return Response.json({ ok: true, hidden });
}

export async function DELETE(_req: NextRequest, ctx: { params: Promise<{ id: string }> }) {
  const id = Number((await ctx.params).id);
  const r = deleteSources([id]);
  if (!r.deleted) return Response.json({ detail: "not found" }, { status: 404 });
  return Response.json(r);
}
