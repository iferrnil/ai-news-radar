"use client";
// Shared add/edit fields: section picker (with inline "新建分区") and the per-source filter rule with a live preview.
import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Button } from "@/components/ui/button";
import { AlertTriangle, Eye, EyeOff, Filter, Loader2, SmilePlus } from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectSeparator, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { allSections } from "./common";
import { SectionIcon } from "./section-icon";

export const NEW_SECTION = "__new";
export type NewSection = { name: string; icon: string; inAll: boolean };
export const EMPTY_NEW: NewSection = { name: "", icon: "", inAll: true };

const IconPickerPanel = dynamic(() => import("./icon-picker"), { ssr: false, loading: () => <div className="grid h-[280px] w-[300px] place-items-center"><Loader2 className="size-4 animate-spin text-faint" /></div> });

/** Section icon button: shows the current emoji (or a placeholder) and opens the lazy picker panel. */
export function IconPicker({ value, onChange, label = "选择图标", className }: { value: string; onChange: (v: string) => void; label?: string; className?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger render={<Button type="button" variant="outline" aria-label={label} className={cn("w-full px-0 text-[16px]", className)} />}>
        {value ? <SectionIcon icon={value} /> : <SmilePlus className="text-faint" />}
      </PopoverTrigger>
      <PopoverContent align="start" className="w-auto p-3">
        {open && <IconPickerPanel value={value} onPick={(v) => { onChange(v); setOpen(false); }} />}
      </PopoverContent>
    </Popover>
  );
}

export function SectionSelect({ value, onChange, allowNew = true, exclude, className }: { value: string; onChange: (v: string) => void; allowNew?: boolean; exclude?: string; className?: string }) {
  const opts = allSections().filter((s) => s.key !== exclude);
  return (
    <Select value={value} onValueChange={(v) => onChange(v as string)}>
      <SelectTrigger className={cn("w-full", className)} aria-label="分区">
        <SelectValue>{(v: string) => (v === NEW_SECTION ? "＋ 新建分区…" : (() => { const s = opts.find((o) => o.key === v); return s ? s.zh : "选择分区"; })())}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        {opts.map((s) => <SelectItem key={s.key} value={s.key}>{s.zh}</SelectItem>)}
        {allowNew && <><SelectSeparator /><SelectItem value={NEW_SECTION}>＋ 新建分区…</SelectItem></>}
      </SelectContent>
    </Select>
  );
}

/** Inline fields shown when "新建分区" is picked: name, optional emoji icon, include in 全部 (default on). */
export function NewSectionFields({ v, set }: { v: NewSection; set: (v: NewSection) => void }) {
  return (
    <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_4.5rem] items-end gap-3 rounded-lg border border-dashed bg-muted/30 p-3 sm:grid-cols-[minmax(0,1fr)_4.5rem_auto]">
      <div className="grid min-w-0 gap-1.5"><Label htmlFor="sec-name">新分区名称</Label>
        <Input id="sec-name" maxLength={24} placeholder="如：视频生成、UI设计" value={v.name} onChange={(e) => set({ ...v, name: e.target.value })} /></div>
      <div className="grid min-w-0 gap-1.5"><Label>图标</Label><IconPicker value={v.icon} onChange={(icon) => set({ ...v, icon })} label="新分区图标" /></div>
      <Label className="col-span-2 h-8 font-normal sm:col-span-1"><Switch checked={v.inAll} onCheckedChange={(b) => set({ ...v, inAll: b })} />显示在「全部」</Label>
    </div>
  );
}

export type RuleForm = { include: string; exclude: string; scope: "title" | "all"; cs: boolean };
export const EMPTY_RULE: RuleForm = { include: "", exclude: "", scope: "title", cs: false };
export const ruleOf = (s: { f_include?: string | null; f_exclude?: string | null; f_scope?: string | null; f_cs?: number | null }): RuleForm =>
  ({ include: s.f_include ?? "", exclude: s.f_exclude ?? "", scope: s.f_scope === "all" ? "all" : "title", cs: !!s.f_cs });
type Preview = { items: { title: string; keep: boolean }[] };

/** Include / exclude regex inputs + server-validated live preview (sourceId: stored items; samples: titles from 「测试」). */
export function RuleFields({ rule, set, sourceId, samples, onError }: { rule: RuleForm; set: (r: RuleForm) => void; sourceId?: number; samples?: string[]; onError?: (e: string) => void }) {
  const [pv, setPv] = useState<Preview | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const active = !!(rule.include.trim() || rule.exclude.trim());
  const sampleKey = samples?.join("\n") ?? "";
  useEffect(() => {
    if (!sourceId && !sampleKey) { setPv(null); setErr(""); onError?.(""); return; }
    setBusy(true);
    const t = setTimeout(async () => {
      try {
        const r = await fetch("/api/sources/rule-check", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ ...rule, sourceId, samples: sampleKey ? sampleKey.split("\n") : undefined }) });
        const j = await r.json();
        if (!r.ok) { setErr(j.detail ?? "规则无效"); onError?.(j.detail ?? "规则无效"); setPv(null); } else { setErr(""); onError?.(""); setPv(j); }
      } catch { /* offline: keep the last preview */ }
      setBusy(false);
    }, 350);
    return () => clearTimeout(t);
  }, [rule, sourceId, sampleKey]); // eslint-disable-line react-hooks/exhaustive-deps
  const kept = pv?.items.filter((i) => i.keep).length ?? 0, total = pv?.items.length ?? 0;
  return (
    <div className="grid min-w-0 gap-3">
      <div className="grid min-w-0 gap-3 sm:grid-cols-2">
        <div className="grid min-w-0 gap-1.5"><Label htmlFor="rule-inc">只保留匹配（包含）</Label>
          <Input id="rule-inc" className="font-mono" maxLength={200} spellCheck={false} placeholder="如 OpenAI|Claude" value={rule.include} onChange={(e) => set({ ...rule, include: e.target.value })} /></div>
        <div className="grid min-w-0 gap-1.5"><Label htmlFor="rule-exc">隐藏匹配（排除）</Label>
          <Input id="rule-exc" className="font-mono" maxLength={200} spellCheck={false} placeholder="如 not much happened" value={rule.exclude} onChange={(e) => set({ ...rule, exclude: e.target.value })} /></div>
      </div>
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        <Label className="font-normal"><Checkbox checked={rule.scope === "all"} onCheckedChange={(b) => set({ ...rule, scope: b ? "all" : "title" })} />同时匹配摘要</Label>
        <Label className="font-normal"><Checkbox checked={rule.cs} onCheckedChange={(b) => set({ ...rule, cs: !!b })} />区分大小写</Label>
        <span className="type-meta text-faint">JavaScript 正则，默认忽略大小写；最多 200 字符</span>
      </div>
      {err && <Alert variant="destructive" className="p-3"><AlertTriangle /><AlertDescription className="type-body" data-testid="rule-error">{err}</AlertDescription></Alert>}
      {!err && (sourceId || sampleKey) ? (
        <div className="min-w-0 overflow-hidden rounded-lg border bg-muted/30" data-testid="rule-preview">
          <div className="flex items-center gap-2 border-b px-3 py-2 type-caption text-muted-foreground">
            <Filter className="size-3.5" />{active ? <>预览：保留 <b className="font-mono text-foreground">{kept}</b> / 隐藏 <b className="font-mono text-foreground">{total - kept}</b>（最近 {total} 条）</> : <>最近 {total} 条（未设置规则，全部保留）</>}
            {busy && <Loader2 className="ml-auto size-3.5 animate-spin" />}
          </div>
          <ul className="max-h-40 min-w-0 space-y-1 overflow-y-auto overflow-x-hidden p-3 type-caption">
            {pv?.items.map((it, i) => (
              <li key={i} className={cn("flex min-w-0 items-center gap-2", !it.keep && "text-faint line-through decoration-faint/60")}>
                {it.keep ? <Eye className="size-3.5 shrink-0 text-emerald-500" /> : <EyeOff className="size-3.5 shrink-0" />}
                <span className="min-w-0 flex-1 truncate" title={it.title}>{it.title}</span>
              </li>))}
            {pv && !pv.items.length && <li className="text-faint">暂无可预览的条目</li>}
          </ul>
        </div>
      ) : !err && <p className="type-meta text-faint">点「测试」后可预览哪些条目会被保留 / 隐藏。</p>}
    </div>
  );
}
