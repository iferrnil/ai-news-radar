import { getDb, RETENTION_HOURS } from "@/lib/db";
import { schedulerState } from "@/lib/scheduler";
import { fxMirrorState } from "@/lib/fetchers";
import { notifyState } from "@/lib/notify";
export const dynamic = "force-dynamic";

export function GET() {
  const db = getDb();
  const items = db.prepare(`SELECT COUNT(*) total, COALESCE(SUM(is_x),0) x, COALESCE(SUM(CASE WHEN also_on_x!='[]' THEN 1 ELSE 0 END),0) with_x,
    COALESCE(SUM(CASE WHEN merged_into IS NOT NULL THEN 1 ELSE 0 END),0) merged FROM items i JOIN sources s ON s.id=i.source_id WHERE s.enabled=1`).get();
  const sources = db.prepare(`SELECT COUNT(*) total, COALESCE(SUM(enabled AND supported),0) active, COALESCE(SUM(supported=0),0) unsupported,
    COALESCE(SUM(last_error IS NOT NULL AND enabled AND supported),0) errors,
    COALESCE(SUM(last_fetched IS NOT NULL AND enabled AND supported),0) fetched FROM sources`).get();
  return Response.json({ items, sources, retentionHours: RETENTION_HOURS, scheduler: schedulerState(),
    fx: fxMirrorState(), notify: { ...notifyState(), recent: undefined } });
}
