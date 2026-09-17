# CLAUDE.md

本仓库的协作约定以根目录 [`AGENTS.md`](AGENTS.md) 为唯一事实源，请先完整阅读它。

## 速览

- **项目**：FlowSage —— CyberStrikeAI v1.7.17 的二次开发项目
- **目标形态**：浏览器 Web 流量抓包 + CLI 交互式安全测试 Agent
- **技术栈**：Go（需 CGO）+ gin + SQLite + 纯静态前端

## 硬性约束

1. 修改上游已有文件，必须在文件顶部加 `// Modified by FlowSage: <说明>`（Apache-2.0 §4b 要求）
2. 不要动：`3df1aff` 基线提交、`LICENSE`、`NOTICE`、`web/static/vendor/`
3. 不要提交：`config.yaml`（含密钥）、真实凭据与目标数据
4. 服务必须从项目根目录启动（前端用相对路径）

## 常用命令

```bash
go build -o cyberstrike-ai.exe ./cmd/server   # 构建验证
go test ./internal/...                        # 单测
node --test web/static/js/*.test.cjs          # 前端单测
run-windows.cmd --http                        # 启动服务（Windows）
```

## 收尾

开发结束更新 `docs/dev/04-progress.md`；架构决策记入 `docs/dev/05-decisions.md`。
环境与迁移说明见 `docs/dev/01-environment.md`，架构地图见 `docs/dev/02-architecture.md`。