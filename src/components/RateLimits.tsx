"use client";
// Client component: the static export has no server, so the live usage is computed from /api/sources (served by the Go backend).
import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { AlertTriangle, BookOpen, ChevronRight, Hash, ExternalLink, Gauge, Info, ShieldCheck } from "lucide-react";
import data from "@/data/rate-limits.json";
import { computeUsage, RISK_ZH, type RLEntry, type UsageSource } from "@/lib/rate-usage";
import { softNav } from "@/lib/client-url";


const RISK_V = { low: "success", medium: "warning", high: "destructive" } as const;

type ApiSource = UsageSource & { enabled?: number; supported?: number };

export default function RateLimits() {
  const entries = data.entries as RLEntry[];
  const [rows, setRows] = useState<UsageSource[]>([]);
  useEffect(() => {
    fetch("/api/sources").then((r) => (r.ok ? r.json() : [])).then((all: ApiSource[]) =>
      setRows(all.filter((s) => s.enabled && s.supported).map(({ name, type, url, poll_interval }) => ({ name, type, url, poll_interval })))).catch(() => {});
  }, []);
  const usage = computeUsage(entries, rows);
  const total = Object.values(usage).reduce((a, u) => a + u.perMin, 0);
  const documented = entries.filter((e) => e.official).length;

  return (
    <div className="mx-auto w-full max-w-[1000px] px-4 pb-24 pt-6 sm:px-6">
      <nav className="mb-4 flex items-center gap-1 type-caption text-faint"><button type="button" onClick={() => softNav("/docs")} className="transition-colors duration-150 ease-out hover:text-foreground">文档</button><ChevronRight className="size-3.5" /><span className="text-muted-foreground">抓取限制</span></nav>
      <div className="flex items-start gap-3">
        <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-brand-soft text-brand"><BookOpen className="size-4" /></div>
        <div>
          <h1 className="type-h1 font-semibold leading-8 tracking-[-0.015em]">抓取限制与速率</h1>
          <p className="max-w-[62ch] type-body leading-5 text-muted-foreground">
            每类数据源的官方限额（附一手来源链接）、{data.checked} 的实测响应头，以及按当前启用来源与轮询间隔实时计算的请求量。找不到官方文档的会明确标注，不做推测。
          </p>
        </div>
      </div>

      <div className="mt-6 grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-border shadow-card sm:grid-cols-4">
        {([["当前总请求", `${total.toFixed(1)} 次/分`], ["启用来源", rows.length], ["有官方限额文档", `${documented} / ${entries.length}`], ["核对日期", data.checked]] as const).map(([l, v]) => (
          <div key={l} className="bg-card px-4 py-3">
            <div className="type-caption leading-4 text-faint">{l}</div>
            <div className="mt-1 font-mono type-h3 font-medium leading-6 tabular-nums tracking-[-0.02em]">{v}</div>
          </div>
        ))}
      </div>

      <div className="mt-6 overflow-hidden rounded-xl bg-card shadow-card">
        <Table className="type-body">
          <TableHeader>
            <TableRow className="bg-muted/30 hover:bg-muted/30 [&>th]:h-9 [&>th]:px-4 [&>th]:type-meta [&>th]:font-medium [&>th]:text-faint">
              <TableHead>类型</TableHead><TableHead>文档限额</TableHead><TableHead className="text-right">我们的用量</TableHead><TableHead>风险</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((e) => {
              const u = usage[e.id];
              return (
                <TableRow key={e.id} className="border-border/50 hover:bg-hover">
                  <TableCell className="min-w-[150px] max-w-[260px] whitespace-normal px-4 py-2.5 font-medium leading-5"><a href={`#${e.id}`} className="transition-colors duration-150 ease-out hover:text-brand">{e.name}</a></TableCell>
                  <TableCell className="min-w-[240px] max-w-[360px] whitespace-normal px-4 py-2.5 leading-5 text-muted-foreground">
                    {e.official ? <span className="line-clamp-2">{e.documented}</span> : <span className="text-amber-700 dark:text-amber-300">未找到官方限额文档</span>}
                  </TableCell>
                  <TableCell className="px-4 py-2.5 text-right font-mono tabular-nums">{u.perMin} <span className="text-faint">次/分 · {u.count} 源</span></TableCell>
                  <TableCell className="px-4 py-2.5"><Badge size="sm" variant={RISK_V[e.risk]}>{RISK_ZH[e.risk]}</Badge></TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>

      <div className="mt-8 grid gap-4">
        {entries.map((e) => {
          const u = usage[e.id];
          return (
            <section key={e.id} id={e.id} className="scroll-mt-20 rounded-xl bg-card px-6 py-5 shadow-card">
              <div className="flex flex-wrap items-start gap-2">
                <h2 className="group/h relative type-title font-semibold leading-6 tracking-[-0.01em]"><a href={`#${e.id}`} aria-label="链接到本节" className="absolute -left-5 top-0 hidden h-6 items-center text-faint opacity-0 transition-opacity duration-150 ease-out hover:text-brand group-hover/h:opacity-100 sm:flex"><Hash className="size-3.5" /></a>{e.name}</h2>
                <Badge size="sm" variant={RISK_V[e.risk]}>风险 {RISK_ZH[e.risk]}</Badge>
                {!e.official && <Badge size="sm" variant="warning">无官方限额文档</Badge>}
              </div>
              <code className="mt-1.5 block break-all font-mono type-caption text-faint">{e.endpoint}</code>
              <dl className="mt-4 grid gap-x-8 gap-y-5 type-body leading-[1.7] md:grid-cols-2 [&_dd]:max-w-[60ch]">
                <div>
                  <dt className="mb-1 flex items-center gap-1.5 type-caption font-medium text-faint"><ShieldCheck className="size-3.5" />文档限额</dt>
                  <dd className="text-foreground/90">{e.documented}</dd>
                  <dd className="mt-2 space-y-1">
                    {e.sources.length ? e.sources.map((s) => (
                      <a key={s.url} href={s.url} target="_blank" rel="noopener" className="flex items-center gap-1 type-caption text-brand hover:underline">
                        <ExternalLink className="size-3.5 shrink-0" />{s.label}
                      </a>)) : <span className="type-caption text-amber-700 dark:text-amber-300">未找到官方文档</span>}
                  </dd>
                </div>
                <div>
                  <dt className="mb-1 flex items-center gap-1.5 type-caption font-medium text-faint"><Info className="size-3.5" />实测（{data.checked}）</dt>
                  <dd className="text-muted-foreground">{e.observed}</dd>
                </div>
                <div>
                  <dt className="mb-1 flex items-center gap-1.5 type-caption font-medium text-faint"><Gauge className="size-3.5" />我们的用量（实时）</dt>
                  <dd><span className="font-mono tabular-nums">{u.perMin}</span> 次/分钟 · <span className="font-mono tabular-nums">{u.perHour}</span> 次/小时 · {u.count} 个源</dd>
                  {u.names.length > 0 && <dd className="mt-1 line-clamp-2 type-caption text-faint">{u.names.join("、")}</dd>}
                </div>
                <div>
                  <dt className="mb-1 flex items-center gap-1.5 type-caption font-medium text-faint"><AlertTriangle className="size-3.5" />风险评估</dt>
                  <dd className="text-muted-foreground">{e.riskNote}</dd>
                </div>
              </dl>
            </section>
          );
        })}
      </div>
      <p className="mt-8 type-caption text-faint">Markdown 版本：docs/rate-limits.md（<code className="font-mono">npm run docs:rate-limits</code> 重新生成）。用量不含失败重试与手动“立即抓取”。</p>
    </div>
  );
}
