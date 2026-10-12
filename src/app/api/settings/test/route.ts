import { NextRequest } from "next/server";
import { cleanBase, cleanKey, settingsLocked, testDeepseek } from "@/lib/secrets";
export const dynamic = "force-dynamic";
/** Test the DeepSeek connection: with the stored key, or (unlocked only) a key/base URL typed in the form. */
export async function POST(req: NextRequest) {
  const b = await req.json().catch(() => ({}));
  const typed = (b.key && String(b.key).trim()) || (b.baseUrl && String(b.baseUrl).trim());
  if (typed && settingsLocked()) return Response.json({ ok: false, error: "设置已锁定" }, { status: 403 });
  const key = b.key ? cleanKey(b.key) : undefined;
  if (b.key && !key) return Response.json({ ok: false, error: "API key 格式不正确" });
  const base = b.baseUrl ? cleanBase(b.baseUrl) : undefined;
  if (b.baseUrl && !base) return Response.json({ ok: false, error: "Base URL 不合法" });
  return Response.json(await testDeepseek(key ?? undefined, base || undefined), { headers: { "Cache-Control": "no-store" } });
}
