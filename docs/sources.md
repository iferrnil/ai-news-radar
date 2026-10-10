# AI 新闻爬虫数据源清单（已实测 + 去重）

> 生成时间：2026-10-10 18:26（北京时间）。数据：`sources.yaml`；表格版：`docs/sources.xlsx`。
> 流程：`scripts/verify_sources.py`（实测）→ `scripts/verify_x_fxembed.py`（X 账号经 FxEmbed 实测）→ `scripts/dedupe_sources.py`（去重）→ `scripts/x_vs_blog.py` + `scripts/apply_x_vs_blog.py`（同一组织 X 与官网二选一，见 `docs/x_vs_blog.md`）→ `scripts/render_sources_md.py`（本文档与 xlsx）。均用 `uv run python` 运行。

## 核心 50（当前启用）

按 `scripts/core50.yaml` 选出，轮询间隔：P0 = X 120s / feed 300s，P1 = X 300s / feed 600s，P2 = 1800s。其余来源保留在 sources.yaml，`enabled: false`（`disabled_reason: trimmed to core 50`），可以在网站「数据源管理」重新启用或添加。

| 分组 | 优先级 | 名称 | 类型 | URL | 间隔(s) | 最新 |
|---|---|---|---|---|---|---|
| 官方实验室 | P0 | OpenAI News | rss | <https://openai.com/news/rss.xml> | 300 | 2026-10-09 15:00 |
| 官方实验室 | P0 | Claude Platform release notes | rss | <https://platform.claude.com/docs/en/release-notes/feed.xml> | 300 |  |
| 官方实验室 | P0 | Google Blog - Gemini | rss | <https://blog.google/products/gemini/rss/> | 300 | 2026-10-02 23:00 |
| 官方实验室 | P0 | Google DeepMind Blog | rss | <https://deepmind.google/blog/rss.xml> | 300 | 2026-10-07 03:57 |
| 官方实验室 | P1 | Google Developers Blog | rss | <https://developers.googleblog.com/feeds/posts/default> | 600 |  |
| 官方实验室 | P1 | Google Research Blog | rss | <https://research.google/blog/rss/> | 600 | 2026-10-08 04:19 |
| 官方实验室 | P1 | Hugging Face Blog | rss | <https://huggingface.co/blog/feed.xml> | 600 | 2026-10-09 23:20 |
| 官方实验室 | P1 | Mistral AI News | rss | <https://mistral.ai/news/rss> | 600 | 2026-10-06 20:00 |
| 官方实验室 | P2 | Microsoft Research Blog | rss | <https://www.microsoft.com/en-us/research/feed/> | 1800 | 2026-10-08 00:00 |
| 官方实验室 | P2 | Apple Machine Learning Research | rss | <https://machinelearning.apple.com/rss.xml> | 1800 | 2026-10-08 08:00 |
| 爆料与榜单X | P0 | X @testingcatalog | x | <https://x.com/testingcatalog> | 120 | 2026-10-10 15:16 |
| 爆料与榜单X | P0 | X @btibor91 | x | <https://x.com/btibor91> | 120 | 2026-10-10 03:22 |
| 爆料与榜单X | P0 | X @AiBreakfast | x | <https://x.com/AiBreakfast> | 120 | 2026-10-10 07:36 |
| 爆料与榜单X | P0 | X @ArtificialAnlys | x | <https://x.com/ArtificialAnlys> | 120 | 2026-10-10 06:43 |
| 爆料与榜单X | P0 | X @arena | x | <https://x.com/arena> | 120 | 2026-10-10 05:44 |
| 爆料与榜单X | P1 | X @apples_jimmy | x | <https://x.com/apples_jimmy> | 300 | 2026-10-10 04:27 |
| 爆料与榜单X | P1 | X @kimmonismus | x | <https://x.com/kimmonismus> | 300 | 2026-10-10 16:42 |
| 人物与产品X | P0 | X @OpenAI (OpenAI) | x | <https://x.com/OpenAI> | 120 | 2026-10-08 03:03 |
| 人物与产品X | P0 | X @OpenAIDevs (OpenAI Developers) | x | <https://x.com/OpenAIDevs> | 120 | 2026-10-09 23:00 |
| 人物与产品X | P0 | X @AnthropicAI (Anthropic) | x | <https://x.com/AnthropicAI> | 120 | 2026-10-09 05:04 |
| 人物与产品X | P1 | X @claudeai (Claude) | x | <https://x.com/claudeai> | 300 | 2026-10-09 11:49 |
| 人物与产品X | P1 | X @SpaceXAI (SpaceXAI) | x | <https://x.com/SpaceXAI> | 300 | 2026-10-09 02:13 |
| 人物与产品X | P1 | X @OfficialLoganK (Logan Kilpatrick) | x | <https://x.com/OfficialLoganK> | 300 | 2026-10-08 22:55 |
| 人物与产品X | P1 | X @sama (Sam Altman) | x | <https://x.com/sama> | 300 | 2026-10-08 04:03 |
| 人物与产品X | P1 | X @karpathy (Andrej Karpathy) | x | <https://x.com/karpathy> | 300 | 2026-10-02 14:35 |
| 人物与产品X | P1 | X @cursor_ai (Cursor) | x | <https://x.com/cursor_ai> | 300 | 2026-10-09 04:13 |
| 人物与产品X | P1 | X @bcherny | x | <https://x.com/bcherny> | 300 | 2026-10-08 09:57 |
| 人物与产品X | P1 | X @_akhaliq (AK) | x | <https://x.com/_akhaliq> | 300 | 2026-10-09 07:08 |
| 国内 | P0 | 量子位 | rss | <https://www.qbitai.com/feed> | 300 | 2026-10-09 17:19 |
| 国内 | P0 | 机器之心 | api | <https://www.jiqizhixin.com/api/article_library/articles.json?sort=time&page=1> | 300 | 2026-10-09 17:46 |
| 国内 | P1 | 极客公园 | rss | <https://www.geekpark.net/rss> | 600 | 2026-10-09 12:34 |
| 国内 | P0 | X @dotey | x | <https://x.com/dotey> | 120 | 2026-10-10 14:33 |
| 国内 | P1 | X @op7418 | x | <https://x.com/op7418> | 300 | 2026-10-10 10:46 |
| 国内 | P0 | X @deepseek_ai (DeepSeek) | x | <https://x.com/deepseek_ai> | 120 | 2026-10-02 15:00 |
| 国内 | P0 | X @Alibaba_Qwen (Qwen) | x | <https://x.com/Alibaba_Qwen> | 120 | 2026-10-09 21:24 |
| 国内 | P1 | X @Kimi_Moonshot (Kimi.ai) | x | <https://x.com/Kimi_Moonshot> | 300 | 2026-09-22 20:20 |
| 国内 | P1 | X @Zai_org (Z.ai) | x | <https://x.com/Zai_org> | 300 | 2026-10-06 08:49 |
| 开发与工具 | P0 | OpenRouter models API | api | <https://openrouter.ai/api/v1/models> | 300 | 2026-10-08 20:34 |
| 开发与工具 | P1 | Hugging Face Daily Papers API | api | <https://huggingface.co/api/daily_papers> | 600 | 2026-10-08 04:00 |
| 开发与工具 | P1 | hnrss newest points>=100 | rss | <https://hnrss.org/newest?points=100> | 600 | 2026-10-10 12:40 |
| 开发与工具 | P1 | GitHub Releases anthropics/claude-code | rss | <https://github.com/anthropics/claude-code/releases.atom> | 600 | 2026-10-10 03:28 |
| 开发与工具 | P1 | GitHub Releases openai/codex | rss | <https://github.com/openai/codex/releases.atom> | 600 | 2026-10-10 09:27 |
| 开发与工具 | P2 | GitHub Releases ollama/ollama | rss | <https://github.com/ollama/ollama/releases.atom> | 1800 | 2026-10-09 11:37 |
| 开发与工具 | P1 | Cursor Changelog | rss | <https://cursor.com/changelog/rss.xml> | 600 | 2026-10-06 08:00 |
| 媒体与聚合 | P0 | Techmeme | rss | <https://www.techmeme.com/feed.xml> | 300 | 2026-10-10 16:40 |
| 媒体与聚合 | P1 | The Decoder | rss | <https://the-decoder.com/feed/> | 600 | 2026-10-10 01:54 |
| 媒体与聚合 | P1 | TechCrunch AI | rss | <https://techcrunch.com/category/artificial-intelligence/feed/> | 600 | 2026-10-10 00:56 |
| 媒体与聚合 | P1 | The Verge AI | rss | <https://www.theverge.com/rss/ai-artificial-intelligence/index.xml> | 600 | 2026-10-10 02:06 |
| Newsletter | P1 | Simon Willison's Weblog | rss | <https://simonwillison.net/atom/everything/> | 600 | 2026-10-09 23:02 |
| Newsletter | P2 | Latent Space | rss | <https://www.latent.space/feed> | 1800 | 2026-10-09 07:29 |

## 汇总

**启用 50** 个数据源（✅ 可用 50）；因同组织 X/官网重复而停用 **233** 个（保留在 sources.yaml，`enabled: false`）；合并重复 **11** 个；移除失效 **22** 个。

| 分组 | 合计 | ✅ 可用 | 🛠 需爬取 | ⛔ 被拦截 |
|---|---|---|---|---|
| 国外官方/研究 | 12 | 12 | 0 | 0 |
| 国外媒体与newsletter | 5 | 5 | 0 | 0 |
| 论文与社区 | 6 | 6 | 0 | 0 |
| 国内 | 3 | 3 | 0 | 0 |
| 基础设施 | 0 | 0 | 0 | 0 |
| X账号 · 爆料/速报/榜单 | 7 | 7 | 0 | 0 |
| X账号 · AI公司/实验室 | 10 | 10 | 0 | 0 |
| X账号 · 关键人物 | 7 | 7 | 0 | 0 |
| X账号 · 基础设施/开发者 | 0 | 0 | 0 | 0 |

状态：**可用**=feed/API/X时间线实测返回近期条目；**需爬取**=无feed，需解析HTML（备注里标了需无头浏览器的）；**被拦截**=403/429/WAF/超时（可能只针对本机出口IP）。
X 账号经 FxEmbed 公共 API（`api.fxtwitter.com/2/profile/{username}/statuses`）抓取，无需 token；该服务偶发 404，需重试。公共 X List 可用 `syndication.twitter.com/srv/timeline-list/list-id/{id}` 读取（见文末）。

## 国外官方/研究（12）

| 名称 | 类型 | URL | 状态 | 最新 | 备注 |
|---|---|---|---|---|---|
| Apple Machine Learning Research | rss | <https://machinelearning.apple.com/rss.xml> | ✅ 可用 | 2026-10-08 08:00 |  |
| Claude Platform release notes | rss | <https://platform.claude.com/docs/en/release-notes/feed.xml> | ✅ 可用 |  | Claude API/平台 release notes 官方 RSS（docs 页 <link rel=alternate> 发现） |
| Cursor Changelog | rss | <https://cursor.com/changelog/rss.xml> | ✅ 可用 | 2026-10-06 08:00 |  |
| Google Blog - Gemini | rss | <https://blog.google/products/gemini/rss/> | ✅ 可用 | 2026-10-02 23:00 |  |
| Google DeepMind Blog | rss | <https://deepmind.google/blog/rss.xml> | ✅ 可用 | 2026-10-07 03:57 | www.deepmind.com/blog/rss.xml 也可（同内容） |
| Google Developers Blog | rss | <https://developers.googleblog.com/feeds/posts/default> | ✅ 可用 |  | 条目无发布日期字段，需用 link 去重 |
| Google Research Blog | rss | <https://research.google/blog/rss/> | ✅ 可用 | 2026-10-08 04:19 | 旧 ai.googleblog.com 已失效 |
| Hugging Face Blog | rss | <https://huggingface.co/blog/feed.xml> | ✅ 可用 | 2026-10-09 23:20 |  |
| Microsoft Research Blog | rss | <https://www.microsoft.com/en-us/research/feed/> | ✅ 可用 | 2026-10-08 00:00 |  |
| Mistral AI News | rss | <https://mistral.ai/news/rss> | ✅ 可用 | 2026-10-06 20:00 | 通过页面 <link rel=alternate> 自动发现 |
| OpenAI News | rss | <https://openai.com/news/rss.xml> | ✅ 可用 | 2026-10-09 15:00 | 全量历史（1000+条），openai.com/blog/rss.xml 是同一份 |
| OpenRouter models API | api | <https://openrouter.ai/api/v1/models> | ✅ 可用 | 2026-10-08 20:34 | JSON；按模型 id 去重即可发现新模型上线 |

## 国外媒体与newsletter（5）

| 名称 | 类型 | URL | 状态 | 最新 | 备注 |
|---|---|---|---|---|---|
| Latent Space | rss | <https://www.latent.space/feed> | ✅ 可用 | 2026-10-09 07:29 |  |
| TechCrunch AI | rss | <https://techcrunch.com/category/artificial-intelligence/feed/> | ✅ 可用 | 2026-10-10 00:56 |  |
| Techmeme | rss | <https://www.techmeme.com/feed.xml> | ✅ 可用 | 2026-10-10 16:40 |  |
| The Decoder | rss | <https://the-decoder.com/feed/> | ✅ 可用 | 2026-10-10 01:54 |  |
| The Verge AI | rss | <https://www.theverge.com/rss/ai-artificial-intelligence/index.xml> | ✅ 可用 | 2026-10-10 02:06 |  |

## 论文与社区（6）

| 名称 | 类型 | URL | 状态 | 最新 | 备注 |
|---|---|---|---|---|---|
| GitHub Releases anthropics/claude-code | rss | <https://github.com/anthropics/claude-code/releases.atom> | ✅ 可用 | 2026-10-10 03:28 |  |
| GitHub Releases ollama/ollama | rss | <https://github.com/ollama/ollama/releases.atom> | ✅ 可用 | 2026-10-09 11:37 |  |
| GitHub Releases openai/codex | rss | <https://github.com/openai/codex/releases.atom> | ✅ 可用 | 2026-10-10 09:27 |  |
| Hugging Face Daily Papers API | api | <https://huggingface.co/api/daily_papers> | ✅ 可用 | 2026-10-08 04:00 | 官方JSON（含upvotes、摘要），支持 ?limit= / ?date=；备选：Hugging Face Daily Papers (page) <https://huggingface.co/papers>，Takara TLDR - HF Daily Papers feed <https://papers.takara.ai/api/feed> |
| Simon Willison's Weblog | rss | <https://simonwillison.net/atom/everything/> | ✅ 可用 | 2026-10-09 23:02 |  |
| hnrss newest points>=100 | rss | <https://hnrss.org/newest?points=100> | ✅ 可用 | 2026-10-10 12:40 | 第三方；hnrss.org 第三方；HN ≥100 分 |

## 国内（3）

| 名称 | 类型 | URL | 状态 | 最新 | 备注 |
|---|---|---|---|---|---|
| 机器之心 | api | <https://www.jiqizhixin.com/api/article_library/articles.json?sort=time&page=1> | ✅ 可用 | 2026-10-09 17:46 | 官网前端使用的JSON接口（非公开文档API）；/rss 不是feed |
| 极客公园 | rss | <https://www.geekpark.net/rss> | ✅ 可用 | 2026-10-09 12:34 |  |
| 量子位 | rss | <https://www.qbitai.com/feed> | ✅ 可用 | 2026-10-09 17:19 |  |

## 基础设施（0）

| 名称 | 类型 | URL | 状态 | 最新 | 备注 |
|---|---|---|---|---|---|

## X账号 · 爆料/速报/榜单（7）

| 名称 | 类型 | URL | 状态 | 最新 | 备注 |
|---|---|---|---|---|---|
| X @AiBreakfast | x | <https://x.com/AiBreakfast> | ✅ 可用 | 2026-10-10 07:36 | 241,142 粉丝，未认证；；经 FxEmbed 抓取（第三方） |
| X @ArtificialAnlys | x | <https://x.com/ArtificialAnlys> | ✅ 可用 | 2026-10-10 06:43 | 157,142 粉丝，未认证；；经 FxEmbed 抓取（第三方） |
| X @apples_jimmy | x | <https://x.com/apples_jimmy> | ✅ 可用 | 2026-10-10 04:27 | 60,031 粉丝，未认证；传闻号，准确率参差；经 FxEmbed 抓取（第三方） |
| X @arena | x | <https://x.com/arena> | ✅ 可用 | 2026-10-10 05:44 | 233,332 粉丝，未认证；；经 FxEmbed 抓取（第三方） |
| X @btibor91 | x | <https://x.com/btibor91> | ✅ 可用 | 2026-10-10 03:22 | 40,486 粉丝，未认证；；经 FxEmbed 抓取（第三方） |
| X @kimmonismus | x | <https://x.com/kimmonismus> | ✅ 可用 | 2026-10-10 16:42 | 149,788 粉丝，未认证；；经 FxEmbed 抓取（第三方） |
| X @testingcatalog | x | <https://x.com/testingcatalog> | ✅ 可用 | 2026-10-10 15:16 | 79,545 粉丝，未认证；；经 FxEmbed 抓取（第三方） |

## X账号 · AI公司/实验室（10）

| 名称 | 类型 | URL | 状态 | 最新 | 备注 |
|---|---|---|---|---|---|
| X @Alibaba_Qwen (Qwen) | x | <https://x.com/Alibaba_Qwen> | ✅ 可用 | 2026-10-09 21:24 | 293,814 粉丝，企业认证 |
| X @Kimi_Moonshot (Kimi.ai) | x | <https://x.com/Kimi_Moonshot> | ✅ 可用 | 2026-09-22 20:20 | 365,321 粉丝，未认证；未认证但为官方（简介含 moonshot.ai PR 邮箱） |
| X @Zai_org (Z.ai) | x | <https://x.com/Zai_org> | ✅ 可用 | 2026-10-06 08:49 | 163,424 粉丝，企业认证；智谱 |
| X @deepseek_ai (DeepSeek) | x | <https://x.com/deepseek_ai> | ✅ 可用 | 2026-10-02 15:00 | 1,140,999 粉丝，蓝标 |
| X @AnthropicAI (Anthropic) | x | <https://x.com/AnthropicAI> | ✅ 可用 | 2026-10-09 05:04 | 1,784,112 粉丝，企业认证 |
| X @OpenAI (OpenAI) | x | <https://x.com/OpenAI> | ✅ 可用 | 2026-10-08 03:03 | 5,422,192 粉丝，企业认证 |
| X @OpenAIDevs (OpenAI Developers) | x | <https://x.com/OpenAIDevs> | ✅ 可用 | 2026-10-09 23:00 | 454,219 粉丝，企业认证 |
| X @SpaceXAI (SpaceXAI) | x | <https://x.com/SpaceXAI> | ✅ 可用 | 2026-10-09 02:13 | 2,073,520 粉丝，企业认证；xAI 现用账号（@xai 简介写 Now at @SpaceXAI） |
| X @claudeai (Claude) | x | <https://x.com/claudeai> | ✅ 可用 | 2026-10-09 11:49 | 1,886,251 粉丝，蓝标 |
| X @cursor_ai (Cursor) | x | <https://x.com/cursor_ai> | ✅ 可用 | 2026-10-09 04:13 | 487,140 粉丝，企业认证 |

## X账号 · 关键人物（7）

| 名称 | 类型 | URL | 状态 | 最新 | 备注 |
|---|---|---|---|---|---|
| X @dotey | x | <https://x.com/dotey> | ✅ 可用 | 2026-10-10 14:33 | 256,946 粉丝，未认证；宝玉；经 FxEmbed 抓取（第三方） |
| X @op7418 | x | <https://x.com/op7418> | ✅ 可用 | 2026-10-10 10:46 | 179,667 粉丝，未认证；歸藏；经 FxEmbed 抓取（第三方） |
| X @OfficialLoganK (Logan Kilpatrick) | x | <https://x.com/OfficialLoganK> | ✅ 可用 | 2026-10-08 22:55 | 374,645 粉丝，蓝标 |
| X @_akhaliq (AK) | x | <https://x.com/_akhaliq> | ✅ 可用 | 2026-10-09 07:08 | 531,725 粉丝，蓝标；论文推送 |
| X @bcherny | x | <https://x.com/bcherny> | ✅ 可用 | 2026-10-08 09:57 | 596,945 粉丝，未认证；Claude Code 负责人；经 FxEmbed 抓取（第三方） |
| X @karpathy (Andrej Karpathy) | x | <https://x.com/karpathy> | ✅ 可用 | 2026-10-02 14:35 | 4,329,199 粉丝，蓝标 |
| X @sama (Sam Altman) | x | <https://x.com/sama> | ✅ 可用 | 2026-10-08 04:03 | 6,359,243 粉丝，蓝标；OpenAI CEO |

## X账号 · 基础设施/开发者（0）

| 名称 | 类型 | URL | 状态 | 最新 | 备注 |
|---|---|---|---|---|---|

## 已停用（233）

停用原因：`trimmed to core 50`（精简为核心 50），括号里是此前的停用原因（如 `docs/x_vs_blog.md` 的 X/官网去重）。

| 名称 | 类型 | URL | 原状态 | 停用原因 |
|---|---|---|---|---|
| 36氪 | rss | <https://36kr.com/feed> | ⛔ 被拦截 | trimmed to core 50 |
| AI News (artificialintelligence-news.com) | rss | <https://www.artificialintelligence-news.com/feed/> | ⛔ 被拦截 | trimmed to core 50 |
| AIbase 资讯 | html | <https://www.aibase.com/zh/news> | 🛠 需爬取 | trimmed to core 50 |
| AWS Machine Learning Blog | rss | <https://aws.amazon.com/blogs/machine-learning/feed/> | ✅ 可用 | trimmed to core 50 |
| AWS News Blog | rss | <https://aws.amazon.com/blogs/aws/feed/> | ✅ 可用 | trimmed to core 50 |
| AWS What's New | rss | <https://aws.amazon.com/about-aws/whats-new/recent/feed/> | ✅ 可用 | trimmed to core 50 |
| Ahead of AI (Sebastian Raschka) | rss | <https://magazine.sebastianraschka.com/feed> | ✅ 可用 | trimmed to core 50 |
| Ai2 (Allen Institute) Blog | rss | <https://allenai.org/rss.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @allen_ai（网站条目在X覆盖率 100%，窗口 60.0 天）；X 覆盖网站 100% 条目，另有 16 条网站没有的内容） |
| Amazon Science | rss | <https://www.amazon.science/index.rss> | ✅ 可用 | trimmed to core 50 |
| Andrej Karpathy blog | rss | <https://karpathy.bearblog.dev/feed/> | ✅ 可用 | trimmed to core 50 |
| Anthropic Engineering | html | <https://www.anthropic.com/engineering> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @AnthropicAI, @claudeai（网站条目在X覆盖率 67%，窗口 60.0 天）；X 覆盖网站 67%；去掉网站只丢 12/71（Anthropic News:5；Anthropic Research:6；Anthropic Engineering:1），去掉 X 会丢 35） |
| Anthropic News | html | <https://www.anthropic.com/news> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @AnthropicAI, @claudeai（网站条目在X覆盖率 67%，窗口 60.0 天）；X 覆盖网站 67%；去掉网站只丢 12/71（Anthropic News:5；Anthropic Research:6；Anthropic Engineering:1），去掉 X 会丢 35） |
| Anthropic Research | html | <https://www.anthropic.com/research> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @AnthropicAI, @claudeai（网站条目在X覆盖率 67%，窗口 60.0 天）；X 覆盖网站 67%；去掉网站只丢 12/71（Anthropic News:5；Anthropic Research:6；Anthropic Engineering:1），去掉 X 会丢 35） |
| Ars Technica AI | rss | <https://arstechnica.com/ai/feed/> | ✅ 可用 | trimmed to core 50 |
| Azure Updates | rss | <https://www.microsoft.com/releasecommunications/api/v2/azure/rss> | ✅ 可用 | trimmed to core 50 |
| BAIR Blog | rss | <https://bair.berkeley.edu/blog/feed.xml> | ✅ 可用 | trimmed to core 50 |
| Ben's Bites | rss | <https://www.bensbites.com/feed> | ✅ 可用 | trimmed to core 50 |
| Bun Blog | rss | <https://bun.sh/rss.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @bunjavascript（网站条目在X覆盖率 100%，窗口 60.0 天）；X 覆盖网站 100% 条目，另有 23 条网站没有的内容） |
| Bun Releases (GitHub) | rss | <https://github.com/oven-sh/bun/releases.atom> | ✅ 可用 | trimmed to core 50（此前：covered by X @bunjavascript（网站条目在X覆盖率 100%，窗口 60.0 天）；X 覆盖网站 100% 条目，另有 23 条网站没有的内容） |
| Cloudflare Blog | rss | <https://blog.cloudflare.com/rss/> | ✅ 可用 | trimmed to core 50 |
| Cloudflare Changelog | rss | <https://developers.cloudflare.com/changelog/rss/index.xml> | ✅ 可用 | trimmed to core 50 |
| Cloudflare Status | rss | <https://www.cloudflarestatus.com/history.atom> | ✅ 可用 | trimmed to core 50 |
| Cohere Blog | html | <https://cohere.com/blog> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @cohere（网站条目在X覆盖率 60%，窗口 60.0 天）；X 覆盖网站 60%；去掉网站只丢 8/60（Cohere Blog:8），去掉 X 会丢 40） |
| DeepSeek API Docs 新闻 | html | <https://api-docs.deepseek.com/news/news250120> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @deepseek_ai（网站条目在X覆盖率 50%，窗口 60.0 天）；X 覆盖网站 50%；去掉网站只丢 1/6（DeepSeek API Docs 新闻:1），去掉 X 会丢 4） |
| DeepSeek 新闻 | html | <https://www.deepseek.com/news/> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @deepseek_ai（网站条目在X覆盖率 50%，窗口 60.0 天）；X 覆盖网站 50%；去掉网站只丢 1/6（DeepSeek API Docs 新闻:1），去掉 X 会丢 4） |
| Deno Blog | rss | <https://deno.com/feed> | ✅ 可用 | trimmed to core 50 |
| Deno Releases (GitHub) | rss | <https://github.com/denoland/deno/releases.atom> | ✅ 可用 | trimmed to core 50 |
| DigitalOcean Blog | rss | <https://www.digitalocean.com/rss/blog.atom> | ✅ 可用 | trimmed to core 50 |
| DigitalOcean Release Notes | rss | <https://docs.digitalocean.com/release-notes/index.xml> | ✅ 可用 | trimmed to core 50 |
| DigitalOcean Status | rss | <https://status.digitalocean.com/history.atom> | ✅ 可用 | trimmed to core 50 |
| Docker Blog | rss | <https://www.docker.com/blog/feed/> | ✅ 可用 | trimmed to core 50（此前：covered by X @Docker（网站条目在X覆盖率 100%，窗口 60.0 天）；X 覆盖网站 100% 条目，另有 94 条网站没有的内容） |
| Engineering at Meta | rss | <https://engineering.fb.com/feed/> | ✅ 可用 | trimmed to core 50 |
| Fastly Blog | rss | <https://www.fastly.com/blog_rss.xml> | ✅ 可用 | trimmed to core 50 |
| Fly.io Blog | rss | <https://fly.io/blog/feed.xml> | ✅ 可用 | trimmed to core 50 |
| Fly.io Fresh Produce (changelog) | rss | <https://community.fly.io/c/fresh-produce.rss> | ✅ 可用 | trimmed to core 50 |
| Fly.io Status | rss | <https://status.flyio.net/history.atom> | ✅ 可用 | trimmed to core 50 |
| GitHub Blog | rss | <https://github.blog/feed/> | ✅ 可用 | trimmed to core 50（此前：covered by X @github, @githubstatus（网站条目在X覆盖率 76%，窗口 60.0 天）；X 覆盖网站 76%；去掉网站只丢 11/313（GitHub Blog:4；GitHub Changelog:6；GitHub Status:1），去掉 X 会丢 268） |
| GitHub Changelog | rss | <https://github.blog/changelog/feed/> | ✅ 可用 | trimmed to core 50（此前：covered by X @github, @githubstatus（网站条目在X覆盖率 76%，窗口 60.0 天）；X 覆盖网站 76%；去掉网站只丢 11/313（GitHub Blog:4；GitHub Changelog:6；GitHub Status:1），去掉 X 会丢 268） |
| GitHub Status | rss | <https://www.githubstatus.com/history.atom> | ✅ 可用 | trimmed to core 50（此前：covered by X @github, @githubstatus（网站条目在X覆盖率 76%，窗口 60.0 天）；X 覆盖网站 76%；去掉网站只丢 11/313（GitHub Blog:4；GitHub Changelog:6；GitHub Status:1），去掉 X 会丢 268） |
| GitHub Trending | html | <https://github.com/trending> | 🛠 需爬取 | trimmed to core 50 |
| Google Blog - AI | rss | <https://blog.google/technology/ai/rss/> | ✅ 可用 | trimmed to core 50 |
| Google Cloud Blog (all) | rss | <https://cloudblog.withgoogle.com/rss/> | ✅ 可用 | trimmed to core 50 |
| Google Cloud Blog - AI & ML | rss | <https://cloudblog.withgoogle.com/products/ai-machine-learning/rss/> | ✅ 可用 | trimmed to core 50 |
| Google Cloud Release Notes | rss | <https://cloud.google.com/feeds/gcp-release-notes.xml> | ✅ 可用 | trimmed to core 50 |
| Google Cloud Service Health | rss | <https://status.cloud.google.com/en/feed.atom> | ✅ 可用 | trimmed to core 50 |
| HN Algolia Search API | api | <https://hn.algolia.com/api/v1/search_by_date?query=LLM&tags=story&hitsPerPage=20> | ✅ 可用 | trimmed to core 50 |
| Hacker News Front Page (official RSS) | rss | <https://news.ycombinator.com/rss> | ✅ 可用 | trimmed to core 50 |
| HashiCorp Blog | rss | <https://www.hashicorp.com/blog/feed.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @HashiCorp（网站条目在X覆盖率 79%，窗口 60.0 天）；X 覆盖网站 79%；去掉网站只丢 3/61（HashiCorp Blog:3），去掉 X 会丢 47） |
| IT之家 | rss | <https://www.ithome.com/rss/> | ✅ 可用 | trimmed to core 50 |
| Import AI (Jack Clark) | rss | <https://importai.substack.com/feed> | ✅ 可用 | trimmed to core 50 |
| InfoQ 中国 | rss | <https://www.infoq.cn/feed.xml> | ✅ 可用 | trimmed to core 50 |
| Interconnects (Nathan Lambert) | rss | <https://www.interconnects.ai/feed> | ✅ 可用 | trimmed to core 50 |
| Kimi Blog (Moonshot AI) | html | <https://www.kimi.com/blog/> | 🛠 需爬取 | trimmed to core 50 |
| Kubernetes Blog | rss | <https://kubernetes.io/feed.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @kubernetesio（网站条目在X覆盖率 68%，窗口 60.0 天）；X 覆盖网站 68%；去掉网站只丢 10/62（Kubernetes Releases (GitHub):10），去掉 X 会丢 31） |
| Kubernetes Releases (GitHub) | rss | <https://github.com/kubernetes/kubernetes/releases.atom> | ✅ 可用 | trimmed to core 50（此前：covered by X @kubernetesio（网站条目在X覆盖率 68%，窗口 60.0 天）；X 覆盖网站 68%；去掉网站只丢 10/62（Kubernetes Releases (GitHub):10），去掉 X 会丢 31） |
| Last Week in AI | rss | <https://lastweekin.ai/feed> | ✅ 可用 | trimmed to core 50 |
| Lil'Log (Lilian Weng) | rss | <https://lilianweng.github.io/index.xml> | ✅ 可用 | trimmed to core 50 |
| Linear Changelog | rss | <https://linear.app/rss/changelog.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @linear（网站条目在X覆盖率 100%，窗口 60.0 天）；X 覆盖网站 100% 条目，另有 29 条网站没有的内容） |
| MIT News - AI | rss | <https://news.mit.edu/topic/mitartificial-intelligence2-rss.xml> | ✅ 可用 | trimmed to core 50 |
| MIT Technology Review AI | rss | <https://www.technologyreview.com/topic/artificial-intelligence/feed> | ✅ 可用 | trimmed to core 50 |
| Marcus on AI | rss | <https://garymarcus.substack.com/feed> | ✅ 可用 | trimmed to core 50 |
| MarkTechPost | rss | <https://www.marktechpost.com/feed/> | ✅ 可用 | trimmed to core 50 |
| Meta AI Blog | html | <https://ai.meta.com/blog/> | 🛠 需爬取 | trimmed to core 50 |
| Meta Newsroom | rss | <https://about.fb.com/news/feed/> | ✅ 可用 | trimmed to core 50 |
| Microsoft AI Blog (Cloud Blog) | rss | <https://www.microsoft.com/en-us/ai/blog/feed/> | ✅ 可用 | trimmed to core 50 |
| Microsoft Azure Blog | rss | <https://azure.microsoft.com/en-us/blog/feed/> | ✅ 可用 | trimmed to core 50 |
| Microsoft Source - AI | rss | <https://news.microsoft.com/source/topics/ai/feed/> | ✅ 可用 | trimmed to core 50 |
| MiniMax 新闻 | html | <https://www.minimaxi.com/news> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @MiniMax_AI（网站条目在X覆盖率 100%，窗口 60.0 天）；X 覆盖网站 100% 条目，另有 49 条网站没有的内容） |
| NVIDIA Blog | rss | <https://blogs.nvidia.com/feed/> | ✅ 可用 | trimmed to core 50 |
| NVIDIA Blog - Generative AI | rss | <https://blogs.nvidia.com/blog/category/generative-ai/feed/> | ✅ 可用 | trimmed to core 50 |
| NVIDIA Technical Blog | rss | <https://developer.nvidia.com/blog/feed/> | ✅ 可用 | trimmed to core 50 |
| Neon Blog | rss | <https://neon.com/blog/rss.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @neondatabase（网站条目在X覆盖率 95%，窗口 60.0 天）；X 覆盖网站 95% 条目，另有 17 条网站没有的内容） |
| Neon Changelog | rss | <https://neon.com/docs/changelog/rss.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @neondatabase（网站条目在X覆盖率 95%，窗口 60.0 天）；X 覆盖网站 95% 条目，另有 17 条网站没有的内容） |
| Neon Status | rss | <https://status.neon.tech/history.atom> | ⛔ 被拦截 | trimmed to core 50（此前：covered by X @neondatabase（网站条目在X覆盖率 95%，窗口 60.0 天）；X 覆盖网站 95% 条目，另有 17 条网站没有的内容） |
| Netlify Blog | rss | <https://www.netlify.com/feed.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @Netlify（网站条目在X覆盖率 66%，窗口 58.6 天）；X 覆盖网站 66%；去掉网站只丢 18/143（Netlify Changelog:10；Netlify Status:8），去掉 X 会丢 90） |
| Netlify Changelog | rss | <https://www.netlify.com/changelog/feed.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @Netlify（网站条目在X覆盖率 66%，窗口 58.6 天）；X 覆盖网站 66%；去掉网站只丢 18/143（Netlify Changelog:10；Netlify Status:8），去掉 X 会丢 90） |
| Netlify Status | rss | <https://www.netlifystatus.com/history.atom> | ✅ 可用 | trimmed to core 50（此前：covered by X @Netlify（网站条目在X覆盖率 66%，窗口 58.6 天）；X 覆盖网站 66%；去掉网站只丢 18/143（Netlify Changelog:10；Netlify Status:8），去掉 X 会丢 90） |
| Next.js Blog | rss | <https://nextjs.org/feed.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @nextjs（网站条目在X覆盖率 100%，窗口 60.0 天）；X 覆盖网站 100% 条目，另有 2 条网站没有的内容） |
| Next.js Releases (GitHub) | rss | <https://github.com/vercel/next.js/releases.atom> | ✅ 可用 | trimmed to core 50（此前：covered by X @nextjs（网站条目在X覆盖率 100%，窗口 60.0 天）；X 覆盖网站 100% 条目，另有 2 条网站没有的内容） |
| Node.js Blog | rss | <https://nodejs.org/en/feed/blog.xml> | ✅ 可用 | trimmed to core 50 |
| One Useful Thing (Ethan Mollick) | rss | <https://www.oneusefulthing.org/feed> | ✅ 可用 | trimmed to core 50 |
| Perplexity Hub | html | <https://www.perplexity.ai/hub> | ⛔ 被拦截 | trimmed to core 50（此前：covered by X @perplexity_ai（网站条目在X覆盖率 n/a，窗口 60.0 天）；网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取） |
| PlanetScale Blog | html | <https://planetscale.com/blog> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @PlanetScale（网站条目在X覆盖率 52%，窗口 60.0 天）；X 覆盖网站 52%；去掉网站只丢 21/80（PlanetScale Changelog:17；PlanetScale Status:4），去掉 X 会丢 36） |
| PlanetScale Changelog | html | <https://planetscale.com/changelog> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @PlanetScale（网站条目在X覆盖率 52%，窗口 60.0 天）；X 覆盖网站 52%；去掉网站只丢 21/80（PlanetScale Changelog:17；PlanetScale Status:4），去掉 X 会丢 36） |
| PlanetScale Status | rss | <https://www.planetscalestatus.com/history.atom> | ✅ 可用 | trimmed to core 50（此前：covered by X @PlanetScale（网站条目在X覆盖率 52%，窗口 60.0 天）；X 覆盖网站 52%；去掉网站只丢 21/80（PlanetScale Changelog:17；PlanetScale Status:4），去掉 X 会丢 36） |
| Qwen Blog (qwenlm.github.io) | rss | <https://qwenlm.github.io/blog/index.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @Alibaba_Qwen（网站条目在X覆盖率 n/a，窗口 60.0 天）；窗口内网站无新条目，X 有 64 条原创） |
| Qwen Research (qwen.ai) | html | <https://qwen.ai/research> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @Alibaba_Qwen（网站条目在X覆盖率 n/a，窗口 60.0 天）；窗口内网站无新条目，X 有 64 条原创） |
| RSSHub 公共实例 (rsshub.app) | rss | <https://rsshub.app/anthropic/news> | ⛔ 被拦截 | trimmed to core 50 |
| Railway Blog | rss | <https://blog.railway.com/rss.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @Railway（网站条目在X覆盖率 67%，窗口 60.0 天）；X 覆盖网站 67%；去掉网站只丢 6/44（Railway Blog:6），去掉 X 会丢 26） |
| Railway Changelog | rss | <https://railway.com/changelog/rss.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @Railway（网站条目在X覆盖率 67%，窗口 60.0 天）；X 覆盖网站 67%；去掉网站只丢 6/44（Railway Blog:6），去掉 X 会丢 26） |
| React Blog | rss | <https://react.dev/rss.xml> | ✅ 可用 | trimmed to core 50 |
| React Releases (GitHub) | rss | <https://github.com/facebook/react/releases.atom> | ✅ 可用 | trimmed to core 50 |
| Reddit r/LocalLLaMA | rss | <https://www.reddit.com/r/LocalLLaMA/.rss> | ⛔ 被拦截 | trimmed to core 50 |
| Reddit r/MachineLearning | rss | <https://www.reddit.com/r/MachineLearning/.rss> | ⛔ 被拦截 | trimmed to core 50 |
| Render Blog | rss | <https://render.com/blog/feed.xml> | ✅ 可用 | trimmed to core 50 |
| Render Changelog | rss | <https://render.com/changelog/feed.xml> | ✅ 可用 | trimmed to core 50 |
| Render Status | rss | <https://status.render.com/history.atom> | ✅ 可用 | trimmed to core 50 |
| SemiAnalysis | rss | <https://newsletter.semianalysis.com/feed> | ✅ 可用 | trimmed to core 50 |
| Solidot | rss | <https://www.solidot.org/index.rss> | ✅ 可用 | trimmed to core 50 |
| Stability AI News | html | <https://stability.ai/news> | ⛔ 被拦截 | trimmed to core 50（此前：covered by X @StabilityAI（网站条目在X覆盖率 n/a，窗口 60.0 天）；网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取） |
| Stanford HAI News | html | <https://hai.stanford.edu/news> | 🛠 需爬取 | trimmed to core 50 |
| Stripe API Changelog | html | <https://docs.stripe.com/changelog> | 🛠 需爬取 | trimmed to core 50 |
| Stripe Blog | rss | <https://stripe.com/blog/feed.rss> | ✅ 可用 | trimmed to core 50 |
| Supabase Blog | rss | <https://supabase.com/rss.xml> | ✅ 可用 | trimmed to core 50 |
| Supabase Changelog | html | <https://supabase.com/changelog> | 🛠 需爬取 | trimmed to core 50 |
| Supabase Status | rss | <https://status.supabase.com/history.atom> | ✅ 可用 | trimmed to core 50 |
| The Batch (DeepLearning.AI) | html | <https://www.deeplearning.ai/the-batch/> | 🛠 需爬取 | trimmed to core 50 |
| The Gradient | rss | <https://thegradient.pub/rss/> | ✅ 可用 | trimmed to core 50 |
| Together AI Blog | rss | <https://www.together.ai/blog/rss.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @togethercompute（网站条目在X覆盖率 88%，窗口 60.0 天）；X 覆盖网站 88%；去掉网站只丢 2/83（Together AI Blog:2），去掉 X 会丢 67） |
| Turso Blog | html | <https://turso.tech/blog> | 🛠 需爬取 | trimmed to core 50 |
| Upstash Blog | rss | <https://upstash.com/blog/feed.xml> | ✅ 可用 | trimmed to core 50（此前：covered by X @upstash（网站条目在X覆盖率 67%，窗口 60.0 天）；X 覆盖网站 67%；去掉网站只丢 10/48（Upstash Blog:10），去掉 X 会丢 18） |
| VentureBeat AI | rss | <https://venturebeat.com/category/ai/feed/> | ✅ 可用 | trimmed to core 50 |
| Vercel News (blog + changelog) | rss | <https://vercel.com/atom> | ✅ 可用 | trimmed to core 50 |
| Vercel Status | rss | <https://www.vercel-status.com/history.atom> | ✅ 可用 | trimmed to core 50 |
| WIRED AI | rss | <https://www.wired.com/feed/tag/ai/latest/rss> | ✅ 可用 | trimmed to core 50 |
| arXiv cs.AI | rss | <https://rss.arxiv.org/rss/cs.AI> | ✅ 可用 | trimmed to core 50 |
| arXiv cs.CL | rss | <https://rss.arxiv.org/rss/cs.CL> | ✅ 可用 | trimmed to core 50 |
| arXiv cs.CV | rss | <https://rss.arxiv.org/rss/cs.CV> | ✅ 可用 | trimmed to core 50 |
| arXiv cs.LG | rss | <https://rss.arxiv.org/rss/cs.LG> | ✅ 可用 | trimmed to core 50 |
| cnBeta | rss | <https://www.cnbeta.com.tw/backend.php> | ✅ 可用 | trimmed to core 50 |
| hnrss - HN AI keywords | rss | <https://hnrss.org/newest?q=AI+OR+LLM+OR+GPT&points=50> | ✅ 可用 | trimmed to core 50 |
| xAI News | html | <https://x.ai/news> | 🛠 需爬取 | trimmed to core 50 |
| 字节跳动 Seed Blog | html | <https://seed.bytedance.com/zh/blog> | 🛠 需爬取 | trimmed to core 50 |
| 宝玉的分享 | rss | <https://baoyu.io/feed.xml> | ✅ 可用 | trimmed to core 50 |
| 少数派 | rss | <https://sspai.com/feed> | ✅ 可用 | trimmed to core 50 |
| 开源中国 OSCHINA | rss | <https://www.oschina.net/news/rss> | ⛔ 被拦截 | trimmed to core 50 |
| 新智元 (aiera.com.cn) | html | <https://www.aiera.com.cn/asi-articles.html> | 🛠 需爬取 | trimmed to core 50 |
| 智东西 | html | <https://www.zhidx.com/> | 🛠 需爬取 | trimmed to core 50 |
| 智源社区 | html | <https://hub.baai.ac.cn/> | 🛠 需爬取 | trimmed to core 50 |
| 智谱 Z.ai 新闻 | html | <https://www.zhipuai.cn/news> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @Zai_org（网站条目在X覆盖率 n/a，窗口 60.0 天）；网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取） |
| 爱范儿 | rss | <https://www.ifanr.com/feed> | ✅ 可用 | trimmed to core 50 |
| 百度文心 Blog | html | <https://yiyan.baidu.com/blog/> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @Baidu_Inc（网站条目在X覆盖率 n/a，窗口 60.0 天）；网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取） |
| 站长之家 AI 频道 | html | <https://www.chinaz.com/ai/> | 🛠 需爬取 | trimmed to core 50 |
| 腾讯混元 | html | <https://hunyuan.tencent.com/> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @TencentHunyuan（网站条目在X覆盖率 n/a，窗口 60.0 天）；网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取） |
| 虎嗅 | rss | <https://www.huxiu.com/rss/0.xml> | ⛔ 被拦截 | trimmed to core 50 |
| 钛媒体 | rss | <https://www.tmtpost.com/rss.xml> | ✅ 可用 | trimmed to core 50 |
| 阮一峰的网络日志（科技爱好者周刊） | rss | <https://www.ruanyifeng.com/blog/atom.xml> | ✅ 可用 | trimmed to core 50 |
| 阶跃星辰 StepFun | html | <https://www.stepfun.com/> | 🛠 需爬取 | trimmed to core 50（此前：covered by X @StepFun_ai（网站条目在X覆盖率 n/a，窗口 60.0 天）；网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取） |
| 雷峰网 AI 频道 | rss | <https://www.leiphone.com/feed/categoryRss/name/ai> | ✅ 可用 | trimmed to core 50 |
| 雷峰网 全站 | rss | <https://www.leiphone.com/feed> | ✅ 可用 | trimmed to core 50 |
| X @AIatMeta (AI at Meta) | x | <https://x.com/AIatMeta> | ✅ 可用 | trimmed to core 50 |
| X @AndrewYNg (Andrew Ng) | x | <https://x.com/AndrewYNg> | ✅ 可用 | trimmed to core 50 |
| X @AravSrinivas (Aravind Srinivas) | x | <https://x.com/AravSrinivas> | ✅ 可用 | trimmed to core 50 |
| X @Azure (Microsoft Azure) | x | <https://x.com/Azure> | ✅ 可用 | trimmed to core 50 |
| X @Baidu_Inc (Baidu Inc.) | x | <https://x.com/Baidu_Inc> | ✅ 可用 | trimmed to core 50 |
| X @ClementDelangue (clem 🤗) | x | <https://x.com/ClementDelangue> | ✅ 可用 | trimmed to core 50 |
| X @Cloudflare (Cloudflare) | x | <https://x.com/Cloudflare> | ✅ 可用 | trimmed to core 50 |
| X @CloudflareDev (Cloudflare Developers) | x | <https://x.com/CloudflareDev> | ✅ 可用 | trimmed to core 50 |
| X @DarioAmodei (Dario Amodei) | x | <https://x.com/DarioAmodei> | ✅ 可用 | trimmed to core 50 |
| X @Docker (Docker) | x | <https://x.com/Docker> | ✅ 可用 | trimmed to core 50 |
| X @DrJimFan (Jim Fan) | x | <https://x.com/DrJimFan> | ✅ 可用 | trimmed to core 50 |
| X @GaryMarcus (Gary Marcus) | x | <https://x.com/GaryMarcus> | ✅ 可用 | trimmed to core 50 |
| X @GeminiApp (Google Gemini) | x | <https://x.com/GeminiApp> | ✅ 可用 | trimmed to core 50 |
| X @GoogleAI (Google AI) | x | <https://x.com/GoogleAI> | ✅ 可用 | trimmed to core 50 |
| X @GoogleDeepMind (Google DeepMind) | x | <https://x.com/GoogleDeepMind> | ✅ 可用 | trimmed to core 50 |
| X @HashiCorp (HashiCorp, an IBM Company) | x | <https://x.com/HashiCorp> | ✅ 可用 | trimmed to core 50 |
| X @JeffDean (Jeff Dean) | x | <https://x.com/JeffDean> | ✅ 可用 | trimmed to core 50 |
| X @LangChain (LangChain) | x | <https://x.com/LangChain> | ✅ 可用 | trimmed to core 50 |
| X @MicrosoftAI (Microsoft AI) | x | <https://x.com/MicrosoftAI> | ✅ 可用 | trimmed to core 50 |
| X @MiniMax_AI (MiniMax (official)) | x | <https://x.com/MiniMax_AI> | ✅ 可用 | trimmed to core 50 |
| X @MistralAI (Mistral AI) | x | <https://x.com/MistralAI> | ✅ 可用 | trimmed to core 50（此前：website more complete: Mistral AI News；网站有 4 条 X 未发（Mistral AI News:4，全文/changelog）；X 额外内容仅 3/12） |
| X @NVIDIAAI (NVIDIA AI) | x | <https://x.com/NVIDIAAI> | ✅ 可用 | trimmed to core 50 |
| X @Netlify (Netlify) | x | <https://x.com/Netlify> | ✅ 可用 | trimmed to core 50 |
| X @PlanetScale (PlanetScale) | x | <https://x.com/PlanetScale> | ✅ 可用 | trimmed to core 50 |
| X @Railway (Railway) | x | <https://x.com/Railway> | ✅ 可用 | trimmed to core 50 |
| X @StabilityAI (Stability AI) | x | <https://x.com/StabilityAI> | ✅ 可用 | trimmed to core 50 |
| X @StepFun_ai (StepFun) | x | <https://x.com/StepFun_ai> | ✅ 可用 | trimmed to core 50 |
| X @StripeDev (Stripe Developers) | x | <https://x.com/StripeDev> | ✅ 可用 | trimmed to core 50 |
| X @TencentHunyuan (Tencent Hy) | x | <https://x.com/TencentHunyuan> | ✅ 可用 | trimmed to core 50 |
| X @alexandr_wang (Alexandr Wang) | x | <https://x.com/alexandr_wang> | ✅ 可用 | trimmed to core 50 |
| X @allen_ai (Ai2) | x | <https://x.com/allen_ai> | ✅ 可用 | trimmed to core 50 |
| X @arthurmensch (Arthur Mensch) | x | <https://x.com/arthurmensch> | ✅ 可用 | trimmed to core 50 |
| X @astrodotbuild (Astro) | x | <https://x.com/astrodotbuild> | ✅ 可用 | trimmed to core 50 |
| X @awscloud (Amazon Web Services) | x | <https://x.com/awscloud> | ✅ 可用 | trimmed to core 50 |
| X @awsdevelopers (AWS Developers) | x | <https://x.com/awsdevelopers> | ✅ 可用 | trimmed to core 50 |
| X @bunjavascript (Bun) | x | <https://x.com/bunjavascript> | ✅ 可用 | trimmed to core 50 |
| X @code (Visual Studio Code) | x | <https://x.com/code> | ✅ 可用 | trimmed to core 50 |
| X @cohere (Cohere) | x | <https://x.com/cohere> | ✅ 可用 | trimmed to core 50 |
| X @demishassabis (Demis Hassabis) | x | <https://x.com/demishassabis> | ✅ 可用 | trimmed to core 50 |
| X @deno_land (Deno) | x | <https://x.com/deno_land> | ✅ 可用 | trimmed to core 50（此前：website more complete: Deno Blog, Deno Releases (GitHub)；网站有 1 条 X 未发（Deno Blog:1，全文/changelog）；X 额外内容仅 0/3） |
| X @digitalocean (DigitalOcean) | x | <https://x.com/digitalocean> | ✅ 可用 | trimmed to core 50 |
| X @elonmusk (Elon Musk) | x | <https://x.com/elonmusk> | ✅ 可用 | trimmed to core 50 |
| X @emollick (Ethan Mollick) | x | <https://x.com/emollick> | ✅ 可用 | trimmed to core 50 |
| X @fastly (Fastly) | x | <https://x.com/fastly> | ✅ 可用 | trimmed to core 50 |
| X @fchollet (François Chollet) | x | <https://x.com/fchollet> | ✅ 可用 | trimmed to core 50 |
| X @flydotio (Fly.io) | x | <https://x.com/flydotio> | ✅ 可用 | trimmed to core 50 |
| X @gdb (Greg Brockman) | x | <https://x.com/gdb> | ✅ 可用 | trimmed to core 50 |
| X @github (GitHub) | x | <https://x.com/github> | ✅ 可用 | trimmed to core 50 |
| X @githubstatus (GitHub Status) | x | <https://x.com/githubstatus> | ✅ 可用 | trimmed to core 50 |
| X @goodside (Riley Goodside) | x | <https://x.com/goodside> | ✅ 可用 | trimmed to core 50 |
| X @googlecloud (Google Cloud) | x | <https://x.com/googlecloud> | ✅ 可用 | trimmed to core 50 |
| X @grok (Grok) | x | <https://x.com/grok> | ✅ 可用 | trimmed to core 50 |
| X @hardmaru (hardmaru) | x | <https://x.com/hardmaru> | ✅ 可用 | trimmed to core 50 |
| X @huggingface (Hugging Face) | x | <https://x.com/huggingface> | ✅ 可用 | trimmed to core 50（此前：website more complete: Hugging Face Blog；网站有 15 条 X 未发（Hugging Face Blog:15，全文/changelog）；X 额外内容仅 5/29） |
| X @ilyasut (Ilya Sutskever) | x | <https://x.com/ilyasut> | ✅ 可用 | trimmed to core 50 |
| X @jackclarkSF (Jack Clark) | x | <https://x.com/jackclarkSF> | ✅ 可用 | trimmed to core 50 |
| X @jarredsumner (Jarred Sumner) | x | <https://x.com/jarredsumner> | ✅ 可用 | trimmed to core 50 |
| X @kubernetesio (Kubernetes) | x | <https://x.com/kubernetesio> | ✅ 可用 | trimmed to core 50 |
| X @leerob (Lee Robinson) | x | <https://x.com/leerob> | ✅ 可用 | trimmed to core 50 |
| X @lilianweng (Lilian Weng) | x | <https://x.com/lilianweng> | ✅ 可用 | trimmed to core 50 |
| X @linear (Linear) | x | <https://x.com/linear> | ✅ 可用 | trimmed to core 50 |
| X @mustafasuleyman (Mustafa Suleyman) | x | <https://x.com/mustafasuleyman> | ✅ 可用 | trimmed to core 50 |
| X @natolambert (Nathan Lambert) | x | <https://x.com/natolambert> | ✅ 可用 | trimmed to core 50 |
| X @neondatabase (Neon) | x | <https://x.com/neondatabase> | ✅ 可用 | trimmed to core 50 |
| X @nextjs (Next.js) | x | <https://x.com/nextjs> | ✅ 可用 | trimmed to core 50 |
| X @nodejs (Node.js) | x | <https://x.com/nodejs> | ✅ 可用 | trimmed to core 50（此前：website more complete: Node.js Blog；网站有 8 条 X 未发（Node.js Blog:8，全文/changelog）；X 额外内容仅 2/13） |
| X @nvidia (NVIDIA) | x | <https://x.com/nvidia> | ✅ 可用 | trimmed to core 50 |
| X @ollama (ollama) | x | <https://x.com/ollama> | ✅ 可用 | trimmed to core 50 |
| X @perplexity_ai (Perplexity) | x | <https://x.com/perplexity_ai> | ✅ 可用 | trimmed to core 50 |
| X @polynoamial (Noam Brown) | x | <https://x.com/polynoamial> | ✅ 可用 | trimmed to core 50 |
| X @posthog (PostHog) | x | <https://x.com/posthog> | ✅ 可用 | trimmed to core 50 |
| X @prisma (Prisma) | x | <https://x.com/prisma> | ✅ 可用 | trimmed to core 50 |
| X @rasbt (Sebastian Raschka) | x | <https://x.com/rasbt> | ✅ 可用 | trimmed to core 50 |
| X @rauchg (Guillermo Rauch) | x | <https://x.com/rauchg> | ✅ 可用 | trimmed to core 50 |
| X @reactjs (React) | x | <https://x.com/reactjs> | ✅ 可用 | trimmed to core 50（此前：website more complete: React Blog, React Releases (GitHub)；网站有 2 条 X 未发（React Releases (GitHub):2，全文/changelog）；X 额外内容仅 0/3） |
| X @render (Render) | x | <https://x.com/render> | ✅ 可用 | trimmed to core 50 |
| X @rough__sea (Ryan Dahl) | x | <https://x.com/rough__sea> | ✅ 可用 | trimmed to core 50 |
| X @sentry (Sentry) | x | <https://x.com/sentry> | ✅ 可用 | trimmed to core 50 |
| X @simonw (Simon Willison) | x | <https://x.com/simonw> | ✅ 可用 | trimmed to core 50 |
| X @soumithchintala (Soumith Chintala) | x | <https://x.com/soumithchintala> | ✅ 可用 | trimmed to core 50 |
| X @stripe (Stripe) | x | <https://x.com/stripe> | ✅ 可用 | trimmed to core 50 |
| X @supabase (Supabase) | x | <https://x.com/supabase> | ✅ 可用 | trimmed to core 50 |
| X @swyx (swyx) | x | <https://x.com/swyx> | ✅ 可用 | trimmed to core 50 |
| X @thdxr (dax) | x | <https://x.com/thdxr> | ✅ 可用 | trimmed to core 50 |
| X @togethercompute (Together AI) | x | <https://x.com/togethercompute> | ✅ 可用 | trimmed to core 50 |
| X @tursodatabase (Turso) | x | <https://x.com/tursodatabase> | ✅ 可用 | trimmed to core 50（此前：website more complete: Turso Blog；网站有 12 条 X 未发（Turso Blog:12，全文/changelog）；X 额外内容仅 4/24） |
| X @typescript (TypeScript) | x | <https://x.com/typescript> | ✅ 可用 | trimmed to core 50 |
| X @upstash (Upstash) | x | <https://x.com/upstash> | ✅ 可用 | trimmed to core 50 |
| X @vercel (Vercel) | x | <https://x.com/vercel> | ✅ 可用 | trimmed to core 50（此前：website more complete: Vercel News (blog + changelog), Vercel Status；网站有 50 条 X 未发（Vercel News (blog + changelog):30；Vercel Status:20，全文/changelog）；X 额外内容仅 20/232） |
| X @vercel_dev (Vercel Developers) | x | <https://x.com/vercel_dev> | ✅ 可用 | trimmed to core 50（此前：website more complete: Vercel News (blog + changelog), Vercel Status；网站有 50 条 X 未发（Vercel News (blog + changelog):30；Vercel Status:20，全文/changelog）；X 额外内容仅 20/232） |
| X @vite_js (Vite ⚡) | x | <https://x.com/vite_js> | ✅ 可用 | trimmed to core 50 |
| X @vuejs (Vue) | x | <https://x.com/vuejs> | ✅ 可用 | trimmed to core 50 |
| X @ylecun (Yann LeCun) | x | <https://x.com/ylecun> | ✅ 可用 | trimmed to core 50 |

## 合并的重复项（11）

| 保留 | 被合并 | 被合并URL | 原因 |
|---|---|---|---|
| Node.js Blog | Node.js Releases | <https://nodejs.org/en/feed/releases.xml> | 内容重叠：当前 814 条中 814 条（100%）也出现在《Node.js Blog》（1059 条），保留更宽的feed |
| Anthropic News | Anthropic News (Olshansk feed) | <https://raw.githubusercontent.com/Olshansk/rss-feeds/main/feeds/feed_anthropic_news.xml> | 同一内容：第三方feed是官网页面的镜像（最新条目一致），保留官方页面，feed作为 alternate |
| Anthropic Research | Anthropic Research (Olshansk feed) | <https://raw.githubusercontent.com/Olshansk/rss-feeds/main/feeds/feed_anthropic_research.xml> | 同一内容：第三方镜像feed，作为 alternate |
| xAI News | xAI News (Olshansk feed) | <https://raw.githubusercontent.com/Olshansk/rss-feeds/main/feeds/feed_xainews.xml> | 同一内容：第三方镜像feed，作为 alternate |
| Meta AI Blog | Meta AI Blog (Olshansk feed) | <https://raw.githubusercontent.com/Olshansk/rss-feeds/main/feeds/feed_meta_ai.xml> | 同一内容的第三方镜像（且更新滞后），作为 alternate |
| Anthropic Engineering | Anthropic Engineering (Olshansk feed) | <https://raw.githubusercontent.com/Olshansk/rss-feeds/main/feeds/feed_anthropic_engineering.xml> | 同一内容的第三方镜像（且更新滞后），作为 alternate |
| Hugging Face Daily Papers API | Hugging Face Daily Papers (page) | <https://huggingface.co/papers> | 同一数据：页面即API内容，保留API |
| Hugging Face Daily Papers API | Takara TLDR - HF Daily Papers feed | <https://papers.takara.ai/api/feed> | 同一数据：第三方把 HF Daily Papers 转成RSS，作为 alternate |
| arXiv cs.AI | arXiv API (cs.AI latest) | <https://export.arxiv.org/api/query?search_query=cat:cs.AI&sortBy=submittedDate&sortOrder=descending&max_results=20> | 同一数据（cs.AI 新论文），RSS 为每日批量，API 可按需查询，作为 alternate |
| InfoQ 中国 | InfoQ AI&大模型 专题 | <https://www.infoq.cn/topic/AI&LLM> | 专题是全站内容的子集，且专题页需JS渲染；保留全站feed（按AI关键词过滤） |
| Hacker News Front Page (official RSS) | HN Firebase API (top stories) | <https://hacker-news.firebaseio.com/v0/topstories.json> | 同为 HN 首页排名（RSS=前30，API=前500的ID），保留RSS，API作为 alternate |

去重规则：① 规范化URL（协议/www/末尾斜杠/查询参数/重定向后的最终地址）相同即合并；② 抓取所有可用feed比较条目链接，小feed ≥80% 条目出现在另一feed中则并入更宽的feed；③ 同一内容的不同访问方式（官方页面 vs 第三方镜像feed、API vs 页面）合并为一条并保留 `alternates`；④ 部分重叠但内容不同的（如 arXiv 各分类交叉、NVIDIA 生成式AI分类）保留并在备注说明；⑤ 组织的网站与 X 账号不算重复。

## 移除的失效项（22）

| 名称 | 类型 | URL | 原因 |
|---|---|---|---|
| Google AI Blog (legacy) | rss | <https://ai.googleblog.com/feeds/posts/default> | HTTP 404；旧地址，已迁移至 research.google/blog |
| Microsoft AI Blog (legacy blogs.microsoft.com/ai) | rss | <https://blogs.microsoft.com/ai/feed/> | HTTP 410；HTTP 410，站点已归档 |
| LangChain Blog | rss | <https://blog.langchain.com/rss/> | URL returns HTML, not a feed；/rss/ 返回HTML而非feed |
| Papers with Code | html | <https://paperswithcode.com/> | redirected → https://huggingface.co/papers/trending (Trending Papers - Hugging Face)；已关停：重定向到 Hugging Face Trending Papers |
| 百度研究院 Blog | html | <https://research.baidu.com/Blog> | site under maintenance: '百度研究院 - 网站维护中'；页面显示“网站维护中” |
| 品玩 PingWest | rss | <https://www.pingwest.com/feed> | feed URL returns a 404 HTML page；返回404页面 |
| Cloudflare Changelog (page-declared alternate) | rss | <https://developers.cloudflare.com/changelog/index.xml> | HTTP 404；页面 <link rel=alternate> 声明的地址但实际 404；请用 /changelog/rss/index.xml |
| Azure Updates (legacy feed) | rss | <https://azure.microsoft.com/en-us/updates/feed/> | URL returns HTML, not a feed；旧地址现返回HTML页面 |
| X @xai (xAI) | x | <https://api.fxtwitter.com/2/profile/xai/statuses> | not the official/active account；空壳：0 帖 0 粉，简介 'Now at @SpaceXAI'，请改用 @SpaceXAI |
| X @bytedanceSeed (BytedanceSeed) | x | <https://api.fxtwitter.com/2/profile/bytedanceSeed/statuses> | not the official/active account；占位账号：0 帖、11 粉，未找到字节 Seed 官方账号 |
| X @GroqInc (GroqInc) | x | <https://api.fxtwitter.com/2/profile/GroqInc/statuses> | not the official/active account；占位账号（2026-08 新建、0 帖）；@groq 也是无关个人账号 |
| X @groq (Gilles Roquefeuil) | x | <https://api.fxtwitter.com/2/profile/groq/statuses> | not the official/active account；无关个人账号，非 Groq 公司 |
| X @ChatGPTapp | x | <https://api.fxtwitter.com/2/profile/ChatGPTapp/statuses> | user not found (X API and FxEmbed)；X API: user not found |
| X @LangChainAI | x | <https://api.fxtwitter.com/2/profile/LangChainAI/statuses> | user not found (X API and FxEmbed)；X API: user not found（官方为 @LangChain） |
| X @leeerob (Lee Robinson) | x | <https://api.fxtwitter.com/2/profile/leeerob/statuses> | not the official/active account；仿冒/占位（0 帖），正确为 @leerob |
| X @flyio (flyio) | x | <https://api.fxtwitter.com/2/profile/flyio/statuses> | not the official/active account；非官方（官方为 @flydotio） |
| X @railway_app (Railway) | x | <https://api.fxtwitter.com/2/profile/railway_app/statuses> | not the official/active account；占位账号（0 帖），官方为 @Railway |
| X @denoland (Doug Noland) | x | <https://api.fxtwitter.com/2/profile/denoland/statuses> | not the official/active account；无关个人账号，不是 Deno（官方为 @deno_land） |
| X @getsentry (Sentry) | x | <https://api.fxtwitter.com/2/profile/getsentry/statuses> | not the official/active account；2025 新建的小号（391 粉/10 帖），主账号为 @sentry |
| X @sveltejs (Svelte) | x | <https://api.fxtwitter.com/2/profile/sveltejs/statuses> | inactive: last post/repost 2024-12-24 |
| X @renderdotcom | x | <https://api.fxtwitter.com/2/profile/renderdotcom/statuses> | user not found (X API and FxEmbed)；X API: user not found（官方为 @render） |
| X @ClerkDev | x | <https://api.fxtwitter.com/2/profile/ClerkDev/statuses> | user not found (X API and FxEmbed)；X API: user not found |

## 附：公共 X List（无需登录）

- `GET https://syndication.twitter.com/srv/timeline-list/list-id/{list_id}`：返回最新约 65–100 条（含转推，无分页），限速约 30 次/15 分钟/IP，缓存 60s；实现见 `src/ai_news_crawler/x_syndication.py`。
- FxEmbed 无列表接口；`x.com/i/lists/{id}` 网页需登录/JS。Syndication 为未公开接口，可能随时变化。
