"use client";
import { useRef, useState } from "react";
import { AlertTriangle, CheckCircle2, FileText, FileUp, Loader2, X } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { MAX_BYTES, SECTION_KEYS, SECTION_ZH, parseImport, type Entry, type ImportMode, type ImportPlan, type ParsedRow, type RowStatus } from "@/lib/portable";
import { api } from "./common";

const MODES: { v: ImportMode; label: string; hint: string }[] = [
  { v: "merge", label: "合并", hint: "只添加新来源，已存在的保持不变（推荐）" },
  { v: "update", label: "合并并更新", hint: "同时按文件更新已存在来源的分区、优先级、频率与启用状态" },
  { v: "replace", label: "覆盖", hint: "删除本地不在文件中的来源及其内容，需二次确认" },
];
const ok = "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 shadow-[inset_0_0_0_1px_color-mix(in_oklch,currentColor_22%,transparent)]";
const ST: Record<RowStatus | "remove", { label: string; cls: string }> = {
  new: { label: "新增", cls: ok },
  restore: { label: "恢复", cls: ok },
  update: { label: "更新", cls: "bg-brand-soft text-brand shadow-[inset_0_0_0_1px_color-mix(in_oklch,var(--brand)_22%,transparent)]" },
  dup: { label: "已存在", cls: "text-muted-foreground shadow-[inset_0_0_0_1px_var(--border)]" },
  invalid: { label: "无效", cls: "bg-red-500/10 text-red-600 dark:text-red-400 shadow-[inset_0_0_0_1px_color-mix(in_oklch,currentColor_22%,transparent)]" },
  remove: { label: "删除", cls: "bg-red-500/10 text-red-600 dark:text-red-400 shadow-[inset_0_0_0_1px_color-mix(in_oklch,currentColor_22%,transparent)]" },
};
const Pill = ({ k }: { k: keyof typeof ST }) => <span className={cn("inline-flex h-5 items-center rounded-[5px] px-1.5 type-2xs font-medium whitespace-nowrap", ST[k].cls)}>{ST[k].label}</span>;
const fmtSize = (n: number) => (n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KB`);

export default function ImportDialog({ open, onOpenChange, onDone }: { open: boolean; onOpenChange: (b: boolean) => void; onDone: () => void }) {
  const [file, setFile] = useState<{ name: string; text: string } | null>(null);
  const [parsed, setParsed] = useState<{ format: "json" | "opml"; rows: ParsedRow[] } | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [plan, setPlan] = useState<ImportPlan | null>(null);
  const [mode, setMode] = useState<ImportMode>("merge");
  const [osec, setOsec] = useState<Entry["section"]>("media");
  const [test, setTest] = useState(true);
  const [busy, setBusy] = useState(false);
  const [drag, setDrag] = useState(false);
  const [loading, setLoading] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const inp = useRef<HTMLInputElement>(null);

  const reset = () => { setFile(null); setParsed(null); setErr(null); setPlan(null); setMode("merge"); setConfirm(false); };
  const entries = (p = parsed) => (p?.rows ?? []).map((r) => r.entry ?? r.raw);
  const preview = async (f = file, m = mode, s = osec) => {
    if (!f) return;
    setErr(null); setPlan(null); setLoading(true);
    try {
      const p = parseImport(f.text, f.name, s); setParsed(p);
      setPlan(await api<ImportPlan>("/api/sources/import", { method: "POST", body: JSON.stringify({ entries: entries(p), mode: m, dryRun: true }) }));
    } catch (e) { setParsed(null); setErr(e instanceof Error ? e.message : String(e)); }
    setLoading(false);
  };
  const pick = async (f: File | undefined) => {
    if (!f) return;
    if (f.size > MAX_BYTES) { setErr("文件超过 1 MB"); return; }
    const next = { name: f.name, text: await f.text() }; setFile(next); await preview(next);
  };
  const apply = async () => {
    setBusy(true);
    try {
      const r = await api<ImportPlan & { added: number }>("/api/sources/import", { method: "POST", body: JSON.stringify({ entries: entries(), mode, dryRun: false, test }) });
      const c = r.counts;
      toast.success("导入完成", { description: [`新增 ${c.new + c.restore}`, mode === "update" || mode === "replace" ? `更新 ${c.update}` : `跳过已存在 ${c.dup}`, c.invalid ? `无效 ${c.invalid}` : "", c.remove ? `删除 ${c.remove}` : ""].filter(Boolean).join(" · ") + (test && r.added ? "；正在测试抓取新来源" : "") });
      onDone(); onOpenChange(false); reset();
    } catch (e) { toast.error("导入失败", { description: e instanceof Error ? e.message : String(e) }); }
    setBusy(false);
  };
  const c = plan?.counts;
  const willChange = c ? c.new + c.restore + (mode === "merge" ? 0 : c.update) + c.remove : 0;

  return (
    <>
      <Dialog open={open} onOpenChange={(b) => { onOpenChange(b); if (!b) reset(); }}>
        <DialogContent className="max-h-[calc(100dvh-2rem)] grid-cols-[minmax(0,1fr)] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>导入订阅</DialogTitle>
            <DialogDescription>支持本应用导出的 JSON，或其他阅读器导出的 OPML（≤ 1 MB）。只导入来源设置，不含内容。</DialogDescription>
          </DialogHeader>

          {!file ? (
            <button type="button"
              onDragOver={(e) => { e.preventDefault(); setDrag(true); }} onDragLeave={() => setDrag(false)}
              onDrop={(e) => { e.preventDefault(); setDrag(false); pick(e.dataTransfer.files[0]); }}
              onClick={() => inp.current?.click()}
              className={cn("group flex min-w-0 flex-col items-center justify-center gap-2 rounded-xl border border-dashed px-4 py-8 text-center outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50",
                drag ? "border-brand bg-brand-soft" : "border-foreground/15 bg-muted/20 hover:border-foreground/30 hover:bg-muted/40")}>
              <span className="grid size-10 place-items-center rounded-full bg-muted text-muted-foreground transition-colors group-hover:text-foreground"><FileUp className="size-5" /></span>
              <span className="type-ui font-medium">拖放文件到这里，或点击选择</span>
              <span className="type-meta text-faint">.json（本应用导出）· .opml / .xml（其他阅读器）· 最大 1 MB</span>
            </button>
          ) : (
            <div className="flex min-w-0 items-center gap-3 rounded-xl border bg-muted/30 px-3 py-2.5">
              <span className="grid size-8 shrink-0 place-items-center rounded-lg bg-card text-muted-foreground shadow-card"><FileText className="size-4" /></span>
              <div className="min-w-0 flex-1">
                <div className="truncate type-body font-medium" title={file.name}>{file.name}</div>
                <div className="type-meta text-faint">{parsed ? `${parsed.format.toUpperCase()} · ${parsed.rows.length} 个条目 · ` : ""}{fmtSize(new Blob([file.text]).size)}</div>
              </div>
              {loading ? <Loader2 className="size-4 animate-spin text-muted-foreground" /> : parsed && <CheckCircle2 className="size-4 text-emerald-500" />}
              <Button variant="ghost" size="icon-sm" aria-label="移除文件" onClick={reset}><X /></Button>
            </div>
          )}
          <input ref={inp} type="file" accept=".json,.opml,.xml,application/json,text/xml" className="hidden" onChange={(e) => { pick(e.target.files?.[0]); e.target.value = ""; }} />

          {err && <div role="alert" className="flex min-w-0 items-start gap-2 rounded-lg bg-red-500/10 px-3 py-2 type-body text-red-700 dark:text-red-400"><AlertTriangle className="mt-0.5 size-4 shrink-0" /><span className="break-words">{err}</span></div>}

          {parsed && (
            <div className="grid min-w-0 gap-2">
              <Label className="type-caption text-muted-foreground">导入方式</Label>
              <div role="radiogroup" aria-label="导入方式" className="grid min-w-0 gap-1.5 sm:grid-cols-3">
                {MODES.map((m) => {
                  const on = mode === m.v, danger = m.v === "replace";
                  return (
                    <button key={m.v} type="button" role="radio" aria-checked={on}
                      onClick={() => { setMode(m.v); preview(file, m.v); }}
                      className={cn("min-w-0 rounded-lg border px-3 py-2 text-left outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50",
                        on ? (danger ? "border-red-500/60 bg-red-500/[0.06]" : "border-brand/60 bg-brand-soft") : "hover:bg-muted/50")}>
                      <span className="flex items-center gap-2">
                        <span className={cn("grid size-3.5 shrink-0 place-items-center rounded-full border", on ? (danger ? "border-red-500" : "border-brand") : "border-foreground/30")}>
                          {on && <span className={cn("size-1.5 rounded-full", danger ? "bg-red-500" : "bg-brand")} />}
                        </span>
                        <span className={cn("type-body font-medium", danger && "text-red-600 dark:text-red-400")}>{m.label}</span>
                      </span>
                      <span className="mt-0.5 block pl-5.5 type-meta text-faint">{m.hint}</span>
                    </button>
                  );
                })}
              </div>
              {parsed.format === "opml" && (
                <div className="mt-1 flex min-w-0 flex-wrap items-center gap-2">
                  <Label className="type-caption text-muted-foreground">OPML 默认分区</Label>
                  <Select value={osec} onValueChange={(v) => { setOsec(v as Entry["section"]); preview(file, mode, v as Entry["section"]); }}>
                    <SelectTrigger size="sm" className="w-[120px]"><SelectValue>{(v: string) => SECTION_ZH[v as Entry["section"]]}</SelectValue></SelectTrigger>
                    <SelectContent>{SECTION_KEYS.map((k) => <SelectItem key={k} value={k}>{SECTION_ZH[k]}</SelectItem>)}</SelectContent>
                  </Select>
                  <span className="type-meta text-faint">条目的 category 或文件夹名与分区同名时按其归类</span>
                </div>
              )}
            </div>
          )}

          {parsed && !plan && loading && <div className="grid gap-1.5">{Array.from({ length: 4 }, (_, i) => <Skeleton key={i} className="h-8 rounded-md" />)}</div>}

          {plan && c && (
            <div className="grid min-w-0 gap-2">
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="inline-flex items-center gap-1.5 type-caption text-muted-foreground"><Pill k="new" />{c.new + c.restore}</span>
                <span className="inline-flex items-center gap-1.5 type-caption text-muted-foreground"><Pill k={mode === "merge" ? "dup" : "update"} />{mode === "merge" ? c.dup : c.update}</span>
                <span className="inline-flex items-center gap-1.5 type-caption text-muted-foreground"><Pill k="invalid" />{c.invalid}</span>
                {mode === "replace" && <span className="inline-flex items-center gap-1.5 type-caption text-muted-foreground"><Pill k="remove" />{c.remove}</span>}
              </div>
              <div className="max-h-[min(16rem,40vh)] overflow-auto rounded-lg border">
                <table className="w-full table-fixed type-body">
                  <thead className="sticky top-0 z-10 bg-popover/95 type-meta text-faint backdrop-blur"><tr className="shadow-[inset_0_-1px_0_var(--border)]"><th className="w-[4.5rem] px-3 py-1.5 text-left font-normal">状态</th><th className="px-3 py-1.5 text-left font-normal">来源</th><th className="hidden w-[42%] px-3 py-1.5 text-left font-normal sm:table-cell">说明</th></tr></thead>
                  <tbody>
                    {plan.rows.map((r) => {
                      const e = parsed?.rows[r.i]?.entry;
                      const note = r.reason ?? (e ? `${SECTION_ZH[e.section]} · ${e.priority}` : "");
                      return (
                        <tr key={r.i} className="border-t border-border/40 first:border-t-0">
                          <td className="px-3 py-1.5"><Pill k={r.status} /></td>
                          <td className="px-3 py-1.5"><div className="truncate" title={r.name}>{r.name}</div><div className="truncate type-meta text-faint sm:hidden">{note}</div></td>
                          <td className="hidden truncate px-3 py-1.5 type-caption text-muted-foreground sm:table-cell" title={note}>{note}</td>
                        </tr>
                      );
                    })}
                    {mode === "replace" && plan.remove.map((r) => (
                      <tr key={`rm-${r.id}`} className="border-t border-border/40">
                        <td className="px-3 py-1.5"><Pill k="remove" /></td>
                        <td className="truncate px-3 py-1.5" title={r.name}>{r.name}</td>
                        <td className="hidden truncate px-3 py-1.5 type-caption text-muted-foreground sm:table-cell">不在文件中，连同内容删除</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <label className="flex w-fit items-center gap-2 type-body text-muted-foreground"><Checkbox checked={test} onCheckedChange={(v) => setTest(!!v)} />导入后测试抓取新来源</label>
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
            <Button variant={mode === "replace" ? "destructive" : "default"} disabled={!plan || busy || willChange === 0}
              onClick={() => (mode === "replace" ? setConfirm(true) : apply())}>
              {busy && <Loader2 className="animate-spin" />}{busy ? "正在导入…" : willChange === 0 && plan ? "没有需要导入的内容" : mode === "replace" ? `覆盖导入 · ${willChange}` : `导入${willChange ? ` · ${willChange}` : ""}`}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <AlertDialog open={confirm} onOpenChange={setConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>覆盖本地订阅？</AlertDialogTitle>
            <AlertDialogDescription>将删除 {c?.remove ?? 0} 个不在文件中的本地来源及其已保存的内容（内置源可在「已删除的内置源」中恢复），并按文件更新其余来源。此操作不能撤销。</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => { setConfirm(false); apply(); }}>确认覆盖</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
