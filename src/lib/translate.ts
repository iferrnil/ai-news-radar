// Fetch-time translation of non-Chinese titles/summaries via DeepSeek (OpenAI-compatible chat API).
// Disabled unless a key is configured (in-app 设置 or DEEPSEEK_API_KEY, see secrets.ts); can be switched off with TRANSLATE_ENABLED=0 or the UI setting (DB `settings`).
// Cost controls: only items that look non-Chinese, title + summary (capped), batched, cached by content hash, never twice.
import { createHash } from "node:crypto";
import { getDb } from "./db";
import { publish } from "./bus";
import { queryItems } from "./store";
import { chatUrl, deepseekKey, deepseekModel } from "./secrets";

const BATCH = Number(process.env.TRANSLATE_BATCH ?? 10);
const CONC = Number(process.env.TRANSLATE_CONCURRENCY ?? 2);
const SUMMARY_CAP = Number(process.env.TRANSLATE_SUMMARY_CHARS ?? 1500);
const SYSTEM = '把每项的 t(标题) 和 s(摘要) 译成简体中文；人名、产品名、代码、链接、@账号保持原样；不要解释。只输出 JSON：{"r":[{"i":序号,"t":"…","s":"…"}]}';

type St = { queue: number[]; queued: Set<number>; running: number; done: number; cached: number; failed: number; calls: number; lastError: string | null; authFailed: boolean; backfilled: boolean; tokIn: number; tokOut: number };
const g = globalThis as unknown as { __tr?: St };
const st: St = (g.__tr ??= { queue: [], queued: new Set(), running: 0, done: 0, cached: 0, failed: 0, calls: 0, lastError: null, authFailed: false, backfilled: false, tokIn: 0, tokOut: 0 });

export const hasKey = () => !!deepseekKey();
/** a new/changed key gets a fresh chance (clears the 401 latch) */
export function resetAuth() { st.authFailed = false; st.lastError = null; }
export function getSetting(k: string): string | null {
  return (getDb().prepare("SELECT v FROM settings WHERE k=?").get(k) as { v: string } | undefined)?.v ?? null;
}
export function setSetting(k: string, v: string) { getDb().prepare("INSERT INTO settings(k,v) VALUES(?,?) ON CONFLICT(k) DO UPDATE SET v=excluded.v").run(k, v); }
export function translateEnabled() {
  if (!hasKey() || st.authFailed) return false;
  const s = getSetting("translate");
  return s != null ? s === "1" : process.env.TRANSLATE_ENABLED !== "0";
}
export const translateStatus = () => ({
  available: hasKey(), enabled: translateEnabled(), setting: getSetting("translate"), env: process.env.TRANSLATE_ENABLED ?? null,
  queued: st.queue.length, running: st.running, done: st.done, cached: st.cached, failed: st.failed, calls: st.calls, tokensIn: st.tokIn, tokensOut: st.tokOut, lastError: st.lastError, model: deepseekModel(),
});

const CJK = /[\u3400-\u9fff\u3040-\u30ff\uac00-\ud7af]/g;
const LATIN = /[A-Za-z\u00C0-\u024F\u0400-\u04FF]/g;
/** true when the text is mostly non-CJK (latin/cyrillic letters outnumber CJK chars 2:1). */
export function needsTranslation(title: string, summary: string) {
  if ((title.match(CJK) ?? []).length >= 4) return false; // a Chinese/Japanese title with some English terms is fine as-is
  const x = `${title} ${summary}`.slice(0, 500);
  const l = (x.match(LATIN) ?? []).length, c = (x.match(CJK) ?? []).length;
  return l > 8 && l > c * 2;
}
const hashOf = (t: string, s: string) => createHash("sha1").update(`${t}\u0000${s}`).digest("base64url").slice(0, 22);

export function enqueueTranslation(ids: number[]) {
  if (!ids.length || !translateEnabled()) return;
  for (const id of ids) if (!st.queued.has(id)) { st.queued.add(id); st.queue.push(id); }
  pump();
}
/** Queue every retained item that still lacks a translation (once per process / when re-enabled). */
export function backfillTranslations(force = false) {
  if (!translateEnabled() || (st.backfilled && !force)) return;
  st.backfilled = true;
  const ids = (getDb().prepare("SELECT id FROM items WHERE title_zh IS NULL AND merged_into IS NULL ORDER BY published DESC").all() as { id: number }[]).map((r) => r.id);
  enqueueTranslation(ids);
}

function pump() {
  while (st.running < CONC && st.queue.length && translateEnabled()) {
    const ids = st.queue.splice(0, BATCH);
    st.running++;
    run(ids).catch((e) => { st.failed += ids.length; st.lastError = String(e); console.error("[translate]", e); })
      .finally(() => { st.running--; ids.forEach((i) => st.queued.delete(i)); pump(); });
  }
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));
async function callApi(batch: { i: number; t: string; s: string }[]) {
  for (let attempt = 0; ; attempt++) {
    st.calls++;
    let res: Response;
    try {
      res = await fetch(chatUrl(), {
        method: "POST", signal: AbortSignal.timeout(90_000),
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${deepseekKey()}` },
        body: JSON.stringify({ model: deepseekModel(), temperature: 0.2, response_format: { type: "json_object" }, max_tokens: 6000,
          messages: [{ role: "system", content: SYSTEM }, { role: "user", content: JSON.stringify(batch) }] }),
      });
    } catch (e) { if (attempt < 3) { await sleep(2000 * 2 ** attempt); continue; } throw e; }
    if (res.status === 401 || res.status === 402) { st.authFailed = res.status === 401; throw new Error(`DeepSeek HTTP ${res.status}（${res.status === 401 ? "API key 无效" : "余额不足"}）`); }
    if ((res.status === 429 || res.status >= 500) && attempt < 3) { await sleep(Number(res.headers.get("retry-after")) * 1000 || 2000 * 2 ** attempt); continue; }
    if (!res.ok) throw new Error(`DeepSeek HTTP ${res.status}: ${(await res.text()).slice(0, 200)}`);
    const j = await res.json() as any;
    st.tokIn += j.usage?.prompt_tokens ?? 0; st.tokOut += j.usage?.completion_tokens ?? 0;
    const txt = String(j.choices?.[0]?.message?.content ?? "").trim();
    try { return (JSON.parse(txt).r ?? []) as { i: number; t?: string; s?: string }[]; }
    catch { if (attempt < 1) continue; throw new Error("DeepSeek 返回的不是合法 JSON"); }
  }
}

async function run(ids: number[]) {
  const db = getDb();
  const rows = db.prepare(`SELECT id,title,summary FROM items WHERE title_zh IS NULL AND id IN (${ids.map(() => "?").join(",")})`).all(...ids) as { id: number; title: string; summary: string }[];
  const set = db.prepare("UPDATE items SET title_zh=?, summary_zh=? WHERE id=?");
  const getC = db.prepare("SELECT t,s FROM tcache WHERE h=?");
  const putC = db.prepare("INSERT OR REPLACE INTO tcache(h,t,s,ts) VALUES(?,?,?,?)");
  const doneIds: number[] = [];
  const todo: { id: number; h: string; t: string; s: string }[] = [];
  for (const r of rows) {
    const t = r.title ?? "", s = (r.summary ?? "").slice(0, SUMMARY_CAP);
    // "" marks "checked, no translation needed" so the item is never looked at again
    if (!needsTranslation(t, s)) { set.run("", "", r.id); continue; }
    const h = hashOf(t, s), c = getC.get(h) as { t: string; s: string } | undefined;
    if (c) { set.run(c.t, c.s, r.id); st.cached++; doneIds.push(r.id); } else todo.push({ id: r.id, h, t, s });
  }
  if (todo.length) {
    const out = await callApi(todo.map((x, i) => ({ i, t: x.t, s: x.s })));
    const now = Date.now() / 1000;
    db.transaction(() => {
      for (const o of out) {
        const x = todo[o.i]; if (!x || !o.t) continue;
        const s = x.s ? (o.s ?? "") : "";
        set.run(o.t.slice(0, 400), s.slice(0, SUMMARY_CAP * 2), x.id); putC.run(x.h, o.t.slice(0, 400), s, now);
        st.done++; doneIds.push(x.id);
      }
    })();
    st.failed += todo.length - out.filter((o) => todo[o.i] && o.t).length;
    st.lastError = null;
  }
  if (doneIds.length) for (const it of queryItems(`AND i.id IN (${doneIds.map(() => "?").join(",")})`, doneIds, doneIds.length, false, true)) publish({ event: "update", item: it });
}
