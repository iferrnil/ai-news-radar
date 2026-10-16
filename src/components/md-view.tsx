"use client";
// Small markdown renderer shared by the brief and article dialogs.
/** Inline markdown: [[n]](url) → small superscript source link, [text](url), **bold**. */
export function Inline({ text }: { text: string }) {
  const parts: React.ReactNode[] = [];
  const re = /\[\[(\d+)\]\]\((https?:\/\/[^)\s]+)\)|\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)|\*\*([^*]+)\*\*/g;
  let last = 0, m: RegExpExecArray | null, k = 0;
  while ((m = re.exec(text))) {
    if (m.index > last) parts.push(text.slice(last, m.index));
    if (m[1]) parts.push(<a key={k++} href={m[2]} target="_blank" rel="noreferrer" title={m[2]} data-testid="brief-ref" className="mx-0.5 align-super text-[10px] font-medium text-brand/80 no-underline hover:text-brand">[{m[1]}]</a>);
    else if (m[3]) parts.push(<a key={k++} href={m[4]} target="_blank" rel="noreferrer" className="text-brand underline-offset-2 hover:underline">{m[3]}</a>);
    else parts.push(<strong key={k++} className="font-semibold">{m[5]}</strong>);
    last = re.lastIndex;
  }
  parts.push(text.slice(last));
  return <>{parts}</>;
}

/** Small markdown renderer for the AI-polished brief (headings, lists, quotes, paragraphs). */
export function Markdown({ md }: { md: string }) {
  const out: React.ReactNode[] = [];
  let list: string[] = [];
  const flush = () => { if (list.length) { out.push(<ol key={out.length} className="my-2 list-decimal space-y-0.5 pl-5 type-caption text-muted-foreground">{list.map((l, i) => <li key={i}><Inline text={l} /></li>)}</ol>); list = []; } };
  for (const raw of md.split("\n")) {
    const l = raw.trimEnd();
    const li = /^\s*(?:[-*]|\d+\.)\s+(.*)$/.exec(l);
    if (li) { list.push(li[1]); continue; }
    flush();
    if (!l.trim()) continue;
    const h = /^(#{1,4})\s+(.*)$/.exec(l);
    if (h) {
      const n = h[1].length;
      out.push(n === 1 ? null : n === 2
        ? <h3 key={out.length} className="mt-5 mb-2 border-b border-border/60 pb-1 type-ui font-semibold first:mt-0">{<Inline text={h[2]} />}</h3>
        : <h4 key={out.length} className="mt-3 mb-1 type-body font-semibold"><Inline text={h[2]} /></h4>);
      continue;
    }
    if (l.startsWith(">")) { out.push(<p key={out.length} className="my-2 border-l-2 border-border pl-3 text-muted-foreground"><Inline text={l.replace(/^>\s?/, "")} /></p>); continue; }
    out.push(<p key={out.length} className="my-2.5"><Inline text={l} /></p>);
  }
  flush();
  return <div className="type-body leading-[1.7] text-foreground/90">{out}</div>;
}

