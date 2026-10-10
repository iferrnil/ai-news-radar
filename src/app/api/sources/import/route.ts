import { NextRequest } from "next/server";
import { z } from "zod";
import { getDb, intervalFor, seed } from "@/lib/db";
import { publish } from "@/lib/bus";
import { deleteSources } from "@/lib/store";
import { pollOne } from "@/lib/scheduler";
import { normUrl } from "@/lib/util";
import { EntrySchema, MAX_BYTES, MAX_ENTRIES, SECTION_GRP, xProfileUrl, type Entry, type ImportPlan, type PlanRow } from "@/lib/portable";
import type { SourceRow } from "@/lib/types";
export const dynamic = "force-dynamic";

// POST { entries, mode: merge|update|replace, dryRun, test } — dryRun returns the plan only.
// Entries are re-validated here (client parsing is a convenience, not a trust boundary).
const Body = z.object({
  entries: z.array(z.unknown()).max(MAX_ENTRIES),
  mode: z.enum(["merge", "update", "replace"]).default("merge"),
  dryRun: z.boolean().default(true),
  test: z.boolean().default(false),
});
const OV = "enabled,priority,poll_interval"; // imported settings are user overrides: seed sync must not revert them

export async function POST(req: NextRequest) {
  const text = await req.text();
  if (text.length > MAX_BYTES * 1.5) return Response.json({ detail: "请求过大（文件限制 1 MB）" }, { status: 413 });
  let body; try { body = Body.parse(JSON.parse(text)); } catch { return Response.json({ detail: "请求格式错误" }, { status: 400 }); }
  const db = getDb();
  const local = db.prepare("SELECT * FROM sources").all() as SourceRow[];
  const byUrl = new Map(local.map((s) => [normUrl(s.url), s]));
  const byUser = new Map(local.filter((s) => s.username).map((s) => [s.username!.toLowerCase(), s]));
  const byName = new Map(local.map((s) => [s.name.toLowerCase(), s]));
  const tomb = new Set((db.prepare("SELECT name FROM deleted_seeds").all() as { name: string }[]).map((r) => r.name));

  const rows: PlanRow[] = []; const valid: { e: Entry; url: string; match?: SourceRow; restore: boolean; i: number }[] = [];
  const seenKeys = new Set<string>();
  body.entries.forEach((raw, i) => {
    const r = EntrySchema.safeParse(raw);
    if (!r.success) { rows.push({ i, name: String((raw as { name?: unknown })?.name ?? `#${i + 1}`).slice(0, 120), status: "invalid", reason: r.error.issues.map((x) => x.message).join("；") }); return; }
    const e = r.data; const url = e.type === "x" ? xProfileUrl(e.username!) : e.url!;
    const key = e.type === "x" ? `x:${e.username!.toLowerCase()}` : normUrl(url);
    if (seenKeys.has(key)) { rows.push({ i, name: e.name, status: "invalid", reason: "文件内重复" }); return; }
    seenKeys.add(key);
    const match = (e.type === "x" ? byUser.get(e.username!.toLowerCase()) : undefined) ?? byUrl.get(normUrl(url)) ?? byName.get(e.name.toLowerCase());
    const restore = !match && tomb.has(e.name);
    rows.push({ i, name: e.name, status: match ? (body.mode === "merge" ? "dup" : "update") : restore ? "restore" : "new", match: match?.name,
      reason: match ? (match.name.toLowerCase() === e.name.toLowerCase() && normUrl(match.url) !== normUrl(url) ? "同名来源已存在" : "已存在（URL / X 账号相同）") : restore ? "已删除的内置源，将恢复" : undefined });
    valid.push({ e, url, match, restore, i });
  });
  const keep = new Set(valid.filter((v) => v.match).map((v) => v.match!.id));
  const remove = body.mode === "replace" ? local.filter((s) => !keep.has(s.id)).map((s) => ({ id: s.id, name: s.name })) : [];
  const counts = { new: 0, restore: 0, dup: 0, update: 0, invalid: 0, remove: remove.length };
  for (const r of rows) counts[r.status]++;
  const plan: ImportPlan = { rows, remove, counts };
  if (body.dryRun) return Response.json(plan);

  const now = Date.now() / 1000; const added: number[] = [];
  const ins = db.prepare(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,priority,poll_interval,origin,created,grp,overrides)
    VALUES(@name,@region,@category,@type,@url,@homepage,@username,@enabled,1,@priority,@interval,'user',@now,@grp,@ov)`);
  const upd = db.prepare(`UPDATE sources SET enabled=@enabled, priority=@priority, poll_interval=@interval, grp=@grp,
    region=@region, category=@category, overrides=@ov WHERE id=@id`);
  const vals = (e: Entry) => ({
    enabled: e.enabled ? 1 : 0, priority: e.priority, interval: e.interval ?? intervalFor(e.type, e.priority), grp: SECTION_GRP[e.section],
    region: e.section === "cn" ? "cn" : "global", category: e.section === "lab" ? "official" : e.section === "x" ? "community" : "media", ov: OV,
  });
  db.transaction(() => {
    if (remove.length) deleteSources(remove.map((r) => r.id));
    const restores = valid.filter((v) => v.restore);
    if (restores.length) { for (const v of restores) db.prepare("DELETE FROM deleted_seeds WHERE name=?").run(v.e.name); seed(db); }
    for (const v of valid) {
      if (v.match) { if (body.mode !== "merge") upd.run({ ...vals(v.e), id: v.match.id }); continue; }
      const existing = v.restore ? (db.prepare("SELECT id FROM sources WHERE name=?").get(v.e.name) as { id: number } | undefined) : undefined;
      if (existing) { upd.run({ ...vals(v.e), id: existing.id }); added.push(existing.id); continue; }
      const res = ins.run({ ...vals(v.e), name: v.e.name, type: v.e.type, url: v.url, homepage: v.e.homepage ?? null, username: v.e.type === "x" ? v.e.username : null, now });
      added.push(Number(res.lastInsertRowid));
    }
  })();
  publish({ event: "sources" });
  if (body.test && added.length) {
    const list = db.prepare(`SELECT * FROM sources WHERE id IN (${added.map(() => "?").join(",")}) AND enabled=1`).all(...added) as SourceRow[];
    void (async () => { for (let k = 0; k < list.length; k += 4) await Promise.allSettled(list.slice(k, k + 4).map((s) => pollOne(s))); })();
  }
  return Response.json({ ...plan, applied: true, added: added.length });
}
