# 贡献指南

感谢你愿意改进 AI News Radar！Issue、文档修正和 PR 都欢迎。

## 开发环境

- Go（版本见 `server/go.mod`）：后端与桌面版
- Node 22：只用于构建界面（运行时不需要 Node）

```bash
npm ci
(cd server && PORT=3001 go run ./cmd/radar-server) &   # 后端（API + 轮询），数据在 server/data
npm run dev          # http://localhost:3000，/api 代理到 :3001
```

## 提交前检查

```bash
npm run lint -- --max-warnings=0
npm run typecheck
npm run build                      # 静态导出到 out/
(cd server && gofmt -l . && go vet ./... && go test ./...)
npx playwright install chromium    # 首次
bash e2e/runall.sh                 # 端到端测试：Go 服务 + 全新数据库 + 本地 mock 上游，不访问外网
(cd desktop && go vet ./... && go test ./...)
```

- 要求零警告；CI（`.github/workflows/ci.yml`）在每次 push / PR 时运行同样的检查。
- `bash e2e/runall.sh "" t13 t14` 只运行指定测试；测试默认占用 3000 端口并替换 `data/news.db`，可用 `PORT=3092 E2E_DATA=/tmp/e2e` 换端口和数据目录。
- 新增外部请求请走 `server/radar/net.go` 的 `netFetch`，并在 `e2e/mock-upstream.mjs` 中提供对应的 mock（模型接口也有 mock，测试不需要真实 key）。
- 新功能请同时补 Go 单元测试（`server/radar/*_test.go`）和端到端测试（`e2e/tNN.mjs`，加入 `e2e/runall.sh` 的列表）。

## 代码约定

- UI 使用 shadcn/ui（Base UI）组件与 Tailwind v4，注意不要增加首页的 JS 体积。
- 只接入有 RSS/Atom 或 JSON API 的源，不做 HTML 爬取；新增源类型见 [AGENTS.md](AGENTS.md)。
- AI 相关功能（简报、生成文章、Skill、写作风格、抓取 API）只在桌面版开放，网页版接口返回 404；其余功能两端保持一致。
- 界面文字尽量少；不引入需要访问外部 CDN / 字体服务的资源。
- 修改内置 Skill（`server/radar/defaultskill/`）时请更新 `version`，并用真实条目生成几篇样稿检查效果。
- 不要提交密钥、`.env*`、`data/` 或本机路径。

## 提交与 PR

- 提交信息建议使用 [Conventional Commits](https://www.conventionalcommits.org/)：`feat:`、`fix:`、`docs:`、`ci:` 等。
- 用户可见的改动请在 [CHANGELOG.md](CHANGELOG.md) 的「未发布」或对应版本下记录。
- PR 请说明动机、改动和测试方式，界面改动附截图。

## 发布桌面版

更新 `CHANGELOG.md`（需有 `## [x.y.z]` 小节）与 `desktop/mygo.json` 的版本，然后推送 `desktop-vx.y.z` 标签，`desktop-release.yml` 会构建 dmg 与签名的更新包并发布到 GitHub Releases。

## 许可

提交的代码以 [MIT](LICENSE) 许可发布，版权 © 2026 Lang 及贡献者。
