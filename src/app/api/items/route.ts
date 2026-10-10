import { NextRequest } from "next/server";
import { gzipSync } from "node:zlib";
import { queryItems } from "@/lib/store";
export const dynamic = "force-dynamic";

export function GET(req: NextRequest) {
  const p = req.nextUrl.searchParams;
  const w: string[] = []; const a: unknown[] = [];
  const category = p.get("category"), region = p.get("region"), stype = p.get("stype"), pri = p.get("priority"), q = p.get("q"), before = p.get("before");
  if (category) { w.push("s.category=?"); a.push(category); }
  if (region) { w.push("s.region=?"); a.push(region); }
  if (stype === "x") w.push("i.is_x=1"); else if (stype === "web") w.push("i.is_x=0");
  if (pri) { w.push("s.priority=?"); a.push(pri); }
  const sid = p.get("source"); if (sid) { w.push("s.id=?"); a.push(Number(sid)); }
  if (q) { w.push("(i.title LIKE ? OR i.summary LIKE ? OR s.name LIKE ?)"); a.push(`%${q}%`, `%${q}%`, `%${q}%`); }
  if (before) { w.push("i.published < ?"); a.push(Number(before)); }
  const id = p.get("id"); if (id) { w.push("i.id=?"); a.push(Number(id)); }
  const body = JSON.stringify(queryItems(w.map((x) => ` AND ${x}`).join(""), a, Number(p.get("limit") ?? 50), !!id, p.get("zh") === "1"), (_k, v) => (v === null ? undefined : v)); // drop nulls: ~10% smaller
  const headers: Record<string, string> = { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "no-store", Vary: "Accept-Encoding" };
  if (body.length > 1024 && /\bgzip\b/.test(req.headers.get("accept-encoding") ?? "")) {
    return new Response(gzipSync(body, { level: 6 }), { headers: { ...headers, "Content-Encoding": "gzip" } });
  }
  return new Response(body, { headers });
}
