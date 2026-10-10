# 抓取限制与速率说明（Rate limits）

> 核对日期：2026-10-10（北京时间）。“文档限额”只写官方/一手来源明确写出的内容；找不到时明确标注“未找到官方文档”。“实测”来自当日对真实端点的请求与响应头。“我们的用量”按 sources/sources.json 中启用的来源与轮询间隔计算（不含失败重试与手动“立即抓取”）。

本文件由 `node scripts/gen-rate-limits-md.mts` 生成；网页版见站内 `/docs`。

## 总览

| 类型 | 文档限额 | 官方来源 | 我们的用量 | 风险 |
|---|---|---|---|---|
| FxEmbed / FxTwitter API（X 账号时间线） | API v2 每个 IP 每分钟 1000 次（约 16.7 次/秒）；超出建议自建实例。 | [FxEmbed API Overview · Rate limiting](https://docs.fxembed.com/api/introduction)<br>[FxEmbed GitHub README（指向文档）](https://github.com/FxEmbed/FxEmbed) | 27 个源 · 8.4 次/分钟（504 次/小时） | 低 |
| X syndication 嵌入时间线（timeline-list / timeline-profile） | 未找到官方文档（X 开发者文档只列 X API v2 端点的限额，不含 syndication 嵌入接口）。 | [X API Rate Limits（仅 API v2，不含此接口）](https://docs.x.com/x-api/fundamentals/rate-limits)<br>[社区观察：wet-boew issue #10047（30 次 / 约 15 分钟）](https://github.com/wet-boew/wet-boew/issues/10047) | 0 个源 · 0 次/分钟（0 次/小时） | 高 |
| GitHub Releases Atom（releases.atom） | 未找到 Atom feed 的专门限额文档。.atom 属于 github.com 网页端而非 REST API；作为参考，REST API 未认证为每 IP 60 次/小时，另有二级限额（并发 ≤100、每分钟 900 点等）。 | [GitHub Docs · Rate limits for the REST API（参考）](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api) | 0 个源 · 0 次/分钟（0 次/小时） | 低 |
| Hugging Face（Daily Papers API + Blog RSS） | Hub 限额按 5 分钟固定窗口计：匿名用户（按 IP）API 500 次、Pages 100 次、Resolvers 3000 次；免费用户 API 1000 次。超限返回 429，并带 RateLimit / RateLimit-Policy 头。 | [Hugging Face Docs · Hub Rate limits](https://huggingface.co/docs/hub/rate-limits) | 1 个源 · 0.1 次/分钟（6 次/小时） | 低 |
| OpenRouter 模型列表 API | 文档的限额针对推理请求（免费模型 20 次/分钟等）与 Cloudflare DDoS 防护；未给出 /models 列表端点的具体限额。 | [OpenRouter Docs · API Limits](https://openrouter.ai/docs/api/reference/limits) | 0 个源 · 0 次/分钟（0 次/小时） | 低 |
| hnrss.org（Hacker News RSS） | 未找到明确的数值限额。文档仅要求对 favorites 类 feed“请格外保守地设置刷新频率”，count 上限 100。 | [hnrss 项目页（官方文档）](https://hnrss.github.io/)<br>[hnrss GitHub README](https://github.com/hnrss/hnrss) | 0 个源 · 0 次/分钟（0 次/小时） | 低 |
| 机器之心 文章 JSON | 未找到官方文档（站点前端使用的内部 JSON 接口，非公开 API）。 | 未找到官方文档 | 1 个源 · 0.2 次/分钟（12 次/小时） | 中 |
| Substack 通讯 feed（Latent Space 等） | 未找到官方限额文档。 | 未找到官方文档 | 1 个源 · 0.03 次/分钟（2 次/小时） | 低 |
| Techmeme RSS | 未找到官方限额文档。 | 未找到官方文档 | 1 个源 · 0.2 次/分钟（12 次/小时） | 低 |
| 网易娱乐 newsdata（JSONP） | 未找到官方文档：这是网易娱乐首页前端使用的 JSONP 数据文件，并非公开 API。 | 未找到官方文档 | 1 个源 · 0.1 次/分钟（6 次/小时） | 中 |
| B 站热门 popular（web 接口） | 未找到官方文档：bilibili 未公开 web-interface 接口文档及限额（社区记录过高频请求会返回 -412 风控）。 | 未找到官方文档 | 1 个源 · 0.1 次/分钟（6 次/小时） | 中 |
| 抖音热搜 billboard（web 接口） | 未找到官方文档：抖音分享页使用的非公开接口。 | 未找到官方文档 | 1 个源 · 0.1 次/分钟（6 次/小时） | 高 |
| DeepSeek API（翻译，可选） | 官方按账号限制并发（非 QPS）：deepseek-flash 2500 并发、deepseek-v4-pro 500 并发，超出返回 HTTP 429；未开始推理 10 分钟后服务端断开。计费按 token（flash：输入未命中缓存 $0.15–0.30/M，输出 $0.6–1.2/M，非高峰半价）。 | [DeepSeek · Rate Limit & Isolation](https://api-docs.deepseek.com/quick_start/rate_limit)<br>[DeepSeek · Models & Pricing](https://api-docs.deepseek.com/quick_start/pricing) | 0 个源 · 0 次/分钟（0 次/小时） | 低 |
| 其他通用 RSS / Atom（官方博客与媒体） | 各站点均未公布 feed 限额。RSS 2.0 规范仅提供可选的 &lt;ttl>（建议缓存分钟数）作为礼仪约定。 | [RSS 2.0 Specification · <ttl>](https://www.rssboard.org/rss-specification#ltttlgtSubelementOfLtchannelgt) | 23 个源 · 2.67 次/分钟（160 次/小时） | 低 |

合计约 **11.9 次/分钟**（57 个启用来源）。

## 分类型详情

### FxEmbed / FxTwitter API（X 账号时间线）

- 端点：`https://api.fxtwitter.com/2/profile/{handle}/statuses?count=20`
- 文档限额：API v2 每个 IP 每分钟 1000 次（约 16.7 次/秒）；超出建议自建实例。
- 来源：[FxEmbed API Overview · Rate limiting](https://docs.fxembed.com/api/introduction)；[FxEmbed GitHub README（指向文档）](https://github.com/FxEmbed/FxEmbed)
- 实测（2026-10-10）：响应头无 x-rate-limit-* / RateLimit 字段（server: cloudflare）。偶发“假 404”（真实账号返回 User not found），重试后恢复；我们对 404/429/5xx 最多重试 5 次、线性退避。
- 我们的用量：27 个源，约 8.4 次/分钟（504 次/小时）：X @testingcatalog、X @btibor91、X @AiBreakfast、X @ArtificialAnlys、X @arena、X @apples_jimmy、X @kimmonismus、X @OpenAI (OpenAI)、X @OpenAIDevs (OpenAI Developers)、X @AnthropicAI (Anthropic)、X @claudeai (Claude)、X @SpaceXAI (SpaceXAI)、X @OfficialLoganK (Logan Kilpatrick)、X @sama (Sam Altman)、X @karpathy (Andrej Karpathy)、X @cursor_ai (Cursor)、X @bcherny、X @_akhaliq (AK)、X @dotey、X @op7418、X @deepseek_ai (DeepSeek)、X @Alibaba_Qwen (Qwen)、X @Kimi_Moonshot (Kimi.ai)、X @Zai_org (Z.ai)、X @PopBase、X @PopCrave、X @DiscussingFilm
- 风险：**低** — 我们约 8 次/分钟，远低于 1000/分钟；主要风险是第三方服务稳定性与 X 侧变更，而非限流。

### X syndication 嵌入时间线（timeline-list / timeline-profile）

- 端点：`https://syndication.twitter.com/srv/timeline-list/list-id/{id}`
- 文档限额：未找到官方文档（X 开发者文档只列 X API v2 端点的限额，不含 syndication 嵌入接口）。（**未找到官方限额文档**）
- 来源：[X API Rate Limits（仅 API v2，不含此接口）](https://docs.x.com/x-api/fundamentals/rate-limits)；[社区观察：wet-boew issue #10047（30 次 / 约 15 分钟）](https://github.com/wet-boew/wet-boew/issues/10047)
- 实测（2026-10-10）：2026-10-10 实测响应头 x-rate-limit-limit: 30、x-rate-limit-remaining、x-rate-limit-reset（约 15 分钟窗口，按 IP）；本机当时已返回 429 且 remaining=0。cache-control: no-cache。
- 我们的用量：0 个源，约 0 次/分钟（0 次/小时）
- 风险：**高** — 未公开接口，额度仅约 30 次/15 分钟/IP，随时可能变更或关闭。核心 50 中未使用；若添加 X 列表，间隔不应低于 2 分钟且同一 IP 只放 1~2 个列表。

### GitHub Releases Atom（releases.atom）

- 端点：`https://github.com/{owner}/{repo}/releases.atom`
- 文档限额：未找到 Atom feed 的专门限额文档。.atom 属于 github.com 网页端而非 REST API；作为参考，REST API 未认证为每 IP 60 次/小时，另有二级限额（并发 ≤100、每分钟 900 点等）。（**未找到官方限额文档**）
- 来源：[GitHub Docs · Rate limits for the REST API（参考）](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)
- 实测（2026-10-10）：响应头无 x-ratelimit-*；有 ETag（W/"…"）与 cache-control: max-age=0, private, must-revalidate，可用 If-None-Match 条件请求。
- 我们的用量：0 个源，约 0 次/分钟（0 次/小时）
- 风险：**低** — 3 个仓库合计约 14 次/小时；即使按 REST 未认证 60/小时的保守标准也在范围内。（2026-10-10 起「开发」分区已移除，核心列表中 0 个源）

### Hugging Face（Daily Papers API + Blog RSS）

- 端点：`https://huggingface.co/api/daily_papers ；https://huggingface.co/blog/feed.xml`
- 文档限额：Hub 限额按 5 分钟固定窗口计：匿名用户（按 IP）API 500 次、Pages 100 次、Resolvers 3000 次；免费用户 API 1000 次。超限返回 429，并带 RateLimit / RateLimit-Policy 头。
- 来源：[Hugging Face Docs · Hub Rate limits](https://huggingface.co/docs/hub/rate-limits)
- 实测（2026-10-10）：实测 /api/daily_papers 响应头：ratelimit: "api";r=499;t=246，ratelimit-policy: "fixed window";"api";q=500;w=300（与文档一致）。
- 我们的用量：1 个源，约 0.1 次/分钟（6 次/小时）：Hugging Face Blog
- 风险：**低** — 两个端点各每 10 分钟 1 次，相当于每 5 分钟 0.5 次，远低于 500（API）/100（Pages）。（2026-10-10 起「开发」分区已移除，核心列表中 0 个源）

### OpenRouter 模型列表 API

- 端点：`https://openrouter.ai/api/v1/models`
- 文档限额：文档的限额针对推理请求（免费模型 20 次/分钟等）与 Cloudflare DDoS 防护；未给出 /models 列表端点的具体限额。
- 来源：[OpenRouter Docs · API Limits](https://openrouter.ai/docs/api/reference/limits)
- 实测（2026-10-10）：响应头 cache-control: public, max-age=120, stale-while-revalidate=3600；cf-cache-status: HIT（CDN 缓存），无 X-RateLimit-* 头（文档说明只在 429 时返回）。
- 我们的用量：0 个源，约 0 次/分钟（0 次/小时）
- 风险：**低** — 每 5 分钟 1 次，且命中 CDN 缓存；无需 API Key。（2026-10-10 起「开发」分区已移除，核心列表中 0 个源）

### hnrss.org（Hacker News RSS）

- 端点：`https://hnrss.org/newest?points=100`
- 文档限额：未找到明确的数值限额。文档仅要求对 favorites 类 feed“请格外保守地设置刷新频率”，count 上限 100。（**未找到官方限额文档**）
- 来源：[hnrss 项目页（官方文档）](https://hnrss.github.io/)；[hnrss GitHub README](https://github.com/hnrss/hnrss)
- 实测（2026-10-10）：响应头 cache-control: max-age=600、x-cache: HIT、last-modified；即服务端缓存 10 分钟，更快的轮询没有意义。
- 我们的用量：0 个源，约 0 次/分钟（0 次/小时）
- 风险：**低** — 我们 10 分钟 1 次，正好与其缓存时间一致。（2026-10-10 起「开发」分区已移除，核心列表中 0 个源）

### 机器之心 文章 JSON

- 端点：`https://www.jiqizhixin.com/api/article_library/articles.json?sort=time&page=1`
- 文档限额：未找到官方文档（站点前端使用的内部 JSON 接口，非公开 API）。（**未找到官方限额文档**）
- 来源：无（未找到官方文档）
- 实测（2026-10-10）：nginx，响应头有 ETag，cache-control: max-age=0, private, must-revalidate；无限额相关头。
- 我们的用量：1 个源，约 0.2 次/分钟（12 次/小时）：机器之心
- 风险：**中** — 非公开接口，可能无预警变更；保持 5 分钟间隔、带正常 UA。

### Substack 通讯 feed（Latent Space 等）

- 端点：`https://www.latent.space/feed`
- 文档限额：未找到官方限额文档。（**未找到官方限额文档**）
- 来源：无（未找到官方文档）
- 实测（2026-10-10）：Cloudflare，cache-control: no-cache，带 ETag 与 age（实测约 1983 秒，说明前面有缓存层）。
- 我们的用量：1 个源，约 0.03 次/分钟（2 次/小时）：Latent Space
- 风险：**低** — 每 30 分钟 1 次。

### Techmeme RSS

- 端点：`https://www.techmeme.com/feed.xml`
- 文档限额：未找到官方限额文档。（**未找到官方限额文档**）
- 来源：无（未找到官方文档）
- 实测（2026-10-10）：Cloudflare（cf-cache-status: DYNAMIC），有 Last-Modified 与 ETag，可做条件请求。
- 我们的用量：1 个源，约 0.2 次/分钟（12 次/小时）：Techmeme
- 风险：**低** — 每 5 分钟 1 次。

### 网易娱乐 newsdata（JSONP）

- 端点：`https://ent.163.com/special/000380VU/newsdata_index.js`
- 文档限额：未找到官方文档：这是网易娱乐首页前端使用的 JSONP 数据文件，并非公开 API。（**未找到官方限额文档**）
- 来源：无（未找到官方文档）
- 实测（2026-10-10）：2026-10-10 实测 HTTP 200，openresty，cache-control: max-age=60，无限额响应头；返回 40 条（UTF-8，data_callback([...]) 包裹）。
- 我们的用量：1 个源，约 0.1 次/分钟（6 次/小时）：网易娱乐
- 风险：**中** — 非公开接口，格式或地址可能随网站改版变化；我们 10 分钟 1 次，远低于其 60 秒 CDN 缓存。

### B 站热门 popular（web 接口）

- 端点：`https://api.bilibili.com/x/web-interface/popular?ps=20`
- 文档限额：未找到官方文档：bilibili 未公开 web-interface 接口文档及限额（社区记录过高频请求会返回 -412 风控）。（**未找到官方限额文档**）
- 来源：无（未找到官方文档）
- 实测（2026-10-10）：2026-10-10 实测 GET 200（code 0，20 条）；HEAD 返回 405；无限额响应头。
- 我们的用量：1 个源，约 0.1 次/分钟（6 次/小时）：B站热门
- 风险：**中** — 非公开接口，有风控（-412）；30 分钟 1 次。

### 抖音热搜 billboard（web 接口）

- 端点：`https://www.iesdouyin.com/web/api/v2/hotsearch/billboard/word/`
- 文档限额：未找到官方文档：抖音分享页使用的非公开接口。（**未找到官方限额文档**）
- 来源：无（未找到官方文档）
- 实测（2026-10-10）：2026-10-10 实测 GET 200，返回 50 个热词（只有词与热度，无时间、无链接；我们用抖音搜索页作为链接，用抓取时间作为时间）；HEAD 404；无限额响应头。
- 我们的用量：1 个源，约 0.1 次/分钟（6 次/小时）：抖音热搜
- 风险：**高** — 非公开接口，随时可能失效或加签；10 分钟 1 次。

### DeepSeek API（翻译，可选）

- 端点：`POST https://api.deepseek.com/chat/completions（model deepseek-chat，JSON 输出）`
- 文档限额：官方按账号限制并发（非 QPS）：deepseek-flash 2500 并发、deepseek-v4-pro 500 并发，超出返回 HTTP 429；未开始推理 10 分钟后服务端断开。计费按 token（flash：输入未命中缓存 $0.15–0.30/M，输出 $0.6–1.2/M，非高峰半价）。
- 来源：[DeepSeek · Rate Limit & Isolation](https://api-docs.deepseek.com/quick_start/rate_limit)；[DeepSeek · Models & Pricing](https://api-docs.deepseek.com/quick_start/pricing)
- 实测（2026-10-10）：2026-10-10 实测：3 条标题+摘要一批，HTTP 200，约 0.85 s，184 输入 + 107 输出 token。
- 我们的用量：0 个源，约 0 次/分钟（0 次/小时）
- 风险：**低** — 仅在设置 DEEPSEEK_API_KEY 时启用；每批 10 条、并发 2，远低于并发上限；不计入下方「轮询用量」（按新条目触发）。

### 其他通用 RSS / Atom（官方博客与媒体）

- 端点：`OpenAI、Google/DeepMind、Claude 发布说明、量子位、极客公园、TechCrunch、The Verge 等；娱乐：TMZ、Page Six、E! News、Variety、Deadline 等`
- 文档限额：各站点均未公布 feed 限额。RSS 2.0 规范仅提供可选的 &lt;ttl>（建议缓存分钟数）作为礼仪约定。（**未找到官方限额文档**）
- 来源：[RSS 2.0 Specification · <ttl>](https://www.rssboard.org/rss-specification#ltttlgtSubelementOfLtchannelgt)
- 实测（2026-10-10）：多数经 Cloudflare/CDN（如 openai.com cache-control: public, max-age=0，age≈1843），未见限额头；目前全部 200。
- 我们的用量：23 个源，约 2.67 次/分钟（160 次/小时）：OpenAI News、Claude Platform release notes、Google Blog - Gemini、Google DeepMind Blog、Google Developers Blog、Google Research Blog、Mistral AI News、Microsoft Research Blog、Apple Machine Learning Research、量子位、极客公园、The Decoder、TechCrunch AI、The Verge AI、Simon Willison's Weblog、TMZ、Page Six、E! News、Just Jared、Deadline、Variety、Bored Panda、UPI Odd News
- 风险：**低** — 单站点每 5~30 分钟 1 次；同一主机（如 Google 多个博客）合计也不到 1 次/分钟。

## 礼仪与约定

- 带真实 UA（`ai-news-crawler/1.0`），超时 15~20 秒；429/5xx 退避重试，FxEmbed 404 视为可能的假阴性最多重试 5 次。
- 不要把间隔调到低于源站缓存时间（hnrss 600 秒、OpenRouter 120 秒）——更快只会拿到同样的缓存内容。
- X 列表（syndication）额度极小，添加时谨慎；单实例部署，切勿多实例同时轮询同一出口 IP。
- 优先使用条件请求（ETag / Last-Modified）是后续可做的优化（当前未实现）。
