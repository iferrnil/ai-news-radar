"use client";
import { useCallback, useEffect, useState } from "react";
import { Check, Copy, Download, Loader2, Sparkles, FileText } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { api } from "./common";

export type BriefOut = {
  markdown: string; title: string; scope: string; scopeLabel: string;
  count: number; sources: number; generatedAt: number; ai: boolean; cached: boolean;
  aiAvailable: boolean; aiReason: string | null;
};

/** Dialog: rule-based section brief + optional AI 润色; copy / export markdown. */
export default function BriefDialog({ open, onOpenChange, section, sectionLabel }: {
  open: boolean; onOpenChange: (o: boolean) => void; section: string | null; sectionLabel: string;
}) {
  const [b, setB] = useState<BriefOut | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [aiBusy, setAiBusy] = useState(false);
  const [copied, setCopied] = useState(false);
  const scope = section || "all";

  const load = useCallback(async (ai = false) => {
    (ai ? setAiBusy : setBusy)(true); setErr("");
    try {
      const r = await api<BriefOut>(`/api/brief?section=${encodeURIComponent(scope)}${ai ? "&ai=1" : ""}`);
      setB(r);
    } catch (e) { setErr((e as Error).message); if (!ai) setB(null); }
    finally { (ai ? setAiBusy : setBusy)(false); }
  }, [scope]);

  useEffect(() => { if (open) { setB(null); setErr(""); void load(false); } }, [open, load]);

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

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[88dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl" data-testid="brief-dialog">
        <DialogHeader className="gap-1 border-b border-border/60 px-5 py-4">
          <DialogTitle className="flex items-center gap-2 pr-6"><FileText className="size-4 text-brand" />{b?.title ?? `「${sectionLabel}」简报`}</DialogTitle>
          <DialogDescription>
            {busy && !b ? "正在汇总近 48 小时内容…" : b ? `${b.count} 条 · ${b.sources} 个来源${b.ai ? " · AI 润色" : " · 规则生成"}${b.cached ? " · 缓存" : ""}` : "按来源分组，优先 P0，附一行摘要。"}
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {err && !b && <p className="type-body text-destructive" data-testid="brief-error">{err}</p>}
          {busy && !b && <div className="flex items-center gap-2 type-body text-muted-foreground"><Loader2 className="size-4 animate-spin" />生成中…</div>}
          {b && (
            <pre className="whitespace-pre-wrap break-words font-sans type-body leading-[1.7] text-foreground/90" data-testid="brief-body">{b.markdown}</pre>
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
            <Button size="sm" disabled={!b || !b.aiAvailable || aiBusy || busy} onClick={() => load(true)} data-testid="brief-ai"
              title={b && !b.aiAvailable ? (b.aiReason ?? "需要 DeepSeek API key") : "用 DeepSeek 润色成更易读的摘要"}>
              {aiBusy ? <Loader2 className="size-3.5 animate-spin" /> : <Sparkles className="size-3.5" />}AI 润色
            </Button>
            <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)}>关闭</Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
