"use client";
// /sources 健康 column: badge + rich tooltip. Kept apart from source-health.tsx so the sidebar (every route) doesn't ship it.
import { Activity, AlertTriangle, CircleSlash, Clock, Gauge, Zap } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { type SourceRow, ago, bj } from "./common";
import { HealthDot, KIND_ZH, LABEL, healthOf, healthText } from "./source-health";

function Spark({ runs }: { runs: string }) {
  return (
    <div className="flex h-3 items-end gap-px" aria-hidden>
      {[...runs.padStart(20, " ")].map((c, i) => <span key={i} className={cn("w-1 rounded-[1px]", c === "1" ? "h-3 bg-emerald-500/80" : c === "0" ? "h-3 bg-red-500/80" : "h-1 bg-border")} />)}
    </div>
  );
}

/** Badge + rich tooltip for the /sources table. */
export function HealthBadge({ s }: { s: SourceRow }) {
  const h = healthOf(s);
  if (h.level === "off") return <span className="type-caption text-faint">—</span>;
  const pct = h.rate == null ? "—" : `${Math.round(h.rate * 100)}%`;
  return (
    <Tooltip>
      <TooltipTrigger render={<button type="button" aria-label={`健康：${healthText(s)}`} data-testid="health" data-level={h.level}
        className="inline-flex h-6 items-center gap-1.5 rounded-md px-1.5 font-mono type-caption tabular-nums text-muted-foreground outline-none hover:bg-hover focus-visible:ring-3 focus-visible:ring-ring/50" />}>
        <HealthDot level={h.level} />{h.open ? <CircleSlash className="size-3 text-red-500" /> : null}{pct}
      </TooltipTrigger>
      <TooltipContent className="w-72 max-w-sm p-3">
        <div className="flex items-center justify-between gap-2">
          <span className="flex items-center gap-1.5 font-medium"><HealthDot level={h.level} />{LABEL[h.level]}</span>
          <span className="font-mono tabular-nums opacity-80">{h.ok}/{h.n}</span>
        </div>
        {h.n > 0 && <div className="mt-2"><Spark runs={h.runs} /></div>}
        <dl className="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 opacity-90">
          <dt className="flex items-center gap-1"><Activity className="size-3" />成功率</dt><dd className="font-mono tabular-nums">{pct}（近 {h.n} 次）</dd>
          <dt className="flex items-center gap-1"><Clock className="size-3" />上次成功</dt><dd>{s.h_last_ok ? `${ago(s.h_last_ok)}（${bj(s.h_last_ok).slice(6)}）` : "—"}</dd>
          <dt className="flex items-center gap-1"><Gauge className="size-3" />延迟</dt><dd className="font-mono tabular-nums">{s.h_lat != null ? `${s.h_lat} ms` : "—"}</dd>
          <dt className="flex items-center gap-1"><Zap className="size-3" />连续失败</dt><dd className="font-mono tabular-nums">{s.h_consec ?? 0}</dd>
          {s.h_err_at ? <><dt className="flex items-center gap-1"><AlertTriangle className="size-3" />上次错误</dt>
            <dd>{ago(s.h_err_at)} · {KIND_ZH[s.h_kind ?? "other"] ?? s.h_kind}</dd></> : null}
          {h.open && <><dt className="flex items-center gap-1"><CircleSlash className="size-3" />熔断</dt><dd>暂停抓取至 {bj(s.h_open!).slice(6)}，之后试探一次</dd></>}
        </dl>
        {s.last_error && <p className="mt-2 line-clamp-3 break-words border-t border-current/15 pt-2 opacity-80">{s.last_error}</p>}
      </TooltipContent>
    </Tooltip>
  );
}
