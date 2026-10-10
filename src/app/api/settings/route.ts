import { NextRequest } from "next/server";
import { backfillTranslations, setSetting, translateStatus } from "@/lib/translate";
export const dynamic = "force-dynamic";
export function GET() { return Response.json({ translate: translateStatus() }); }
export async function PATCH(req: NextRequest) {
  const b = await req.json();
  if (typeof b.translate === "boolean") { setSetting("translate", b.translate ? "1" : "0"); if (b.translate) backfillTranslations(true); }
  return Response.json({ translate: translateStatus() });
}
