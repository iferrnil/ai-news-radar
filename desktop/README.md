# AI News Radar 桌面版（macOS）

用 [MyGo](https://github.com/egoist/mygo)（固定 v0.4.3）做的原生外壳，Go 后端（`../server`）在 App 进程内运行，Next.js 静态界面嵌入二进制，不含 Node：

- 启动时在 `127.0.0.1:47821`（被占用则随机空闲端口）启动内置服务，`/api/health` 就绪后在原生窗口（WKWebView）中打开；
- 数据库在用户数据目录：`~/Library/Application Support/AI News Radar/news.db`（日志 `server.log` 也在这里）；
- 不读取/传递环境变量 `DEEPSEEK_API_KEY`，翻译默认关闭；在 App 的 **设置** 页填写 DeepSeek key（保存到数据目录 `secrets.json`，权限 0600，立即生效）后可开启“抓取时翻译”。没有用 macOS 钥匙串：MyGo 没有钥匙串 API，而调用 `security` 命令会把 key 暴露在进程参数里；
- 站外链接用默认浏览器打开；只允许运行一个实例（再次打开会把窗口调到前台）；
- 关闭窗口 = 隐藏（菜单栏图标仍在），菜单栏 / 应用菜单：显示 / 隐藏（⌘⇧R）、刷新（⌘R）、打开数据目录、退出（⌘Q）；
- 窗口：隐藏标题栏（红绿灯嵌入顶栏，拖动顶栏移动窗口），记住大小和位置（数据目录 `window-state.json`），最小 720×480；默认跟随系统深浅色（在应用里手动切换后以手动为准）；
- 快捷键：⌘B 显示/隐藏侧边栏（菜单「视图」）、⌘, 设置、⌘F 搜索、⌘K 命令面板；
- 退出时停止轮询、关闭 HTTP 服务（最多等 2 秒），再正常关闭 SQLite（WAL 合并）。

## 在自己的 Mac 上构建

需要：Xcode 命令行工具（`xcode-select --install`）、Node 22（仅构建界面）、Go 1.27.1+（`go.mod` 里的 toolchain 会自动下载）。

```sh
git clone https://github.com/iferrnil/ai-news-radar.git && cd ai-news-radar
npm ci
desktop/scripts/package.sh                 # 默认 darwin/universal（Intel + Apple 芯片）
# 网络受限：npm config set registry https://registry.npmmirror.com；export GOPROXY=https://goproxy.cn,direct
# 只要本机架构更小：desktop/scripts/package.sh darwin/arm64
open desktop/build/darwin-universal        # AI News Radar.app 和 .dmg
```

脚本做的事：`npm run build`（静态导出到 `out/`）→ 复制到 `desktop/webui/`（连同 `sources.json`，用 go:embed 嵌入）→ `go tool mygo build`（在 Mac 上会签名并生成 .dmg）。arm64 的 .app 约 28 MB（0.3.0 内置 Node 时为 135 MB）。

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

工作流：[`.github/workflows/desktop-release.yml`](../.github/workflows/desktop-release.yml)。发布新版本：

```bash
git tag desktop-v0.2.0 && git push origin desktop-v0.2.0
```

推送 `desktop-v*` 标签（或 Actions › desktop-release › Run workflow，可选 tag / 草稿）后，在 `macos-latest` 上分别构建 arm64 和 x64 两个 .dmg（版本号取自标签），校验签名后生成 `SHA256SUMS` 和发布说明，创建（或更新）对应的 GitHub Release。

- 没有配置签名 secrets：ad-hoc 签名（按上面“首次打开”处理）；
- 配置了以下 secrets：用 Developer ID 签名并公证（无需任何首次打开操作）：
  `MACOS_CERTIFICATE`（.p12 的 base64）、`MACOS_CERTIFICATE_PASSWORD`、`MACOS_SIGNING_IDENTITY`（如 `Developer ID Application: Name (TEAMID)`）、`APPLE_ID`、`APPLE_TEAM_ID`、`APPLE_APP_PASSWORD`（App 专用密码）。

## 自动更新

App 使用 MyGo 更新器：启动后及每 12 小时检查 GitHub Releases 上的 `update-darwin-<arch>.json`，也可在菜单「检查更新…」手动检查；下载的更新包（完整 `.tar.gz` 或增量 `.delta`）经 ed25519 签名校验后替换 App 并重启。更新签名与 Apple 签名无关，ad-hoc 签名的版本同样可以自动更新。

- 公钥在 `mygo.json` 的 `updates.publicKey`；私钥只保存在仓库 secret `MYGO_UPDATER_PRIVATE_KEY`（和维护者本机备份）中，丢失后已安装的 App 将无法再验证新版本。
- 发布前在 `CHANGELOG.md` 写好 `## [x.y.z]` 小节，更新窗口会显示它。
- 0.1.0 没有更新器，需要手动安装一次 0.2.0。

## 通知

新的 P0 动态由 App 以系统通知推送（合并成批、至少间隔 1 分钟，启动时的积压不通知），点击通知打开对应条目。在 App 的「设置 › 通知」里开关或选择分区；首次通知时 macOS 会请求通知权限。

## 安装包瘦身

不再打包 Node 运行时和 Next 服务端：App 里只有一个 Go 二进制（后端 + 嵌入的静态界面）。`package.sh` 还会去掉 source map 和非中英字体子集。

## 开发 / 测试

```sh
cd desktop
go vet ./... && go test ./...                               # 单元测试 + 进程内服务的启动/健康检查/优雅退出
desktop/scripts/package.sh darwin/universal,linux/amd64     # 也可以在 Linux 上交叉编译（不签名、不出 dmg）
```

`AI_RADAR_UI=<目录>`（如 `../out`）可让 App 使用该目录的界面而不是嵌入的版本。网页版 / Docker 不受影响（`desktop/` 已加入 `.dockerignore`）。
