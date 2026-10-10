import Database from "better-sqlite3";
import fs from "node:fs";
import path from "node:path";
import type { SourceType } from "./types";
import { decode } from "./util";

const DB_PATH = process.env.NEWS_DB ?? path.join(/*turbopackIgnore: true*/ process.cwd(), "data", "news.db");
const SEED_PATH = process.env.SOURCES_JSON ?? path.join(/*turbopackIgnore: true*/ process.cwd(), "sources", "sources.json");
export const RETENTION_HOURS = Number(process.env.RETENTION_HOURS ?? 48);
export const SUPPORTED: SourceType[] = ["rss", "api", "x", "x_list"];

export const TIERS: Record<string, { x: number; feed: number }> = {
  P0: { x: Number(process.env.P0_X ?? 120), feed: Number(process.env.P0_FEED ?? 300) },
  P1: { x: Number(process.env.P1_X ?? 300), feed: Number(process.env.P1_FEED ?? 600) },
  P2: { x: Number(process.env.P2_X ?? 1800), feed: Number(process.env.P2_FEED ?? 1800) },
};
export function intervalFor(type: string, priority?: string | null) {
  const t = TIERS[priority ?? "P1"] ?? TIERS.P1;
  return type === "x" || type === "x_list" ? t.x : t.feed;
}

const SCHEMA = `
CREATE TABLE IF NOT EXISTS sources(
  id INTEGER PRIMARY KEY, name TEXT UNIQUE NOT NULL, region TEXT, category TEXT, type TEXT NOT NULL, url TEXT NOT NULL,
  homepage TEXT, username TEXT, enabled INTEGER DEFAULT 1, supported INTEGER DEFAULT 1, unsupported_reason TEXT,
  priority TEXT, poll_interval INTEGER, last_fetched REAL, last_error TEXT, last_new INTEGER DEFAULT 0,
  origin TEXT DEFAULT 'seed', grp TEXT, note TEXT, created REAL);
CREATE TABLE IF NOT EXISTS items(
  id INTEGER PRIMARY KEY, uid TEXT UNIQUE NOT NULL, source_id INTEGER NOT NULL, title TEXT, url TEXT, summary TEXT,
  published REAL NOT NULL, fetched REAL NOT NULL, is_x INTEGER DEFAULT 0, author TEXT, also_on_x TEXT DEFAULT '[]', merged_into INTEGER);
CREATE INDEX IF NOT EXISTS items_pub ON items(published DESC);
CREATE INDEX IF NOT EXISTS items_src ON items(source_id);
CREATE TABLE IF NOT EXISTS post_links(norm_url TEXT NOT NULL, item_id INTEGER NOT NULL, PRIMARY KEY(norm_url, item_id));
CREATE INDEX IF NOT EXISTS post_links_item ON post_links(item_id);
CREATE TABLE IF NOT EXISTS seen(uid TEXT PRIMARY KEY, ts REAL NOT NULL) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS seen_ts ON seen(ts);
CREATE INDEX IF NOT EXISTS items_src_pub ON items(source_id, published DESC);
DROP INDEX IF EXISTS items_src;
CREATE TABLE IF NOT EXISTS tcache(h TEXT PRIMARY KEY, t TEXT, s TEXT, ts REAL NOT NULL) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS deleted_seeds(name TEXT PRIMARY KEY, ts REAL NOT NULL) WITHOUT ROWID; -- tombstones: built-ins the user deleted
CREATE TABLE IF NOT EXISTS settings(k TEXT PRIMARY KEY, v TEXT) WITHOUT ROWID;
`;

interface SeedSource {
  name: string; region?: string; category?: string; type: SourceType; url: string; homepage?: string; username?: string;
  enabled?: boolean; priority?: string; pollInterval?: number; group?: string; status?: string; note?: string;
}

export function seed(db: Database.Database) {
  if (!fs.existsSync(/*turbopackIgnore: true*/ SEED_PATH)) return;
  const data = JSON.parse(fs.readFileSync(/*turbopackIgnore: true*/ SEED_PATH, "utf8")) as { sources: SeedSource[] };
  const tomb = new Set((db.prepare("SELECT name FROM deleted_seeds").all() as { name: string }[]).map((r) => r.name));
  const have = new Map((db.prepare("SELECT id, name FROM sources").all() as { id: number; name: string }[]).map((r) => [r.name, r.id]));
  const sync = process.env.SYNC_SEED !== "0";
  const ins = db.prepare(`INSERT INTO sources(name,region,category,type,url,homepage,username,enabled,supported,unsupported_reason,priority,poll_interval,origin,grp,note,created)
    VALUES(@name,@region,@category,@type,@url,@homepage,@username,@enabled,@supported,@reason,@priority,@interval,'seed',@grp,@note,@now)`);
  // user overrides win: fields changed in the UI (listed in `overrides`) are never reset by the seed
  const upd = db.prepare(`UPDATE sources SET
    enabled = CASE WHEN instr(overrides, 'enabled') THEN enabled ELSE @enabled END,
    priority = CASE WHEN instr(overrides, 'priority') THEN priority ELSE @priority END,
    poll_interval = CASE WHEN instr(overrides, 'poll_interval') OR instr(overrides, 'priority') THEN poll_interval ELSE @interval END,
    url=@url, username=@username, grp=@grp
    WHERE id=@id AND origin='seed'`);
  const tx = db.transaction(() => {
    for (const s of data.sources) {
      if (tomb.has(s.name)) continue; // deleted by the user: never re-add
      const supported = SUPPORTED.includes(s.type) && (s.status ?? "works") === "works";
      const row = {
        name: s.name, region: s.region ?? null, category: s.category ?? null, type: s.type, url: s.url, homepage: s.homepage ?? null,
        username: s.username ?? null, enabled: s.enabled === false ? 0 : 1, supported: supported ? 1 : 0,
        reason: supported ? null : s.type === "html" ? "需要爬取（HTML，无 feed）" : `状态 ${s.status}（被拦截/不可用）`,
        priority: s.priority ?? null, interval: s.pollInterval ?? intervalFor(s.type, s.priority), grp: s.group ?? null,
        note: s.note ?? null, now: Date.now() / 1000,
      };
      const id = have.get(s.name);
      if (id == null) ins.run(row);
      else if (sync) upd.run({ ...row, id });
    }
    if (process.env.SYNC_DISABLE_USER === "1") db.prepare("UPDATE sources SET enabled=0 WHERE origin='user'").run();
  });
  tx();
}

const g = globalThis as unknown as { __newsDb?: Database.Database };
export function getDb(): Database.Database {
  if (!g.__newsDb) {
    fs.mkdirSync(path.dirname(DB_PATH), { recursive: true });
    const db = new Database(DB_PATH);
    // auto_vacuum must be set before WAL / on an empty DB; convert existing DBs once with VACUUM
    if ((db.pragma("auto_vacuum", { simple: true }) as number) !== 2) { db.pragma("auto_vacuum = INCREMENTAL"); db.exec("VACUUM"); }
    db.pragma("journal_mode = WAL");
    db.pragma("synchronous = NORMAL");      // safe with WAL, far fewer fsyncs
    db.pragma("busy_timeout = 5000");
    db.pragma("cache_size = -4000");        // ~4 MB page cache
    db.pragma("temp_store = MEMORY");
    db.pragma("journal_size_limit = 4194304"); // cap the WAL file at 4 MB after checkpoints
    db.exec(SCHEMA);
    // translation columns (NULL = not processed yet, '' = no translation needed)
    const cols = new Set((db.prepare("PRAGMA table_info(items)").all() as { name: string }[]).map((c) => c.name));
    const scols = new Set((db.prepare("PRAGMA table_info(sources)").all() as { name: string }[]).map((c) => c.name));
    if (!scols.has("overrides")) db.exec("ALTER TABLE sources ADD COLUMN overrides TEXT NOT NULL DEFAULT ''");
    if (!cols.has("image")) db.exec("ALTER TABLE items ADD COLUMN image TEXT");
    if (!cols.has("title_zh")) db.exec("ALTER TABLE items ADD COLUMN title_zh TEXT; ALTER TABLE items ADD COLUMN summary_zh TEXT;");
    seed(db);
    // one-off repair: entities double-escaped in older rows (e.g. "&mdash;")
    const bad = db.prepare("SELECT id,title,summary FROM items WHERE title LIKE '%&%;%' OR summary LIKE '%&%;%'").all() as { id: number; title: string; summary: string }[];
    const fix = db.prepare("UPDATE items SET title=?, summary=? WHERE id=?");
    db.transaction(() => { for (const r of bad) fix.run(decode(r.title ?? ""), decode(r.summary ?? ""), r.id); })();
    g.__newsDb = db;
  }
  return g.__newsDb;
}
