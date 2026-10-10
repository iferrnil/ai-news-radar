import { NextRequest } from "next/server";
import { getDb } from "@/lib/db";
import { pollOne } from "@/lib/scheduler";
import type { SourceRow } from "@/lib/types";
export const dynamic = "force-dynamic";

export async function POST(_req: NextRequest, ctx: { params: Promise<{ id: string }> }) {
  const s = getDb().prepare("SELECT * FROM sources WHERE id=?").get(Number((await ctx.params).id)) as SourceRow | undefined;
  if (!s) return Response.json({ detail: "not found" }, { status: 404 });
  if (!s.supported) return Response.json({ detail: "该来源类型暂不支持抓取" }, { status: 400 });
  return Response.json(await pollOne(s));
}
