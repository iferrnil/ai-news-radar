# 数据源优化建议（AI 自媒体 / 抢先 + 全面 + 低噪声）

> 生成：2026-10-10 17:39（北京时间）。**这份只是建议，未改动 sources.yaml 和数据库。**
> 依据：`data/news.db` 首轮抓取统计（`scripts/analyze_db.py` → `docs/source_stats.json`）；60 天 X 与官网时间差（`docs/scoop_stats.json`，基于 `docs/x_vs_blog_data.json`）；候选 X 账号 FxEmbed 实测（`scripts/verify_candidates_x.py` → `docs/candidates_x.json`，「不存在」的结果用 X API 复核过）；候选网站实测（`scripts/verify_candidates_web.py` → `docs/candidates_web.json`）。

## 0. 关键发现

- **官方博客通常比官方 X 早发**：60 天内 380 对「博客 ↔ 官方 X 帖」的链接匹配（只取带精确发布时间的 feed）中，78% 是博客先发，X 中位数晚 4.2 小时；只有 11% 是 X 先发（如 Next.js、Mistral）。例外是 @githubstatus，比状态页早 1.5 小时（24/24）。所以要抢先：官方 feed 要高频轮询；官方 X 的价值在于网站上没有的内容（灰度上线、小功能、转发），以及爆料号/速报号。
- 注意：feed 的 pubDate 可能是文章的名义发布时间，而且这次数据库里只有约 2.5 小时的轮询历史，所以「谁更快」的结论以 60 天的历史数据为准；建议跑满一周后再按 `also_on_x` 合并记录复核。
- **重新考虑 @OpenAI / @OpenAIDevs**：上一轮「X vs 官网」去重把它们停用了（保留网站），但 X 上有 45 条网站没有的实质内容（如 API 灰度、限时活动）。对抢先型创作者，建议重新启用，并设为 P0；重复内容已经由「也在 𝕏」合并处理。
- arXiv cs.CL/cs.LG/cs.CV 本轮抓到 0 条（周末 feed 为空），cs.AI 60 条/周。对自媒体来说论文用 HF Daily Papers 和 @_akhaliq 筛选更合适。

## 1. 建议去掉（共 80 个，另有 6 个建议保留但加过滤或降级）

### 状态页（与AI无关，量大）（8）

| 来源 | 实测数据 | 理由 |
|---|---|---|
| Cloudflare Status | 25条/周，AI相关0%，最新0.0天前 | 全是机房计划维护/故障 |
| Vercel Status | 3条/周，AI相关0%，最新0.3天前 | 状态事件 |
| Render Status | 2条/周，AI相关0%，最新0.0天前 | 状态事件 |
| Supabase Status | 4条/周，AI相关0%，最新2.7天前 | 状态事件 |
| Fly.io Status | 4条/周，AI相关0%，最新1.5天前 | 状态事件 |
| DigitalOcean Status | 2条/周，AI相关0%，最新4.9天前 | 状态事件 |
| Google Cloud Service Health | 1条/周，AI相关0%，最新0.7天前 | 状态事件 |
| @githubstatus | 20条/周，AI相关0%，最新0.5天前 | X 上推故障早于状态页（中位数 1.5 小时），但内容与 AI 无关 |

### 非 AI 的基础设施网站源（20）

| 来源 | 实测数据 | 理由 |
|---|---|---|
| Cloudflare Changelog | 19条/周，AI相关26%，最新0.4天前 | 产品 changelog，大多不涉及 AI |
| AWS What's New | 36条/周，AI相关31%，最新0.5天前 | 量大，AI 占比低 |
| Azure Updates | 20条/周，AI相关20%，最新0.5天前 | AI 占比低 |
| Google Cloud Release Notes | 1条/周，AI相关0%，最新1.1天前 | 通用云更新 |
| Google Cloud Blog (all) | 11条/周，AI相关73%，最新0.7天前 | 和 Google Cloud AI & ML 重复，留 AI & ML 那个即可 |
| DigitalOcean Blog | 2条/周，AI相关50%，最新0.6天前 | 量少，与 AI 关系弱 |
| DigitalOcean Release Notes | 10条/周，AI相关60%，最新1.4天前 | 通用云更新 |
| Fly.io Blog | 0条/周，AI相关—，最新37.4天前 | 37 天没更新 |
| Fly.io Fresh Produce (changelog) | 1条/周，AI相关0%，最新2.6天前 | AI 相关 0% |
| Render Blog | 1条/周，AI相关0%，最新1.6天前 | AI 相关 0% |
| Render Changelog | 0条/周，AI相关—，最新9.4天前 | 近 9 天没有更新 |
| Supabase Blog | 0条/周，AI相关—，最新7.7天前 | 近一周 0 条 |
| Deno Blog | 0条/周，AI相关—，最新107.0天前 | 107 天没更新 |
| Deno Releases (GitHub) | 0条/周，AI相关—，最新23.0天前 | 与 AI 无关 |
| Node.js Blog | 2条/周，AI相关0%，最新2.4天前 | AI 相关 0% |
| React Blog | 0条/周，AI相关—，最新31.4天前 | 31 天没更新 |
| React Releases (GitHub) | 0条/周，AI相关—，最新30.6天前 | 与 AI 无关 |
| Stripe Blog | 0条/周，AI相关—，最新9.4天前 | 与 AI 无关 |
| Fastly Blog | 3条/周，AI相关33%，最新2.4天前 | 与 AI 关系弱 |
| Microsoft Azure Blog | 2条/周，AI相关50%，最新2.8天前 | 营销稿，和 Microsoft AI Blog 重复 |

### 非 AI 的基础设施 X 账号（28）

| 来源 | 实测数据 | 理由 |
|---|---|---|
| @kubernetesio | 4条/周，AI相关0%，最新0.7天前 | AI 相关 0% |
| @prisma | 1条/周，AI相关0%，最新3.8天前 | AI 相关 0% |
| @astrodotbuild | 1条/周，AI相关0%，最新5.4天前 | AI 相关 0% |
| @vuejs | 0条/周，AI相关—，最新256.9天前 | 257 天没发帖 |
| @vite_js | 0条/周，AI相关—，最新71.7天前 | 72 天没发帖 |
| @typescript | 0条/周，AI相关—，最新22.7天前 | 23 天没发帖 |
| @StripeDev | 0条/周，AI相关—，最新21.4天前 | 21 天没发帖 |
| @stripe | 1条/周，AI相关100%，最新3.6天前 | 与 AI 无关 |
| @neondatabase | 3条/周，AI相关0%，最新2.7天前 | AI 相关 0% |
| @render | 1条/周，AI相关0%，最新0.7天前 | AI 相关 0% |
| @flydotio | 2条/周，AI相关0%，最新1.4天前 | AI 相关 0% |
| @PlanetScale | 10条/周，AI相关10%，最新0.8天前 | AI 相关 10% |
| @fastly | 2条/周，AI相关50%，最新0.5天前 | 与 AI 关系弱 |
| @HashiCorp | 3条/周，AI相关100%，最新0.8天前 | 企业营销 |
| @digitalocean | 14条/周，AI相关64%，最新0.6天前 | 营销为主 |
| @supabase | 17条/周，AI相关12%，最新0.4天前 | AI 相关 12% |
| @awscloud | 17条/周，AI相关12%，最新1.0天前 | AI 相关 12%，营销为主 |
| @awsdevelopers | 12条/周，AI相关17%，最新0.6天前 | AI 相关 17% |
| @Azure | 5条/周，AI相关20%，最新0.7天前 | AI 相关 20% |
| @linear | 5条/周，AI相关20%，最新0.7天前 | AI 相关 20% |
| @bunjavascript | 14条/周，AI相关7%，最新0.2天前 | AI 相关 7% |
| @upstash | 9条/周，AI相关67%，最新1.3天前 | 小众 |
| @Netlify | 9条/周，AI相关89%，最新1.7天前 | 与 AI 关系弱 |
| @Railway | 8条/周，AI相关75%，最新0.5天前 | 与 AI 关系弱 |
| @posthog | 8条/周，AI相关50%，最新0.7天前 | 与 AI 关系弱 |
| @sentry | 12条/周，AI相关50%，最新0.5天前 | 与 AI 关系弱 |
| @nextjs | 6条/周，AI相关33%，最新1.6天前 | 前端框架，与 AI 关系弱 |
| @Cloudflare | 17条/周，AI相关24%，最新0.1天前 | AI 相关 24%；保留 @CloudflareDev（Workers AI/Agents 相关） |

### 噪声人物/机器人账号（6）

| 来源 | 实测数据 | 理由 |
|---|---|---|
| @grok | 2条/周，AI相关0%，最新0.7天前 | 机器人自动回复，AI 资讯 0% |
| @elonmusk | 8条/周，AI相关12%，最新0.2天前 | 100% 是短帖/表情回复；xAI 的消息 @SpaceXAI 已覆盖 |
| @alexandr_wang | 18条/周，AI相关6%，最新0.6天前 | AI 相关 6% |
| @jarredsumner | 12条/周，AI相关8%，最新0.0天前 | Bun 工程细节，AI 相关 8% |
| @thdxr | 16条/周，AI相关19%，最新0.2天前 | AI 相关 19%，闲聊为主 |
| @rough__sea | 8条/周，AI相关25%，最新0.8天前 | Deno 相关，AI 相关 25% |

### 停更、被拦截、重复或低相关的媒体与社区源（18）

| 来源 | 实测数据 | 理由 |
|---|---|---|
| The Gradient | 0条/周，AI相关—，最新233.4天前 | 233 天没更新 |
| VentureBeat AI | 0条/周，AI相关—，最新43.8天前 | 44 天没有新条目，且经常 429 |
| Answer.AI | 0条/周，AI相关—，最新52.4天前 | 52 天没更新（我测试时手动添加的） |
| cnBeta | 61条/周，AI相关23%，最新0.0天前 | 61条/周，AI 相关 23% |
| Solidot | 20条/周，AI相关20%，最新0.1天前 | AI 相关 20% |
| 少数派 | 10条/周，AI相关20%，最新0.3天前 | AI 相关 20% |
| 雷峰网 全站 | 20条/周，AI相关55%，最新0.2天前 | AI 内容与「雷峰网 AI 频道」重复 |
| Hacker News Front Page (official RSS) | 30条/周，AI相关13%，最新0.2天前 | AI 相关 13%；改用 hnrss AI 关键词 / 高分源 |
| arXiv cs.LG | 0条/周，AI相关—，最新None天前 | 论文洪流，对自媒体噪声过大；用 HF Daily Papers + @_akhaliq 代替 |
| arXiv cs.CV | 0条/周，AI相关—，最新None天前 | 同上 |
| Andrej Karpathy blog | 0条/周，AI相关—，最新162.7天前 | 163 天没更新；@karpathy 已覆盖 |
| Microsoft Source - AI | 0条/周，AI相关—，最新29.9天前 | 30 天没更新 |
| Engineering at Meta | 1条/周，AI相关0%，最新3.7天前 | AI 相关 0% |
| AI News (artificialintelligence-news.com) | — | 被拦截且是二手稿 |
| RSSHub 公共实例 (rsshub.app) | — | 公共实例被拦截；要用请自建 |
| Stanford HAI News | — | 需要爬取，频率低 |
| The Batch (DeepLearning.AI) | — | 周刊，需要爬取 |
| 站长之家 AI 频道 | — | 需要爬取，内容是二手转载 |

### 保留但加过滤或降级

| 来源 | 实测数据 | 建议 |
|---|---|---|
| IT之家 | 62条/周，AI相关47%，最新0.0天前 | 国内最快的综合科技快讯源之一（62条/周），但 AI 相关只有 47%：保留，只入库标题命中 AI 关键词的条目 |
| 爱范儿 | 21条/周，AI相关48%，最新0.0天前 | AI 相关 48%，同上，加关键词过滤 |
| 钛媒体 | 19条/周，AI相关42%，最新0.0天前 | AI 相关 42%，同上 |
| OpenAI News | 15条/周，AI相关93%，最新1.1天前 | 混有大量客户案例（如「X cuts … with ChatGPT」）：保留（P0），但过滤或降权客户案例标题 |
| AWS Machine Learning Blog | 20条/周，AI相关90%，最新0.7天前 | 20条/周，多为客户/方案稿：降为 P2，并按标题过滤 |
| Google Blog - AI | 1条/周，AI相关0%，最新2.9天前 | 近一周 AI 关键词命中 0（多为产品营销）：降为 P2 |

## 2. 建议新增（全部已实测）

X 账号 101 个（FxEmbed profile + 时间线首页实测存在且活跃），网站/API 44 个可用（另列出实测不可用的，供参考）。

### X · 抢先/爆料/聚合（速度关键）（15）

| 账号 | 名称 | 粉丝 | 最新原创 | 帖/天 | 优先级 | 说明 |
|---|---|---|---|---|---|---|
| [@testingcatalog](https://x.com/testingcatalog) | 🚨 AI News | TestingCatalog | 79545 | 0.1 天前 | 12.5 | P0 | 抢先挖掘各 App/网页的新功能，几乎是爆料第一源 |
| [@btibor91](https://x.com/btibor91) | Tibor Blaho | 40486 | 0.6 天前 | 1.5 | P0 | 拆包挖 ChatGPT/Claude 的新功能 |
| [@AiBreakfast](https://x.com/AiBreakfast) | AI Breakfast | 241142 | 0.4 天前 | 1.4 | P0 | AI 新闻快讯 |
| [@apples_jimmy](https://x.com/apples_jimmy) | Jimmy Apples 🍎/acc | 60031 | 0.6 天前 | 2.0 | P0 | OpenAI 爆料（准确率参差，需标注「传闻」） |
| [@scaling01](https://x.com/scaling01) | Lisan al Gaib | 60824 | 0.1 天前 | 5.3 | P1 | 模型发布/基准速报 |
| [@kimmonismus](https://x.com/kimmonismus) | Chubby♨️ | 149788 | 0.0 天前 | 17.0 | P1 | 聚合快讯（约 17 帖/天，偏多） |
| [@flowersslop](https://x.com/flowersslop) | Flowers ☾ | 16446 | 0.5 天前 | 8.1 | P1 | 新模型、新功能发现 |
| [@ai_for_success](https://x.com/ai_for_success) | AshutoshShrivastava | 81986 | 0.2 天前 | 9.9 | P1 | 聚合（约 10 帖/天） |
| [@petergostev](https://x.com/petergostev) | Peter Gostev | 28282 | 0.0 天前 | 5.5 | P1 | 模型实测、新发布 |
| [@Yuchenj_UW](https://x.com/Yuchenj_UW) | Yuchen Jin | 303954 | 0.2 天前 | 2.1 | P1 | 行业/算力消息 |
| [@TheRundownAI](https://x.com/TheRundownAI) | The Rundown AI | 229598 | 0.8 天前 | 6.2 | P1 | 日报型聚合 |
| [@rowancheung](https://x.com/rowancheung) | Rowan Cheung | 600755 | 1.5 天前 | 0.3 | P2 | 发帖少，主要转发日报 |
| [@AndrewCurran_](https://x.com/AndrewCurran_) | Andrew Curran | 105699 | 0.5 天前 | 71.8 | P2 | 约 72 帖/天，噪声大，但覆盖全 |
| [@iruletheworldmo](https://x.com/iruletheworldmo) | 🍓🍓🍓 | 116838 | 0.4 天前 | 5.5 | P2 | 传闻号，可信度低，只作线索 |
| [@legit_api](https://x.com/legit_api) | Legit | 14837 | 16.7 天前 | 0.3 | P2 | 爆料号，16 天没发帖 |

### X · 榜单与模型发布（3）

| 账号 | 名称 | 粉丝 | 最新原创 | 帖/天 | 优先级 | 说明 |
|---|---|---|---|---|---|---|
| [@ArtificialAnlys](https://x.com/ArtificialAnlys) | Artificial Analysis | 157142 | 0.5 天前 | 10.8 | P0 | 新模型跑分最快（约 11 帖/天） |
| [@arena](https://x.com/arena) | Arena.ai | 233332 | 0.5 天前 | 14.9 | P0 | LMArena 官方（@lmarena_ai 是空壳） |
| [@OpenRouter](https://x.com/OpenRouter) | OpenRouter | 153227 | 0.5 天前 | 8.7 | P1 | 新模型上线 |

### X · 媒体与记者（6）

| 账号 | 名称 | 粉丝 | 最新原创 | 帖/天 | 优先级 | 说明 |
|---|---|---|---|---|---|---|
| [@markgurman](https://x.com/markgurman) | Mark Gurman | 482950 | 0.5 天前 | 6.3 | P1 | Apple AI 独家 |
| [@kyliebytes](https://x.com/kyliebytes) | Kylie Robison | 52587 | 0.2 天前 | 3.3 | P1 | Wired AI 记者 |
| [@steph_palazzolo](https://x.com/steph_palazzolo) | Stephanie Palazzolo | 23458 | 3.8 天前 | 0.5 | P1 | The Information AI 记者 |
| [@nmasc_](https://x.com/nmasc_) | Natasha Mascarenhas | 56176 | 0.5 天前 | 3.2 | P2 | AI 创投报道 |
| [@Hadas_Gold](https://x.com/Hadas_Gold) | Hadas Gold | 71332 | 0.4 天前 | 1.1 | P2 | CNN |
| [@ProductHunt](https://x.com/ProductHunt) | Product Hunt 😸 | 565527 | 0.5 天前 | 4.4 | P2 | 用 RSS 更好 |

### X · 官方与产品（海外）（33）

| 账号 | 名称 | 粉丝 | 最新原创 | 帖/天 | 优先级 | 说明 |
|---|---|---|---|---|---|---|
| [@OpenAINewsroom](https://x.com/OpenAINewsroom) | OpenAI Newsroom | 165065 | 1.1 天前 | 0.2 | P1 | OpenAI 官方声明 |
| [@GoogleAIStudio](https://x.com/GoogleAIStudio) | Google AI Studio | 208037 | 3.7 天前 | 0.5 | P0 | Gemini API/AI Studio 新功能 |
| [@googleaidevs](https://x.com/googleaidevs) | Google AI Developers | 117527 | 18.6 天前 | 0.4 | P1 |  |
| [@GoogleLabs](https://x.com/GoogleLabs) | Google Labs | 227122 | 2.9 天前 | 0.1 | P2 |  |
| [@bcherny](https://x.com/bcherny) | Boris Cherny | 596945 | 2.3 天前 | 6.6 | P0 | Claude Code 负责人，发版第一手 |
| [@_catwu](https://x.com/_catwu) | cat | 99507 | 2.4 天前 | 0.1 | P1 | Claude Code 产品 |
| [@alexalbert__](https://x.com/alexalbert__) | Alex Albert | 152195 | 2.6 天前 | 0.4 | P1 | Anthropic DevRel |
| [@cognition](https://x.com/cognition) | Cognition | 189376 | 0.4 天前 | 1.7 | P1 | Devin/Windsurf 母公司（@windsurf 已退役） |
| [@Replit](https://x.com/Replit) | Replit ⠕ | 239773 | 0.4 天前 | 4.9 | P1 |  |
| [@Lovable](https://x.com/Lovable) | Lovable | 157623 | 0.8 天前 | 1.4 | P1 |  |
| [@v0](https://x.com/v0) | v0 | 117023 | 0.7 天前 | 0.4 | P1 |  |
| [@boltdotnew](https://x.com/boltdotnew) | bolt.new | 123101 | 0.6 天前 | 0.6 | P2 |  |
| [@runwayml](https://x.com/runwayml) | Runway | 293206 | 1.6 天前 | 1.6 | P1 |  |
| [@pika_labs](https://x.com/pika_labs) | Pika | 150701 | 3.6 天前 | 3.0 | P2 |  |
| [@midjourney](https://x.com/midjourney) | Midjourney | 462278 | 0.4 天前 | 0.2 | P1 | 发帖少但条条重要 |
| [@ElevenLabs](https://x.com/ElevenLabs) | ElevenLabs | 204589 | 0.5 天前 | 3.3 | P1 | @elevenlabsio 不存在 |
| [@suno](https://x.com/suno) | Suno | 111867 | 0.6 天前 | 4.3 | P1 |  |
| [@character_ai](https://x.com/character_ai) | Character.ai | 147843 | 9.5 天前 | 0.4 | P2 |  |
| [@Figure_robot](https://x.com/Figure_robot) | Figure | 225914 | 9.8 天前 | 0.3 | P1 | 具身智能 |
| [@physical_int](https://x.com/physical_int) | Physical Intelligence | 51239 | 0.5 天前 | 0.1 | P1 | 具身智能 |
| [@thinkymachines](https://x.com/thinkymachines) | Thinking Machines | 183989 | 28.5 天前 | 0.3 | P1 | 发帖少、份量大 |
| [@ssi](https://x.com/ssi) | SSI Inc. | 133388 | 74.8 天前 | 0.0 | P2 | 75 天没发帖，但一旦发帖就是大新闻 |
| [@RekaAILabs](https://x.com/RekaAILabs) | Reka | 21501 | 4.8 天前 | 0.2 | P2 |  |
| [@ManusAI](https://x.com/ManusAI) | Manus | 256487 | 2.8 天前 | 1.8 | P1 |  |
| [@genspark_ai](https://x.com/genspark_ai) | Genspark | 61976 | 0.5 天前 | 1.9 | P2 |  |
| [@HeyGen](https://x.com/HeyGen) | HeyGen | 114162 | 0.7 天前 | 1.5 | P2 |  |
| [@krea_ai](https://x.com/krea_ai) | Krea | 118973 | 2.8 天前 | 1.9 | P2 |  |
| [@ideogram_ai](https://x.com/ideogram_ai) | Ideogram | 74211 | 0.7 天前 | 0.6 | P2 |  |
| [@LumaLabsAI](https://x.com/LumaLabsAI) | Luma | 196984 | 0.5 天前 | 1.4 | P1 |  |
| [@FireworksAI_HQ](https://x.com/FireworksAI_HQ) | Fireworks | 33362 | 0.2 天前 | 4.4 | P2 |  |
| [@cerebras](https://x.com/cerebras) | Cerebras | 78774 | 2.4 天前 | 1.5 | P2 |  |
| [@modal](https://x.com/modal) | Modal | 37558 | 0.5 天前 | 2.0 | P2 |  |
| [@zeddotdev](https://x.com/zeddotdev) | Zed | 85270 | 0.6 天前 | 4.8 | P2 | AI 编辑器 |

### X · 官方（国内）（6）

| 账号 | 名称 | 粉丝 | 最新原创 | 帖/天 | 优先级 | 说明 |
|---|---|---|---|---|---|---|
| [@Kling_ai](https://x.com/Kling_ai) | Kling AI | 137071 | 1.2 天前 | 0.6 | P0 | 快手可灵 |
| [@Hailuo_AI](https://x.com/Hailuo_AI) | MiniMax Design (H3) | 82851 | 1.1 天前 | 0.6 | P1 | MiniMax 海螺（视频/设计） |
| [@TencentAI_News](https://x.com/TencentAI_News) | Tencent AI | 24692 | 1.0 天前 | 0.8 | P1 | 腾讯 AI 官方（@Tencent_AI 抓取失败） |
| [@OpenBMB](https://x.com/OpenBMB) | OpenBMB | 10940 | 1.0 天前 | 0.5 | P1 | 面壁/清华 |
| [@BAAIBeijing](https://x.com/BAAIBeijing) | BAAI | 3525 | 28.8 天前 | 0.1 | P2 | 智源，发帖很少 |
| [@intern_lm](https://x.com/intern_lm) | Intern Large Models | 4019 | 22.0 天前 | 0.3 | P2 | 上海 AI Lab 书生 |

### X · 研究者与开发者（22）

| 账号 | 名称 | 粉丝 | 最新原创 | 帖/天 | 优先级 | 说明 |
|---|---|---|---|---|---|---|
| [@_philschmid](https://x.com/_philschmid) | Philipp Schmid | 123777 | 0.9 天前 | 1.3 | P1 | Google DeepMind DevRel |
| [@reach_vb](https://x.com/reach_vb) | Vaibhav (VB) Srivastav | 61213 | 0.7 天前 | 2.3 | P1 | HF |
| [@mervenoyann](https://x.com/mervenoyann) | merve | 92912 | 0.7 天前 | 4.2 | P2 | HF 多模态 |
| [@Thom_Wolf](https://x.com/Thom_Wolf) | Thomas Wolf | 134625 | 1.0 天前 | 2.5 | P2 | HF 联创 |
| [@hwchase17](https://x.com/hwchase17) | Harrison Chase | 133765 | 0.2 天前 | 3.3 | P1 | LangChain |
| [@amasad](https://x.com/amasad) | Amjad Masad | 497238 | 0.7 天前 | 1.7 | P1 | Replit CEO |
| [@tszzl](https://x.com/tszzl) | roon | 503213 | 2.2 天前 | 6.6 | P2 | roon，OpenAI 风向 |
| [@teortaxesTex](https://x.com/teortaxesTex) | Teortaxes▶️ (DeepSeek 推特🐋铁粉 2023 – ∞) | 77360 | 0.0 天前 | 25.0 | P1 | DeepSeek/中国模型观察（约 25 帖/天） |
| [@JustinLin610](https://x.com/JustinLin610) | Junyang Lin | 94210 | 7.0 天前 | 0.4 | P1 | Qwen 负责人 |
| [@danielhanchen](https://x.com/danielhanchen) | Daniel Han | 36779 | 0.4 天前 | 0.4 | P1 | Unsloth，开源模型修复/量化 |
| [@ggerganov](https://x.com/ggerganov) | Georgi Gerganov | 72963 | 2.6 天前 | 1.7 | P1 | llama.cpp |
| [@Teknium](https://x.com/Teknium) | Teknium 🪽 | 132739 | 0.2 天前 | 3.6 | P2 | Nous（旧名 Teknium1） |
| [@omarsar0](https://x.com/omarsar0) | elvis | 323599 | 0.2 天前 | 13.3 | P2 | 论文解读，约 13 帖/天 |
| [@_jasonwei](https://x.com/_jasonwei) | Jason Wei | 114394 | 3.6 天前 | 0.0 | P2 |  |
| [@srush_nlp](https://x.com/srush_nlp) | Sasha Rush | 85490 | 1.6 天前 | 0.6 | P2 |  |
| [@mntruell](https://x.com/mntruell) | Michael Truell | 232077 | 8.6 天前 | 0.1 | P2 | Cursor CEO |
| [@jxnlco](https://x.com/jxnlco) | jason | 136960 | 1.4 天前 | 6.2 | P2 |  |
| [@levelsio](https://x.com/levelsio) | @levelsio | 971663 | 0.5 天前 | 9.1 | P2 | 独立开发者视角 |
| [@MatthewBerman](https://x.com/MatthewBerman) | Matthew Berman | 144353 | 0.4 天前 | 8.3 | P2 | YouTuber 视角 |
| [@deedydas](https://x.com/deedydas) | Deedy | 256784 | 1.7 天前 | 2.1 | P2 |  |
| [@bindureddy](https://x.com/bindureddy) | Bindu Reddy | 394316 | 0.3 天前 | 4.4 | P2 |  |
| [@rohanpaul_ai](https://x.com/rohanpaul_ai) | Rohan Paul | 159754 | 0.0 天前 | 43.9 | P2 | 约 44 帖/天，噪声大 |

### X · 中文 AI 博主/KOL（16）

| 账号 | 名称 | 粉丝 | 最新原创 | 帖/天 | 优先级 | 说明 |
|---|---|---|---|---|---|---|
| [@dotey](https://x.com/dotey) | 宝玉 | 256946 | 0.1 天前 | 0.5 | P0 | 宝玉，翻译/解读最快 |
| [@op7418](https://x.com/op7418) | 歸藏(guizang.ai) | 179667 | 0.3 天前 | 11.9 | P0 | 歸藏，产品与设计类 AI 速报（约 12 帖/天） |
| [@oran_ge](https://x.com/oran_ge) | Orange AI | 183997 | 0.1 天前 | 2.5 | P1 | Orange AI |
| [@vista8](https://x.com/vista8) | 向阳乔木 | 130722 | 0.2 天前 | 7.0 | P1 | 向阳乔木 |
| [@Gorden_Sun](https://x.com/Gorden_Sun) | Gorden Sun | 67888 | 0.0 天前 | 8.5 | P1 | AI 日报 |
| [@shao__meng](https://x.com/shao__meng) | meng shao | 35575 | 0.0 天前 | 15.4 | P1 | meng shao，聚合（约 15 帖/天） |
| [@Svwang1](https://x.com/Svwang1) | 硅谷王川 Chuan | 188211 | 0.4 天前 | 1.1 | P2 | 硅谷王川 |
| [@turingou](https://x.com/turingou) | 郭宇 guoyu.eth | 188120 | 0.0 天前 | 8.7 | P2 | 郭宇 |
| [@xicilion](https://x.com/xicilion) | 响马 | 70804 | 0.9 天前 | 0.5 | P2 | 响马 |
| [@berryxia](https://x.com/berryxia) | Berryxia.AI | 48827 | 0.0 天前 | 15.0 | P2 | 约 15 帖/天 |
| [@lxfater](https://x.com/lxfater) | 铁锤人 | 107430 | 0.2 天前 | 12.0 | P2 | 铁锤人，约 12 帖/天 |
| [@AYi_AInotes](https://x.com/AYi_AInotes) | AYi | 71026 | 0.2 天前 | 34.1 | P2 | 约 34 帖/天，噪声大 |
| [@ezshine](https://x.com/ezshine) | 大帅老猿 | 59915 | 0.2 天前 | 3.4 | P2 | 大帅老猿 |
| [@hongming731](https://x.com/hongming731) | ginobefun | 12192 | 0.1 天前 | 2.4 | P2 | ginobefun |
| [@bozhou_ai](https://x.com/bozhou_ai) | 泊舟 | 29988 | 0.8 天前 | 1.0 | P2 | 泊舟 |
| [@tuturetom](https://x.com/tuturetom) | Tom Huang | 39804 | 9.9 天前 | 1.2 | P2 | Tom Huang，10 天没发帖 |

### X · 实测不可用、不要加

| 用户名 | 结论 |
|---|---|
| windsurf | 已退役，改关注 @cognition |
| lmarena_ai | 空壳账号，用 @arena |
| xai | 空账号，xAI 用 @SpaceXAI |
| GroqInc | 空壳 |
| bfl_ml | 非官方 |
| vidu_ai / Shengshu_ai | 空壳/不活跃，生数没有可用的官方 X |
| doubao_AI | 非官方，5 个粉丝 |
| BytedanceTalk | 5 年没发帖 |
| 01AI_Yi | 682 天没发帖 |
| Ali_TongyiLab | 78 天没发帖 |
| cognition_labs | 旧账号 |
| modal_labs | 已被封号，现为 @modal |
| elevenlabsio / lovable_dev / OpenRouterAI / ManusAI_HQ / Teknium1 | 不存在，正确用户名分别是 @ElevenLabs @Lovable @OpenRouter @ManusAI @Teknium |
| TencentYuanbao / NotebookLM / ChatGPTapp / xiaohuggg / chatgpt21 / slow_developer / indigo11 / ai_explained_ / heygen_com | X API 查无此号 |
| WaytoAGI / nash_su / fuxiangPro / JefferyTatsuya / nearcyan / OriolVinyalsML | 27 天以上没发帖 |
| LIYuanhang | 不是目标人物 |

### 网站/API · API/产品 Release Notes（抢先关键）

| 名称 | URL | 实测 | 优先级 | 说明 |
|---|---|---|---|---|
| Claude Platform release notes | <https://platform.claude.com/docs/en/release-notes/feed.xml> | ✅ RSS 152 条，最新 10-09 | P0 | 官方 RSS，按日更新（最新 10-09），从 docs 页的 <link rel=alternate> 发现 |
| OpenAI API changelog | <https://platform.openai.com/docs/changelog> | 🛠 需爬取 HTTP 200 | P0 | HTML，页面有约 30 个日期，需爬取；页面声明的 developers.openai.com/rss.xml 实测停在 2026-05，不能用 |
| Gemini API changelog | <https://ai.google.dev/gemini-api/docs/changelog> | 🛠 需爬取 HTTP 200 | P0 | HTML，静态页有日期，需爬取 |
| Claude apps release notes | <https://support.claude.com/en/articles/12138966-release-notes> | 🛠 需爬取 HTTP 200 | P1 | HTML，需爬取 |
| Claude Code CHANGELOG.md | <https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md> | ✅ HTTP 200，418 条 | P1 | 纯文本，按版本号 diff；与 releases.atom 二选一 |
| Cursor changelog RSS | <https://cursor.com/changelog/rss.xml> | ✅ HTTP 200，50 条，最新 2026-10-06 | P1 | 50 条，最新 10-06 |
| ChatGPT release notes | <https://help.openai.com/en/articles/6825453-chatgpt-release-notes> | ⛔ HTTP 403 | — | 403 被拦截，暂不可用 |

### 网站/API · GitHub Releases（Atom，均已实测）

| 名称 | URL | 实测 | 优先级 | 说明 |
|---|---|---|---|---|
| GitHub Releases anthropics/claude-code | <https://github.com/anthropics/claude-code/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-09 | P0 |  |
| GitHub Releases openai/codex | <https://github.com/openai/codex/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-10 | P0 |  |
| GitHub Releases google-gemini/gemini-cli | <https://github.com/google-gemini/gemini-cli/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-10 | P1 |  |
| GitHub Releases vllm-project/vllm | <https://github.com/vllm-project/vllm/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-09 | P1 |  |
| GitHub Releases ggml-org/llama.cpp | <https://github.com/ggml-org/llama.cpp/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-10 | P1 |  |
| GitHub Releases ollama/ollama | <https://github.com/ollama/ollama/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-09 | P1 |  |
| GitHub Releases huggingface/transformers | <https://github.com/huggingface/transformers/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-06 | P1 |  |
| GitHub Releases openai/openai-python | <https://github.com/openai/openai-python/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-09 | P1 |  |
| GitHub Releases anthropics/anthropic-sdk-python | <https://github.com/anthropics/anthropic-sdk-python/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-09 | P1 |  |
| GitHub Releases modelcontextprotocol/python-sdk | <https://github.com/modelcontextprotocol/python-sdk/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-02 | P1 |  |
| GitHub Releases sgl-project/sglang | <https://github.com/sgl-project/sglang/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-02 | P2 |  |
| GitHub Releases langchain-ai/langchain | <https://github.com/langchain-ai/langchain/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-08 | P2 |  |
| GitHub Releases huggingface/diffusers | <https://github.com/huggingface/diffusers/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-06 | P2 |  |
| GitHub Releases comfyanonymous/ComfyUI | <https://github.com/comfyanonymous/ComfyUI/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-07 | P2 |  |
| GitHub Releases pytorch/pytorch | <https://github.com/pytorch/pytorch/releases.atom> | ✅ HTTP 200，10 条，最新 2026-10-10 | P2 |  |
| GitHub Releases open-webui/open-webui | <https://github.com/open-webui/open-webui/releases.atom> | ✅ HTTP 200，10 条，最新 2026-09-21 | P2 |  |
| MCP 规范仓库 modelcontextprotocol/modelcontextprotocol | <https://github.com/modelcontextprotocol/modelcontextprotocol/releases.atom> | ⚠️ 停更 HTTP 200，9 条，最新 2026-07-28 | P2 | 可用但很少发版（最新 07-28） |

### 网站/API · 模型榜单与发布

| 名称 | URL | 实测 | 优先级 | 说明 |
|---|---|---|---|---|
| OpenRouter models API | <https://openrouter.ai/api/v1/models> | ✅ HTTP 200，458 条，最新 2026-10-08 | P0 | JSON，458 个模型，带 created 时间戳；按 id diff 就能发现新模型上线 |
| HF trending models API | <https://huggingface.co/api/models?sort=trendingScore&limit=20> | ✅ HTTP 200，20 条，最新 2026-10-09 | P1 | JSON |
| HF trending spaces API | <https://huggingface.co/api/spaces?sort=trendingScore&limit=20> | ✅ HTTP 200，20 条，最新 2026-10-06 | P2 | JSON |
| HF newest models API | <https://huggingface.co/api/models?sort=createdAt&limit=20> | ✅ HTTP 200，20 条，最新 2026-10-10 | — | 量太大没法用，只适合按机构过滤（如 author=deepseek-ai） |
| LMArena leaderboard | <https://lmarena.ai/leaderboard> | 🛠 需爬取 HTTP 200 | P2 | HTML，需爬取；用 @arena 更快 |
| Artificial Analysis | <https://artificialanalysis.ai/> | 🛠 需爬取 HTTP 200 | P2 | HTML，JS 渲染；用 @ArtificialAnlys |

### 网站/API · 社区与产品

| 名称 | URL | 实测 | 优先级 | 说明 |
|---|---|---|---|---|
| hnrss newest points>=100 | <https://hnrss.org/newest?points=100> | ✅ HTTP 200，20 条，最新 2026-10-10 | P1 | 替代 HN 官方首页源；AI 关键词版继续用已有的 hnrss（points=50）。带长关键词串的 points=100 查询实测返回 502 |
| Product Hunt feed | <https://www.producthunt.com/feed?category=artificial-intelligence> | ✅ HTTP 200，50 条，最新 2026-10-10 | P2 | 50 条；category 参数的效果不确定（和全站源只重合 25/50），需自己加 AI 关键词过滤 |
| GitHubTrendingRSS daily（第三方） | <https://mshibanami.github.io/GitHubTrendingRSS/daily/all.xml> | ✅ HTTP 200，11 条 | P2 | 第三方生成，条目无日期；官方 trending 页需爬取 |

### 网站/API · 海外媒体

| 名称 | URL | 实测 | 优先级 | 说明 |
|---|---|---|---|---|
| Techmeme | <https://www.techmeme.com/feed.xml> | ✅ HTTP 200，15 条，最新 2026-10-10 | P0 | 科技头条聚合，速度快 |
| The Information | <https://www.theinformation.com/feed> | ✅ HTTP 200，20 条，最新 2026-10-10 | P1 | 付费墙，RSS 只有标题和摘要，但独家多 |
| Bloomberg Technology | <https://feeds.bloomberg.com/technology/news.rss> | ✅ HTTP 200，20 条，最新 2026-10-10 | P1 | 付费墙，有标题 |
| FT Technology | <https://www.ft.com/technology?format=rss> | ✅ HTTP 200，25 条，最新 2026-10-10 | P2 | 付费墙 |
| CNBC Technology | <https://www.cnbc.com/id/19854910/device/rss/rss.html> | ✅ HTTP 200，30 条，最新 2026-10-10 | P2 |  |
| Axios | <https://api.axios.com/feed/> | ✅ HTTP 200，100 条，最新 2026-10-10 | P2 | 全站 100 条，需关键词过滤 |
| Stratechery | <https://stratechery.com/feed/> | ✅ HTTP 200，10 条，最新 2026-10-09 | P2 |  |
| Platformer | <https://www.platformer.news/rss/> | ✅ HTTP 200，15 条，最新 2026-10-09 | P2 |  |
| 404 Media | <https://www.404media.co/rss/> | ✅ HTTP 200，15 条，最新 2026-10-09 | P2 |  |
| Reuters AI | <https://www.reuters.com/technology/artificial-intelligence/> | ⛔ HTTP 401 | — | 401 验证码拦截，不可用 |

### 网站/API · 状态页（只保留 AI 厂商的）

| 名称 | URL | 实测 | 优先级 | 说明 |
|---|---|---|---|---|
| OpenAI Status | <https://status.openai.com/history.rss> | ✅ HTTP 200，112 条，最新 2026-10-10 | P2 | 112 条；故障可以写成「宕机」类新闻 |
| Claude Status | <https://status.claude.com/history.rss> | ✅ HTTP 200，25 条，最新 2026-10-07 | P2 | status.anthropic.com/history.rss 内容相同，只留一个 |
| Google AI Studio status | <https://aistudio.google.com/status> | 🛠 需爬取 HTTP 200 | — | JS 渲染 |

### 网站/API · 国内

| 名称 | URL | 实测 | 优先级 | 说明 |
|---|---|---|---|---|
| Readhub | <https://readhub.cn/> | 🛠 需爬取 HTTP 200 | P1 | HTML，静态页有约 34 个日期，可以爬；科技热点聚合，速度快 |
| 新智元 / AIbase / 36氪 AI 频道 | <aiera.com.cn / aibase.com/zh/news / 36kr.com/information/AI/> | 🛠 需爬取 | P1 | 都是 JS 渲染或没有 feed，需要无头浏览器（36氪 RSS 已被拦截） |
| wechat2rss（第三方公众号转 RSS） | <https://wechat2rss.xlab.app/> | 🛠 需爬取 HTTP 200 | — | 首页 JS 渲染，没法静态发现 feed 地址；公众号建议用自建方案（见第 4 节） |
| 即刻 web | <https://web.okjike.com/> | 🛠 需爬取 HTTP 200 | — | JS 渲染且需要登录 |

## 3. 优先级分层

### P0（最快：X 每 1-2 分钟、feed/API 每 3-5 分钟）

- 爆料/速报 X：@testingcatalog @btibor91 @AiBreakfast @apples_jimmy @ArtificialAnlys @arena
- 头部官方 X：@OpenAI @OpenAIDevs（建议重新启用，见下）@AnthropicAI @claudeai @GoogleDeepMind @GoogleAI @GoogleAIStudio @OfficialLoganK @sama @SpaceXAI @deepseek_ai @Alibaba_Qwen @Kimi_Moonshot @Zai_org @MiniMax_AI @Kling_ai @cursor_ai @bcherny @_akhaliq
- 中文：@dotey @op7418、机器之心 API、量子位 RSS
- 官方 feed：OpenAI News、Claude Platform release notes、OpenAI API changelog（爬取）、Gemini API changelog（爬取）、Google Blog - Gemini
- 发现类：OpenRouter models API、HF Daily Papers、Techmeme、hnrss AI 关键词、GitHub Releases（claude-code、codex）

### P1（10 分钟）

- 海外媒体：TechCrunch AI、The Verge AI、The Decoder、Ars Technica AI、MIT Technology Review AI、WIRED AI、MarkTechPost、The Information、Bloomberg
- 社区/博客：Simon Willison、Latent Space、hnrss points≥100、HN Algolia API
- 国内：极客公园、InfoQ 中国、IT之家（加过滤）、雷峰网 AI 频道、Readhub（爬取）
- X：国内其余实验室（@StepFun_ai @TencentHunyuan @Baidu_Inc @Hailuo_AI @TencentAI_News @OpenBMB）；产品号（@cognition @Replit @Lovable @v0 @runwayml @midjourney @ElevenLabs @suno @ManusAI @LumaLabsAI @Figure_robot @physical_int @thinkymachines）；研究者/开发者（@karpathy @DrJimFan @JustinLin610 @teortaxesTex @danielhanchen @ggerganov @_philschmid @reach_vb @hwchase17 @amasad @simonw @swyx @natolambert @emollick）；速报二梯队（@kimmonismus @scaling01 @flowersslop @ai_for_success @petergostev @Yuchenj_UW @TheRundownAI @OpenRouter）；记者（@markgurman @kyliebytes @steph_palazzolo）；中文博主（@oran_ge @vista8 @Gorden_Sun @shao__meng）
- GitHub Releases：gemini-cli、vllm、llama.cpp、ollama、transformers、openai-python、anthropic-sdk-python、MCP python-sdk；Cursor changelog；Claude apps release notes；HF trending models

### P2（30-60 分钟或每天一次）

- Newsletter/深度：Import AI、Interconnects、SemiAnalysis、Ben's Bites、Last Week in AI、One Useful Thing、Ahead of AI、Marcus on AI、Stratechery、Platformer、阮一峰周刊
- 研究博客：Google Research、Microsoft Research、Apple ML、NVIDIA Technical/Blog、Hugging Face Blog、Mistral News、Amazon Science、BAIR、Lil'Log、arXiv cs.AI/cs.CL（仅作检索库）
- 保留的基础设施：Vercel News、Cloudflare Blog、Google Cloud Blog - AI & ML、AWS ML Blog（加过滤）、@github @code @googlecloud @CloudflareDev @Docker
- 其余 X：噪声大的聚合号（@AndrewCurran_ @rohanpaul_ai @AYi_AInotes @omarsar0）、传闻号（@iruletheworldmo）、其余中文博主和产品号
- OpenAI/Claude 状态页、Product Hunt、GitHubTrendingRSS、FT/CNBC/Axios、其他 GitHub Releases

实现建议：在 `sources` 表加 `priority` 字段，对应轮询间隔 P0=60-120s（X）/180-300s（feed），P1=600s，P2=1800-3600s；P0 来源的新条目才推「新内容」提醒。
X 账号多了以后，可以把 P1/P2 账号建成**公开 X List**，用 syndication 端点一次读一个列表（单个列表约 30 次/15 分钟），减少 FxEmbed 的调用量；P0 账号仍单独轮询。

## 4. 无法自动化（或不建议自动化）的渠道

| 渠道 | 价值 | 现状 | 建议 |
|---|---|---|---|
| 微信公众号（机器之心、量子位、新智元、APPSO、甲子光年、AI科技评论、夕小瑶、赛博禅心、数字生命卡兹克 等） | 国内首发主阵地 | 没有官方 feed；第三方 wechat2rss 首页是 JS 渲染，无法静态发现地址 | 机器之心、量子位已有网站 API/RSS；其余可自建 WeRSS / wechat2rss（需要微信读书账号，有封号风险，属第三方），或手动订阅；新智元网站需无头浏览器 |
| 即刻（AI 探索站等圈子） | 中文从业者第一手讨论 | web 端 JS 渲染且需要登录 | 手机 App 关注，人工看 |
| Discord（OpenAI/Anthropic/Midjourney/各开源项目服务器） | 灰度消息、发版公告 | 机器人需要管理员授权；用自己账号自动化违反服务条款 | 关注 announcements 频道，开手机通知 |
| Reddit r/LocalLLaMA、r/MachineLearning、r/singularity | 开源模型首发、爆料 | 本机 IP 被 403 拦截 | 申请 Reddit OAuth API（免费额度）后再接入，或换出口 IP |
| Telegram 频道 | 中文 AI 快讯聚合 | 需要 Telegram API（需手机号注册） | 可选：用 Telethon 接入自己账号 |
| 小红书 / B 站 / 抖音 / 视频号 | 选题风向、观众反馈 | 反爬严，没有 API | 人工看 |
| The Information / Bloomberg / FT 全文 | 独家 | 付费墙 | RSS 只拿标题作线索，需要时付费订阅 |
| Reuters | 硬新闻 | 401 验证码拦截 | 走 Techmeme 或 Google News 转引 |
| LinkedIn、播客、YouTube 直播（发布会） | 发布会第一时间 | 没有稳定 API | YouTube 频道 RSS 可以做（下一轮再验证），发布会直播人工看 |

