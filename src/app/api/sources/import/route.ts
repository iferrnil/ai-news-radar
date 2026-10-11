import { NextRequest } from "next/server";
import { z } from "zod";
import { getDb, intervalFor, seed } from "@/lib/db";
import { publish } from "@/lib/bus";
import { deleteSources } from "@/lib/store";
import { pollOne } from "@/lib/scheduler";
import { normUrl } from "@/lib/util";
import { EntrySchema, MAX_BYTES, MAX_ENTRIES, xProfileUrl, type Entry, type ImportPlan, type PlanRow } from "@/lib/portable";
import type { SourceRow } from "@/lib/types";
import { LEGACY_ZH, createSection, isSectionKey, metaFor, seedTarget } from "@/lib/sections";
import { validateRule } from "@/lib/rule-guard";
import { reapplyRule } from "@/lib/store";
export const dynamic = "force-dynamic";

// POST { entries, mode: merge|update|replace, dryRun, test } — dryRun returns the plan only.
// Entries are re-validated here (client parsing is a convenience, not a trust boundary).
const Body = z.object({
  entries: z.array(z.unknown()).max(MAX_ENTRIES),
  mode: z.enum(["merge", "update", "replace"]).default("merge"),
  dryRun: z.boolean().default(true),
  test: z.boolean().default(false),
});
const OV = "enabled,priority,poll_interval,section"; // imported settings (incl. section) are user overrides: seed sync must not revert them

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
    const e = r.data;
    const fr = validateRule(e.filter ? { include: e.filter.include, exclude: e.filter.exclude, scope: e.filter.scope, cs: e.filter.caseSensitive } : {});
    if ("error" in fr) { rows.push({ i, name: e.name, status: "invalid", reason: fr.error }); return; }
    const url = e.type === "x" ? xProfileUrl(e.username!) : e.url!;
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
  const ins = db.prepare(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,priority,poll_interval,origin,created,section,overrides,f_include,f_exclude,f_scope,f_cs)
    VALUES(@name,@region,@category,@type,@url,@homepage,@username,@enabled,1,@priority,@interval,'user',@now,@section,@ov,@fi,@fe,@fs,@fc)`);
  const upd = db.prepare(`UPDATE sources SET enabled=@enabled, priority=@priority, poll_interval=@interval, section=@section,
    region=@region, category=@category, overrides=@ov, f_include=@fi, f_exclude=@fe, f_scope=@fs, f_cs=@fc WHERE id=@id`);
  // sections are matched by NAME first (keys differ between installs); the 5 default keys (lab/cn/…) also match by key
  // (covers renamed defaults and v1 files); otherwise the section is created on demand; last resort 未分类
  const secKey = (e: Entry) => {
    const byName = (n?: string) => n ? (db.prepare("SELECT key FROM sections WHERE name=?").get(n) as { key: string } | undefined)?.key : undefined;
    const hit = byName(e.sectionName) ?? (e.section in LEGACY_ZH && isSectionKey(db, e.section) ? e.section : undefined) ?? (isSectionKey(db, e.section) && !e.sectionName ? e.section : undefined);
    if (hit) return hit;
    const name = e.sectionName ?? LEGACY_ZH[e.section];
    if (name) { const c = createSection(db, { name, icon: e.sectionIcon, inAll: e.sectionInAll ?? e.section !== "ent" }); if ("key" in c) return c.key; }
    return seedTarget(db, "media");
  };
  const vals = (e: Entry) => {
    const k = secKey(e);
    return {
      enabled: e.enabled ? 1 : 0, priority: e.priority, interval: e.interval ?? intervalFor(e.type, e.priority), section: k, ...metaFor(k), ov: OV,
      fi: e.filter?.include || null, fe: e.filter?.exclude || null, fs: e.filter?.scope ?? "title", fc: e.filter?.caseSensitive ? 1 : 0,
    };
  };
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
  for (const v of valid) if (v.match && body.mode !== "merge") reapplyRule(v.match.id); // rules may have changed
  publish({ event: "sources" });
  if (body.test && added.length) {
    const list = db.prepare(`SELECT * FROM sources WHERE id IN (${added.map(() => "?").join(",")}) AND enabled=1`).all(...added) as SourceRow[];
    void (async () => { for (let k = 0; k < list.length; k += 4) await Promise.allSettled(list.slice(k, k + 4).map((s) => pollOne(s))); })();
  }
  return Response.json({ ...plan, applied: true, added: added.length });
}
