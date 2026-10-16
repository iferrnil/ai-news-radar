// LXGW WenKai ships ~200 unicode-range slices. Off-screen cards (content-visibility: auto) don't request their glyph
// slices until they scroll into view, and every slice that arrives mid-scroll is a "fonts changed" relayout of the whole
// feed (≈1,500 layout objects, 30–50 ms at 4× CPU throttle) — the remaining scroll jank. Warming the slices for all
// loaded items once, at idle, moves those loads (and the single relayout) to before the user scrolls.
const seen = new Set<string>();
const FACES = ['400 16px "LXGW WenKai"', '700 16px "LXGW WenKai"'];

export function warmFonts(texts: Iterable<string>) {
  if (typeof document === "undefined" || !document.fonts?.load) return;
  let add = "";
  for (const t of texts) for (const ch of t) if (ch.charCodeAt(0) > 0x2e7f && !seen.has(ch)) { seen.add(ch); add += ch; }
  if (!add) return;
  const run = () => { for (const f of FACES) void document.fonts.load(f, add).catch(() => {}); };
  const ric = (window as Window & { requestIdleCallback?: (cb: () => void, o?: { timeout: number }) => number }).requestIdleCallback;
  if (ric) ric(run, { timeout: 1500 }); else setTimeout(run, 200);
}
