import { NextRequest } from "next/server";
import { getDb, intervalFor } from "@/lib/db";
import { detect } from "@/lib/fetchers";
import { pollOne } from "@/lib/scheduler";
import { errMsg } from "@/lib/util";
import { overlap } from "@/lib/overlap";
import type { SourceRow } from "@/lib/types";
export const dynamic = "force-dynamic";

export function GET() {
  const rows = getDb().prepare(`SELECT s.*, (SELECT COUNT(*) FROM items i WHERE i.source_id=s.id) item_count,
    (SELECT MAX(published) FROM items i WHERE i.source_id=s.id) latest FROM sources s
    ORDER BY s.enabled DESC, s.supported DESC, COALESCE(s.priority,'P9'), s.category, s.name`).all();
  return Response.json(rows);
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
  const res = db.prepare(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,priority,poll_interval,origin,created)
    VALUES(?,?,?,?,?,?,?,1,1,?,?,'user',?)`).run(name, b.region ?? "global", b.category ?? "community", d.type, d.url, d.homepage, d.username,
    priority, intervalFor(d.type, priority), Date.now() / 1000);
  const src = db.prepare("SELECT * FROM sources WHERE id=?").get(res.lastInsertRowid) as SourceRow;
  return Response.json({ source: src, fetch: await pollOne(src) });
}
