import { bus, type BusMsg } from "@/lib/bus";
import { startScheduler } from "@/lib/scheduler";
export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export function GET(req: Request) {
  startScheduler(); // no-op if instrumentation already started it
  const enc = new TextEncoder();
  let cleanup = () => {};
  const stream = new ReadableStream({
    start(ctrl) {
      const send = (s: string) => { try { ctrl.enqueue(enc.encode(s)); } catch { cleanup(); } };
      send("retry: 3000\n\n");
      const onMsg = (m: BusMsg) => send(`event: ${m.event}\ndata: ${JSON.stringify(m)}\n\n`);
      const ping = setInterval(() => send(": ping\n\n"), 15000);
      bus.on("msg", onMsg);
      cleanup = () => { clearInterval(ping); bus.off("msg", onMsg); };
      req.signal.addEventListener("abort", () => { cleanup(); try { ctrl.close(); } catch {} });
    },
    cancel() { cleanup(); },
  });
  return new Response(stream, { headers: { "Content-Type": "text/event-stream; charset=utf-8", "Cache-Control": "no-cache, no-transform", Connection: "keep-alive", "X-Accel-Buffering": "no" } });
}
