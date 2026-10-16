"use client";
import { Fragment, useCallback, useEffect, useState } from "react";
import { Check, Copy, Download, ExternalLink, FileText, Languages, Loader2, Sparkles } from "lucide-react";
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
};

const TR_KEY = "brief-translate";
const fmt = (ms: number) => ms ? new Date(ms).toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" }) : "";

/** Inline markdown: [text](url), **bold**. */
function Inline({ text }: { text: string }) {
  const parts: React.ReactNode[] = [];
  const re = /\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)|\*\*([^*]+)\*\*/g;
  let last = 0, m: RegExpExecArray | null, k = 0;
  while ((m = re.exec(text))) {
    if (m.index > last) parts.push(text.slice(last, m.index));
    parts.push(m[1] ? <a key={k++} href={m[2]} target="_blank" rel="noreferrer" className="text-brand underline-offset-2 hover:underline">{m[1]}</a> : <strong key={k++} className="font-semibold">{m[3]}</strong>);
    last = re.lastIndex;
  }
  parts.push(text.slice(last));
  return <>{parts}</>;
}

/** Small markdown renderer for the AI-polished brief (headings, lists, quotes, paragraphs). */
function Markdown({ md }: { md: string }) {
  const out: React.ReactNode[] = [];
  let list: string[] = [];
  const flush = () => { if (list.length) { out.push(<ul key={out.length} className="my-2 list-disc space-y-1 pl-5">{list.map((l, i) => <li key={i}><Inline text={l} /></li>)}</ul>); list = []; } };
  for (const raw of md.split("\n")) {
    const l = raw.trimEnd();
    const li = /^\s*[-*]\s+(.*)$/.exec(l);
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
    out.push(<p key={out.length} className="my-1.5"><Inline text={l} /></p>);
  }
  flush();
  return <div className="type-body leading-[1.7] text-foreground/90">{out}</div>;
}

function Groups({ groups }: { groups: BriefGroup[] }) {
  return (
    <div className="space-y-6">
      {groups.map((g) => (
        <section key={g.source} data-testid="brief-group">
          <h3 className="sticky top-0 z-[1] -mx-5 mb-3 flex items-center gap-2 border-b border-border/60 bg-popover/95 px-5 py-1.5 type-ui font-semibold">
            {g.source}{g.priority === "P0" && <span className="rounded bg-brand/10 px-1.5 type-meta font-medium text-brand">P0</span>}
            <span className="ml-auto type-meta font-normal text-faint">{g.items.length} 条</span>
          </h3>
          <ol className="space-y-4">
            {g.items.map((it, i) => (
              <li key={it.url + i} className="space-y-1" data-testid="brief-item">
                <h4 className="type-body font-semibold leading-snug text-foreground">{it.title}</h4>
                {it.summary && <p className="type-body leading-relaxed text-foreground/80" data-testid="brief-summary">{it.summary}</p>}
                <div className="flex flex-wrap items-center gap-x-2 type-meta text-faint" data-testid="brief-meta">
                  <span>来源：{it.source}</span>
                  {it.published > 0 && <><span aria-hidden>·</span><span>{fmt(it.published)}</span></>}
                  {it.translated && <><span aria-hidden>·</span><span>译</span></>}
                  <span aria-hidden>·</span>
                  <a href={it.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-0.5 text-brand hover:underline" data-testid="brief-link">原文链接<ExternalLink className="size-3" /></a>
                </div>
              </li>
            ))}
          </ol>
        </section>
      ))}
    </div>
  );
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
  const [trOn, setTrOn] = useState(true);
  useEffect(() => { try { setTrOn(localStorage.getItem(TR_KEY) !== "0"); } catch { /* */ } }, []);
  const scope = section || "all";

  const load = useCallback(async (ai: boolean, t: boolean) => {
    (ai ? setAiBusy : setBusy)(true); setErr("");
    try {
      setB(await api<BriefOut>(`/api/brief?section=${encodeURIComponent(scope)}${ai ? "&ai=1" : ""}${t ? "&tr=1" : ""}`));
    } catch (e) { setErr((e as Error).message); if (!ai) setB(null); else toast.error((e as Error).message); }
    finally { (ai ? setAiBusy : setBusy)(false); }
  }, [scope]);

  useEffect(() => { if (open) { setB(null); setErr(""); void load(false, trOn); } }, [open, load]); // eslint-disable-line react-hooks/exhaustive-deps
  const toggleTr = (v: boolean) => { setTrOn(v); try { localStorage.setItem(TR_KEY, v ? "1" : "0"); } catch { /* */ } void load(!!b?.ai, v); };

  const copy = async () => {
    if (!b) return;
    try { await navigator.clipboard.writeText(b.markdown); setCopied(true); toast.success("已复制 Markdown"); setTimeout(() => setCopied(false), 1500); }
    catch { toast.error("复制失败"); }
  };
  const download = () => {
    if (!b) return;
    const name = `简报-${b.scopeLabel}-${new Date().toISOString().slice(0, 10)}.md`;
    const a = Object.assign(document.createElement("a"), { href: URL.createObjectURL(new Blob([b.markdown], { type: "text/markdown" })), download: name });
    document.body.append(a); a.click(); a.remove();
  };
  const trHint = tr?.available ? "用 DeepSeek 翻译外文条目" : "仅使用已有译文（未配置 DeepSeek key）";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl" data-testid="brief-dialog">
        <DialogHeader className="gap-1 border-b border-border/60 px-5 py-4">
          <DialogTitle className="flex items-center gap-2 pr-6"><FileText className="size-4 text-brand" />{b?.title ?? `「${sectionLabel}」简报`}</DialogTitle>
          <DialogDescription className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span>{busy && !b ? "正在汇总近 48 小时内容…" : b ? `${b.count} 条 · ${b.sources} 个来源${b.ai ? ` · AI 润色${b.skill ? `（${b.skill}）` : ""}` : " · 规则生成"}${b.cached ? " · 缓存" : ""}` : "按来源分组，优先 P0。"}</span>
            <label className="ml-auto inline-flex cursor-pointer items-center gap-2 type-caption text-muted-foreground" title={trHint}>
              <Languages className="size-3.5" />翻译
              <Switch size="sm" checked={trOn} disabled={busy || aiBusy} onCheckedChange={(v) => toggleTr(!!v)} aria-label="翻译简报" data-testid="brief-tr" />
            </label>
          </DialogDescription>
        </DialogHeader>
        <div className="relative min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {err && !b && <p className="type-body text-destructive" data-testid="brief-error">{err}</p>}
          {busy && !b && <div className="flex items-center gap-2 type-body text-muted-foreground"><Loader2 className="size-4 animate-spin" />{trOn && tr?.available ? "生成并翻译中…" : "生成中…"}</div>}
          {b?.translateNote && <p className="mb-3 rounded-md bg-muted px-3 py-2 type-caption text-muted-foreground" data-testid="brief-tr-note">{b.translateNote}</p>}
          {b && (
            <div data-testid="brief-body" className={busy ? "opacity-60 transition-opacity" : undefined}>
              {b.count === 0 ? <p className="type-body text-muted-foreground">近 48 小时暂无内容。</p>
                : b.ai || !b.groups ? <Markdown md={b.markdown} /> : <Groups groups={b.groups} />}
            </div>
          )}
        </div>
        <Separator />
        <DialogFooter className="flex-row flex-wrap items-center gap-2 px-5 py-3 sm:justify-between">
          <div className="flex flex-wrap gap-2">
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
