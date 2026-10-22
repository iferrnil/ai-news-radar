# 安全策略

## 支持的版本

只为最新版本（`main` 分支与最新的 `desktop-v*` 发布）提供安全修复。

## 报告漏洞

请**不要**在公开 Issue 中披露漏洞。通过 GitHub 的 [私下报告漏洞](https://github.com/iferrnil/ai-news-radar/security/advisories/new)（Security → Report a vulnerability）提交，附上影响范围和复现步骤。我们会尽快确认并在修复后致谢（如你愿意）。

## 部署建议

- 公网部署请保持 `SETTINGS_LOCKED=1`（Docker 默认），并只通过环境变量配置 API key。
- 站内保存的 key 存放在 `data/secrets.json`（权限 0600），不会出现在任何接口、导出或日志中。

## 威胁模型（0.14.3）

**桌面版（macOS）**

- 本机服务只监听 `127.0.0.1`（默认端口 47821），局域网里的其他设备无法访问。
- 每次启动生成 32 字节随机令牌：窗口第一次打开时通过 `?_k=` 换成 HttpOnly、SameSite=Strict 的 Cookie，随后从地址里去掉；`/api/*` 请求没有令牌（Cookie 或 `X-Radar-Token`）一律返回 401。Host 必须是 `127.0.0.1:端口` 或 `localhost:端口`（防 DNS 重绑定，否则 421）；来自其他网站的请求即使带着 Cookie 也会被拒绝（403）。同一台 Mac 上的其他用户和其他网页因此无法读写你的数据或触发操作。`/api/health` 不需要令牌，只返回运行状态。
- 数据目录（数据库、密钥文件、技能、图标缓存）权限为 0700，`secrets.json` 为 0600。密钥以明文保存在这个文件里，没有使用钥匙串；能以你的身份读取文件的程序可以读到它。
- 窗口只加载本机页面；其他链接只有 http / https / mailto 会交给系统打开，`file:`、自定义 URL scheme、带账号密码的链接都会被拦截。服务端入库时也会去掉非 http(s) 链接。
- 「工作目录」功能只读取 `封面/` 下的图片，「打开」只打开普通文件夹，不会运行程序。
- 应用不会执行外部命令，不使用 AppleScript；通知走系统原生接口。没有开机自启动。
- 自动更新：更新包使用 Ed25519 签名（公钥内置在应用里），签名不符或版本不更新的包不会安装；下载走 HTTPS（GitHub Releases）。
- 应用使用自签名身份并启用 hardened runtime，**没有经过 Apple 公证**：首次打开需要在「系统设置 → 隐私与安全性」里确认。请只从本仓库的 GitHub Releases 下载，并可用 `SHA256SUMS` 校验。

**网页版**

- 公网部署请保持 `SETTINGS_LOCKED=1`，只通过环境变量配置 key；AI / 技能相关接口在网页版返回 404。
- 跨站请求（`Sec-Fetch-Site: cross-site` 或 `Origin` 与 Host 不符）对 `/api/*` 返回 403；可设 `ALLOWED_HOSTS=news.example.com,…` 只接受这些 Host（防 DNS 重绑定，否则 421）。
- 服务端发起的请求（数据源、正文、图片、头像、技能安装）在建立连接时检查目标 IP，拒绝本机、内网、链路本地、CGNAT 等地址，重定向同样检查。确需抓取内网地址时设 `ALLOW_PRIVATE_FETCH=1`。桌面版的数据源允许本机地址（自建 RSSHub 等），正文、图片和头像仍只访问公网。
- `/api/icon` 不是开放代理：`?host=` 只代取已配置来源的站点；`?x=` 只代取已配置 X 来源的用户名。
- 技能安装：zip / tar 路径经清理，拒绝 `..` 与绝对路径；写入时再次确认目标仍在技能目录内（防 zip-slip）。脚本目录只作参考，不会执行。
- 「测试连接」只会把已保存的 key 发往已保存的 Base URL；测试新地址需要同时填写 key。
- 密钥以明文保存在 `secrets.json`（0600）；桌面版未接入系统钥匙串——能以你身份读文件的程序可以读到它。自动更新包经 Ed25519 校验；应用未公证，请只从本仓库 Releases 下载并用 `SHA256SUMS` 核对。
