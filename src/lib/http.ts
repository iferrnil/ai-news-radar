import { createHash } from "node:crypto";
import { gzipSync } from "node:zlib";

/** JSON response with a content ETag (304 on If-None-Match) and gzip for bodies > 1 KB.
 *  `no-cache` = the browser may keep it but must revalidate, so polling costs a 304, not a full body. */
export function jsonResponse(req: Request, body: string): Response {
  const etag = `W/"${createHash("sha1").update(body).digest("base64url").slice(0, 16)}"`;
  const headers: Record<string, string> = { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "private, no-cache", Vary: "Accept-Encoding", ETag: etag };
  if (req.headers.get("if-none-match") === etag) return new Response(null, { status: 304, headers });
  if (body.length > 1024 && /\bgzip\b/.test(req.headers.get("accept-encoding") ?? "")) {
    return new Response(gzipSync(body, { level: 6 }), { headers: { ...headers, "Content-Encoding": "gzip" } });
  }
  return new Response(body, { headers });
}
