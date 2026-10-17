// 一键复制成平台格式 (生成文章): pure converters from the article Markdown + the clipboard writer.
//   公众号: rich HTML with inline styles (the 公众号 editor keeps them on paste) + plain text, written together.
//   小红书: title / body / tags separately.  推特: the whole thread, or one post.

const esc = (s: string) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
const REF = /\[\[(\d+)\]\]\((https?:\/\/[^)\s]+)\)/g;
const LINK = /\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g;

/** the article without the "## 原文链接" list */
export const bodyOf = (md: string) => md.split(/\n##\s+原文链接\s*\n/)[0];
const linksOf = (md: string) => { const i = md.search(/\n##\s+原文链接\s*\n/); return i < 0 ? "" : md.slice(i).replace(/^\n##\s+原文链接\s*\n/, ""); };

/** Markdown → plain text: refs dropped ([[n]] → [n]), links → "text（url）", **bold** unwrapped */
export function plain(md: string, refs: "num" | "url" | "drop" = "num") {
  return md.replace(REF, (_, n, u) => (refs === "num" ? `[${n}]` : refs === "url" ? u : ""))
    .replace(LINK, "$1（$2）").replace(/\*\*([^*]+)\*\*/g, "$1").replace(/^#{1,6}\s+/gm, "")
    .replace(/[ \t]+$/gm, "").replace(/\n{3,}/g, "\n\n").trim();
}

const S = {
  wrap: "font-family:-apple-system,BlinkMacSystemFont,'PingFang SC','Hiragino Sans GB','Microsoft YaHei',sans-serif;font-size:15px;line-height:1.75;color:#333;letter-spacing:0.5px;",
  p: "margin:0 0 16px;",
  h2: "margin:28px 0 14px;padding-left:10px;border-left:4px solid #6d5dfc;font-size:17px;font-weight:bold;line-height:1.45;color:#1a1a1a;",
  h3: "margin:20px 0 10px;font-size:16px;font-weight:bold;color:#1a1a1a;",
  strong: "font-weight:bold;color:#1a1a1a;",
  quote: "margin:0 0 16px;padding:6px 14px;border-left:3px solid #d0d0d8;color:#666;background:#f7f7fa;",
  list: "margin:0 0 16px;padding-left:22px;",
  li: "margin:0 0 6px;",
  sup: "font-size:11px;color:#999;vertical-align:super;",
  refs: "margin:28px 0 0;padding-top:12px;border-top:1px solid #eee;font-size:12px;line-height:1.6;color:#999;",
};
function inlineHtml(t: string) {
  // 公众号 bodies can't link outside: a source ref becomes a small grey [n], other links show their text
  return esc(t).replace(/\[\[(\d+)\]\]\((https?:\/\/[^)\s]+)\)/g, (_, n) => `<sup style="${S.sup}">[${n}]</sup>`)
    .replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, "$1")
    .replace(/\*\*([^*]+)\*\*/g, `<strong style="${S.strong}">$1</strong>`);
}

/** 公众号: the body as inline-styled HTML (the # title goes into the editor's own title field). */
export function wechatHtml(md: string) {
  const out: string[] = [];
  type L = { tag: "ol" | "ul"; items: string[] };
  const st: { list: L | null } = { list: null };
  const flush = () => { const l = st.list; if (l) { out.push(`<${l.tag} style="${S.list}">${l.items.map((x) => `<li style="${S.li}">${x}</li>`).join("")}</${l.tag}>`); st.list = null; } };
  for (const raw of bodyOf(md).split("\n")) {
    const l = raw.trimEnd();
    const li = /^\s*([-*]|\d+\.)\s+(.*)$/.exec(l);
    if (li) { const tag = /\d/.test(li[1]) ? "ol" : "ul"; if (st.list?.tag !== tag) { flush(); st.list = { tag, items: [] }; } st.list!.items.push(inlineHtml(li[2])); continue; }
    flush();
    if (!l.trim() || /^#\s/.test(l)) continue;
    const h = /^(#{2,4})\s+(.*)$/.exec(l);
    if (h) { out.push(h[1].length === 2 ? `<h2 style="${S.h2}">${inlineHtml(h[2])}</h2>` : `<h3 style="${S.h3}">${inlineHtml(h[2])}</h3>`); continue; }
    if (l.startsWith(">")) { out.push(`<blockquote style="${S.quote}">${inlineHtml(l.replace(/^>\s?/, ""))}</blockquote>`); continue; }
    out.push(`<p style="${S.p}">${inlineHtml(l)}</p>`);
  }
  flush();
  const refs = linksOf(md).split("\n").map((x) => x.trim()).filter(Boolean).map((x) => esc(x.replace(LINK, "$1 $2")));
  if (refs.length) out.push(`<section style="${S.refs}">原文链接<br/>${refs.join("<br/>")}</section>`);
  return `<section style="${S.wrap}">${out.join("")}</section>`;
}
export const wechatTitle = (md: string) => /^#\s+(.+)$/m.exec(md)?.[1].trim() ?? "";
export const wechatText = (md: string) => plain(bodyOf(md).replace(/^#\s+.+\n?/m, "")) + (linksOf(md) ? "\n\n原文链接\n" + plain(linksOf(md)) : "");

const TAGS = /(?:^|\s)#[^\s#]{1,30}/g;
/** 小红书: first line = title; trailing line(s) of #话题 = tags; links list dropped (links aren't clickable there). */
export function xhsParts(md: string) {
  const lines = bodyOf(md).trim().split("\n");
  const title = (lines.shift() ?? "").replace(/^#+\s*/, "").trim();
  let i = lines.length - 1;
  while (i >= 0 && !lines[i].trim()) i--;
  const tagLines: string[] = [];
  while (i >= 0 && lines[i].trim() && lines[i].replace(TAGS, "").trim() === "") { tagLines.unshift(lines[i].trim()); i--; }
  const tags = tagLines.join(" ").match(TAGS)?.map((t) => t.trim()) ?? [];
  return { title, body: plain(lines.slice(0, i + 1).join("\n"), "drop"), tags: tags.join(" ") };
}

/** 推特: the posts of the thread ("1/ …"), links as plain URLs (X counts each as 23). */
export function tweetParts(md: string) {
  const b = bodyOf(md).trim();
  const parts = b.split(/\n\s*\n(?=\s*\d{1,2}\/\s)/).map((p) => plain(p, "url")).filter(Boolean);
  return parts.length ? parts : [plain(b, "url")];
}

/** X's weighted length: CJK / full-width count 2, a URL 23 */
export function xWeight(t: string) {
  let n = 0;
  const rest = t.replace(/https?:\/\/\S+/g, () => { n += 23; return ""; });
  for (const ch of rest) { const c = ch.codePointAt(0)!; n += c <= 0x10ff || (c >= 0x2000 && c <= 0x200d) || (c >= 0x2010 && c <= 0x201f) || (c >= 0x2032 && c <= 0x2037) ? 1 : 2; }
  return n;
}

const isDesktop = () => typeof window !== "undefined" && !!(window as unknown as { __radarDesktop?: boolean }).__radarDesktop;

/** Writes text/html (optional) + text/plain. Desktop: the shell's native pasteboard (WKWebView's async clipboard
 *  is unreliable outside a fresh gesture); browser: ClipboardItem, then a copy-event fallback. */
export async function copyRich(text: string, html?: string): Promise<boolean> {
  if (isDesktop()) {
    try {
      const r = await fetch("/api/desktop/clipboard", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ html: html ?? "", text }) });
      if (r.ok && (await r.json()).ok) return true;
    } catch {}
  }
  try {
    if (html && typeof ClipboardItem !== "undefined" && navigator.clipboard?.write) {
      await navigator.clipboard.write([new ClipboardItem({ "text/html": new Blob([html], { type: "text/html" }), "text/plain": new Blob([text], { type: "text/plain" }) })]);
      return true;
    }
    if (!html && navigator.clipboard?.writeText) { await navigator.clipboard.writeText(text); return true; }
  } catch {}
  // fallback: a synchronous copy event carries both flavours
  let ok = false;
  const h = (e: ClipboardEvent) => { e.preventDefault(); e.clipboardData?.setData("text/plain", text); if (html) e.clipboardData?.setData("text/html", html); ok = true; };
  document.addEventListener("copy", h, { once: true });
  try { document.execCommand("copy"); } catch {}
  document.removeEventListener("copy", h);
  return ok;
}
