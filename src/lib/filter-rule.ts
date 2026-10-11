// Per-source filter rules (isomorphic: used by the dialogs' live preview and by the server).
// include / exclude are JS regexes tested against the title (scope "title") or title + summary (scope "all");
// case-insensitive unless `cs`. An item is kept when it matches include (if set) AND does not match exclude (if set).
export interface Rule { include?: string | null; exclude?: string | null; scope?: "title" | "all" | null; cs?: boolean | number | null }
export const RULE_MAX = 200; // pattern length cap

export const ruleText = (scope: Rule["scope"], title: string, summary?: string | null) =>
  (scope === "all" ? `${title ?? ""}\n${(summary ?? "").slice(0, 2000)}` : title ?? "").slice(0, 2500);

/** Cheap structural checks shared by client and server; the server additionally runs the pattern under a timeout. */
export function staticCheck(p: string): string | null {
  if (p.length > RULE_MAX) return `正则过长（最多 ${RULE_MAX} 字符）`;
  // nested quantifiers like (a+)+ / (\w*)* / (x|y+){2,} are the classic catastrophic-backtracking shapes
  if (/\((?:[^()\\]|\\.)*[+*}](?:[^()\\]|\\.)*\)\s*[+*{]/.test(p)) return "正则含嵌套量词（如 (a+)+），可能导致回溯爆炸";
  if (/\\[1-9]/.test(p)) return "不支持反向引用";
  try { new RegExp(p); } catch (e) { return `正则无效：${(e as Error).message.replace(/^Invalid regular expression: /, "")}`; }
  return null;
}

export function compileRule(r: Rule) {
  const fl = r.cs ? "" : "i";
  return { inc: r.include ? new RegExp(r.include, fl) : null, exc: r.exclude ? new RegExp(r.exclude, fl) : null };
}
export const keeps = (c: ReturnType<typeof compileRule>, text: string) => (!c.inc || c.inc.test(text)) && !(c.exc && c.exc.test(text));
export const hasRule = (r: Rule) => !!(r.include || r.exclude);
