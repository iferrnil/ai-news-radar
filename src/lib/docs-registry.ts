// Docs registry: drives the sidebar "文档" group, the /docs index and the ⌘K palette.
// To add a page: create src/app/docs/<slug>/page.tsx and append an entry here.
export type DocPage = { slug: string; title: string; desc: string; updated: string };
export const DOCS: DocPage[] = [
  { slug: "rate-limits", title: "抓取限制", desc: "各类数据源的官方限额、实测响应头与当前轮询负载。", updated: "2026-10-10" },
];
export const docHref = (d: DocPage) => `/docs/${d.slug}`;
