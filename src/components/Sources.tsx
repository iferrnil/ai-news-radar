"use client";
import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, CheckCircle2, ExternalLink, Link2, Loader2, MoreHorizontal, Plus, RefreshCw, Search, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useApp } from "./app-shell";
import { CAT, REG, TYPE, type SourceRow, ago, api, bj, secZh, sectionOf } from "./common";
import { SourceAvatar } from "./source-avatar";

const IV = [60, 120, 180, 300, 600, 1800, 3600];
const ivLabel = (v: number) => (v < 3600 ? `${v / 60} 分钟` : `${v / 3600} 小时`);

function StatusBadge({ s }: { s: SourceRow }) {
  if (!s.supported) return <Badge variant="outline" className="text-muted-foreground">不支持</Badge>;
  if (!s.enabled) return <Badge variant="outline" className="text-muted-foreground">已停用</Badge>;
  if (s.last_error) return (
    <Tooltip><TooltipTrigger render={<Badge variant="outline" className="max-w-[180px] gap-1 border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-400" />}>
      <AlertTriangle className="size-3" /><span className="truncate">报错</span></TooltipTrigger>
      <TooltipContent className="max-w-sm">{s.last_error}</TooltipContent></Tooltip>);
  if (!s.last_fetched) return <Badge variant="outline" className="gap-1 text-amber-600 dark:text-amber-400"><Loader2 className="size-3 animate-spin" />等待</Badge>;
  return <Badge variant="outline" className="gap-1 border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400"><span className="size-1.5 rounded-full bg-emerald-500" />正常{s.last_new ? ` +${s.last_new}` : ""}</Badge>;
}

type Detect = { type: string; name: string; url: string; count: number; overlap: number; sample: { title: string; published: number | null }[] };

function AddDialog({ open, onOpenChange, onAdded }: { open: boolean; onOpenChange: (b: boolean) => void; onAdded: () => void }) {
  const [form, setForm] = useState({ input: "", name: "", category: "community", region: "global", priority: "P1", force: false });
  const [res, setRes] = useState<Detect | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState<"" | "test" | "add">("");
  const test = async () => {
    setBusy("test"); setErr(""); setRes(null);
    try { setRes(await api<Detect>("/api/sources/test", { method: "POST", body: JSON.stringify(form) })); } catch (e) { setErr((e as Error).message); }
    setBusy("");
  };
  const add = async () => {
    setBusy("add"); setErr("");
    try {
      const d = await api("/api/sources", { method: "POST", body: JSON.stringify(form) });
      toast.success(`已添加「${d.source.name}」`, { description: d.fetch.error ? `首次抓取失败：${d.fetch.error}` : `首次抓取 ${d.fetch.fetched} 条，新增 ${d.fetch.new} 条` });
      onAdded(); onOpenChange(false); setRes(null); setForm({ ...form, input: "", name: "", force: false });
    } catch (e) { setErr((e as Error).message); }
    setBusy("");
  };
  const sel = (k: "category" | "region" | "priority", opts: [string, string][]) => (
    <Select value={form[k]} onValueChange={(v) => setForm({ ...form, [k]: v as string })}>
      <SelectTrigger className="w-full"><SelectValue>{(v: string) => opts.find((o) => o[0] === v)?.[1]}</SelectValue></SelectTrigger>
      <SelectContent>{opts.map(([v, l]) => <SelectItem key={v} value={v}>{l}</SelectItem>)}</SelectContent>
    </Select>
  );
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>添加数据源</DialogTitle>
          <DialogDescription>网页或 Feed URL（自动发现 RSS/Atom）、X 用户名 @handle，或 X 列表链接。</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4">
          <div className="grid gap-1.5">
            <Label htmlFor="src-input">地址 / 账号</Label>
            <div className="flex gap-2">
              <Input id="src-input" autoFocus placeholder="https://simonwillison.net 或 @karpathy" value={form.input}
                onChange={(e) => { setForm({ ...form, input: e.target.value }); setRes(null); }} onKeyDown={(e) => e.key === "Enter" && form.input && test()} />
              <Button variant="outline" onClick={test} disabled={!form.input || !!busy}>{busy === "test" ? <Loader2 className="animate-spin" /> : <RefreshCw />}测试</Button>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div className="col-span-2 grid gap-1.5 sm:col-span-1"><Label>名称</Label><Input placeholder="可选" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></div>
            <div className="grid gap-1.5"><Label>分类</Label>{sel("category", Object.entries(CAT))}</div>
            <div className="grid gap-1.5"><Label>地区</Label>{sel("region", [["global", "国际"], ["cn", "国内"]])}</div>
            <div className="grid gap-1.5"><Label>优先级</Label>{sel("priority", [["P0", "P0"], ["P1", "P1"], ["P2", "P2"]])}</div>
          </div>
          {err && <div className="flex gap-2 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-[13px] text-red-600 dark:text-red-400"><AlertTriangle className="mt-0.5 size-4 shrink-0" />{err}</div>}
          {res && (
            <div className="rounded-lg border bg-muted/30">
              <div className="flex items-start gap-2 border-b p-3">
                <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-500" />
                <div className="min-w-0 text-[13px]">
                  <div className="font-medium">{res.name} <Badge variant="secondary" className="ml-1">{TYPE[res.type]}</Badge></div>
                  <div className="truncate text-muted-foreground">{res.url} · {res.count} 条</div>
                </div>
              </div>
              {res.overlap >= 0.5 && (
                <div className="flex items-start gap-2 border-b bg-amber-500/10 p-3 text-[13px] text-amber-700 dark:text-amber-300">
                  <AlertTriangle className="mt-0.5 size-4 shrink-0" />
                  <div className="flex-1">疑似重复：{Math.round(res.overlap * 100)}% 的示例标题已被现有来源收录。
                    <label className="mt-1.5 flex items-center gap-2"><Checkbox checked={form.force} onCheckedChange={(v) => setForm({ ...form, force: !!v })} />仍然添加</label>
                  </div>
                </div>
              )}
              <ul className="max-h-40 space-y-1.5 overflow-auto p-3 text-[12.5px]">
                {res.sample.map((s, i) => <li key={i} className="flex gap-2"><span className="shrink-0 font-mono text-muted-foreground">{bj(s.published)}</span><span className="truncate">{s.title}</span></li>)}
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
  const [q, setQ] = useState(""); const [ft, setFt] = useState("all"); const [fs, setFs] = useState("active");
  const [busy, setBusy] = useState<Record<number, boolean>>({});
  useEffect(() => { const t = setInterval(reloadSources, 10000); return () => clearInterval(t); }, [reloadSources]);

  const view = useMemo(() => rows.filter((s) =>
    (!q || (s.name + s.url).toLowerCase().includes(q.toLowerCase())) && (ft === "all" || s.type === ft) &&
    (fs === "all" || (fs === "active" ? s.enabled && s.supported : fs === "off" ? !s.enabled || !s.supported : s.enabled && s.supported && s.last_error))), [rows, q, ft, fs]);
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
  const del = async (s: SourceRow) => {
    if (!confirm(`删除「${s.name}」及其条目？`)) return;
    await api(`/api/sources/${s.id}`, { method: "DELETE" }); toast.success(`已删除「${s.name}」`); reloadSources();
  };
  const stats = [["在线来源", nAct], ["报错", nErr], ["48h 条目", nItems], ["目录总数", rows.length]] as const;

  return (
    <div className="mx-auto w-full max-w-[1200px] px-3 pb-20 pt-6 sm:px-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-[22px] font-semibold tracking-tight">数据源</h1>
          <p className="mt-0.5 text-[13px] text-muted-foreground">启用/停用、调整抓取频率、立即抓取，或添加新的 RSS / X 账号 / X 列表。</p>
        </div>
        <Button onClick={() => setAddOpen(true)}><Plus />添加数据源</Button>
      </div>
      <div className="mt-5 grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-border shadow-card sm:grid-cols-4">
        {stats.map(([l, v]) => (
          <div key={l} className="bg-card px-4 py-3">
            <div className="text-[12px] text-faint">{l}</div>
            <div className={cn("mt-0.5 text-[22px] font-semibold tabular-nums tracking-tight", l === "报错" && v ? "text-red-500" : "")}>{v}</div>
          </div>
        ))}
      </div>

      <div className="mt-5 flex flex-wrap items-center gap-2">
        <div className="relative w-full sm:w-64">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input className="pl-8" placeholder="筛选名称或 URL…" value={q} onChange={(e) => setQ(e.target.value)} />
        </div>
        <ToggleGroup value={[fs]} onValueChange={(v: string[]) => v[0] && setFs(v[0])} variant="outline" size="sm" spacing={0}>
          <ToggleGroupItem value="active" className="px-3">在线</ToggleGroupItem>
          <ToggleGroupItem value="err" className="px-3">报错</ToggleGroupItem>
          <ToggleGroupItem value="off" className="px-3">停用</ToggleGroupItem>
          <ToggleGroupItem value="all" className="px-3">全部</ToggleGroupItem>
        </ToggleGroup>
        <Select value={ft} onValueChange={(v) => setFt(v as string)}>
          <SelectTrigger size="sm" className="w-[120px]"><SelectValue>{(v: string) => (v === "all" ? "全部类型" : TYPE[v])}</SelectValue></SelectTrigger>
          <SelectContent><SelectItem value="all">全部类型</SelectItem>{Object.entries(TYPE).map(([k, v]) => <SelectItem key={k} value={k}>{v}</SelectItem>)}</SelectContent>
        </Select>
        <span className="ml-auto font-mono text-[12px] text-muted-foreground">{view.length} / {rows.length}</span>
      </div>

      <div className="mt-3 overflow-hidden rounded-xl bg-card shadow-card">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent [&>th]:h-9 [&>th]:text-[12px] [&>th]:font-medium [&>th]:text-faint">
              <TableHead className="w-12 pl-4">启用</TableHead>
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
              <TableRow key={i}><TableCell colSpan={8}><Skeleton className="h-7 w-full" /></TableCell></TableRow>
            ))}
            {view.map((s) => (
              <TableRow key={s.id} className={cn(!s.enabled && "opacity-60")}>
                <TableCell className="pl-4"><Switch size="sm" checked={!!s.enabled} onCheckedChange={(v) => patch(s, { enabled: v }, `${v ? "已启用" : "已停用"}「${s.name}」`)} /></TableCell>
                <TableCell className="max-w-[320px]">
                  <div className="flex items-center gap-2.5">
                    <SourceAvatar name={s.name} username={s.username} homepage={s.homepage} url={s.url} x={s.type.startsWith("x")} className="size-7" />
                    <div className="min-w-0">
                      <div className="flex items-center gap-1.5">
                        <span className="truncate font-medium">{s.name}</span>
                        {s.priority && <span className={cn("rounded-[5px] px-1.5 py-px font-mono text-[10px] font-medium leading-4", s.priority === "P0" ? "bg-brand-soft text-brand" : "text-faint shadow-[inset_0_0_0_1px_var(--border)]")}>{s.priority}</span>}
                        {s.origin === "user" && <Badge variant="secondary" className="h-4.5 px-1.5 text-[10px]">手动</Badge>}
                      </div>
                      <div className="truncate text-[12px] text-faint">{TYPE[s.type] ?? s.type} · {s.url.replace(/^https?:\/\//, "")}</div>
                    </div>
                  </div>
                </TableCell>
                <TableCell className="hidden text-[13px] text-muted-foreground md:table-cell">{secZh(sectionOf({ ...s, is_x: s.type.startsWith("x") }))} · {REG[s.region ?? ""] ?? ""}</TableCell>
                <TableCell className="hidden sm:table-cell">
                  {s.supported ? (
                    <Select value={String(s.poll_interval)} onValueChange={(v) => patch(s, { poll_interval: Number(v) }, `${s.name} 改为每 ${ivLabel(Number(v))}`)}>
                      <SelectTrigger size="sm" className="h-7 w-[104px] text-[12.5px]"><SelectValue>{(v: string) => `每 ${ivLabel(Number(v))}`}</SelectValue></SelectTrigger>
                      <SelectContent>{[...new Set([...IV, s.poll_interval])].sort((a, b) => a - b).map((v) => <SelectItem key={v} value={String(v)}>每 {ivLabel(v)}</SelectItem>)}</SelectContent>
                    </Select>) : <span className="text-muted-foreground">—</span>}
                </TableCell>
                <TableCell className="hidden font-mono text-[12px] text-muted-foreground lg:table-cell" title={bj(s.last_fetched)}>{ago(s.last_fetched)}</TableCell>
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
                      <DropdownMenuItem onClick={() => { navigator.clipboard?.writeText(s.url); toast.success("已复制 URL"); }}><Link2 />复制 URL</DropdownMenuItem>
                      {s.origin === "user" && <><DropdownMenuSeparator /><DropdownMenuItem variant="destructive" onClick={() => del(s)}><Trash2 />删除</DropdownMenuItem></>}
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            ))}
            {rows.length > 0 && view.length === 0 && <TableRow><TableCell colSpan={8} className="py-16 text-center"><div className="text-[13px] font-medium">没有匹配的来源</div><div className="mt-1 text-[12px] text-faint">调整搜索词或筛选条件后重试</div></TableCell></TableRow>}
          </TableBody>
        </Table>
      </div>
      <AddDialog open={addOpen} onOpenChange={setAddOpen} onAdded={reloadSources} />
    </div>
  );
}
