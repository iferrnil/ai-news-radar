"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { Languages, SlidersHorizontal } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { useApp } from "./app-shell";
import { type TrInfo, api, secZh } from "./common";

// Code-split: app-shell renders a placeholder button and loads this on first click.
// ONE control: translation is switched on per VIEW — 「全部」, a section, or a single source (stored server-side; see lib/translate.ts).
// The page's state is effective = 全部 OR its section OR its source; a broader scope is named and can be turned off from here.
type Scope = { scope: "all" | "section" | "source"; key?: string | number; label: string };
export default function TranslateMenu({ defaultOpen = false }: { defaultOpen?: boolean }) {
  const { f, sources, tr, settingsLocked, reloadTr, trView } = useApp();
  const [open, setOpen] = useState(defaultOpen);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!open) return;
    void reloadTr(); const t = setInterval(() => void reloadTr(), 3000); return () => clearInterval(t);
  }, [open, reloadTr]);

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
  const disabled = !tr || !tr.available || settingsLocked;
  const why = !tr ? "读取中…" : !tr.available ? `${tr.reason ?? "翻译不可用"}。` : settingsLocked ? "设置已锁定（SETTINGS_LOCKED=1），无法切换翻译。" : null;

  const set = async (sc: Scope, on: boolean) => {
    setBusy(true);
    try {
      const r = await api<{ translate: TrInfo }>("/api/settings", { method: "PATCH", body: JSON.stringify({ translate: { scope: sc.scope, key: sc.key, on } }) });
      await reloadTr();
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
        {tr && tr.available && (tr.queued > 0 || tr.done > 0 || tr.failed > 0 || tr.lastError) && (
          <>
            <Separator className="bg-border/50" />
            <div className="px-4 py-2.5 type-caption text-muted-foreground" data-testid="tr-stats">
              已译 {tr.done}（缓存 {tr.cached}）· 排队 {tr.queued}{tr.failed ? ` · 失败 ${tr.failed}` : ""}
              {tr.lastError && <div className="mt-1 type-meta text-red-500">{tr.lastError}</div>}
            </div>
          </>
        )}
        <Separator className="bg-border/50" />
        <Link href="/settings" onClick={() => setOpen(false)} className="flex items-center gap-2 px-4 py-2.5 type-caption text-muted-foreground hover:bg-hover hover:text-foreground"><SlidersHorizontal className="size-3.5" />DeepSeek 设置…</Link>
      </PopoverContent>
    </Popover>
  );
}
