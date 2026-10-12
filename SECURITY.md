# 安全策略

## 支持的版本

只为最新版本（`main` 分支与最新的 `desktop-v*` 发布）提供安全修复。

## 报告漏洞

请**不要**在公开 Issue 中披露漏洞。通过 GitHub 的 [私下报告漏洞](https://github.com/iferrnil/ai-news-radar/security/advisories/new)（Security → Report a vulnerability）提交，附上影响范围和复现步骤。我们会尽快确认并在修复后致谢（如你愿意）。

## 部署建议

- 公网部署请保持 `SETTINGS_LOCKED=1`（Docker 默认），并只通过环境变量配置 API key。
- 站内保存的 key 存放在 `data/secrets.json`（权限 0600），不会出现在任何接口、导出或日志中。
