// Regenerate docs/rate-limits.md from src/data/rate-limits.json + sources/sources.json (enabled sources).
// Run: node scripts/gen-rate-limits-md.mts   (Node 22 type stripping)
import fs from "node:fs";
import { computeUsage, RISK_ZH, type RLEntry } from "../src/lib/rate-usage.ts";

const data = JSON.parse(fs.readFileSync("src/data/rate-limits.json", "utf8")) as { checked: string; entries: RLEntry[] };
const srcs = (JSON.parse(fs.readFileSync("sources/sources.json", "utf8")).sources as any[])
  .filter((s) => s.enabled && ["rss", "api", "x", "x_list"].includes(s.type))
  .map((s) => ({ name: s.name, type: s.type, url: s.url, poll_interval: s.pollInterval ?? 600 }));
const usage = computeUsage(data.entries, srcs);
const total = Object.values(usage).reduce((a, u) => a + u.perMin, 0);
const L: string[] = [];
L.push("# 抓取限制与速率说明（Rate limits）", "");
L.push(`> 核对日期：${data.checked}（北京时间）。“文档限额”只写官方/一手来源明确写出的内容；找不到时明确标注“未找到官方文档”。“实测”来自当日对真实端点的请求与响应头。“我们的用量”按 sources/sources.json 中启用的来源与轮询间隔计算（不含失败重试与手动“立即抓取”）。`, "");
L.push(`本文件由 \`node scripts/gen-rate-limits-md.mts\` 生成；网页版见站内 \`/docs\`。`, "");
L.push("## 总览", "");
L.push("| 类型 | 文档限额 | 官方来源 | 我们的用量 | 风险 |", "|---|---|---|---|---|");
for (const e of data.entries) {
  const u = usage[e.id];
  const src = e.sources.length ? e.sources.map((s) => `[${s.label}](${s.url})`).join("<br>") : "未找到官方文档";
  L.push(`| ${e.name} | ${e.documented.replace(/\|/g, "\\|").replace(/</g, "&lt;")} | ${src} | ${u.count} 个源 · ${u.perMin} 次/分钟（${u.perHour} 次/小时） | ${RISK_ZH[e.risk]} |`);
}
L.push("", `合计约 **${Math.round(total * 10) / 10} 次/分钟**（${srcs.length} 个启用来源）。`, "");
L.push("## 分类型详情", "");
for (const e of data.entries) {
  const u = usage[e.id];
  L.push(`### ${e.name}`, "");
  L.push(`- 端点：\`${e.endpoint}\``);
  L.push(`- 文档限额：${e.documented.replace(/</g, "&lt;")}${e.official ? "" : "（**未找到官方限额文档**）"}`);
  L.push(`- 来源：${e.sources.length ? e.sources.map((s) => `[${s.label}](${s.url})`).join("；") : "无（未找到官方文档）"}`);
  L.push(`- 实测（${data.checked}）：${e.observed}`);
  L.push(`- 我们的用量：${u.count} 个源，约 ${u.perMin} 次/分钟（${u.perHour} 次/小时）${u.names.length ? `：${u.names.join("、")}` : ""}`);
  L.push(`- 风险：**${RISK_ZH[e.risk]}** — ${e.riskNote}`, "");
}
L.push("## 礼仪与约定", "");
L.push("- 带真实 UA（`ai-news-crawler/1.0`），超时 15~20 秒；429/5xx 退避重试，FxEmbed 404 视为可能的假阴性最多重试 5 次。");
L.push("- 不要把间隔调到低于源站缓存时间（hnrss 600 秒、OpenRouter 120 秒）——更快只会拿到同样的缓存内容。");
L.push("- X 列表（syndication）额度极小，添加时谨慎；单实例部署，切勿多实例同时轮询同一出口 IP。");
L.push("- 优先使用条件请求（ETag / Last-Modified）是后续可做的优化（当前未实现）。", "");
fs.writeFileSync("docs/rate-limits.md", L.join("\n"));
console.log("docs/rate-limits.md written; total req/min", total.toFixed(2));
