import { NextRequest } from "next/server";
import { deleteSources } from "@/lib/store";
export const dynamic = "force-dynamic";
// POST { action: "delete", ids: number[] }
export async function POST(req: NextRequest) {
  const b = await req.json();
  if (b.action !== "delete" || !Array.isArray(b.ids)) return Response.json({ detail: "bad request" }, { status: 400 });
  return Response.json(deleteSources(b.ids.map(Number).filter(Number.isFinite)));
}
