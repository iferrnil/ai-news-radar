import { getDb } from "@/lib/db";
import { notifyTest } from "@/lib/notify";
import { settingsLocked } from "@/lib/secrets";
export const dynamic = "force-dynamic";

/** 发送测试通知: pushes one `notify` batch (the newest P0 item, else a placeholder) to every open client. Off when locked. */
export function POST() {
  if (settingsLocked()) return Response.json({ detail: "设置已锁定（SETTINGS_LOCKED=1）" }, { status: 403 });
  const r = getDb().prepare(`SELECT i.id, COALESCE(NULLIF(i.title_zh,''), i.title) title, s.name source, s.section, i.url, i.published FROM items i JOIN sources s ON s.id=i.source_id
    WHERE s.priority='P0' AND i.hidden=0 AND i.merged_into IS NULL AND s.enabled=1 ORDER BY i.published DESC LIMIT 1`).get() as
    { id: number; title: string; source: string; section: string | null; url: string; published: number } | undefined;
  const batch = notifyTest([r ?? { id: 0, title: "这是一条测试通知", source: "AI Radar", section: null, url: "", published: Date.now() / 1000 }]);
  return Response.json({ ok: true, count: batch.count });
}
