---
name: flowsage-dev
description: Work inside the FlowSage repo, a CyberStrikeAI derivative. Use when resuming development, editing Go or web code, syncing upstream, or committing here. Do not use for other projects.
---

# FlowSage 开发

FlowSage 是基于 CyberStrikeAI v1.7.17 的二开项目，目标是"浏览器 Web 流量抓包 + CLI 交互式安全测试 Agent"。

## 开工前必做

1. 读根目录 `AGENTS.md`（唯一事实源，含目录地图与硬性约定）
2. 读 `docs/dev/04-progress.md` 最近 1-2 条，确认上一步做到哪
3. 若涉及设计改动，先查 `docs/dev/05-decisions.md` 是否已定过，避免重复讨论

## 验证命令

```bash
go build -o cyberstrike-ai.exe ./cmd/server    # 必须通过才算改完
go test ./internal/...                         # 改核心逻辑时追加
node --test web/static/js/*.test.cjs           # 改前端 JS 时追加
run-windows.cmd --http                         # 启动（必须从项目根目录）
```

服务启动后可在 `http://127.0.0.1:8080/` 验证；API 登录为 `POST /api/auth/login`，
工具列表为 `GET /api/config/tools?page=1&page_size=100`。

## 硬性约束

- **改上游已有文件**：文件顶部加一行 `// Modified by FlowSage: <说明>`（Apache-2.0 §4b），新增文件不需要
- **不删改** `LICENSE`、`NOTICE`、`3df1aff` 基线提交
- **不动** `web/static/vendor/`（前端运行时依赖，被强制纳入版本控制的忽略例外）
- **不提交** `config.yaml`、真实凭据、真实目标 IP/域名
- **数据库**没有迁移框架，改 `internal/database/database.go` 的 `initTables()` 时自行处理老库兼容
- **`skills/` 是运行时目录**，开发协作文档不要放进去（本 skill 放 `.trae/skills/`）

## 常见改动落点

| 要做什么 | 改哪里 |
|---|---|
| 加 HTTP 接口 | `internal/handler/` 新增 handler，`internal/app/app.go` 注册路由 |
| 加工具（无需 Go 代码） | `tools/*.yaml`，支持热重载 |
| 加运行时 Skill / 角色 / 子 Agent | `skills/`、`roles/`、`agents/` |
| 改前端 | `web/static/js/*.js`，无构建步骤 |
| 加 CLI 子命令 | `cmd/` 下新建目录，复用 `internal/` |
| 浏览器抓包相关 | `plugins/browser-extension/cyberstrikeai-browser-extension/` |
| 接外部工具链（如 Kali） | `external_mcp` 配置 + `internal/mcp/` |

## 收尾必做

改完代码后同步文档，否则换机器会丢上下文：

- 完成功能/修复 → 追加 `docs/dev/04-progress.md`
- 架构性决策 → 追加 `docs/dev/05-decisions.md`
- 环境/命令变化 → 更新 `docs/dev/01-environment.md` 与 `scripts/setup-windows.cmd`