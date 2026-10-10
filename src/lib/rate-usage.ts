// Shared by the /docs page and scripts/gen-rate-limits-md.ts (no imports: runnable with node type-stripping).
export type RLEntry = {
  id: string; name: string; match: { type?: string; host?: string; rest?: boolean }; endpoint: string; documented: string; official: boolean;
  sources: { label: string; url: string }[]; observed: string; risk: "low" | "medium" | "high"; riskNote: string;
};
export type UsageSource = { name: string; type: string; url: string; poll_interval: number };
export type Usage = { count: number; perMin: number; perHour: number; names: string[] };

function hostOf(u: string) { try { return new URL(u).host; } catch { return ""; } }

/** Assign each enabled source to one entry (host match first, then type, then the catch-all `rest`). */
export function computeUsage(entries: RLEntry[], sources: UsageSource[]): Record<string, Usage> {
  const out: Record<string, Usage> = Object.fromEntries(entries.map((e) => [e.id, { count: 0, perMin: 0, perHour: 0, names: [] }]));
  for (const s of sources) {
    const h = hostOf(s.url);
    const e = entries.find((x) => x.match.host && x.match.host === h)
      ?? entries.find((x) => x.match.type === s.type && !x.match.rest && !x.match.host)
      ?? entries.find((x) => x.match.rest && x.match.type === s.type);
    if (!e) continue;
    const u = out[e.id];
    u.count++; u.perMin += 60 / s.poll_interval; u.perHour += 3600 / s.poll_interval; u.names.push(s.name);
  }
  for (const u of Object.values(out)) { u.perMin = Math.round(u.perMin * 100) / 100; u.perHour = Math.round(u.perHour * 10) / 10; }
  return out;
}
export const RISK_ZH = { low: "低", medium: "中", high: "高" } as const;
