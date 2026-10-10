import { NextRequest } from "next/server";
import { detect } from "@/lib/fetchers";
import { errMsg } from "@/lib/util";
import { overlap } from "@/lib/overlap";
export const dynamic = "force-dynamic";

export async function POST(req: NextRequest) {
  const b = await req.json();
  try {
    const d = await detect(String(b.input ?? ""));
    return Response.json({ ...d, overlap: overlap(d.sample) });
  } catch (e) {
    return Response.json({ detail: errMsg(e).replace(/^Error: /, "") }, { status: 400 });
  }
}
