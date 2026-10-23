/** Live-pill view health: green only when SSE is up AND current-view sources aren't degraded. */
export type ViewOkSource = {
  id: number;
  enabled: number | boolean;
  supported: number | boolean;
  type: string;
  section?: string | null;
  last_error?: string | null;
  h_runs?: string;
  h_consec?: number;
  h_open?: number;
};

export type HealthLevel = "ok" | "degraded" | "down" | "open" | "unknown" | "off";

/** Same rules as source-health.healthOf (kept here so Live pill can unit-test without React). */
export function healthLevelOf(s: ViewOkSource, now = Date.now() / 1000): HealthLevel {
  const runs = s.h_runs ?? "";
  const n = runs.length;
  const ok = [...runs].filter((c) => c === "1").length;
  const rate = n ? ok / n : null;
  const open = !!(s.h_open && s.h_open > now);
  if (!s.enabled || !s.supported) return "off";
  if (open) return "open";
  if (!n) return s.last_error ? "down" : "unknown";
  if ((s.h_consec ?? 0) >= 2 || (runs.endsWith("0") && rate! < 0.5)) return "down";
  if (rate! < 0.9 || runs.endsWith("0")) return "degraded";
  return "ok";
}

export function sourceUnhealthy(s: ViewOkSource, now = Date.now() / 1000): boolean {
  const l = healthLevelOf(s, now);
  return l === "degraded" || l === "down" || l === "open" || !!s.last_error;
}

export type ViewOkFilter = { source?: number | null; section?: string | null };

/**
 * Whether the Live pill may show green for this path + filter + sources.
 * Non-feed paths (settings/sources/docs) are always ok (pill still needs SSE).
 * Empty in-view set → ok (nothing degraded to report).
 */
export function viewSourcesOk(
  path: string,
  sources: ViewOkSource[],
  f: ViewOkFilter,
  sectionOf: (s: ViewOkSource) => string,
  inAll: (section: string) => boolean,
  now = Date.now() / 1000,
): boolean {
  if (path !== "/") return true;
  const active = sources.filter((s) => !!s.enabled && !!s.supported);
  const inView = f.source != null
    ? active.filter((s) => s.id === f.source)
    : f.section
      ? active.filter((s) => sectionOf(s) === f.section)
      : active.filter((s) => inAll(sectionOf(s)));
  if (!inView.length) return true;
  return !inView.some((s) => sourceUnhealthy(s, now));
}
