<div align="center">

# AI News Radar · AI 新闻雷达

**自托管、近实时的 AI 资讯雷达**：精选源按优先级轮询，SSE 实时推送；热点聚合、关键词与收藏；macOS 桌面版另有 AI 简报与一键写稿。

[![CI](https://github.com/iferrnil/ai-news-radar/actions/workflows/ci.yml/badge.svg)](https://github.com/iferrnil/ai-news-radar/actions/workflows/ci.yml)
[![macOS 下载](https://img.shields.io/github/v/release/iferrnil/ai-news-radar?filter=desktop-v%2A&label=macOS%20%E4%B8%8B%E8%BD%BD&logo=apple)](https://github.com/iferrnil/ai-news-radar/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-violet.svg)](LICENSE)

[下载](#下载) · [快速开始](#快速开始) · [功能](#功能) · [文档](#文档) · [English](#english)

![AI News Radar 深色信息流](docs/screenshots/feed_dark.png)

</div>

面向 AI 内容创作者：实验室博客、爆料 X 账号、国内 AI 媒体、Techmeme、GitHub 热门仓库榜等，只保留 48 小时；单个 Go 进程 + SQLite。

## 下载

**macOS 桌面版**：[Releases](https://github.com/iferrnil/ai-news-radar/releases/latest)（`…-macos-arm64.dmg` / `…-macos-x64.dmg`，可用 `SHA256SUMS` 校验）。

首次打开若提示无法验证开发者：右键 App → **打开**，或执行 `xattr -dr com.apple.quarantine "/Applications/AI News Radar.app"`。数据在 `~/Library/Application Support/AI News Radar/`。

自己构建：`npm ci && desktop/scripts/package.sh darwin/arm64`（详见 [desktop/README.md](desktop/README.md)）。

## 快速开始

```bash
git clone https://github.com/iferrnil/ai-news-radar.git && cd ai-news-radar
docker compose up -d          # http://localhost:3080 ，数据在 ./data
```

不用 Docker：

```bash
npm ci && npm run build
(cd server && go build -o bin/radar-server ./cmd/radar-server)
server/bin/radar-server -ui out   # http://localhost:3000
```

同一数据目录只跑**一个**实例。网络受限时可换构建源：`NPM_REGISTRY=… GOPROXY=… APK_MIRROR=… docker compose up -d --build`。

## 功能

**网页版与桌面版**

- **信息流：** 约 24 个内置源（RSS / JSON / X via FxEmbed / Trendshift GitHub 日榜·周榜），按分区浏览；P0 近实时轮询，SSE 实时推送；仅保留约 48 小时
- **分区与筛选：** 实验室 / 国内媒体 / 国外媒体 / X 快讯 / 娱乐等可自定义分区与图标；关键词标签、收藏（不受 48h 清理）、已读 / 只看未读、⌘K、深浅色
- **热点：** 纯规则聚合（无外部 LLM）；Trendshift 日榜与周榜在同一分区内按仓库去重，避免重复卡片
- **P0 通知：** 新到 P0 条目批量桌面 / 浏览器通知；悬停预览可标为已读（设置 › 常规）
- **数据源管理：** URL / @handle / 列表自动发现；添加时可指定**抓取频率**（或按类型 / 优先级默认）；列表「频率」列可改；过滤规则、健康度、JSON / OPML 导入导出
- **设置分类：** 左侧菜单（常规 · 通知与推送 · 模型与抓取 · 写作与 Skill · 数据 · 关于）+ 右侧对应面板
- **可选 DeepSeek：** 翻译外文标题与摘要（按视图范围开关）

**仅桌面版（MyGo softNav 网页壳）**

- AI 简报 / 生成文章（推特串推 · 公众号 · 小红书 Skill）、写作风格、抓取 API（Jina / Firecrawl / crawl4ai）
- 定时自动简报并可推送到 Telegram；原生窗口、菜单栏、签名校验的自动更新（含增量包）

## 文档

| 主题 | 链接 |
|---|---|
| 配置与环境变量 | [AGENTS.md](AGENTS.md)（开发）· 应用内「设置」 |
| 更新说明 | [CHANGELOG.md](CHANGELOG.md) |
| 安全与威胁模型 | [SECURITY.md](SECURITY.md) |
| 源目录 / 限额 | [docs/sources.md](docs/sources.md) · [docs/rate-limits.md](docs/rate-limits.md) |
| 贡献 | [CONTRIBUTING.md](CONTRIBUTING.md) |
| 桌面版打包 | [desktop/README.md](desktop/README.md) |

公网部署请保持 `SETTINGS_LOCKED=1`（Docker 默认），AI / Skill 接口仅桌面版开放。

## English

Self-hosted, near-real-time AI/tech news aggregator: one pure-Go server (SQLite) + static Next.js UI. Curated sections, priority polling (per-source interval), live SSE, rule-based hot topics, keywords, favorites, hover preview, optional DeepSeek translation. Trendshift daily/weekly same-section dedupe. Categorized settings. The macOS MyGo desktop app adds AI briefs, article drafts (X / WeChat / Xiaohongshu), Telegram scheduled briefs, and signed auto-update. Quick start: `docker compose up -d` → http://localhost:3080. Credit Lang.

## License

[MIT](LICENSE) © 2026 Lang
