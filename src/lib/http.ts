import { createHash } from "node:crypto";
import { brotliCompressSync, constants, gzipSync } from "node:zlib";

/** JSON response with a content ETag (304 on If-None-Match) and brotli (q5, ~14% smaller than gzip) / gzip for bodies > 1 KB.
 *  `no-cache` = the browser may keep it but must revalidate, so polling costs a 304, not a full body. */
export function jsonResponse(req: Request, body: string): Response {
  const etag = `W/"${createHash("sha1").update(body).digest("base64url").slice(0, 16)}"`;
  const headers: Record<string, string> = { "Content-Type": "application/json; charset=utf-8", "Cache-Control": "private, no-cache", Vary: "Accept-Encoding", ETag: etag };
  if (req.headers.get("if-none-match") === etag) return new Response(null, { status: 304, headers });
  const ae = req.headers.get("accept-encoding") ?? "";
  if (body.length > 1024 && /\bbr\b/.test(ae)) {
    return new Response(brotliCompressSync(body, { params: { [constants.BROTLI_PARAM_QUALITY]: 5, [constants.BROTLI_PARAM_SIZE_HINT]: body.length } }), { headers: { ...headers, "Content-Encoding": "br" } });
  }
  if (body.length > 1024 && /\bgzip\b/.test(ae)) {
    return new Response(gzipSync(body, { level: 6 }), { headers: { ...headers, "Content-Encoding": "gzip" } });
  }
  return new Response(body, { headers });
}
