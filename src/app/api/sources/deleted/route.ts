import { NextRequest } from "next/server";
import { getDb, seed } from "@/lib/db";
import { publish } from "@/lib/bus";
export const dynamic = "force-dynamic";
// Deleted built-in sources (tombstones). POST { names: string[] | "all" } restores them from sources.json.
export function GET() {
  return Response.json(getDb().prepare("SELECT name, ts FROM deleted_seeds ORDER BY ts DESC").all());
}
export async function POST(req: NextRequest) {
  const b = await req.json();
  const db = getDb();
  const n = b.names === "all" ? db.prepare("DELETE FROM deleted_seeds").run().changes
    : db.transaction(() => (b.names as string[] ?? []).reduce((a, nm) => a + db.prepare("DELETE FROM deleted_seeds WHERE name=?").run(nm).changes, 0))();
  seed(db); // re-imports the restored names with their seed settings
  publish({ event: "sources" });
  return Response.json({ restored: n });
}
