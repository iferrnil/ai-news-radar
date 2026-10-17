"use client";
import { useRef, useState } from "react";
import { AlertTriangle, CheckCircle2, FileText, FileUp, Loader2, Star, X } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { MAX_BYTES, SECTION_ZH, parseImport, type Entry, type ImportMode, type ImportPlan, type ParsedRow, type RowStatus } from "@/lib/portable";
import { allSections, api, secZh } from "./common";

const MODES: { v: ImportMode; label: string; hint: string }[] = [
  { v: "merge", label: "合并", hint: "只添加新来源，已存在的保持不变（推荐）" },
  { v: "update", label: "合并并更新", hint: "同时按文件更新已存在来源的分区、优先级、频率与启用状态" },
  { v: "replace", label: "覆盖", hint: "删除本地不在文件中的来源及其内容，需二次确认" },
];
const ST: Record<RowStatus | "remove", { label: string; v: "success" | "brand" | "outline" | "destructive" }> = {
  new: { label: "新增", v: "success" }, restore: { label: "恢复", v: "success" }, update: { label: "更新", v: "brand" },
  dup: { label: "已存在", v: "outline" }, invalid: { label: "无效", v: "destructive" }, remove: { label: "删除", v: "destructive" },
};
const Pill = ({ k }: { k: keyof typeof ST }) => <Badge size="sm" variant={ST[k].v} className={cn(k === "dup" && "text-muted-foreground")}>{ST[k].label}</Badge>;
const fmtSize = (n: number) => (n < 1024 ? `${n} B` : `${(n / 1024).toFixed(1)} KB`);

export default function ImportDialog({ open, onOpenChange, onDone }: { open: boolean; onOpenChange: (b: boolean) => void; onDone: () => void }) {
  const [file, setFile] = useState<{ name: string; text: string } | null>(null);
  const [parsed, setParsed] = useState<{ format: "json" | "opml"; rows: ParsedRow[]; translateAll?: boolean; favorites?: unknown[] } | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [plan, setPlan] = useState<ImportPlan | null>(null);
  const [mode, setMode] = useState<ImportMode>("merge");
  const [osec, setOsec] = useState<Entry["section"]>(() => allSections().find((x) => x.key === "media")?.key ?? allSections()[0]?.key ?? "media");
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
      const p = parseImport(f.text, f.name, s, allSections().map((x) => ({ key: x.key, name: x.zh }))); setParsed(p);
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
      const r = await api<ImportPlan & { added: number }>("/api/sources/import", { method: "POST", body: JSON.stringify({ entries: entries(), mode, dryRun: false, test, ...(parsed?.translateAll ? { translateAll: true } : {}) }) });
      // favorites go after the sources so their rows can attach to the just-imported sources
      const fv = nFav ? await api<{ added: number; existing: number; skipped: number }>("/api/favorites/import", { method: "POST", body: JSON.stringify({ favorites: parsed?.favorites }) }).catch(() => null) : null;
      const c = r.counts;
      toast.success("导入完成", { description: [`新增 ${c.new + c.restore}`, mode === "update" || mode === "replace" ? `更新 ${c.update}` : `跳过已存在 ${c.dup}`, c.invalid ? `无效 ${c.invalid}` : "", c.remove ? `删除 ${c.remove}` : ""].filter(Boolean).concat(fv ? [`收藏 ${fv.added + fv.existing}`] : nFav ? ["收藏导入失败"] : []).join(" · ") + (test && r.added ? "；正在测试抓取新来源" : "") });
      onDone(); onOpenChange(false); reset();
    } catch (e) { toast.error("导入失败", { description: e instanceof Error ? e.message : String(e) }); }
    setBusy(false);
  };
  const c = plan?.counts;
  const nFav = parsed?.favorites?.length ?? 0;
  const willChange = (nFav ? 1 : 0) + (c ? c.new + c.restore + (mode === "merge" ? 0 : c.update) + c.remove : 0);

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
            <div className="flex min-w-0 items-center gap-3 rounded-xl border bg-muted/30 px-3 py-3">
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

          {err && <Alert variant="destructive" className="min-w-0"><AlertTriangle /><AlertDescription className="break-words type-body">{err}</AlertDescription></Alert>}

          {parsed && (
            <div className="grid min-w-0 gap-2">
              <Label className="type-caption text-muted-foreground">导入方式</Label>
              <RadioGroup aria-label="导入方式" value={mode} onValueChange={(v) => { setMode(v as ImportMode); preview(file, v as ImportMode); }} className="grid min-w-0 gap-1.5 sm:grid-cols-3">
                {MODES.map((m) => {
                  const on = mode === m.v, danger = m.v === "replace";
                  return (
                    <Label key={m.v} className={cn("min-w-0 cursor-pointer flex-col items-start gap-0.5 rounded-lg border px-3 py-2 font-normal transition-colors has-focus-visible:ring-3 has-focus-visible:ring-ring/50",
                      on ? (danger ? "border-red-500/60 bg-red-500/[0.06]" : "border-brand/60 bg-brand-soft") : "hover:bg-muted/50")}>
                      <span className="flex items-center gap-2">
                        <RadioGroupItem value={m.v} className={cn("size-3.5", danger ? "data-checked:border-red-500 data-checked:bg-red-500" : "data-checked:border-brand data-checked:bg-brand")} />
                        <span className={cn("type-body font-medium", danger && "text-red-600 dark:text-red-400")}>{m.label}</span>
                      </span>
                      <span className="block pl-5.5 type-meta text-faint">{m.hint}</span>
                    </Label>
                  );
                })}
              </RadioGroup>
              {parsed.format === "opml" && (
                <div className="mt-1 flex min-w-0 flex-wrap items-center gap-2">
                  <Label className="type-caption text-muted-foreground">OPML 默认分区</Label>
                  <Select value={osec} onValueChange={(v) => { setOsec(v as Entry["section"]); preview(file, mode, v as Entry["section"]); }}>
                    <SelectTrigger size="sm" className="w-[120px]"><SelectValue>{(v: string) => secZh(v)}</SelectValue></SelectTrigger>
                    <SelectContent>{allSections().map((x) => <SelectItem key={x.key} value={x.key}>{x.zh}</SelectItem>)}</SelectContent>
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
                {nFav > 0 && <span className="inline-flex items-center gap-1 type-caption text-muted-foreground" data-testid="import-fav"><Star className="size-3 text-amber-500" />收藏 {nFav}</span>}
              </div>
              <div className="max-h-[min(16rem,40vh)] overflow-auto rounded-lg border [&_[data-slot=table-container]]:overflow-visible">
                <Table className="table-fixed type-body">
                  <TableHeader className="sticky top-0 z-10 bg-popover/95 backdrop-blur"><TableRow className="hover:bg-transparent"><TableHead className="h-8 w-[4.5rem] px-3 type-meta font-normal text-faint">状态</TableHead><TableHead className="h-8 px-3 type-meta font-normal text-faint">来源</TableHead><TableHead className="hidden h-8 w-[42%] px-3 type-meta font-normal text-faint sm:table-cell">说明</TableHead></TableRow></TableHeader>
                  <TableBody>
                    {plan.rows.map((r) => {
                      const e = parsed?.rows[r.i]?.entry;
                      const note = r.reason ?? (e ? `${e.sectionName ?? (allSections().some((x) => x.key === e.section) ? secZh(e.section) : SECTION_ZH[e.section as keyof typeof SECTION_ZH] ?? "未分类")} · ${e.priority}${e.filter ? " · 过滤规则" : ""}` : "");
                      return (
                        <TableRow key={r.i} className="border-border/50">
                          <TableCell className="px-3 py-1.5"><Pill k={r.status} /></TableCell>
                          <TableCell className="whitespace-normal px-3 py-1.5"><div className="truncate" title={r.name}>{r.name}</div><div className="truncate type-meta text-faint sm:hidden">{note}</div></TableCell>
                          <TableCell className="hidden truncate px-3 py-1.5 type-caption text-muted-foreground sm:table-cell" title={note}>{note}</TableCell>
                        </TableRow>
                      );
                    })}
                    {mode === "replace" && plan.remove.map((r) => (
                      <TableRow key={`rm-${r.id}`} className="border-border/50">
                        <TableCell className="px-3 py-1.5"><Pill k="remove" /></TableCell>
                        <TableCell className="truncate px-3 py-1.5" title={r.name}>{r.name}</TableCell>
                        <TableCell className="hidden truncate px-3 py-1.5 type-caption text-muted-foreground sm:table-cell">不在文件中，连同内容删除</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
              <Label className="w-fit type-body font-normal text-muted-foreground"><Checkbox checked={test} onCheckedChange={(v) => setTest(!!v)} />导入后测试抓取新来源</Label>
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
            <Button variant={mode === "replace" ? "destructive" : "default"} disabled={!plan || busy || willChange === 0}
              onClick={() => (mode === "replace" ? setConfirm(true) : apply())}>
              {busy && <Loader2 className="animate-spin" />}{busy ? "正在导入…" : willChange === 0 && plan ? "没有需要导入的内容" : mode === "replace" ? `覆盖导入 · ${willChange - (nFav ? 1 : 0) + nFav}` : `导入${willChange ? ` · ${willChange - (nFav ? 1 : 0) + nFav}` : ""}`}
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
