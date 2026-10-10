"use client";
import { useRef, useState } from "react";
import { AlertTriangle, FileUp, Loader2, Upload } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { MAX_BYTES, SECTION_KEYS, SECTION_ZH, parseImport, type Entry, type ImportMode, type ImportPlan, type ParsedRow, type RowStatus } from "@/lib/portable";
import { api } from "./common";

const MODES: { v: ImportMode; label: string; hint: string }[] = [
  { v: "merge", label: "合并（跳过已存在）", hint: "只添加新的来源，已存在的保持不变" },
  { v: "update", label: "合并并更新已存在的设置", hint: "已存在的来源按文件更新分区 / 优先级 / 频率 / 启用状态" },
  { v: "replace", label: "覆盖（删除本地未在文件中的源）", hint: "危险：文件中没有的本地来源及其内容将被删除" },
];
const ST: Record<RowStatus, { label: string; cls: string }> = {
  new: { label: "新增", cls: "text-emerald-600 dark:text-emerald-400" },
  restore: { label: "恢复内置", cls: "text-emerald-600 dark:text-emerald-400" },
  update: { label: "更新", cls: "text-brand" },
  dup: { label: "已存在", cls: "text-muted-foreground" },
  invalid: { label: "无效", cls: "text-red-500" },
};

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
  const [confirm, setConfirm] = useState(false);
  const inp = useRef<HTMLInputElement>(null);

  const reset = () => { setFile(null); setParsed(null); setErr(null); setPlan(null); setMode("merge"); setConfirm(false); };
  const entries = (p = parsed) => (p?.rows ?? []).map((r) => r.entry ?? r.raw);
  const preview = async (f = file, m = mode, s = osec) => {
    if (!f) return;
    setErr(null); setPlan(null);
    try {
      const p = parseImport(f.text, f.name, s); setParsed(p);
      setPlan(await api<ImportPlan>("/api/sources/import", { method: "POST", body: JSON.stringify({ entries: entries(p), mode: m, dryRun: true }) }));
    } catch (e) { setParsed(null); setErr(e instanceof Error ? e.message : String(e)); }
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
        <DialogContent className="grid-cols-[minmax(0,1fr)] sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>导入订阅</DialogTitle>
            <DialogDescription>支持本应用导出的 JSON，或其他阅读器导出的 OPML（≤ 1 MB）。只导入来源设置，不含内容。</DialogDescription>
          </DialogHeader>

          <div
            onDragOver={(e) => { e.preventDefault(); setDrag(true); }} onDragLeave={() => setDrag(false)}
            onDrop={(e) => { e.preventDefault(); setDrag(false); pick(e.dataTransfer.files[0]); }}
            className={cn("flex min-w-0 flex-col items-center justify-center gap-1.5 rounded-lg border border-dashed px-4 py-5 text-center transition-colors", drag ? "border-brand bg-brand-soft" : "border-border")}
          >
            <FileUp className="size-5 text-muted-foreground" />
            <div className="type-body">{file ? <span className="break-all font-medium">{file.name}</span> : "拖放文件到这里"}</div>
            <Button size="sm" variant="outline" onClick={() => inp.current?.click()}><Upload />{file ? "重新选择" : "选择文件"}</Button>
            <input ref={inp} type="file" accept=".json,.opml,.xml,application/json,text/xml" className="hidden" onChange={(e) => { pick(e.target.files?.[0]); e.target.value = ""; }} />
          </div>

          {err && <div className="flex min-w-0 items-start gap-2 rounded-lg bg-red-500/10 px-3 py-2 type-body text-red-600 dark:text-red-400"><AlertTriangle className="mt-0.5 size-4 shrink-0" /><span className="break-words">{err}</span></div>}

          {parsed && (
            <div className="grid min-w-0 gap-3 sm:grid-cols-2">
              <div className="grid min-w-0 gap-1.5">
                <Label>导入方式</Label>
                <Select value={mode} onValueChange={(v) => { setMode(v as ImportMode); preview(file, v as ImportMode); }}>
                  <SelectTrigger className="w-full min-w-0"><SelectValue>{(v: string) => MODES.find((m) => m.v === v)?.label}</SelectValue></SelectTrigger>
                  <SelectContent>{MODES.map((m) => <SelectItem key={m.v} value={m.v} className={m.v === "replace" ? "text-red-600 dark:text-red-400" : ""}>{m.label}</SelectItem>)}</SelectContent>
                </Select>
                <p className={cn("type-meta", mode === "replace" ? "text-red-600 dark:text-red-400" : "text-faint")}>{MODES.find((m) => m.v === mode)?.hint}</p>
              </div>
              {parsed.format === "opml" && (
                <div className="grid min-w-0 gap-1.5">
                  <Label>OPML 默认分区</Label>
                  <Select value={osec} onValueChange={(v) => { setOsec(v as Entry["section"]); preview(file, mode, v as Entry["section"]); }}>
                    <SelectTrigger className="w-full min-w-0"><SelectValue>{(v: string) => SECTION_ZH[v as Entry["section"]]}</SelectValue></SelectTrigger>
                    <SelectContent>{SECTION_KEYS.map((k) => <SelectItem key={k} value={k}>{SECTION_ZH[k]}</SelectItem>)}</SelectContent>
                  </Select>
                  <p className="type-meta text-faint">分类（category）或文件夹名与分区同名时按其归类</p>
                </div>
              )}
            </div>
          )}

          {plan && c && (
            <div className="min-w-0">
              <div className="mb-2 flex flex-wrap gap-1.5 type-caption">
                <Badge variant="outline">新增 {c.new + c.restore}</Badge>
                <Badge variant="outline">{mode === "merge" ? `已存在 ${c.dup}` : `更新 ${c.update}`}</Badge>
                <Badge variant="outline" className={c.invalid ? "text-red-500" : ""}>无效 {c.invalid}</Badge>
                {mode === "replace" && <Badge variant="outline" className="text-red-500">将删除 {c.remove}</Badge>}
              </div>
              <div className="max-h-64 overflow-auto rounded-lg border">
                <table className="w-full table-fixed type-body">
                  <thead className="sticky top-0 bg-card type-meta text-faint"><tr><th className="w-20 px-3 py-1.5 text-left font-normal">状态</th><th className="px-3 py-1.5 text-left font-normal">来源</th><th className="w-[40%] px-3 py-1.5 text-left font-normal">说明</th></tr></thead>
                  <tbody>
                    {plan.rows.map((r) => (
                      <tr key={r.i} className="border-t border-border/50">
                        <td className={cn("px-3 py-1.5 whitespace-nowrap", ST[r.status].cls)}>{ST[r.status].label}</td>
                        <td className="truncate px-3 py-1.5" title={r.name}>{r.name}</td>
                        <td className="truncate px-3 py-1.5 type-caption text-muted-foreground" title={r.reason}>{r.reason ?? (parsed?.rows[r.i]?.entry ? `${SECTION_ZH[parsed.rows[r.i].entry!.section]} · ${parsed.rows[r.i].entry!.priority}` : "")}</td>
                      </tr>
                    ))}
                    {mode === "replace" && plan.remove.map((r) => (
                      <tr key={`rm-${r.id}`} className="border-t border-border/50">
                        <td className="px-3 py-1.5 whitespace-nowrap text-red-500">删除</td>
                        <td className="truncate px-3 py-1.5" title={r.name}>{r.name}</td>
                        <td className="truncate px-3 py-1.5 type-caption text-muted-foreground">不在文件中</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <label className="mt-3 flex items-center gap-2 type-body"><Checkbox checked={test} onCheckedChange={(v) => setTest(!!v)} />导入后测试抓取新来源</label>
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
            <Button variant={mode === "replace" ? "destructive" : "default"} disabled={!plan || busy || willChange === 0}
              onClick={() => (mode === "replace" ? setConfirm(true) : apply())}>
              {busy && <Loader2 className="animate-spin" />}{willChange === 0 && plan ? "没有需要导入的内容" : `导入${willChange ? ` · ${willChange}` : ""}`}
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
