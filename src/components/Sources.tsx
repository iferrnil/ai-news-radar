"use client";
import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, TextCursorInput, Download, Filter, FolderCog, Pencil, FileJson, FileUp, Rss, SearchX, ExternalLink, Link2, Loader2, MoreHorizontal, Plus, RefreshCw, RotateCcw, Search, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import dynamic from "next/dynamic";
import { useRouter, useSearchParams } from "next/navigation";
import { buildJson, buildOpml, exportFileName, type ExportSource } from "@/lib/portable-export";

const ImportDialog = dynamic(() => import("./import-dialog"), { ssr: false });
const EditSourceDialog = dynamic(() => import("./source-edit-dialogs").then((m) => m.EditSourceDialog), { ssr: false });
const SectionsDialog = dynamic(() => import("./source-edit-dialogs").then((m) => m.SectionsDialog), { ssr: false });
const AddDialog = dynamic(() => import("./add-source-dialog"), { ssr: false });
const DeleteConfirm = dynamic(() => import("./sources-dialogs").then((m) => m.DeleteConfirm), { ssr: false });
const RestoreDialog = dynamic(() => import("./sources-dialogs").then((m) => m.RestoreDialog), { ssr: false });

/** Source name on /sources: click to rename inline (Enter / blur saves, Esc cancels). Saved as a user override. */
function InlineName({ s, editing, setEditing, onSaved }: { s: SourceRow; editing: boolean; setEditing: (b: boolean) => void; onSaved: () => void }) {
  const [v, setV] = useState(s.name); const [busy, setBusy] = useState(false);
  useEffect(() => { if (editing) setV(s.name); }, [editing, s.name]);
  const save = async () => {
    const name = v.replace(/\s+/g, " ").trim();
    if (name === s.name) { setEditing(false); return; }
    if (!name) { toast.error("名称不能为空"); return; }
    setBusy(true);
    try { await api(`/api/sources/${s.id}`, { method: "PATCH", body: JSON.stringify({ name }) }); toast.success(`已重命名为「${name}」`); setEditing(false); onSaved(); }
    catch (e) { toast.error("重命名失败", { description: (e as Error).message }); }
    setBusy(false);
  };
  if (editing) return (
    <Input autoFocus aria-label={`重命名 ${s.name}`} maxLength={80} disabled={busy} value={v} onChange={(e) => setV(e.target.value)} onFocus={(e) => e.target.select()}
      onKeyDown={(e) => { if (e.key === "Enter") save(); if (e.key === "Escape") { e.stopPropagation(); setEditing(false); } }} onBlur={() => !busy && save()}
      aria-invalid={!v.trim() || undefined} className="h-7 min-w-0 max-w-[280px] px-2 font-medium" />
  );
  return (
    <button type="button" onClick={() => setEditing(true)} title="点击重命名"
      className="min-w-0 truncate rounded-sm text-left font-medium decoration-dashed decoration-faint underline-offset-4 outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50">{s.name}</button>
  );
}

function download(name: string, text: string, mime: string) {
  const url = URL.createObjectURL(new Blob([text], { type: mime }));
  const a = Object.assign(document.createElement("a"), { href: url, download: name }); document.body.append(a); a.click(); a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ButtonGroup } from "@/components/ui/button-group";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { EmptyState } from "./empty-state";
import { useApp } from "./app-shell";
import { type SectionKey, TYPE, type SourceRow, ago, api, bj, allSections, secZh, sourceSection } from "./common";
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


export default function Sources() {
  const { sources: rows, reloadSources, addOpen, setAddOpen, sections } = useApp();
  const [renaming, setRenaming] = useState<number | null>(null);
  const [editSrc, setEditSrc] = useState<SourceRow | null>(null); const [editUsed, setEditUsed] = useState(false);
  useEffect(() => { if (editSrc) setEditUsed(true); }, [editSrc]);
  const [secOpen, setSecOpen] = useState(false); const [secUsed, setSecUsed] = useState(false);
  useEffect(() => { if (secOpen) setSecUsed(true); }, [secOpen]);
  // sidebar "管理分区" links here with ?sections=1
  const sp = useSearchParams(); const router = useRouter();
  useEffect(() => { if (sp.get("sections") === "1") { setSecOpen(true); router.replace("/sources", { scroll: false }); } }, [sp, router]);
  const [q, setQ] = useState(""); const [ft, setFt] = useState("all"); const [fsec, setFsec] = useState("all"); const [fs, setFs] = useState<StatusKey>("active");
  const [busy, setBusy] = useState<Record<number, boolean>>({});
  useEffect(() => { const t = setInterval(() => { if (!document.hidden) reloadSources(); }, 15000); return () => clearInterval(t); }, [reloadSources]);

  const view = useMemo(() => rows.filter((s) =>
    (!q || (s.name + s.url).toLowerCase().includes(q.toLowerCase())) && (ft === "all" || s.type === ft) && (fsec === "all" || sourceSection(s) === fsec) &&
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
  // dialogs are code-split and mounted on first use (keeps Dialog/AlertDialog/Select-heavy code out of first load)
  const [addUsed, setAddUsed] = useState(false); useEffect(() => { if (addOpen) setAddUsed(true); }, [addOpen]);
  const [delUsed, setDelUsed] = useState(false); useEffect(() => { if (toDel) setDelUsed(true); }, [toDel]);
  const [restoreUsed, setRestoreUsed] = useState(false); useEffect(() => { if (restoreOpen) setRestoreUsed(true); }, [restoreOpen]);
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
    const list: ExportSource[] = base.filter((r) => r.type !== "html").map((r) => ({ ...r, section: sourceSection(r) }));
    const custom = allSections().map((c) => ({ key: c.key, name: c.zh, icon: c.icon, inAll: c.includeInAll !== false }));
    if (!list.length) { toast.error("没有可导出的来源"); return; }
    const d = new Date();
    if (fmt === "json") download(exportFileName("json", d), buildJson(list, d, custom), "application/json");
    else download(exportFileName("opml", d), buildOpml(list, d, custom), "text/x-opml");
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
          <Button variant="outline" onClick={() => setSecOpen(true)}><FolderCog />分区</Button>
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
              className="min-w-0 flex-auto gap-0.5 px-1 type-caption text-muted-foreground min-[380px]:gap-1 min-[380px]:px-1.5 min-[380px]:type-body sm:flex-none sm:gap-1.5 sm:px-2.5 aria-pressed:bg-muted aria-pressed:text-foreground dark:aria-pressed:bg-input/60">
              {x.label}<Badge variant={x.badge} className="h-4.5 min-w-4.5 px-0.5 font-mono tabular-nums min-[380px]:min-w-5 min-[380px]:px-1">{fsCount[x.v]}</Badge>
            </Button>
          ))}
        </ButtonGroup>
        <Select value={fsec} onValueChange={(v) => setFsec(v as string)}>
          <SelectTrigger className="w-[120px]"><SelectValue>{(v: string) => (v === "all" ? "全部分区" : secZh(v as SectionKey))}</SelectValue></SelectTrigger>
          <SelectContent><SelectItem value="all">全部分区</SelectItem>{sections.map((x) => <SelectItem key={x.key} value={x.key}>{x.icon ? `${x.icon} ` : ""}{x.zh}</SelectItem>)}</SelectContent>
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
                        <InlineName s={s} editing={renaming === s.id} setEditing={(b) => setRenaming(b ? s.id : null)} onSaved={reloadSources} />
                        {s.priority && <Badge size="sm" variant={s.priority === "P0" ? "brand" : "outline"} className={cn("font-mono font-medium", s.priority !== "P0" && "text-faint")}>{s.priority}</Badge>}
                        {s.origin === "user" && <Badge size="sm" variant="secondary">手动</Badge>}
                        {(s.f_include || s.f_exclude) && (
                          <Tooltip><TooltipTrigger render={<Badge size="sm" variant="muted" className="cursor-default" aria-label="有过滤规则" />}><Filter />过滤{s.hidden_count ? ` ${s.hidden_count}` : ""}</TooltipTrigger>
                            <TooltipContent className="max-w-sm font-mono">{[s.f_include && `包含 /${s.f_include}/`, s.f_exclude && `排除 /${s.f_exclude}/`].filter(Boolean).join(" · ")}{s.f_scope === "all" ? "（标题+摘要）" : "（标题）"}{s.hidden_count ? ` · 已隐藏 ${s.hidden_count} 条` : ""}</TooltipContent></Tooltip>)}
                      </div>
                      <div className="truncate type-caption text-faint">{TYPE[s.type] ?? s.type} · {s.url.replace(/^https?:\/\//, "")}</div>
                    </div>
                  </div>
                </TableCell>
                <TableCell className="hidden type-body text-muted-foreground md:table-cell">{secZh(sourceSection(s))}</TableCell>
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
                      <DropdownMenuItem onClick={() => setTimeout(() => setRenaming(s.id), 120)}><TextCursorInput />重命名</DropdownMenuItem>
                      <DropdownMenuItem onClick={() => setEditSrc(s)}><Pencil />编辑</DropdownMenuItem>
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
      {addUsed && <AddDialog open={addOpen} onOpenChange={setAddOpen} onAdded={reloadSources} />}
      {delUsed && <DeleteConfirm toDel={toDel} onCancel={() => setToDel(null)} onConfirm={confirmDelete} />}
      {restoreUsed && <RestoreDialog open={restoreOpen} onOpenChange={setRestoreOpen} deleted={deleted} restore={restore} />}
      {editUsed && <EditSourceDialog src={editSrc} onClose={() => setEditSrc(null)} onSaved={reloadSources} />}
      {secUsed && <SectionsDialog open={secOpen} onOpenChange={setSecOpen} sources={rows} onChanged={reloadSources} />}
      {impOpen && <ImportDialog open={impOpen} onOpenChange={setImpOpen} onDone={reloadSources} />}
    </div>
  );
}
