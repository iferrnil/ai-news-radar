"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Check, Copy, Download, Eye, FileText, Languages, Loader2, Pencil, Sparkles, Undo2 } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { api } from "./common";
import { Markdown } from "./md-view";
import { useApp } from "./app-shell";

type BriefItem = { title: string; summary: string; url: string; source: string; published: number; translated: boolean };
type BriefGroup = { source: string; priority: string | null; items: BriefItem[] };
export type BriefOut = {
  markdown: string; title: string; scope: string; scopeLabel: string; groups: BriefGroup[] | null;
  count: number; sources: number; generatedAt: number; ai: boolean; cached: boolean;
  aiAvailable: boolean; aiReason: string | null; translate: boolean; translateNote?: string; skill?: string;
  verified?: boolean; manual?: boolean; saved?: string; noFullText?: { n: number; title: string; source: string; url: string; status: string; reason: string }[];
};

const TR_KEY = "brief-translate";

/** Dialog: rule-based section brief (optionally translated) + optional AI 润色; copy / export markdown. */
export default function BriefDialog({ open, onOpenChange, section, sectionLabel, ids }: {
  open: boolean; onOpenChange: (o: boolean) => void; section: string | null; sectionLabel: string; ids?: number[];
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
  const idq = ids?.length ? `&ids=${ids.join(",")}` : "";
  // Only the newest request may land: a slower response for an earlier section / selection / mode (e.g. the
  // dialog stayed open while ⌘K switched sections) must never replace the current brief.
  const seq = useRef(0);
  const want = useRef("");
  useEffect(() => { want.current = scope + idq; seq.current++; }, [scope, idq]); // a scope change voids what is in flight

  const load = useCallback(async (ai: boolean, t: boolean) => {
    const my = ++seq.current, key = scope + idq;
    (ai ? setAiBusy : setBusy)(true); setErr("");
    const stale = () => my !== seq.current || want.current !== key;
    try {
      const r = await api<BriefOut>(`/api/brief?section=${encodeURIComponent(scope)}${idq}${ai ? "&ai=1" : ""}${t ? "&tr=1" : ""}`);
      if (stale() || r.scope !== scope) return;
      setB(r); setText(r.markdown); setEditing(false);
    } catch (e) { if (stale()) return; setErr((e as Error).message); if (!ai) setB(null); else toast.error((e as Error).message); }
    finally { if (my === seq.current) { setBusy(false); setAiBusy(false); } }
  }, [scope, idq]);

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
            <span>{busy && !b ? (ids?.length ? "正在汇总所选内容…" : "正在汇总近 48 小时内容…") : b ? `${b.count} 条 · ${b.sources} 个来源${b.ai ? ` · AI 润色${b.skill ? `（${b.skill}）` : ""}` : " · 规则生成"}${b.cached ? " · 缓存" : ""}${edited ? " · 已编辑" : ""}` : ""}{b?.saved && <span className="ml-2 text-faint" title={b.saved} data-testid="brief-saved">已保存</span>}</span>
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
        <DialogFooter className="m-0 flex-row flex-wrap items-center gap-2 rounded-b-xl border-t-0 px-5 pt-3 pb-4 sm:justify-between">
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
