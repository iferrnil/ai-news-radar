// Server-side regex execution for filter rules, sandboxed with a hard timeout (node:vm interrupts runaway backtracking).
import vm from "node:vm";
import { type Rule, hasRule, ruleText, staticCheck } from "./filter-rule";

const ctx = vm.createContext({});
const script = new vm.Script(`(() => {
  const fl = cs ? "" : "i"; const inc = include ? new RegExp(include, fl) : null; const exc = exclude ? new RegExp(exclude, fl) : null;
  return texts.map((t) => (!inc || inc.test(t)) && !(exc && exc.test(t)));
})()`);

/** keep-flags for each text, or null if the rule timed out / threw (callers then keep everything). */
export function evalRule(r: Rule, texts: string[], timeout = 250): boolean[] | null {
  if (!hasRule(r)) return texts.map(() => true);
  Object.assign(ctx, { include: r.include || null, exclude: r.exclude || null, cs: !!r.cs, texts });
  try { return Array.from(script.runInContext(ctx, { timeout }) as boolean[]); } catch { return null; } finally { ctx.texts = null; }
}

/** Normalize + validate a rule from user input / import. Returns an error message (Chinese) or the clean rule. */
export function validateRule(raw: Partial<Record<keyof Rule, unknown>>, samples: string[] = []): { rule: Required<Rule> } | { error: string } {
  const include = typeof raw.include === "string" && raw.include.trim() ? raw.include.trim() : null;
  const exclude = typeof raw.exclude === "string" && raw.exclude.trim() ? raw.exclude.trim() : null;
  const rule = { include, exclude, scope: raw.scope === "all" ? "all" as const : "title" as const, cs: !!raw.cs };
  for (const [k, p] of [["包含", include], ["排除", exclude]] as const) {
    if (!p) continue;
    const e = staticCheck(p); if (e) return { error: `${k}规则：${e}` };
  }
  if (!hasRule(rule)) return { rule };
  // adversarial probes built from the pattern's own characters + real samples, all under one short timeout
  const chars = [...new Set(`${include ?? ""}${exclude ?? ""}`.replace(/[^\p{L}\p{N} _-]/gu, ""))].slice(0, 12);
  const probes = [...samples.slice(0, 200), ...chars.map((c) => c.repeat(2000) + "!"), "a".repeat(3000) + "!", " ".repeat(3000) + "!"];
  if (!evalRule(rule, probes, 150)) return { error: "正则执行超时（过于复杂，可能导致回溯爆炸），请简化" };
  return { rule };
}

export { ruleText };
