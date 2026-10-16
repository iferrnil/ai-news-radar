"use client";
import { useCallback, useEffect, useState } from "react";
import { Check, Copy, Download, Eye, FileText, Languages, Loader2, Pencil, Sparkles, Undo2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { api } from "./common";
import { useApp } from "./app-shell";

type BriefItem = { title: string; summary: string; url: string; source: string; published: number; translated: boolean };
type BriefGroup = { source: string; priority: string | null; items: BriefItem[] };
export type BriefOut = {
  markdown: string; title: string; scope: string; scopeLabel: string; groups: BriefGroup[] | null;
  count: number; sources: number; generatedAt: number; ai: boolean; cached: boolean;
  aiAvailable: boolean; aiReason: string | null; translate: boolean; translateNote?: string; skill?: string;
  verified?: boolean; noFullText?: { n: number; title: string; source: string; url: string; status: string; reason: string }[];
};

const TR_KEY = "brief-translate";

/** Inline markdown: [[n]](url) → small superscript source link, [text](url), **bold**. */
function Inline({ text }: { text: string }) {
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
function Markdown({ md }: { md: string }) {
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

/** Dialog: rule-based section brief (optionally translated) + optional AI 润色; copy / export markdown. */
export default function BriefDialog({ open, onOpenChange, section, sectionLabel }: {
  open: boolean; onOpenChange: (o: boolean) => void; section: string | null; sectionLabel: string;
}) {
  const { tr } = useApp();
  const [b, setB] = useState<BriefOut | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [aiBusy, setAiBusy] = useState(false);
  const [copied, setCopied] = useState(false);
  // off unless the user turned it on (remembered)
  const [trOn, setTrOn] = useState(false);
  useEffect(() => { try { setTrOn(localStorage.getItem(TR_KEY) === "1"); } catch { /* */ } }, []);
  // the brief is editable: copy / export use the edited text; regenerating replaces it
  const [text, setText] = useState("");
  const [editing, setEditing] = useState(false);
  const edited = !!b && text !== b.markdown;
  const scope = section || "all";

  const load = useCallback(async (ai: boolean, t: boolean) => {
    (ai ? setAiBusy : setBusy)(true); setErr("");
    try {
      const r = await api<BriefOut>(`/api/brief?section=${encodeURIComponent(scope)}${ai ? "&ai=1" : ""}${t ? "&tr=1" : ""}`);
      setB(r); setText(r.markdown); setEditing(false);
    } catch (e) { setErr((e as Error).message); if (!ai) setB(null); else toast.error((e as Error).message); }
    finally { (ai ? setAiBusy : setBusy)(false); }
  }, [scope]);

  useEffect(() => { if (open) { setB(null); setErr(""); void load(false, trOn); } }, [open, load]); // eslint-disable-line react-hooks/exhaustive-deps
  const toggleTr = (v: boolean) => { setTrOn(v); try { localStorage.setItem(TR_KEY, v ? "1" : "0"); } catch { /* */ } void load(!!b?.ai, v); };

  const copy = async () => {
    if (!b) return;
    try { await navigator.clipboard.writeText(text); setCopied(true); toast.success("已复制 Markdown"); setTimeout(() => setCopied(false), 1500); }
    catch { toast.error("复制失败"); }
  };
  const download = () => {
    if (!b) return;
    const name = `简报-${b.scopeLabel}-${new Date().toISOString().slice(0, 10)}.md`;
    const a = Object.assign(document.createElement("a"), { href: URL.createObjectURL(new Blob([text], { type: "text/markdown" })), download: name });
    document.body.append(a); a.click(); a.remove();
  };
  const trHint = tr?.available ? "用 DeepSeek 翻译外文条目" : "仅使用已有译文（未配置 DeepSeek key）";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl" data-testid="brief-dialog">
        <DialogHeader className="gap-1 border-b border-border/60 px-5 py-4">
          <DialogTitle className="flex items-center gap-2 pr-6"><FileText className="size-4 text-brand" />{b?.title ?? `「${sectionLabel}」简报`}</DialogTitle>
          <DialogDescription className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span>{busy && !b ? "正在汇总近 48 小时内容…" : b ? `${b.count} 条 · ${b.sources} 个来源${b.ai ? ` · AI 润色${b.skill ? `（${b.skill}）` : ""}` : " · 规则生成"}${b.cached ? " · 缓存" : ""}${edited ? " · 已编辑" : ""}` : ""}</span>
            <label className="ml-auto inline-flex cursor-pointer items-center gap-2 type-caption text-muted-foreground" title={trHint}>
              <Languages className="size-3.5" />翻译
              <Switch size="sm" checked={trOn} disabled={busy || aiBusy} onCheckedChange={(v) => toggleTr(!!v)} aria-label="翻译简报" data-testid="brief-tr" />
            </label>
          </DialogDescription>
        </DialogHeader>
        <div className="relative min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {err && !b && <p className="type-body text-destructive" data-testid="brief-error">{err}</p>}
          {busy && !b && <div className="flex items-center gap-2 type-body text-muted-foreground"><Loader2 className="size-4 animate-spin" />{trOn && tr?.available ? "生成并翻译中…" : "生成中…"}</div>}
          {b?.ai && (b.noFullText?.length ?? 0) > 0 && (
            <details className="mb-3 rounded-md bg-muted px-3 py-2 type-caption text-muted-foreground" data-testid="brief-nofull">
              <summary className="cursor-pointer select-none">{b.noFullText!.length} 条未取到原文全文，只按标题 / 摘要写{b.verified ? " · 已核对事实" : ""}</summary>
              <ul className="mt-1.5 space-y-0.5">
                {b.noFullText!.map((m) => (
                  <li key={m.n} className="truncate"><span className="font-mono text-faint">[{m.n}]</span> <a href={m.url} target="_blank" rel="noreferrer" className="hover:underline">{m.title}</a> <span className="text-faint">· {m.source} · {m.reason}</span></li>
                ))}
              </ul>
            </details>
          )}
          {b?.ai && b.verified && !(b.noFullText?.length) && <p className="mb-3 type-meta text-faint" data-testid="brief-verified">已抓取全部原文并核对事实</p>}
          {b?.translateNote && <p className="mb-3 rounded-md bg-muted px-3 py-2 type-caption text-muted-foreground" data-testid="brief-tr-note">{b.translateNote}</p>}
          {b && (editing
            ? <textarea value={text} onChange={(e) => setText(e.target.value)} spellCheck={false} aria-label="编辑简报" data-testid="brief-editor"
                className="block h-full min-h-[50dvh] w-full resize-none rounded-md border border-border bg-background p-3 font-mono text-[13px] leading-relaxed outline-none focus-visible:ring-2 focus-visible:ring-ring/40" />
            : <div data-testid="brief-body" className={busy ? "opacity-60 transition-opacity" : undefined}><Markdown md={text} /></div>)}
        </div>
        <Separator />
        <DialogFooter className="flex-row flex-wrap items-center gap-2 px-5 py-3 sm:justify-between">
          <div className="flex flex-wrap gap-2">
            <Button variant={editing ? "secondary" : "outline"} size="sm" disabled={!b || busy} onClick={() => setEditing(!editing)} aria-pressed={editing} data-testid="brief-edit">
              {editing ? <Eye className="size-3.5" /> : <Pencil className="size-3.5" />}{editing ? "预览" : "编辑"}
            </Button>
            {edited && <Button variant="ghost" size="sm" onClick={() => b && setText(b.markdown)} data-testid="brief-revert"><Undo2 className="size-3.5" />还原</Button>}
            <Button variant="outline" size="sm" disabled={!b || busy} onClick={copy} data-testid="brief-copy">
              {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}{copied ? "已复制" : "复制"}
            </Button>
            <Button variant="outline" size="sm" disabled={!b || busy} onClick={download} data-testid="brief-export"><Download className="size-3.5" />导出 Markdown</Button>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {b && !b.aiAvailable && <span className="type-caption text-faint" title={b.aiReason ?? ""}>AI 润色不可用</span>}
            {b?.ai ? (
              <Button variant="outline" size="sm" disabled={busy} onClick={() => load(false, trOn)} data-testid="brief-rule">查看原始简报</Button>
            ) : (
              <Button size="sm" disabled={!b || !b.aiAvailable || aiBusy || busy} onClick={() => load(true, trOn)} data-testid="brief-ai"
                title={b && !b.aiAvailable ? (b.aiReason ?? "需要 DeepSeek API key") : "用 DeepSeek 按简报 Skill 润色"}>
                {aiBusy ? <Loader2 className="size-3.5 animate-spin" /> : <Sparkles className="size-3.5" />}AI 润色
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)}>关闭</Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
