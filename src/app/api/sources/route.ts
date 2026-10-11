import { jsonResponse } from "@/lib/http";
import { NextRequest } from "next/server";
import { getDb, intervalFor } from "@/lib/db";
import { detect } from "@/lib/fetchers";
import { pollOne } from "@/lib/scheduler";
import { errMsg } from "@/lib/util";
import { overlap } from "@/lib/overlap";
import type { SourceRow } from "@/lib/types";
import { createSection, isSectionKey, listSections, metaFor } from "@/lib/sections";
import { validateRule } from "@/lib/rule-guard";
export const dynamic = "force-dynamic";

// Polled by every tab (sidebar every 20 s): gzip + drop nulls, 161 KB -> ~20 KB on the wire.
export function GET(req: NextRequest) {
  const rows = getDb().prepare(`SELECT s.*, (SELECT COUNT(*) FROM items i WHERE i.source_id=s.id AND i.hidden=0) item_count, (SELECT COUNT(*) FROM items i WHERE i.source_id=s.id AND i.hidden=1) hidden_count,
    (SELECT MAX(published) FROM items i WHERE i.source_id=s.id) latest FROM sources s
    ORDER BY s.enabled DESC, s.supported DESC, COALESCE(s.priority,'P9'), s.category, s.name`).all();
  const body = JSON.stringify(rows, (_k, v) => (v === null ? undefined : v));
  return jsonResponse(req, body);
}

export async function POST(req: NextRequest) {
  const b = await req.json();
  let d;
  try { d = await detect(String(b.input ?? "")); } catch (e) { return Response.json({ detail: errMsg(e).replace(/^Error: /, "") }, { status: 400 }); }
  const name = String(b.name || d.name).trim();
  const db = getDb();
  if (db.prepare("SELECT 1 FROM sources WHERE url=? OR name=?").get(d.url, name)) return Response.json({ detail: "已存在相同 URL 或名称的来源" }, { status: 409 });
  const ov = overlap(d.sample);
  if (ov >= 0.5 && !b.force) return Response.json({ detail: `疑似重复：示例条目中 ${Math.round(ov * 100)}% 的标题已由现有来源收录。确需添加请勾选「强制添加」` }, { status: 409 });
  const priority = ["P0", "P1", "P2"].includes(b.priority) ? b.priority : "P1";
  // the chosen sidebar section is stored as the seed-style `grp`; region/category are derived (internal only)
  const v = validateRule(b.rule ?? {}, d.sample.map((x) => x.title));
  if ("error" in v) return Response.json({ detail: v.error }, { status: 400 });
  // section: a built-in/custom key, or {newSection:{name,icon,inAll}} to create one on the fly
  const fallback = isSectionKey(db, d.type.startsWith("x") ? "x" : "media") ? (d.type.startsWith("x") ? "x" : "media") : listSections(db)[0]?.key;
  let sec = isSectionKey(db, b.section) ? (b.section as string) : fallback;
  if (!sec && !b.newSection) return Response.json({ detail: "请先选择或新建一个分区" }, { status: 400 });
  if (b.newSection) { const c = createSection(db, b.newSection); if ("error" in c) return Response.json({ detail: c.error }, { status: 400 }); sec = c.key; }
  const { region, category } = metaFor(sec!);
  const res = db.prepare(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,priority,poll_interval,origin,created,section,f_include,f_exclude,f_scope,f_cs)
    VALUES(?,?,?,?,?,?,?,1,1,?,?,'user',?,?,?,?,?,?)`).run(name, region, category, d.type, d.url, d.homepage, d.username,
    priority, intervalFor(d.type, priority), Date.now() / 1000, sec, v.rule.include, v.rule.exclude, v.rule.scope, v.rule.cs ? 1 : 0);
  const src = db.prepare("SELECT * FROM sources WHERE id=?").get(res.lastInsertRowid) as SourceRow;
  return Response.json({ source: src, fetch: await pollOne(src) });
}
