import { NextRequest } from "next/server";
import { getDb } from "@/lib/db";
import { evalRule, ruleText, validateRule } from "@/lib/rule-guard";
export const dynamic = "force-dynamic";

/** Validate a filter rule and preview it: against a stored source's recent items (sourceId, incl. currently hidden ones)
 *  or against sample titles from the add dialog's detection (samples). */
export async function POST(req: NextRequest) {
  const b = await req.json().catch(() => ({}));
  let rows: { title: string; summary?: string }[] = [];
  if (b.sourceId) rows = getDb().prepare("SELECT title, summary FROM items WHERE source_id=? ORDER BY published DESC LIMIT 40").all(Number(b.sourceId)) as typeof rows;
  else if (Array.isArray(b.samples)) rows = b.samples.slice(0, 40).map((t: unknown) => ({ title: String(t ?? "").slice(0, 500) }));
  const v = validateRule(b, rows.map((r) => ruleText(b.scope, r.title, r.summary)));
  if ("error" in v) return Response.json({ detail: v.error }, { status: 400 });
  const keep = evalRule(v.rule, rows.map((r) => ruleText(v.rule.scope, r.title, r.summary))) ?? rows.map(() => true);
  return Response.json({ rule: v.rule, items: rows.map((r, i) => ({ title: r.title, keep: keep[i] })) });
}
