# AI News Radar 桌面版（macOS）

用 [MyGo](https://github.com/egoist/mygo)（固定 v0.4.3）做的原生外壳，内置 Node 22 和 Next.js standalone 服务：

- 启动时在 `127.0.0.1` 的随机空闲端口启动内置服务，`/api/health` 就绪后在原生窗口（WKWebView）中打开；
- 数据库在用户数据目录：`~/Library/Application Support/AI News Radar/news.db`（日志 `server.log` 也在这里）；
- 不读取/传递环境变量 `DEEPSEEK_API_KEY`，翻译默认关闭；在 App 的 **设置** 页填写 DeepSeek key（保存到数据目录 `secrets.json`，权限 0600，立即生效）后可开启“抓取时翻译”。没有用 macOS 钥匙串：MyGo 没有钥匙串 API，而调用 `security` 命令会把 key 暴露在进程参数里；
- 站外链接用默认浏览器打开；只允许运行一个实例（再次打开会把窗口调到前台）；
- 关闭窗口 = 隐藏（菜单栏图标仍在），菜单栏 / 应用菜单：显示 / 隐藏（⌘⇧R）、刷新（⌘R）、打开数据目录、退出（⌘Q）；
- 退出时先关闭内置服务的 stdin（服务随即退出并正常关闭 SQLite），再依次 SIGTERM、强杀兜底。

## 在自己的 Mac 上构建

需要：Xcode 命令行工具（`xcode-select --install`）、Node 22、Go 1.27.1+（`go.mod` 里的 toolchain 会自动下载）。

```sh
git clone https://github.com/iferrnil/ai-news-radar.git && cd ai-news-radar
npm ci
desktop/scripts/package.sh                 # 默认 darwin/universal（Intel + Apple 芯片）
# 国内网络：MIRROR=cn desktop/scripts/package.sh   （Node 从 npmmirror 下载）
# 只要本机架构更小：desktop/scripts/package.sh darwin/arm64
open desktop/build/darwin-universal        # AI News Radar.app 和 .dmg
```

脚本做的事：`STANDALONE=1 npm run build` → 组装 `desktop/resources/server`（standalone + static + public + sources.json；`.next` 改名为 `next-dist`，因为 MyGo 不打包隐藏文件）→ 下载对应架构的 Node 官方二进制 → 放入 better-sqlite3 自带的 darwin 预编译模块（Node-API，无需编译）→ `go tool mygo build`（在 Mac 上会签名并生成 .dmg）。

本机构建默认是 ad-hoc 签名，可在本机直接运行。要发给别人用，在 `mygo.json` 的 `macos` 里加上 `signingIdentity`（Developer ID）和 `notarize: { keychainProfile: "notary" }`，并先执行一次 `xcrun notarytool store-credentials notary`。

## 首次打开（未公证的版本）

从网上下载的未公证 App 会被 Gatekeeper 拦截（“无法验证开发者”/“已损坏”）：

- 方法一：在「应用程序」里右键 AI News Radar → 打开 → 再点「打开」；或到「系统设置 › 隐私与安全性」点「仍要打开」；
- 方法二（终端）：`xattr -dr com.apple.quarantine "/Applications/AI News Radar.app"`

从 Linux 交叉编译出来的 .app 完全没有签名，Apple 芯片的 Mac 拒绝运行未签名代码，需要先在 Mac 上补一个 ad-hoc 签名：

```sh
codesign --force --deep --sign - "AI News Radar.app"
xattr -dr com.apple.quarantine "AI News Radar.app"
```

## GitHub Actions / Releases

工作流在 `desktop/ci/desktop-release.yml`（推送 `.github/workflows/` 需要 token 的 `workflow` 权限，当前 token 只有 `gist, read:org, repo`）。启用步骤：

```bash
gh auth refresh -h github.com -s workflow
mkdir -p .github/workflows && git mv desktop/ci/desktop-release.yml .github/workflows/desktop-release.yml
git commit -m "ci: desktop release workflow" && git push
git tag desktop-v0.1.0 && git push --tags
```

推送 `desktop-v*` 标签（或 Actions › desktop-release › Run workflow，可选 tag / 草稿）后，在 `macos-latest` 上分别构建 arm64 和 x64 两个 .dmg（版本号取自标签），校验签名后生成 `SHA256SUMS` 和发布说明，创建（或更新）对应的 GitHub Release。

- 没有配置签名 secrets：ad-hoc 签名（按上面“首次打开”处理）；
- 配置了以下 secrets：用 Developer ID 签名并公证（无需任何首次打开操作）：
  `MACOS_CERTIFICATE`（.p12 的 base64）、`MACOS_CERTIFICATE_PASSWORD`、`MACOS_SIGNING_IDENTITY`（如 `Developer ID Application: Name (TEAMID)`）、`APPLE_ID`、`APPLE_TEAM_ID`、`APPLE_APP_PASSWORD`（App 专用密码）。

## 开发 / 测试

```sh
cd desktop
go vet ./... && go test ./...                               # 单元测试
AI_RADAR_TEST_RES=$PWD/build/linux-amd64 go test ./...      # 用已构建的 Linux 包端到端测服务的启动/健康检查/优雅退出
desktop/scripts/package.sh darwin/universal,linux/amd64     # 也可以在 Linux 上交叉编译（不签名、不出 dmg）
```

`AI_RADAR_NODE` / `AI_RADAR_SERVER` 可覆盖内置的 node 与服务目录。网页版 / Docker 不受影响（`desktop/` 已加入 `.dockerignore`）。
