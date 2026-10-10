import { getDb } from "@/lib/db";
export const dynamic = "force-dynamic";
// Liveness/readiness probe for Docker: DB reachable -> 200.
export function GET() {
  try { getDb().prepare("SELECT 1").get(); return Response.json({ ok: true }); }
  catch (e) { return Response.json({ ok: false, error: String(e) }, { status: 503 }); }
}
