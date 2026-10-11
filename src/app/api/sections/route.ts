import { NextRequest } from "next/server";
import { getDb } from "@/lib/db";
import { publish } from "@/lib/bus";
import { createSection, listSections } from "@/lib/sections";
export const dynamic = "force-dynamic";

export function GET() { return Response.json(listSections(getDb())); }

export async function POST(req: NextRequest) {
  const r = createSection(getDb(), await req.json().catch(() => ({})));
  if ("error" in r) return Response.json({ detail: r.error }, { status: 400 });
  publish({ event: "sources" });
  return Response.json(r);
}
