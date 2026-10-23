"use client";
import { cn } from "@/lib/utils";
import { healthLevelOf, type HealthLevel } from "@/lib/view-ok";
import { type SourceRow, bj } from "./common";

export type { HealthLevel };
export const KIND_ZH: Record<string, string> = {
  timeout: "超时", network: "网络错误", http: "HTTP 错误", rate_limit: "被限流 (429)", not_found: "不存在 (404)",
  false_404: "FxEmbed 伪 404", parse: "解析失败", other: "其他错误",
};

/** Health summary of one source from the h_* columns of /api/sources. */
export function healthOf(s: SourceRow, now = Date.now() / 1000) {
  const runs = s.h_runs ?? "";
  const n = runs.length, ok = [...runs].filter((c) => c === "1").length;
  const rate = n ? ok / n : null;
  const open = !!(s.h_open && s.h_open > now);
  const level = healthLevelOf(s, now);
  return { level, rate, n, ok, open, runs };
}

const DOT: Record<HealthLevel, string> = {
  ok: "bg-emerald-500/90", degraded: "bg-amber-400", down: "bg-red-500 shadow-[0_0_6px_var(--color-red-500)]",
  open: "bg-red-500 ring-2 ring-red-500/30", unknown: "bg-amber-400/70", off: "bg-faint/50",
};
export const LABEL: Record<HealthLevel, string> = { ok: "健康", degraded: "不稳定", down: "失败", open: "熔断中", unknown: "等待", off: "—" };
export const healthLabel = (l: HealthLevel) => LABEL[l];

export function HealthDot({ level, className }: { level: HealthLevel; className?: string }) {
  return <span className={cn("inline-block size-1.5 shrink-0 rounded-full", DOT[level], className)} />;
}

/** last N outcomes as tiny bars (oldest left) */
export function healthText(s: SourceRow) {
  const h = healthOf(s);
  const parts = [LABEL[h.level]];
  if (h.rate != null) parts.push(`成功率 ${Math.round(h.rate * 100)}%（近 ${h.n} 次）`);
  if (s.h_consec) parts.push(`连续失败 ${s.h_consec} 次`);
  if (h.open) parts.push(`熔断至 ${bj(s.h_open!).slice(6)}`);
  return parts.join(" · ");
}
