"use client";
import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, CheckCircle2, Download, FileJson, FileUp, Rss, SearchX, ExternalLink, Link2, Loader2, MoreHorizontal, Plus, RefreshCw, RotateCcw, Search, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import dynamic from "next/dynamic";
import { buildJson, buildOpml, exportFileName, type ExportSource } from "@/lib/portable-export";

const ImportDialog = dynamic(() => import("./import-dialog"), { ssr: false });

function download(name: string, text: string, mime: string) {
  const url = URL.createObjectURL(new Blob([text], { type: mime }));
  const a = Object.assign(document.createElement("a"), { href: url, download: name }); document.body.append(a); a.click(); a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ButtonGroup } from "@/components/ui/button-group";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { EmptyState } from "./empty-state";
import { useApp } from "./app-shell";
import { SECTIONS, type SectionKey, TYPE, type SourceRow, ago, api, bj, secZh, sectionOf } from "./common";
import { SourceAvatar } from "./source-avatar";

type StatusKey = "active" | "err" | "off" | "paused" | "all";
// 在线 = enabled & fetchable · 报错 = 在线 with last_error · 停用 = switched off · 暂停 = enabled but not fetchable (unsupported)
const STATUS: { v: StatusKey; label: string; badge: "success" | "destructive" | "muted" | "warning" | "outline" }[] = [
  { v: "active", label: "在线", badge: "success" }, { v: "err", label: "报错", badge: "destructive" }, { v: "off", label: "停用", badge: "muted" },
  { v: "paused", label: "暂停", badge: "warning" }, { v: "all", label: "全部", badge: "outline" },
];
const statusOk = (s: SourceRow, k: StatusKey) => k === "all" ? true : k === "active" ? !!(s.enabled && s.supported) : k === "err" ? !!(s.enabled && s.supported && s.last_error) : k === "off" ? !s.enabled : !!(s.enabled && !s.supported);

const IV = [60, 120, 180, 300, 600, 1800, 3600];
const ivLabel = (v: number) => (v < 3600 ? `${v / 60} 分钟` : `${v / 3600} 小时`);

function StatusBadge({ s }: { s: SourceRow }) {
  if (!s.enabled) return <Badge variant="muted">已停用</Badge>;
  if (!s.supported) return <Badge variant="warning" title={s.unsupported_reason ?? undefined}>暂停 · 不支持</Badge>;
  if (s.last_error) return (
    <Tooltip><TooltipTrigger render={<Badge variant="destructive" className="max-w-[180px]" />}>
      <AlertTriangle className="size-3" /><span className="truncate">报错</span></TooltipTrigger>
      <TooltipContent className="max-w-sm">{s.last_error}</TooltipContent></Tooltip>);
  if (!s.last_fetched) return <Badge variant="warning"><Loader2 className="animate-spin" />等待</Badge>;
  return <Badge variant="success"><span className="size-1.5 rounded-full bg-emerald-500" />正常{s.last_new ? ` +${s.last_new}` : ""}</Badge>;
}

type Detect = { type: string; name: string; url: string; count: number; overlap: number; sample: { title: string; published: number | null }[] };

function AddDialog({ open, onOpenChange, onAdded }: { open: boolean; onOpenChange: (b: boolean) => void; onAdded: () => void }) {
  const [form, setForm] = useState({ input: "", name: "", section: "media" as SectionKey, priority: "P1", force: false });
  const [secTouched, setSecTouched] = useState(false); // stop auto-suggesting once the user picks a section
  const suggest = (input: string, type?: string) => (/^@|x\.com\/|twitter\.com\//i.test(input.trim()) || type?.startsWith("x") ? "x" : /\.cn(\/|$)|[\u4e00-\u9fff]/.test(input) ? "cn" : "media") as SectionKey;
  const [res, setRes] = useState<Detect | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState<"" | "test" | "add">("");
  const test = async () => {
    setBusy("test"); setErr(""); setRes(null);
    try { const r = await api<Detect>("/api/sources/test", { method: "POST", body: JSON.stringify(form) }); setRes(r); if (!secTouched) setForm((f) => ({ ...f, section: suggest(f.input, r.type) })); } catch (e) { setErr((e as Error).message); }
    setBusy("");
  };
  const add = async () => {
    setBusy("add"); setErr("");
    try {
      const d = await api("/api/sources", { method: "POST", body: JSON.stringify(form) });
      toast.success(`已添加「${d.source.name}」`, { description: d.fetch.error ? `首次抓取失败：${d.fetch.error}` : `首次抓取 ${d.fetch.fetched} 条，新增 ${d.fetch.new} 条` });
      onAdded(); onOpenChange(false); setRes(null); setSecTouched(false); setForm({ ...form, input: "", name: "", force: false });
    } catch (e) { setErr((e as Error).message); }
    setBusy("");
  };
  const sel = (k: "section" | "priority", opts: [string, string][]) => (
    <Select value={form[k]} onValueChange={(v) => { if (k === "section") setSecTouched(true); setForm({ ...form, [k]: v as never }); }}>
      <SelectTrigger className="w-full"><SelectValue>{(v: string) => opts.find((o) => o[0] === v)?.[1]}</SelectValue></SelectTrigger>
      <SelectContent>{opts.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
    </Select>
  );
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="grid-cols-[minmax(0,1fr)] sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>添加数据源</DialogTitle>
          <DialogDescription>网页或 Feed URL（自动发现 RSS/Atom）、X 用户名 @handle，或 X 列表链接。</DialogDescription>
        </DialogHeader>
        <div className="grid min-w-0 gap-4">
          <div className="grid min-w-0 gap-1.5">
            <Label htmlFor="src-input">地址 / 账号</Label>
            <div className="flex min-w-0 gap-2">
              <Input id="src-input" className="min-w-0 flex-1" autoFocus placeholder="https://simonwillison.net 或 @karpathy" value={form.input}
                onChange={(e) => { const v = e.target.value; setForm({ ...form, input: v, section: secTouched ? form.section : suggest(v) }); setRes(null); }} onKeyDown={(e) => e.key === "Enter" && form.input && test()} />
              <Button variant="outline" className="shrink-0" onClick={test} disabled={!form.input || !!busy}>{busy === "test" ? <Loader2 className="animate-spin" /> : <RefreshCw />}测试</Button>
            </div>
          </div>
          <div className="grid min-w-0 grid-cols-2 gap-3 sm:grid-cols-[minmax(0,2fr)_minmax(0,1.3fr)_minmax(0,1fr)]">
            <div className="col-span-2 grid min-w-0 gap-1.5 sm:col-span-1"><Label>名称</Label><Input className="w-full" placeholder="可选" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></div>
            <div className="grid min-w-0 gap-1.5"><Label>分区</Label>{sel("section", SECTIONS.map((x) => [x.key, x.zh]))}</div>
            <div className="grid min-w-0 gap-1.5"><Label>优先级</Label>{sel("priority", [["P0", "P0"], ["P1", "P1"], ["P2", "P2"]])}</div>
          </div>
          {err && <Alert variant="destructive" className="p-3"><AlertTriangle /><AlertDescription className="type-body">{err}</AlertDescription></Alert>}
          {res && (
            <div className="min-w-0 overflow-hidden rounded-lg border bg-muted/30">
              <div className="flex items-start gap-2 border-b p-3">
                <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-500" />
                <div className="min-w-0 flex-1 type-body">
                  <div className="flex min-w-0 items-center gap-1 font-medium"><span className="truncate">{res.name}</span><Badge variant="secondary" className="ml-1 shrink-0">{TYPE[res.type]}</Badge></div>
                  <div className="break-all text-muted-foreground">{res.url} · {res.count} 条</div>
                </div>
              </div>
              {res.overlap >= 0.5 && (
                <Alert variant="warning" className="rounded-none border-0 border-b p-3">
                  <AlertTriangle />
                  <AlertDescription className="type-body">疑似重复：{Math.round(res.overlap * 100)}% 的示例标题已被现有来源收录。
                    <Label className="mt-1.5 font-normal"><Checkbox checked={form.force} onCheckedChange={(v) => setForm({ ...form, force: !!v })} />仍然添加</Label>
                  </AlertDescription>
                </Alert>
              )}
              <ul className="max-h-48 min-w-0 space-y-1.5 overflow-y-auto overflow-x-hidden p-3 type-caption">
                {res.sample.map((s, i) => <li key={i} className="flex min-w-0 gap-2"><span className="shrink-0 font-mono text-muted-foreground">{bj(s.published)}</span><span className="min-w-0 flex-1 truncate" title={s.title}>{s.title}</span></li>)}
              </ul>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>取消</Button>
          <Button onClick={add} disabled={!form.input || !!busy}>{busy === "add" && <Loader2 className="animate-spin" />}添加并订阅</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default function Sources() {
  const { sources: rows, reloadSources, addOpen, setAddOpen } = useApp();
  const [q, setQ] = useState(""); const [ft, setFt] = useState("all"); const [fsec, setFsec] = useState("all"); const [fs, setFs] = useState<StatusKey>("active");
  const [busy, setBusy] = useState<Record<number, boolean>>({});
  useEffect(() => { const t = setInterval(() => { if (!document.hidden) reloadSources(); }, 15000); return () => clearInterval(t); }, [reloadSources]);

  const view = useMemo(() => rows.filter((s) =>
    (!q || (s.name + s.url).toLowerCase().includes(q.toLowerCase())) && (ft === "all" || s.type === ft) && (fsec === "all" || sectionOf({ ...s, is_x: s.type.startsWith("x") }) === fsec) &&
    statusOk(s, fs)), [rows, q, ft, fs, fsec]);
  const fsCount = useMemo(() => Object.fromEntries(STATUS.map((x) => [x.v, rows.filter((s) => statusOk(s, x.v)).length])) as Record<StatusKey, number>, [rows]);
  const nAct = rows.filter((s) => s.enabled && s.supported).length;
  const nErr = rows.filter((s) => s.enabled && s.supported && s.last_error).length;
  const nItems = rows.reduce((a, s) => a + (s.enabled ? s.item_count : 0), 0);

  const patch = async (s: SourceRow, body: object, msg?: string) => {
    await api(`/api/sources/${s.id}`, { method: "PATCH", body: JSON.stringify(body) }); await reloadSources(); if (msg) toast.success(msg);
  };
  const fetchNow = async (s: SourceRow) => {
    setBusy((b) => ({ ...b, [s.id]: true }));
    const p = api(`/api/sources/${s.id}/fetch`, { method: "POST" }).then((r) => { if (r.error) throw new Error(r.error); return r; });
    toast.promise(p, { loading: `正在抓取 ${s.name}…`, success: (r: any) => `${s.name}：获取 ${r.fetched} 条，新增 ${r.new} 条`, error: (e: Error) => `${s.name} 抓取失败：${e.message}` });
    await p.catch(() => {}); setBusy((b) => ({ ...b, [s.id]: false })); reloadSources();
  };
  const [sel, setSel] = useState<Set<number>>(new Set());
  const [toDel, setToDel] = useState<SourceRow[] | null>(null);
  const [deleted, setDeleted] = useState<{ name: string; ts: number }[]>([]);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const loadDeleted = useCallback(() => api<{ name: string; ts: number }[]>("/api/sources/deleted").then(setDeleted).catch(() => {}), []);
  useEffect(() => { loadDeleted(); }, [loadDeleted, rows.length]);
  const confirmDelete = async () => {
    const list = toDel ?? []; setToDel(null);
    try {
      const r = await api<{ deleted: number; items: number }>("/api/sources/bulk", { method: "POST", body: JSON.stringify({ action: "delete", ids: list.map((x) => x.id) }) });
      toast.success(list.length === 1 ? `已删除「${list[0].name}」` : `已删除 ${r.deleted} 个来源`, { description: `同时删除 ${r.items} 条已存内容${list.some((x) => x.origin !== "user") ? "；内置源可在「已删除的内置源」中恢复" : ""}` });
      setSel(new Set()); await reloadSources(); loadDeleted();
    } catch (e) { toast.error("删除失败", { description: (e as Error).message }); }
  };
  const restore = async (names: string[] | "all") => {
    const r = await api<{ restored: number }>("/api/sources/deleted", { method: "POST", body: JSON.stringify({ names }) });
    toast.success(`已恢复 ${r.restored} 个内置源`); await reloadSources(); loadDeleted();
  };
  const toggleSel = (id: number, on: boolean) => setSel((cur) => { const n = new Set(cur); if (on) n.add(id); else n.delete(id); return n; });
  const allSel = view.length > 0 && view.every((s) => sel.has(s.id));
  const [impOpen, setImpOpen] = useState(false);
  const [scope, setScope] = useState<"all" | "enabled" | "view" | "sel">("all");
  const doExport = (fmt: "json" | "opml") => {
    const base = scope === "enabled" ? rows.filter((r) => r.enabled) : scope === "view" ? view : scope === "sel" ? rows.filter((r) => sel.has(r.id)) : rows;
    const list: ExportSource[] = base.filter((r) => r.type !== "html").map((r) => ({ ...r, section: sectionOf(r) }));
    if (!list.length) { toast.error("没有可导出的来源"); return; }
    const d = new Date();
    if (fmt === "json") download(exportFileName("json", d), buildJson(list, d), "application/json");
    else download(exportFileName("opml", d), buildOpml(list, d), "text/x-opml");
    const nx = list.filter((r) => r.type.startsWith("x")).length;
    toast.success(`已导出 ${list.length} 个来源`, { description: fmt === "opml" && nx ? `${nx} 个 X 来源以自定义 type="x" 写入，其他阅读器会忽略它们` : undefined });
  };
  const stats = [["在线来源", nAct], ["报错", nErr], ["48h 条目", nItems], ["目录总数", rows.length]] as const;

  return (
    <div className="mx-auto w-full max-w-[1200px] px-3 pb-20 pt-6 sm:px-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="type-h1 font-semibold tracking-tight">数据源</h1>
          <p className="mt-0.5 type-body text-muted-foreground">启用/停用、调整抓取频率、立即抓取，或添加新的 RSS / X 账号 / X 列表。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {deleted.length > 0 && <Button variant="outline" onClick={() => setRestoreOpen(true)}><RotateCcw />已删除的内置源 · {deleted.length}</Button>}
          <ButtonGroup aria-label="导入导出">
            <Button variant="outline" onClick={() => setImpOpen(true)}><FileUp />导入</Button>
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button variant="outline" />}><Download />导出</DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-64">
                <DropdownMenuGroup>
                  <DropdownMenuLabel className="type-meta">导出范围</DropdownMenuLabel>
                  <DropdownMenuRadioGroup value={scope} onValueChange={(v) => setScope(v as typeof scope)}>
                    {([["all", "全部来源", rows.length], ["enabled", "仅启用", rows.filter((r) => r.enabled).length], ["view", "当前筛选", view.length], ["sel", "已选中的行", sel.size]] as const).map(([v, l, n]) => (
                      <DropdownMenuRadioItem key={v} value={v} closeOnClick={false} disabled={n === 0} className="type-body">
                        {l}<span className="ml-auto pr-5 font-mono type-meta tabular-nums text-faint">{n}</span>
                      </DropdownMenuRadioItem>
                    ))}
                  </DropdownMenuRadioGroup>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                <DropdownMenuGroup>
                  <DropdownMenuLabel className="type-meta">格式</DropdownMenuLabel>
                  <DropdownMenuItem onClick={() => doExport("json")} className="items-start py-1.5">
                    <FileJson className="mt-0.5" /><span className="grid"><span className="type-body">JSON</span><span className="type-meta text-faint">完整设置，可再导入本应用</span></span>
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => doExport("opml")} className="items-start py-1.5">
                    <Rss className="mt-0.5" /><span className="grid"><span className="type-body">OPML 2.0</span><span className="type-meta text-faint">通用格式，可导入其他阅读器</span></span>
                  </DropdownMenuItem>
                </DropdownMenuGroup>
              </DropdownMenuContent>
            </DropdownMenu>
          </ButtonGroup>
          <Button onClick={() => setAddOpen(true)}><Plus />添加数据源</Button>
        </div>
      </div>
      <div className="mt-5 grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-border shadow-card sm:grid-cols-4">
        {stats.map(([l, v]) => (
          <div key={l} className="bg-card px-4 py-3">
            <div className="type-caption text-faint">{l}</div>
            <div className={cn("mt-0.5 type-h1 font-semibold tabular-nums tracking-tight", l === "报错" && v ? "text-red-500" : "")}>{v}</div>
          </div>
        ))}
      </div>

      <div className="mt-5 flex flex-wrap items-center gap-2">
        <div className="relative w-full sm:w-64">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input className="pl-8" placeholder="筛选名称或 URL…" value={q} onChange={(e) => setQ(e.target.value)} />
        </div>
        <ButtonGroup aria-label="按状态筛选" className="w-full sm:w-fit">
          {STATUS.map((x) => (
            <Button key={x.v} variant="outline" aria-pressed={fs === x.v} onClick={() => setFs(x.v)}
              className="min-w-0 flex-1 gap-1 px-1.5 type-body text-muted-foreground sm:flex-none sm:gap-1.5 sm:px-2.5 aria-pressed:bg-muted aria-pressed:text-foreground dark:aria-pressed:bg-input/60">
              {x.label}<Badge variant={x.badge} className="h-4.5 min-w-5 px-1 font-mono tabular-nums">{fsCount[x.v]}</Badge>
            </Button>
          ))}
        </ButtonGroup>
        <Select value={fsec} onValueChange={(v) => setFsec(v as string)}>
          <SelectTrigger className="w-[120px]"><SelectValue>{(v: string) => (v === "all" ? "全部分区" : secZh(v as SectionKey))}</SelectValue></SelectTrigger>
          <SelectContent><SelectItem value="all">全部分区</SelectItem>{SECTIONS.map((x) => <SelectItem key={x.key} value={x.key}>{x.zh}</SelectItem>)}</SelectContent>
        </Select>
        <Select value={ft} onValueChange={(v) => setFt(v as string)}>
          <SelectTrigger className="w-[120px]"><SelectValue>{(v: string) => (v === "all" ? "全部类型" : TYPE[v])}</SelectValue></SelectTrigger>
          <SelectContent><SelectItem value="all">全部类型</SelectItem>{Object.entries(TYPE).map(([k, v]) => <SelectItem key={k} value={k}>{v}</SelectItem>)}</SelectContent>
        </Select>
        {sel.size > 0 && <Button size="sm" variant="destructive" onClick={() => setToDel(rows.filter((r) => sel.has(r.id)))}><Trash2 />删除所选 · {sel.size}</Button>}
        <span className="ml-auto font-mono type-caption text-muted-foreground">{view.length} / {rows.length}</span>
      </div>

      <div className="mt-3 overflow-hidden rounded-xl bg-card shadow-card">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent [&>th]:h-9 [&>th]:type-caption [&>th]:font-medium [&>th]:text-faint">
              <TableHead className="w-8 pl-4"><Checkbox aria-label="全选当前列表" checked={allSel} onCheckedChange={(v) => setSel(v ? new Set(view.map((x) => x.id)) : new Set())} /></TableHead>
              <TableHead className="w-12">启用</TableHead>
              <TableHead>来源</TableHead>
              <TableHead className="hidden md:table-cell">分区</TableHead>
              <TableHead className="hidden sm:table-cell">频率</TableHead>
              <TableHead className="hidden lg:table-cell">最近抓取</TableHead>
              <TableHead className="hidden text-right md:table-cell">条目</TableHead>
              <TableHead>状态</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 && Array.from({ length: 8 }, (_, i) => (
              <TableRow key={i}><TableCell colSpan={9}><Skeleton className="h-7 w-full" /></TableCell></TableRow>
            ))}
            {view.map((s) => (
              <TableRow key={s.id} className={cn(!s.enabled && "opacity-60")}>
                <TableCell className="pl-4"><Checkbox aria-label={`选择 ${s.name}`} checked={sel.has(s.id)} onCheckedChange={(v) => toggleSel(s.id, !!v)} /></TableCell>
                <TableCell><Switch size="sm" checked={!!s.enabled} onCheckedChange={(v) => patch(s, { enabled: v }, `${v ? "已启用" : "已停用"}「${s.name}」`)} /></TableCell>
                <TableCell className="max-w-[320px]">
                  <div className="flex items-center gap-2.5">
                    <SourceAvatar name={s.name} username={s.username} homepage={s.homepage} url={s.url} x={s.type.startsWith("x")} className="size-7" />
                    <div className="min-w-0">
                      <div className="flex items-center gap-1.5">
                        <span className="truncate font-medium">{s.name}</span>
                        {s.priority && <Badge size="sm" variant={s.priority === "P0" ? "brand" : "outline"} className={cn("font-mono font-medium", s.priority !== "P0" && "text-faint")}>{s.priority}</Badge>}
                        {s.origin === "user" && <Badge size="sm" variant="secondary">手动</Badge>}
                      </div>
                      <div className="truncate type-caption text-faint">{TYPE[s.type] ?? s.type} · {s.url.replace(/^https?:\/\//, "")}</div>
                    </div>
                  </div>
                </TableCell>
                <TableCell className="hidden type-body text-muted-foreground md:table-cell">{secZh(sectionOf({ ...s, is_x: s.type.startsWith("x") }))}</TableCell>
                <TableCell className="hidden sm:table-cell">
                  {s.supported ? (
                    <Select value={String(s.poll_interval)} onValueChange={(v) => patch(s, { poll_interval: Number(v) }, `${s.name} 改为每 ${ivLabel(Number(v))}`)}>
                      <SelectTrigger size="sm" className="h-7 w-[104px] type-caption"><SelectValue>{(v: string) => `每 ${ivLabel(Number(v))}`}</SelectValue></SelectTrigger>
                      <SelectContent>{[...new Set([...IV, s.poll_interval])].sort((a, b) => a - b).map((v) => <SelectItem key={v} value={String(v)}>每 {ivLabel(v)}</SelectItem>)}</SelectContent>
                    </Select>) : <span className="text-muted-foreground">—</span>}
                </TableCell>
                <TableCell className="hidden font-mono type-caption text-muted-foreground lg:table-cell" title={bj(s.last_fetched)}>{ago(s.last_fetched)}</TableCell>
                <TableCell className="hidden text-right font-mono tabular-nums md:table-cell">{s.item_count}</TableCell>
                <TableCell><StatusBadge s={s} /></TableCell>
                <TableCell className="pr-3">
                  <DropdownMenu>
                    <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label="操作" />}>
                      {busy[s.id] ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" className="w-44">
                      <DropdownMenuItem disabled={!s.supported} onClick={() => fetchNow(s)}><RefreshCw />立即抓取</DropdownMenuItem>
                      <DropdownMenuItem onClick={() => window.open(s.homepage || s.url, "_blank")}><ExternalLink />打开主页</DropdownMenuItem>
                      <DropdownMenuItem onClick={() => { navigator.clipboard?.writeText(s.url); toast.success("已复制链接"); }}><Link2 />复制 URL</DropdownMenuItem>
                      <DropdownMenuSeparator /><DropdownMenuItem variant="destructive" onClick={() => setToDel([s])}><Trash2 />删除</DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            ))}
            {rows.length > 0 && view.length === 0 && <TableRow className="hover:bg-transparent"><TableCell colSpan={9} className="p-0">
              <EmptyState icon={SearchX} title="没有匹配的来源" hint="调整搜索词或筛选条件后重试。">
                <Button variant="outline" size="sm" onClick={() => { setQ(""); setFs("all"); setFsec("all"); setFt("all"); }}>清除筛选</Button>
              </EmptyState></TableCell></TableRow>}
          </TableBody>
        </Table>
      </div>
      <AddDialog open={addOpen} onOpenChange={setAddOpen} onAdded={reloadSources} />
      <AlertDialog open={!!toDel} onOpenChange={(o) => !o && setToDel(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{toDel?.length === 1 ? `删除「${toDel[0].name}」？` : `删除 ${toDel?.length ?? 0} 个来源？`}</AlertDialogTitle>
            <AlertDialogDescription>
              将移除来源本身以及它已保存的全部内容（共 {toDel?.reduce((a, x) => a + x.item_count, 0) ?? 0} 条），此操作不能撤销。
              {toDel?.some((x) => x.origin !== "user") ? " 内置源删除后不会因更新代码或重启而回来，可随时在「已删除的内置源」中恢复。" : ""}
              {" "}只想暂停抓取？用左侧开关停用即可。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={confirmDelete}><Trash2 />删除</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      <Dialog open={restoreOpen} onOpenChange={setRestoreOpen}>
        <DialogContent className="grid-cols-[minmax(0,1fr)] sm:max-w-md">
          <DialogHeader><DialogTitle>已删除的内置源</DialogTitle><DialogDescription>恢复后按 sources.json 的默认设置重新加入并开始抓取。</DialogDescription></DialogHeader>
          <ul className="max-h-72 min-w-0 divide-y divide-border/60 overflow-y-auto rounded-lg border">
            {deleted.map((d) => (
              <li key={d.name} className="flex min-w-0 items-center gap-2 px-3 py-2 type-body">
                <span className="min-w-0 flex-1 truncate">{d.name}</span>
                <span className="shrink-0 font-mono type-meta text-faint">{ago(d.ts, true)}</span>
                <Button size="sm" variant="ghost" onClick={() => restore([d.name])}><RotateCcw />恢复</Button>
              </li>
            ))}
            {deleted.length === 0 && <li className="px-3 py-6 text-center type-body text-faint">没有已删除的内置源</li>}
          </ul>
          <DialogFooter><Button variant="outline" disabled={!deleted.length} onClick={() => restore("all")}>全部恢复</Button></DialogFooter>
        </DialogContent>
      </Dialog>
      {impOpen && <ImportDialog open={impOpen} onOpenChange={setImpOpen} onDone={reloadSources} />}
    </div>
  );
}
