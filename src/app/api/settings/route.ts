import { NextRequest } from "next/server";
import { backfillTranslations, resetAuth, setSetting, translateStatus } from "@/lib/translate";
import { cleanBase, cleanKey, cleanModel, deepseekPublic, settingsLocked, updateDeepseek } from "@/lib/secrets";
export const dynamic = "force-dynamic";
const out = () => Response.json({ translate: translateStatus(), deepseek: deepseekPublic() }, { headers: { "Cache-Control": "no-store" } });
export function GET() { return out(); }

/** { translate?: boolean, deepseek?: { key?, clearKey?, baseUrl?, model? } } — the key is write-only */
export async function PATCH(req: NextRequest) {
  const b = await req.json().catch(() => ({}));
  if (b.deepseek && typeof b.deepseek === "object") {
    if (settingsLocked()) return Response.json({ detail: "设置已锁定（SETTINGS_LOCKED=1），请通过环境变量配置" }, { status: 403 });
    const d = b.deepseek; const p: Parameters<typeof updateDeepseek>[0] = {};
    if (d.key !== undefined && d.key !== "") { const k = cleanKey(d.key); if (!k) return Response.json({ detail: "API key 格式不正确" }, { status: 400 }); p.key = k; }
    if (d.clearKey === true) p.clearKey = true;
    if (d.baseUrl !== undefined) { const u = cleanBase(d.baseUrl); if (u == null) return Response.json({ detail: "Base URL 需为 https:// 地址（本机可用 http://localhost）" }, { status: 400 }); p.baseUrl = u; }
    if (d.model !== undefined) { const m = cleanModel(d.model); if (m == null) return Response.json({ detail: "模型名格式不正确" }, { status: 400 }); p.model = m; }
    updateDeepseek(p); resetAuth();
  }
  if (typeof b.translate === "boolean") { setSetting("translate", b.translate ? "1" : "0"); if (b.translate) backfillTranslations(true); }
  return out();
}
