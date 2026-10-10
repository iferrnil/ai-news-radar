# X 账号 vs 官网/博客/changelog：实测覆盖对比

> 数据抓取：2026-10-10 16:34（北京时间）；脚本 `scripts/x_vs_blog.py`（原始数据 `docs/x_vs_blog_data.json`，结果 `docs/x_vs_blog.json`），应用 `scripts/apply_x_vs_blog.py`。

## 方法

- **X 侧**：FxEmbed 时间线（每账号最多 10 页、自动重试），同一组织多个账号合并去重；FxEmbed 已把 t.co 展开，其余短链（bit.ly、goo.gle 等）跟随重定向解析；自回复串（thread）合并为一条。
- **网站侧**：该组织的博客/changelog/状态页 feed（HTML 列表页则解析列表；无日期的列表页取前 20 条）。
- **窗口**：最近 60 天，若 X 可回溯不足 60 天则取 X 最早帖子至今。
- **匹配**：网站条目链接出现在帖子（含整串 thread、引用帖）的展开链接中，或标题与帖子文本高度相似（英文词重合 ≥60% / 中文二元组重合 ≥50%）。
- **X 独有实质内容**：组织自己账号发的、非回复的帖子（thread 只算首帖），正文 ≥80 字符或带外链，且未匹配任何网站条目。
- **判定**：U = 网站条目数 + X 独有实质帖数。X 覆盖网站 ≥90% 且有额外内容 → 保留 X；去掉任一侧都会丢失 ≥30% 的 U → 两边都保留；否则保留丢失更少的一侧（丢网站=网站未上X的条目，丢X=X独有帖）。
- 网站是全文（博客/changelog 有完整细节），X 帖多为简短预告+链接；被停用的一侧在 `sources.yaml` 中标记 `enabled: false` + `disabled_reason`，未删除。

## 结论汇总：保留 X 26 个，保留网站 8 个，两边都保留 15 个（共 49 个组织）

### 保留 X（26）

| 组织 | X 账号 | 网站来源 | 窗口(天) | 网站条目 | X覆盖率 | X独有实质帖 | 决定 | 理由 | 数据不足 |
|---|---|---|---|---|---|---|---|---|---|
| Anthropic | @AnthropicAI, @claudeai | Anthropic News<br>Anthropic Research<br>Anthropic Engineering | 60.0 | 36 | 67% | 35 | 保留 X | X 覆盖网站 67%；去掉网站只丢 12/71（Anthropic News:5；Anthropic Research:6；Anthropic Engineering:1），去掉 X 会丢 35 | 网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较 |
| Google DeepMind | @GoogleDeepMind | Google DeepMind Blog | 60.0 | 18 | 78% | 12 | 保留 X | X 覆盖网站 78%；去掉网站只丢 4/30（Google DeepMind Blog:4），去掉 X 会丢 12 |  |
| Cohere | @cohere | Cohere Blog | 60.0 | 20 | 60% | 40 | 保留 X | X 覆盖网站 60%；去掉网站只丢 8/60（Cohere Blog:8），去掉 X 会丢 40 | 网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较 |
| Stability AI | @StabilityAI | Stability AI News | 60.0 | 0 | — | 5 | 保留 X | 网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取 | 网站侧无法获取条目：Stability AI News |
| Together AI | @togethercompute | Together AI Blog | 60.0 | 16 | 88% | 67 | 保留 X | X 覆盖网站 88%；去掉网站只丢 2/83（Together AI Blog:2），去掉 X 会丢 67 |  |
| Ai2 | @allen_ai | Ai2 (Allen Institute) Blog | 60.0 | 14 | 100% | 16 | 保留 X | X 覆盖网站 100% 条目，另有 16 条网站没有的内容 |  |
| Perplexity | @perplexity_ai | Perplexity Hub | 60.0 | 0 | — | 33 | 保留 X | 网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取 | 网站侧无法获取条目：Perplexity Hub |
| DeepSeek | @deepseek_ai | DeepSeek 新闻<br>DeepSeek API Docs 新闻 | 60.0 | 2 | 50% | 4 | 保留 X | X 覆盖网站 50%；去掉网站只丢 1/6（DeepSeek API Docs 新闻:1），去掉 X 会丢 4 |  |
| Qwen | @Alibaba_Qwen | Qwen Blog (qwenlm.github.io)<br>Qwen Research (qwen.ai) | 60.0 | 0 | — | 64 | 保留 X | 窗口内网站无新条目，X 有 64 条原创 | 网站侧无法获取条目：Qwen Research (qwen.ai) |
| 智谱 Z.ai | @Zai_org | 智谱 Z.ai 新闻 | 60.0 | 0 | — | 8 | 保留 X | 网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取 | 网站侧无法获取条目：智谱 Z.ai 新闻 |
| MiniMax | @MiniMax_AI | MiniMax 新闻 | 60.0 | 1 | 100% | 49 | 保留 X | X 覆盖网站 100% 条目，另有 49 条网站没有的内容 | 网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较 |
| 阶跃星辰 | @StepFun_ai | 阶跃星辰 StepFun | 60.0 | 0 | — | 18 | 保留 X | 网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取 | 网站侧无法获取条目：阶跃星辰 StepFun |
| 腾讯混元 | @TencentHunyuan | 腾讯混元 | 60.0 | 0 | — | 16 | 保留 X | 网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取 | 网站侧无法获取条目：腾讯混元 |
| 百度 | @Baidu_Inc | 百度文心 Blog | 60.0 | 0 | — | 30 | 保留 X | 网站侧无法静态获取条目（JS 渲染/被拦截），X 账号活跃且可直接抓取 | 网站侧无法获取条目：百度文心 Blog |
| Next.js | @nextjs | Next.js Blog<br>Next.js Releases (GitHub) | 60.0 | 22 | 100% | 2 | 保留 X | X 覆盖网站 100% 条目，另有 2 条网站没有的内容 |  |
| GitHub | @github, @githubstatus | GitHub Blog<br>GitHub Changelog<br>GitHub Status | 60.0 | 45 | 76% | 268 | 保留 X | X 覆盖网站 76%；去掉网站只丢 11/313（GitHub Blog:4；GitHub Changelog:6；GitHub Status:1），去掉 X 会丢 268 |  |
| Netlify | @Netlify | Netlify Blog<br>Netlify Changelog<br>Netlify Status | 58.6 | 53 | 66% | 90 | 保留 X | X 覆盖网站 66%；去掉网站只丢 18/143（Netlify Changelog:10；Netlify Status:8），去掉 X 会丢 90 |  |
| Railway | @Railway | Railway Blog<br>Railway Changelog | 60.0 | 18 | 67% | 26 | 保留 X | X 覆盖网站 67%；去掉网站只丢 6/44（Railway Blog:6），去掉 X 会丢 26 |  |
| Neon | @neondatabase | Neon Blog<br>Neon Changelog<br>Neon Status | 60.0 | 42 | 95% | 17 | 保留 X | X 覆盖网站 95% 条目，另有 17 条网站没有的内容 | 网站侧无法获取条目：Neon Status |
| PlanetScale | @PlanetScale | PlanetScale Blog<br>PlanetScale Changelog<br>PlanetScale Status | 60.0 | 44 | 52% | 36 | 保留 X | X 覆盖网站 52%；去掉网站只丢 21/80（PlanetScale Changelog:17；PlanetScale Status:4），去掉 X 会丢 36 | 网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较 |
| Bun | @bunjavascript | Bun Blog<br>Bun Releases (GitHub) | 60.0 | 8 | 100% | 23 | 保留 X | X 覆盖网站 100% 条目，另有 23 条网站没有的内容 |  |
| Docker | @Docker | Docker Blog | 60.0 | 10 | 100% | 94 | 保留 X | X 覆盖网站 100% 条目，另有 94 条网站没有的内容 |  |
| Kubernetes | @kubernetesio | Kubernetes Blog<br>Kubernetes Releases (GitHub) | 60.0 | 31 | 68% | 31 | 保留 X | X 覆盖网站 68%；去掉网站只丢 10/62（Kubernetes Releases (GitHub):10），去掉 X 会丢 31 |  |
| HashiCorp | @HashiCorp | HashiCorp Blog | 60.0 | 14 | 79% | 47 | 保留 X | X 覆盖网站 79%；去掉网站只丢 3/61（HashiCorp Blog:3），去掉 X 会丢 47 |  |
| Upstash | @upstash | Upstash Blog | 60.0 | 30 | 67% | 18 | 保留 X | X 覆盖网站 67%；去掉网站只丢 10/48（Upstash Blog:10），去掉 X 会丢 18 |  |
| Linear | @linear | Linear Changelog | 60.0 | 6 | 100% | 29 | 保留 X | X 覆盖网站 100% 条目，另有 29 条网站没有的内容 |  |

### 保留网站（8）

| 组织 | X 账号 | 网站来源 | 窗口(天) | 网站条目 | X覆盖率 | X独有实质帖 | 决定 | 理由 | 数据不足 |
|---|---|---|---|---|---|---|---|---|---|
| OpenAI | @OpenAI, @OpenAIDevs | OpenAI News | 60.0 | 131 | 38% | 45 | 保留网站 | 网站有 81 条 X 未发（OpenAI News:81，全文/changelog）；X 额外内容仅 45/176 |  |
| Mistral AI | @MistralAI | Mistral AI News | 60.0 | 9 | 56% | 3 | 保留网站 | 网站有 4 条 X 未发（Mistral AI News:4，全文/changelog）；X 额外内容仅 3/12 |  |
| Hugging Face | @huggingface | Hugging Face Blog | 41.6 | 24 | 38% | 5 | 保留网站 | 网站有 15 条 X 未发（Hugging Face Blog:15，全文/changelog）；X 额外内容仅 5/29 |  |
| Vercel | @vercel, @vercel_dev | Vercel News (blog + changelog)<br>Vercel Status | 60.0 | 212 | 76% | 20 | 保留网站 | 网站有 50 条 X 未发（Vercel News (blog + changelog):30；Vercel Status:20，全文/changelog）；X 额外内容仅 20/232 |  |
| Deno | @deno_land | Deno Blog<br>Deno Releases (GitHub) | 60.0 | 3 | 67% | 0 | 保留网站 | 网站有 1 条 X 未发（Deno Blog:1，全文/changelog）；X 额外内容仅 0/3 |  |
| Node.js | @nodejs | Node.js Blog | 60.0 | 11 | 27% | 2 | 保留网站 | 网站有 8 条 X 未发（Node.js Blog:8，全文/changelog）；X 额外内容仅 2/13 |  |
| React | @reactjs | React Blog<br>React Releases (GitHub) | 60.0 | 3 | 33% | 0 | 保留网站 | 网站有 2 条 X 未发（React Releases (GitHub):2，全文/changelog）；X 额外内容仅 0/3 |  |
| Turso | @tursodatabase | Turso Blog | 60.0 | 20 | 40% | 4 | 保留网站 | 网站有 12 条 X 未发（Turso Blog:12，全文/changelog）；X 额外内容仅 4/24 | 网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较 |

### 两边都保留（15）

| 组织 | X 账号 | 网站来源 | 窗口(天) | 网站条目 | X覆盖率 | X独有实质帖 | 决定 | 理由 | 数据不足 |
|---|---|---|---|---|---|---|---|---|---|
| Google AI / Gemini | @GoogleAI, @GeminiApp | Google Research Blog<br>Google Blog - AI<br>Google Blog - Gemini | 60.0 | 62 | 26% | 37 | 两边都保留 | 去掉网站会丢 46/99（46%，Google Research Blog:21；Google Blog - AI:17；Google Blog - Gemini:8，全文），去掉 X 会丢 37/99（37%） |  |
| Meta AI | @AIatMeta | Meta AI Blog | 60.0 | 10 | 30% | 9 | 两边都保留 | 去掉网站会丢 7/19（37%，Meta AI Blog:7，全文），去掉 X 会丢 9/19（47%） | 网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较 |
| xAI | @SpaceXAI | xAI News | 60.0 | 17 | 53% | 9 | 两边都保留 | 去掉网站会丢 8/26（31%，xAI News:8，全文），去掉 X 会丢 9/26（35%） |  |
| NVIDIA | @nvidia, @NVIDIAAI | NVIDIA Blog<br>NVIDIA Blog - Generative AI<br>NVIDIA Technical Blog | 60.0 | 122 | 33% | 82 | 两边都保留 | 去掉网站会丢 82/204（40%，NVIDIA Blog:11；NVIDIA Blog - Generative AI:8；NVIDIA Technical Blog:63，全文），去掉 X 会丢 82/204（40%） |  |
| Kimi / Moonshot | @Kimi_Moonshot | Kimi Blog (Moonshot AI) | 60.0 | 9 | 0% | 4 | 两边都保留 | 去掉网站会丢 9/13（69%，Kimi Blog (Moonshot AI):9，全文），去掉 X 会丢 4/13（31%） | 网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较 |
| AWS | @awscloud, @awsdevelopers | AWS Machine Learning Blog<br>AWS News Blog<br>AWS What's New | 60.0 | 140 | 6% | 85 | 两边都保留 | 去掉网站会丢 132/225（59%，AWS Machine Learning Blog:20；AWS News Blog:16；AWS What's New:96，全文），去掉 X 会丢 85/225（38%） |  |
| Google Cloud | @googlecloud | Google Cloud Blog (all)<br>Google Cloud Blog - AI & ML<br>Google Cloud Release Notes<br>Google Cloud Service Health | 60.0 | 42 | 24% | 87 | 两边都保留 | 用户手动覆盖（自动判定为 keep_x）：网站全文不在 X 上的条目较多，两边都保留；原依据：X 覆盖网站 24%；去掉网站只丢 32/129（Google Cloud Blog (all):16；Google Cloud Blog - AI & ML:14；Google Cloud Release Notes:1；Google Cloud Service Health:1），去掉 X 会丢 87 |  |
| Microsoft Azure | @Azure | Microsoft Azure Blog<br>Azure Updates<br>Microsoft AI Blog (Cloud Blog) | 60.0 | 17 | 47% | 30 | 两边都保留 | 用户手动覆盖（自动判定为 keep_x）：网站全文不在 X 上的条目较多，两边都保留；原依据：X 覆盖网站 47%；去掉网站只丢 9/47（Microsoft Azure Blog:3；Azure Updates:1；Microsoft AI Blog (Cloud Blog):5），去掉 X 会丢 30 |  |
| Cloudflare | @Cloudflare, @CloudflareDev | Cloudflare Blog<br>Cloudflare Changelog<br>Cloudflare Status | 60.0 | 231 | 21% | 169 | 两边都保留 | 去掉网站会丢 183/400（46%，Cloudflare Changelog:183，全文），去掉 X 会丢 169/400（42%） |  |
| Supabase | @supabase | Supabase Blog<br>Supabase Changelog<br>Supabase Status | 60.0 | 66 | 12% | 63 | 两边都保留 | 去掉网站会丢 58/129（45%，Supabase Blog:2；Supabase Changelog:19；Supabase Status:37，全文），去掉 X 会丢 63/129（49%） | 网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较 |
| Fly.io | @flydotio | Fly.io Blog<br>Fly.io Fresh Produce (changelog)<br>Fly.io Status | 60.0 | 48 | 0% | 46 | 两边都保留 | 去掉网站会丢 48/94（51%，Fly.io Blog:1；Fly.io Fresh Produce (changelog):8；Fly.io Status:39，全文），去掉 X 会丢 46/94（49%） |  |
| Render | @render | Render Blog<br>Render Changelog<br>Render Status | 60.0 | 26 | 46% | 24 | 两边都保留 | 用户手动覆盖（自动判定为 keep_x）：网站全文不在 X 上的条目较多，两边都保留；原依据：X 覆盖网站 46%；去掉网站只丢 14/50（Render Blog:2；Render Changelog:4；Render Status:8），去掉 X 会丢 24 |  |
| Stripe | @stripe, @StripeDev | Stripe Blog<br>Stripe API Changelog | 60.0 | 30 | 7% | 26 | 两边都保留 | 去掉网站会丢 28/56（50%，Stripe Blog:8；Stripe API Changelog:20，全文），去掉 X 会丢 26/56（46%） | 网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较 |
| Fastly | @fastly | Fastly Blog | 60.0 | 25 | 40% | 11 | 两边都保留 | 去掉网站会丢 15/36（42%，Fastly Blog:15，全文），去掉 X 会丢 11/36（31%） |  |
| DigitalOcean | @digitalocean | DigitalOcean Blog<br>DigitalOcean Release Notes<br>DigitalOcean Status | 60.0 | 83 | 46% | 67 | 两边都保留 | 去掉网站会丢 45/150（30%，DigitalOcean Blog:4；DigitalOcean Release Notes:26；DigitalOcean Status:15，全文），去掉 X 会丢 67/150（45%） |  |

## 数据不足/需人工复核的组织（17）

- **Anthropic**（保留 X）：网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较
- **Meta AI**（两边都保留）：网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较
- **Cohere**（保留 X）：网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较
- **Stability AI**（保留 X）：网站侧无法获取条目：Stability AI News
- **Perplexity**（保留 X）：网站侧无法获取条目：Perplexity Hub
- **Qwen**（保留 X）：网站侧无法获取条目：Qwen Research (qwen.ai)
- **Kimi / Moonshot**（两边都保留）：网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较
- **智谱 Z.ai**（保留 X）：网站侧无法获取条目：智谱 Z.ai 新闻
- **MiniMax**（保留 X）：网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较
- **阶跃星辰**（保留 X）：网站侧无法获取条目：阶跃星辰 StepFun
- **腾讯混元**（保留 X）：网站侧无法获取条目：腾讯混元
- **百度**（保留 X）：网站侧无法获取条目：百度文心 Blog
- **Supabase**（两边都保留）：网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较
- **Neon**（保留 X）：网站侧无法获取条目：Neon Status
- **PlanetScale**（保留 X）：网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较
- **Stripe**（两边都保留）：网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较
- **Turso**（保留网站）：网站列表页无日期，取列表前20条与已抓取的全部 X 帖比较

## 未受影响

- 人物账号、只有一侧的组织（如 Ollama、Cursor、LangChain、VS Code、TypeScript、Vite、Vue、Prisma、PostHog、Sentry、Astro、Microsoft AI 等只有 X；媒体/论文/社区/Chinese media 等只有网站）不参与比较。
- 某组织中不属于其官方博客体系的来源（如 Meta Newsroom/Engineering、Microsoft Research、Amazon Science、Google Developers Blog）未纳入比较，保持启用。

## 典型未覆盖示例

- **OpenAI**：网站未上X 例：[OpenAI News] Sophos cuts threat investigation time by 96% with OpenAI Day；[OpenAI News] Asana cuts model costs 76x in browser tests with GPT-6.1 Sol；[OpenAI News] How Oracle turns days of work into minutes with ChatGPT and ；X 独有例：https://x.com/OpenAI/status/2104987422308335828 https://x.com/OpenAI/status/2104986129686741046 https://x.com/OpenAI/status/2104976337316589726
- **Anthropic**：网站未上X 例：[Anthropic News] Oct 8, 2026 Announcements 2026 Usage Policy update；[Anthropic News] Oct 2, 2026 Announcements Anthropic invests $100 million to ；[Anthropic News] Oct 1, 2026 Announcements Barclays scales Claude to upgrade ；X 独有例：https://x.com/AnthropicAI/status/2102897863097545197 https://x.com/AnthropicAI/status/2098097512544444447 https://x.com/AnthropicAI/status/2097679796687769689
- **Google DeepMind**：网站未上X 例：[Google DeepMind Blog] Gemini 4 Argon: our next era of frontier intelligence；[Google DeepMind Blog] Advancing Private AI Compute with secure, server-side memory；[Google DeepMind Blog] Introducing WeatherNext 3, our most advanced and accurate gl；X 独有例：https://x.com/GoogleDeepMind/status/2107834249680499136 https://x.com/GoogleDeepMind/status/2105707085287567709 https://x.com/GoogleDeepMind/status/2105388084154056939
- **Google AI / Gemini**：网站未上X 例：[Google Research Blog] Does better work always mean better workers?；[Google Research Blog] Unlocking Earth AI’s planetary geospatial foundation models ；[Google Research Blog] Open and Emergent Problems in Agentic Privacy and Security: ；X 独有例：https://x.com/GoogleAI/status/2108550067724407281 https://x.com/GoogleAI/status/2107836750257107133 https://x.com/GoogleAI/status/2106049984164463069
- **Meta AI**：网站未上X 例：[Meta AI Blog] assistive robotics university of pittsburgh sam dino；[Meta AI Blog] genesis mission lawrence berkeley national laboratory segmen；[Meta AI Blog] brain2qwerty brain ai human communication；X 独有例：https://x.com/AIatMeta/status/2106099776035152231 https://x.com/AIatMeta/status/2102997291732766943 https://x.com/AIatMeta/status/2097406461118943599
- **Mistral AI**：网站未上X 例：[Mistral AI News] Hallo, Deutschland!；[Mistral AI News] Cloudera and Mistral Partner to Bring Specialized, Sovereign；[Mistral AI News] Modernizing complex legacy code with AI agents.；X 独有例：https://x.com/MistralAI/status/2107834891844899292 https://x.com/MistralAI/status/2100824200126529725 https://x.com/MistralAI/status/2095951508978209153
- **xAI**：网站未上X 例：[xAI News] Product · Sep 28, 2026 Team Bots: shared AI teammates that l；[xAI News] Product · Sep 22, 2026 How SpaceXAI is using Grok Bot to sca；[xAI News] Product · Sep 16, 2026 Memory in Grok Build；X 独有例：https://x.com/SpaceXAI/status/2102069815225586149 https://x.com/SpaceXAI/status/2101005248311726387 https://x.com/SpaceXAI/status/2100310550127464682
- **Hugging Face**：网站未上X 例：[Hugging Face Blog] Impactful scheduling for GPU clusters；[Hugging Face Blog] The model that didn't exist, so you made it yourself；[Hugging Face Blog] Introducing Falcon ASR；X 独有例：https://x.com/huggingface/status/2108252563015422389 https://x.com/huggingface/status/2106034221005312448 https://x.com/huggingface/status/2105683434035384360
- **NVIDIA**：网站未上X 例：[NVIDIA Blog] Into the Omniverse: How Developers Turn Ideas Into Simulatio；[NVIDIA Blog] Rally Up: ‘Gears of War: E-Day’ Launches on GeForce NOW；[NVIDIA Blog] NVIDIA, Microsoft Kick Off a New Beginning for Windows PCs W；X 独有例：https://x.com/nvidia/status/2108291409623314770 https://x.com/nvidia/status/2107892242807406899 https://x.com/nvidia/status/2106133259663089977
- **Cohere**：网站未上X 例：[Cohere Blog] The future of work debate has an evidence problem；[Cohere Blog] shared or dedicated inference for embed rerank；[Cohere Blog] building multilingual bridges；X 独有例：https://x.com/cohere/status/2108573097741992297 https://x.com/cohere/status/2108343817392853205 https://x.com/cohere/status/2108215310700462153
- **Stability AI**：网站未上X 例：—；X 独有例：https://x.com/StabilityAI/status/2108278193316630711 https://x.com/StabilityAI/status/2106148130731467187 https://x.com/StabilityAI/status/2099957905067839808
- **Together AI**：网站未上X 例：[Together AI Blog] How a global fintech scaled coding agent traffic with Dedica；[Together AI Blog] ；X 独有例：https://x.com/togethercompute/status/2108307877995917688 https://x.com/togethercompute/status/2107256154862809216 https://x.com/togethercompute/status/2107139875522879939
- **Ai2**：网站未上X 例：—；X 独有例：https://x.com/allen_ai/status/2108552280320737526 https://x.com/allen_ai/status/2107914576662614505 https://x.com/allen_ai/status/2107862550259884361
- **Perplexity**：网站未上X 例：—；X 独有例：https://x.com/perplexity_ai/status/2107866029745746177 https://x.com/perplexity_ai/status/2105692220280226296 https://x.com/perplexity_ai/status/2105373989262827915
- **DeepSeek**：网站未上X 例：[DeepSeek API Docs 新闻] News；X 独有例：https://x.com/deepseek_ai/status/2105915715241062644 https://x.com/deepseek_ai/status/2090730032574631962 https://x.com/deepseek_ai/status/2087887408440164663
- **Qwen**：网站未上X 例：—；X 独有例：https://x.com/Alibaba_Qwen/status/2108549075218120949 https://x.com/Alibaba_Qwen/status/2108401307577090259 https://x.com/Alibaba_Qwen/status/2105256053403508849
- **Kimi / Moonshot**：网站未上X 例：[Kimi Blog (Moonshot AI)] kimi k3；[Kimi Blog (Moonshot AI)] PerceptionBench；[Kimi Blog (Moonshot AI)] kimi k2 6；X 独有例：https://x.com/Kimi_Moonshot/status/2102372557190230244 https://x.com/Kimi_Moonshot/status/2102244258531213596 https://x.com/Kimi_Moonshot/status/2097636366544937067
- **智谱 Z.ai**：网站未上X 例：—；X 独有例：https://x.com/Zai_org/status/2107272032073437185 https://x.com/Zai_org/status/2094769612730532172 https://x.com/Zai_org/status/2093354097122455713
- **MiniMax**：网站未上X 例：—；X 独有例：https://x.com/MiniMax_AI/status/2108793182968504771 https://x.com/MiniMax_AI/status/2108339348584448365 https://x.com/MiniMax_AI/status/2107604812540072143
- **阶跃星辰**：网站未上X 例：—；X 独有例：https://x.com/StepFun_ai/status/2108182763002278158 https://x.com/StepFun_ai/status/2108671660840894652 https://x.com/StepFun_ai/status/2108618943468630483
- **腾讯混元**：网站未上X 例：—；X 独有例：https://x.com/TencentHunyuan/status/2108386202080657757 https://x.com/TencentHunyuan/status/2105288808929558891 https://x.com/TencentHunyuan/status/2103020524192542766
- **百度**：网站未上X 例：—；X 独有例：https://x.com/Baidu_Inc/status/2107833323846590893 https://x.com/Baidu_Inc/status/2107108546089525600 https://x.com/Baidu_Inc/status/2105659003821011288
- **AWS**：网站未上X 例：[AWS Machine Learning Blog] ICYMI: What landed for AI builders in September 2026；[AWS Machine Learning Blog] How Postman runs Agent Mode for 40 million developers on Ama；[AWS Machine Learning Blog] Pay-per-inference for AI agents: How BlockRun and Incarna us；X 独有例：https://x.com/awscloud/status/2108498239439643004 https://x.com/awscloud/status/2108291558902485153 https://x.com/awscloud/status/2108291324625482054
- **Google Cloud**：网站未上X 例：[Google Cloud Blog (all)] Modernizing Unstructured Data Workflows: Alteryx Live Query ；[Google Cloud Blog (all)] Innovation in Ireland: How Irish brands scale with Gemini En；[Google Cloud Blog (all)] Empowering SMBs to do more with Gemini；X 独有例：https://x.com/googlecloud/status/2108266977366081905 https://x.com/googlecloud/status/2108266949356449855 https://x.com/googlecloud/status/2108265293319467324
- **Microsoft Azure**：网站未上X 例：[Microsoft Azure Blog] AI transformation across the infrastructure lifecycle: From ；[Microsoft Azure Blog] SQL Server on Azure Local is now generally available；[Microsoft Azure Blog] Enhancing Microsoft Azure Virtual Machine lifecycle；X 独有例：https://x.com/Azure/status/2108588298541097159 https://x.com/Azure/status/2108256108921950248 https://x.com/Azure/status/2107863523409748215
- **Vercel**：网站未上X 例：[Vercel News (blog + changelog)] New Pro teams now default to 30-day deployment retention；[Vercel News (blog + changelog)] Deployment Storage billing begins for existing Pro teams；[Vercel News (blog + changelog)] Skip sending request bodies to Routing Middleware；X 独有例：https://x.com/vercel/status/2093364921073549667 https://x.com/vercel/status/2092999180780556643 https://x.com/vercel_dev/status/2090857388924661915
- **Next.js**：网站未上X 例：—；X 独有例：https://x.com/nextjs/status/2102516295715741912 https://x.com/nextjs/status/2102188275578642622
- **Cloudflare**：网站未上X 例：[Cloudflare Changelog] Agents - Cloudflare API MCP server serves Cloudflare skills；[Cloudflare Changelog] Cloudflare Fundamentals - Improved HTTP/3 client cancellatio；[Cloudflare Changelog] Cloudflare Fundamentals - More efficient Markdown for Agents；X 独有例：https://x.com/Cloudflare/status/2108676633951654302 https://x.com/Cloudflare/status/2108573989920481532 https://x.com/Cloudflare/status/2108567369622692225
- **Supabase**：网站未上X 例：[Supabase Blog] Operate with confidence；[Supabase Blog] Build anything: Supabase from code, and an MCP server for yo；[Supabase Changelog] Status page moves to incident.io；X 独有例：https://x.com/supabase/status/2108694001876382155 https://x.com/supabase/status/2108580939173372142 https://x.com/supabase/status/2108543185714585609
- **GitHub**：网站未上X 例：[GitHub Blog] How one bug bounty researcher chooses the features they inve；[GitHub Blog] AI is changing developer work. Here are three skills to stre；[GitHub Blog] Developer policy update: Transparency, state policy, and wha；X 独有例：https://x.com/github/status/2108687734734827803 https://x.com/github/status/2108212456388448383 https://x.com/github/status/2107896916595884177
- **Netlify**：网站未上X 例：[Netlify Changelog] Know who is visiting your protected site, right from your fu；[Netlify Changelog] Subscribe to Netlify events from ChatGPT；[Netlify Changelog] Comment on specific lines of code in Agent Runners；X 独有例：https://x.com/Netlify/status/2108235783899423192 https://x.com/Netlify/status/2107896840092013050 https://x.com/Netlify/status/2107854331844169902
- **Fly.io**：网站未上X 例：[Fly.io Blog] Your Agent Speaks MCP. Give It a Computer.；[Fly.io Fresh Produce (changelog)] Rotate MPG user passwords from flyctl；[Fly.io Fresh Produce (changelog)] 'List' endpoints in Machines API now support pagination；X 独有例：https://x.com/flydotio/status/2105375405293379818 https://x.com/flydotio/status/2105056978276086227 https://x.com/flydotio/status/2104714509185396751
- **Railway**：网站未上X 例：[Railway Blog] Incident Report: September 30, 2026: Domain Routing Disrupti；[Railway Blog] How to Deploy and Scale Docker Containers on Railway in 2026；[Railway Blog] Usage-Based vs. Fixed Pricing: Which Is Cheaper in 2026?；X 独有例：https://x.com/Railway/status/2108602171444887733 https://x.com/Railway/status/2107533437963837910 https://x.com/Railway/status/2107154606224462132
- **Render**：网站未上X 例：[Render Blog] Four minutes after midnight, Codex said the page was live；[Render Blog] Search for agents splits discovery from proof；[Render Changelog] Added two Key Value plans and lowered prices for larger exis；X 独有例：https://x.com/render/status/2103564481989353866 https://x.com/render/status/2103186759278637164 https://x.com/render/status/2103164264316100623
- **Neon**：网站未上X 例：[Neon Blog] Neon gives you 100 projects for free, with 1 GB of Postgres ；[Neon Blog] Comparing token economics across 42 AI models；X 独有例：https://x.com/neondatabase/status/2106125850542575900 https://x.com/neondatabase/status/2106028700638785617 https://x.com/neondatabase/status/2105741556884107341
- **PlanetScale**：网站未上X 例：[PlanetScale Changelog] MCP server: Branch metrics；[PlanetScale Changelog] Retry a partially failed deploy request；[PlanetScale Changelog] Limit MCP query access for an organization；X 独有例：https://x.com/PlanetScale/status/2108577128421757304 https://x.com/PlanetScale/status/2107890572652343333 https://x.com/PlanetScale/status/2107540265141432534
- **Deno**：网站未上X 例：[Deno Blog] Deno is joining Cloudflare；X 独有例：—
- **Bun**：网站未上X 例：—；X 独有例：https://x.com/bunjavascript/status/2108441841498804715 https://x.com/bunjavascript/status/2108100798572040331 https://x.com/bunjavascript/status/2107560647525548116
- **Node.js**：网站未上X 例：[Node.js Blog] Node.js 26.11.1 (Current)；[Node.js Blog] Node.js 26.11.0 (Current)；[Node.js Blog] Node.js 22.23.3 (LTS)；X 独有例：https://x.com/nodejs/status/2107557633590964534 https://x.com/nodejs/status/2092646833331810507
- **Docker**：网站未上X 例：—；X 独有例：https://x.com/Docker/status/2108601070415118361 https://x.com/Docker/status/2108588415243419858 https://x.com/Docker/status/2108271332949381593
- **Kubernetes**：网站未上X 例：[Kubernetes Releases (GitHub)] v1.38.0-alpha.2；[Kubernetes Releases (GitHub)] v1.38.0-alpha.1；[Kubernetes Releases (GitHub)] v1.37.1；X 独有例：https://x.com/kubernetesio/status/2108582968503550353 https://x.com/kubernetesio/status/2107925935399833653 https://x.com/kubernetesio/status/2106007230310920398
- **HashiCorp**：网站未上X 例：[HashiCorp Blog] HCP Vagrant deprecation: important dates and migration guida；[HashiCorp Blog] Relaunching HashiCorp Validated Designs with improved usabil；[HashiCorp Blog] Streamline identity lifecycle management on HCP with SCIM pr；X 独有例：https://x.com/HashiCorp/status/2108578484830261736 https://x.com/HashiCorp/status/2108286310460842331 https://x.com/HashiCorp/status/2107863160568877506
- **Stripe**：网站未上X 例：[Stripe Blog] Why I tried to kill token billing (and why we kept it)；[Stripe Blog] Travel’s AI dilemma at Skift Global Forum；[Stripe Blog] New trends in global card fraud: How 3D Secure and regional ；X 独有例：https://x.com/stripe/status/2107533255646089319 https://x.com/stripe/status/2105332074886635853 https://x.com/stripe/status/2105046474791035215
- **React**：网站未上X 例：[React Releases (GitHub)] 19.3.0 (September 9, 2026)；[React Releases (GitHub)] react-devtools@8.0.0: React DevTools 7.0.1 -> 8.0.0 (#37546)；X 独有例：—
- **Turso**：网站未上X 例：[Turso Blog] let your agents run free with turso accident protection；[Turso Blog] sqlite encryption in production；[Turso Blog] encrypting sqlite databases with turso；X 独有例：https://x.com/tursodatabase/status/2106054567125532976 https://x.com/tursodatabase/status/2105349137856450970 https://x.com/tursodatabase/status/2104958085492081025
- **Upstash**：网站未上X 例：[Upstash Blog] AWS S3 Pricing Explained (2026): Storage, Requests and Data ；[Upstash Blog] Grok Bot vs Muse vs OpenClaw vs Hermes: Personal AI Assistan；[Upstash Blog] Building A Multi-File Drag-and-Drop Uploader With Upstash Bl；X 独有例：https://x.com/upstash/status/2108368797946315060 https://x.com/upstash/status/2107532184341156080 https://x.com/upstash/status/2107144613072978321
- **Fastly**：网站未上X 例：[Fastly Blog] The Video Data Myth: Why AI Hasn't "Watched" the Internet Ye；[Fastly Blog] High Performance by Default: Why Speed Shouldn't Be a Setup ；[Fastly Blog] Back to Basics: Insecure Deserialization；X 独有例：https://x.com/fastly/status/2108660146691878988 https://x.com/fastly/status/2107493729380532600 https://x.com/fastly/status/2105332372694544729
- **DigitalOcean**：网站未上X 例：[DigitalOcean Blog] DigitalOcean MicroVMs: Fast, isolated compute for your AI ag；[DigitalOcean Blog] Account Control, Every Layer: From Your Organization to Ever；[DigitalOcean Blog] Introducing Agent Droplets: everything an agent needs, one p；X 独有例：https://x.com/digitalocean/status/2108596605611819435 https://x.com/digitalocean/status/2108325729700139311 https://x.com/digitalocean/status/2108284161559531970
- **Linear**：网站未上X 例：—；X 独有例：https://x.com/linear/status/2108598795487633821 https://x.com/linear/status/2107879412775444749 https://x.com/linear/status/2104175900887842908
