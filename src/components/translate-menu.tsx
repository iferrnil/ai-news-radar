"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { Languages, Loader2, Settings } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { softNav } from "@/lib/client-url";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { useApp, useFilter } from "./app-shell";
import { type TrInfo, api, secZh } from "./common";

// Code-split: app-shell renders a placeholder button and loads this on first click.
// ONE control: translation is switched on per VIEW — 「全部」, a section, or a single source (stored server-side; see lib/translate.ts).
// The page's state is effective = 全部 OR its section OR its source; a broader scope is named and can be turned off from here.
type Scope = { scope: "all" | "section" | "source"; key?: string | number; label: string };
type Progress = { total: number; translated: number; pending: number; skipped: number; inQueue: number; queued: number; running: number; done: number; failed: number; lastError: string | null; available: boolean; reason: string | null };
export default function TranslateMenu({ defaultOpen = false }: { defaultOpen?: boolean }) {
  const { sources, tr, reloadTr, onStream } = useApp();
  const { f, trView } = useFilter();
  const [open, setOpen] = useState(defaultOpen);
  const [busy, setBusy] = useState(false);
  const [p, setP] = useState<Progress | null>(null);
  const q = f.source != null ? `source=${f.source}` : f.section ? `section=${encodeURIComponent(f.section)}` : "";
  const loadP = useCallback(async () => { try { setP(await api<Progress>(`/api/translate/progress?${q}`)); } catch { /* keep last */ } }, [q]);
  // live: the server publishes a (throttled) SSE "translate" event whenever the queue moves; item updates also count
  const tm = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (!open) return;
    void reloadTr(); void loadP();
    const off = onStream((ev) => {
      if (ev !== "translate" && ev !== "update" && ev !== "item") return;
      if (tm.current) return;
      tm.current = setTimeout(() => { tm.current = null; void loadP(); void reloadTr(); }, 400);
    });
    return () => { off(); if (tm.current) { clearTimeout(tm.current); tm.current = null; } };
  }, [open, reloadTr, loadP, onStream]);

  const src = f.source != null ? sources.find((s) => s.id === f.source) : undefined;
  const secKey = src ? src.section : f.section || null;
  const here: Scope = f.source != null ? { scope: "source", key: f.source, label: `「${src?.name ?? "此来源"}」` } : f.section ? { scope: "section", key: f.section, label: `「${secZh(f.section)}」分区` } : { scope: "all", label: "「全部」" };
  const s = tr?.scopes;
  const ownOn = !!s && (here.scope === "all" ? s.all : here.scope === "section" ? s.sections.includes(String(here.key)) : s.sources.includes(Number(here.key)));
  // broader scopes that already switch this view on (they win; this page can't turn them off by itself)
  const broader: Scope[] = !s ? [] : [
    ...(here.scope !== "all" && s.all ? [{ scope: "all" as const, label: "「全部」" }] : []),
    ...(here.scope === "source" && secKey && s.sections.includes(secKey) ? [{ scope: "section" as const, key: secKey, label: `「${secZh(secKey)}」分区` }] : []),
  ];
  // the switch is the user's own; the only blocker is a missing / rejected DeepSeek key (reason shown)
  const disabled = !tr || !tr.available;
  const why = !tr ? "读取中…" : !tr.available ? `无法翻译：${tr.reason ?? "翻译不可用"}。` : null;

  const set = async (sc: Scope, on: boolean) => {
    setBusy(true);
    try {
      const r = await api<{ translate: TrInfo }>("/api/settings", { method: "PATCH", body: JSON.stringify({ translate: { scope: sc.scope, key: sc.key, on } }) });
      await reloadTr(); void loadP();
      toast.success(on ? `已开启${sc.label}的翻译` : `已关闭${sc.label}的翻译`, { description: on && r.translate.queued ? `正在翻译 ${r.translate.queued} 条外文内容…` : undefined });
    } catch (e) { toast.error((e as Error).message); }
    finally { setBusy(false); }
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger render={<Button variant="ghost" size="icon" aria-label="翻译" aria-pressed={trView} data-testid="tr-toggle" className={trView ? "text-brand" : ""} />}><Languages className="size-4" /></PopoverTrigger>
      <PopoverContent align="end" className="w-[320px] gap-0 p-0" data-testid="tr-menu">
        <label className="flex cursor-pointer items-start gap-3 p-4">
          <div className="min-w-0 flex-1">
            <div className="type-body font-medium">翻译{here.label}</div>
            <div className="mt-0.5 type-caption leading-snug text-muted-foreground" data-testid="tr-hint">
              {why ?? (broader.length && !ownOn ? `已由${broader[0].label}开启。` : trView ? "外文标题与摘要显示中文译文（标「译」），悬停卡片和详情里可看原文。" : `开启后翻译${here.label}最近 48 小时的外文内容，并自动翻译新内容。`)}
            </div>
          </div>
          <Switch checked={trView} disabled={disabled || busy || (trView && !ownOn)} onCheckedChange={(v) => set(here, !!v)} aria-label={`翻译${here.label}`} />
        </label>
        {!disabled && broader.length > 0 && (
          <div className="-mt-2 space-y-1.5 px-4 pb-3">
            <p className="type-caption text-muted-foreground">{ownOn ? `${broader.map((b) => b.label).join("、")}也已开启翻译；只关闭此处不会生效。` : "要在这里关闭翻译，需要关闭更大范围的开关："}</p>
            {broader.map((b) => <Button key={b.scope} variant="outline" size="sm" disabled={busy} onClick={() => set(b, false)} data-testid={`tr-off-${b.scope}`}>关闭{b.label}的翻译</Button>)}
          </div>
        )}
        {p && !disabled && (
          <>
            <Separator className="bg-border/50" />
            <div className="space-y-2 px-4 py-3" data-testid="tr-stats">
              <div className="flex items-baseline justify-between type-caption text-muted-foreground">
                <span>本页 {p.total} 条 · 已译 <b className="font-medium text-foreground tabular-nums" data-testid="tr-done-scope">{p.translated}</b></span>
                <span className="tabular-nums" data-testid="tr-pending">待翻译 {p.pending}</span>
              </div>
              {p.total - p.skipped > 0 && (
                <div className="h-1.5 overflow-hidden rounded-full bg-muted" role="progressbar" aria-valuemin={0} aria-valuemax={p.total - p.skipped} aria-valuenow={p.translated}>
                  <div className="h-full rounded-full bg-brand transition-[width] duration-300" style={{ width: `${Math.round((p.translated / Math.max(1, p.total - p.skipped)) * 100)}%` }} />
                </div>
              )}
              <div className="grid grid-cols-4 gap-1 text-center type-meta text-muted-foreground" data-testid="tr-queue">
                {([["排队", p.queued], ["进行中", p.running], ["已完成", p.done], ["失败", p.failed]] as const).map(([k, v]) => (
                  <div key={k} className={k === "失败" && v ? "text-red-500" : ""}><div className="type-caption font-medium tabular-nums text-foreground">{k === "进行中" && v ? <Loader2 className="mx-auto my-0.5 size-3 animate-spin" /> : v}</div>{k}</div>
                ))}
              </div>
              {p.skipped > 0 && <p className="type-meta text-faint">{p.skipped} 条已是中文，无需翻译</p>}
              {p.lastError && <p className="type-meta text-red-500">{p.lastError}</p>}
            </div>
          </>
        )}
        <Separator className="bg-border/50" />
        <button type="button" onClick={() => { setOpen(false); softNav("/settings"); }} className="flex w-full items-center gap-2 px-4 py-2.5 type-caption text-muted-foreground hover:bg-hover hover:text-foreground"><Settings className="size-3.5" />DeepSeek 设置…</button>
      </PopoverContent>
    </Popover>
  );
}
