// LXGW WenKai ships ~200 unicode-range slices. Off-screen cards (content-visibility: auto) don't request their glyph
// slices until they scroll into view, and every slice that arrives mid-scroll is a "fonts changed" relayout of the whole
// feed (≈1,500 layout objects, 30–50 ms at 4× CPU throttle) — the remaining scroll jank. Warming the slices for the
// cards the user can scroll to, once, at idle, moves those loads (and the single relayout) to before the user scrolls.
// Only what is reachable is warmed (the current view, not every loaded item), and each face only for the text set in
// it (titles → bold, summaries / source names → regular): warming both faces for all 48 h of items pulled ~120 slices
// (6.5 MB) on first load.
const seen = { 400: new Set<string>(), 700: new Set<string>() };

function pick(w: 400 | 700, texts: Iterable<string>) {
  let add = "";
  for (const t of texts) for (const ch of t) if (ch.charCodeAt(0) > 0x2e7f && !seen[w].has(ch)) { seen[w].add(ch); add += ch; }
  return add;
}

export function warmFonts(regular: Iterable<string>, bold: Iterable<string>) {
  if (typeof document === "undefined" || !document.fonts?.load) return;
  const r = pick(400, regular), b = pick(700, bold);
  if (!r && !b) return;
  const run = () => {
    if (r) void document.fonts.load('400 16px "LXGW WenKai"', r).catch(() => {});
    if (b) void document.fonts.load('700 16px "LXGW WenKai"', b).catch(() => {});
  };
  const ric = (window as Window & { requestIdleCallback?: (cb: () => void, o?: { timeout: number }) => number }).requestIdleCallback;
  if (ric) ric(run, { timeout: 1500 }); else setTimeout(run, 200);
}
