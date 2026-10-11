import { hasRule } from "./filter-rule";
import { evalRule, ruleText } from "./rule-guard";
import { getDb, RETENTION_HOURS } from "./db";
import { publish } from "./bus";
import { decode, normUrl } from "./util";
import { enqueueTranslation } from "./translate";
import type { FetchedItem, ItemOut, SourceRow } from "./types";

const ITEM_SQL = `SELECT i.id,i.title,i.url,i.summary,i.published,i.fetched,i.is_x,i.author,i.also_on_x,i.image,i.title_zh,i.summary_zh,
  s.id AS source_id, s.name AS source, s.category, s.region, s.type AS source_type, s.priority, s.homepage, s.username, s.grp
  FROM items i JOIN sources s ON s.id=i.source_id WHERE i.merged_into IS NULL AND i.hidden=0 AND s.enabled=1`;

export const PREVIEW_CHARS = 280; // cards clamp to 3 lines; hover card / drawer fetch the full text via ?id=
/** full: untruncated summary (single-item view). zh: include title_zh/summary_zh (omitted otherwise to keep the list payload small). */
export function queryItems(where = "", args: unknown[] = [], limit = 50, full = false, zh = false): ItemOut[] {
  const rows = getDb().prepare(`${ITEM_SQL} ${where} ORDER BY i.published DESC LIMIT ${Math.min(Math.max(1, limit), 1000)}`).all(...args) as any[];
  const clip = (x: string) => (full || x.length <= PREVIEW_CHARS ? x : x.slice(0, PREVIEW_CHARS));
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  return rows.map(({ fetched: _f, title_zh, summary_zh, ...r }) => {
    const summary = decode(r.summary ?? ""), cut = clip(summary);
    const out: ItemOut = { ...r, title: decode(r.title ?? ""), summary: cut, truncated: cut.length < summary.length, also_on_x: JSON.parse(r.also_on_x || "[]") };
    if (zh && title_zh) {
      out.title_zh = title_zh; out.summary_zh = clip(summary_zh ?? "");
      // list view in Chinese mode: don't ship the original summary too (hover card / drawer fetch the full item with both)
      if (!full && out.summary_zh) { out.summary = ""; out.truncated = true; }
    }
    return out;
  });
}

const X_HOST = /^(x\.com|twitter\.com|t\.co)\//;

/** Insert new items (dedupe by uid), merge tweets into articles they link to ("also on X"). Returns ids. */
export function storeItems(src: SourceRow, items: FetchedItem[]) {
  const db = getDb();
  const now = Date.now() / 1000;
  const cutoff = now - RETENTION_HOURS * 3600;
  const newIds: number[] = [];
  const updated = new Set<number>();
  const removed: number[] = [];
  const exists = db.prepare("SELECT 1 FROM seen WHERE uid=?");
  const markSeen = db.prepare("INSERT OR IGNORE INTO seen VALUES(?,?)");
  const firstFetch = src.last_fetched == null;
  // per-source filter rule: items it rejects are marked seen but never stored (null = rule timed out -> keep all)
  const rule = { include: src.f_include, exclude: src.f_exclude, scope: src.f_scope, cs: src.f_cs };
  const keep = hasRule(rule) ? evalRule(rule, items.map((it) => ruleText(rule.scope, it.title, it.summary))) : null;
  const findArt = db.prepare("SELECT id, also_on_x FROM items WHERE uid=? AND is_x=0");
  const insItem = db.prepare(`INSERT OR IGNORE INTO items(uid,source_id,title,url,summary,published,fetched,is_x,author,merged_into,image)
    VALUES(?,?,?,?,?,?,?,?,?,?,?)`);
  const insLink = db.prepare("INSERT OR IGNORE INTO post_links VALUES(?,?)");
  const setAlso = db.prepare("UPDATE items SET also_on_x=? WHERE id=?");
  const xsFor = db.prepare(`SELECT i.id, i.url, i.author FROM post_links p JOIN items i ON i.id=p.item_id WHERE p.norm_url=? AND i.merged_into IS NULL`);
  const setMerged = db.prepare("UPDATE items SET merged_into=? WHERE id=?");
  db.transaction(() => {
    for (const [n, it] of items.entries()) {
      // `seen` remembers every uid for SEEN_DAYS, so items dropped by the 48h prune never come back as "new"
      if (exists.get(it.uid)) continue;
      markSeen.run(it.uid, now);
      if (keep && !keep[n]) continue; // filtered out by the source's rule
      if (it.published == null && firstFetch) continue; // undated entries on a source's first fetch are a backlog, not news
      const pub = Math.min(it.published ?? now, now); // future-dated / undated -> fetch time
      if (pub < cutoff) continue; // outside retention window
      if (it.isX) {
        const links = [...new Set((it.links ?? []).map(normUrl))].filter((l) => !X_HOST.test(l));
        let art: { id: number; also_on_x: string } | undefined;
        for (const l of links) { art = findArt.get(l) as typeof art; if (art) break; }
        const res = insItem.run(it.uid, src.id, it.title, it.url, it.summary, pub, now, 1, it.author ?? null, art?.id ?? null, it.image ?? null);
        if (!res.changes) continue;
        const xid = Number(res.lastInsertRowid);
        for (const l of links) insLink.run(l, xid);
        if (art) {
          const also = JSON.parse(art.also_on_x || "[]");
          also.push({ url: it.url, author: it.author ?? null });
          setAlso.run(JSON.stringify(also), art.id);
          updated.add(art.id);
        } else newIds.push(xid);
      } else {
        const res = insItem.run(it.uid, src.id, it.title, it.url, it.summary, pub, now, 0, it.author ?? null, null, it.image ?? null);
        if (!res.changes) continue;
        const aid = Number(res.lastInsertRowid);
        const xs = xsFor.all(it.uid) as { id: number; url: string; author: string | null }[];
        if (xs.length) {
          setAlso.run(JSON.stringify(xs.map((x) => ({ url: x.url, author: x.author }))), aid);
          for (const x of xs) { setMerged.run(aid, x.id); removed.push(x.id); }
        }
        newIds.push(aid);
      }
    }
  })();
  enqueueTranslation(newIds);
  for (const id of removed) publish({ event: "removed", id });
  if (newIds.length) for (const r of queryItems(`AND i.id IN (${newIds.map(() => "?").join(",")})`, newIds, newIds.length)) publish({ event: "item", item: r });
  for (const id of updated) for (const r of queryItems("AND i.id=?", [id], 1)) publish({ event: "update", item: r });
  return { newIds, merged: updated.size };
}

/** Drop items past retention and old seen-IDs, then actually give the space back (incremental vacuum + WAL truncate). */
export function prune() {
  const db = getDb();
  const cutoff = Date.now() / 1000 - RETENTION_HOURS * 3600;
  const n = db.transaction(() => {
    db.prepare("DELETE FROM post_links WHERE item_id IN (SELECT id FROM items WHERE published < ?)").run(cutoff);
    db.prepare("DELETE FROM tcache WHERE ts < ?").run(Date.now() / 1000 - 7 * 86400);
    db.prepare("DELETE FROM seen WHERE ts < ?").run(Date.now() / 1000 - Number(process.env.SEEN_DAYS ?? 30) * 86400);
    return db.prepare("DELETE FROM items WHERE published < ?").run(cutoff).changes;
  })();
  db.pragma("incremental_vacuum");
  db.pragma("wal_checkpoint(TRUNCATE)");
  return n;
}

/** Delete sources (any origin) with their items, links and seen-ids. Built-ins get a tombstone so seed sync won't re-add them. */
export function deleteSources(ids: number[]) {
  const db = getDb();
  let deleted = 0, items = 0;
  db.transaction(() => {
    for (const id of ids) {
      const s = db.prepare("SELECT name, origin FROM sources WHERE id=?").get(id) as { name: string; origin: string } | undefined;
      if (!s) continue;
      db.prepare("DELETE FROM seen WHERE uid IN (SELECT uid FROM items WHERE source_id=?)").run(id);
      db.prepare("DELETE FROM post_links WHERE item_id IN (SELECT id FROM items WHERE source_id=?)").run(id);
      items += db.prepare("DELETE FROM items WHERE source_id=?").run(id).changes;
      db.prepare("DELETE FROM sources WHERE id=?").run(id);
      if (s.origin === "seed") db.prepare("INSERT OR REPLACE INTO deleted_seeds VALUES(?,?)").run(s.name, Date.now() / 1000);
      deleted++;
    }
  })();
  if (deleted) { db.pragma("incremental_vacuum"); publish({ event: "sources" }); }
  return { deleted, items };
}

/** Re-evaluate a source's rule over its stored items (retroactive hide/unhide). Returns hidden count. */
export function reapplyRule(sourceId: number) {
  const db = getDb();
  const src = db.prepare("SELECT f_include, f_exclude, f_scope, f_cs FROM sources WHERE id=?").get(sourceId) as { f_include: string | null; f_exclude: string | null; f_scope: "title" | "all" | null; f_cs: number } | undefined;
  if (!src) return 0;
  const rule = { include: src.f_include, exclude: src.f_exclude, scope: src.f_scope, cs: src.f_cs };
  const rows = db.prepare("SELECT id, title, summary FROM items WHERE source_id=?").all(sourceId) as { id: number; title: string; summary: string }[];
  const keep = evalRule(rule, rows.map((r) => ruleText(rule.scope, r.title, r.summary)), 1000);
  const upd = db.prepare("UPDATE items SET hidden=? WHERE id=?");
  let hidden = 0;
  db.transaction(() => { rows.forEach((r, n) => { const h = keep && !keep[n] ? 1 : 0; hidden += h; upd.run(h, r.id); }); })();
  return hidden;
}
