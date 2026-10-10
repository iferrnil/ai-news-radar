// Next.js calls register() once when the server process starts. The poller runs inside the same
// long-running Node process as the web server, so SSE subscribers get new items directly via an
// in-process event bus. Guarded so it only runs in the Node.js runtime (not Edge) and only once.
export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    const { startScheduler } = await import("./lib/scheduler");
    startScheduler();
  }
}
