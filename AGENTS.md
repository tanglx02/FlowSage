# FlowSage — AI 协作须知

本文件是所有 AI 编码工具（Trae / Cursor / Codex / Claude Code 等）在本仓库工作时的**唯一事实源**。
各 IDE 的专属入口文件（`.trae/rules/`、`.cursor/rules/`、`CLAUDE.md`）都只是指向本文件的薄壳。

## 这是什么项目

**FlowSage** 是基于 [CyberStrikeAI](https://github.com/Ed1s0nZ/CyberStrikeAI) v1.7.17 的二次开发项目。

- **上游形态**：Web 控制台式的 AI 渗透测试平台（Go 单体 + gin + SQLite + 静态前端）
- **本项目目标形态**：**浏览器 Web 流量抓包 + CLI 交互式安全测试 Agent**

上游快照以纯净基线形式保留在 commit `3df1aff`（994 个文件，未做任何改动），
所有 FlowSage 的改动都叠加在其之上。`upstream` remote 指向原仓库，便于跟进上游版本。

## 许可证义务（改动前必读）

上游为 **Apache-2.0**（见 [LICENSE](LICENSE)，版权归 Ed1s0nZ）。二开必须遵守：

1. **不得删除** `LICENSE` 与其中版权声明
2. **修改上游已有文件时，在文件顶部加一行标注**，满足 Apache-2.0 §4(b) 的"显著修改声明"要求：

   ```go
   // Modified by FlowSage: <一句话说明改了什么>
   ```

   各语言注释符自行替换（`#`、`//`、`<!-- -->`）。**新增文件不需要标注**。
3. 不得使用上游作者名义或商标暗示背书

## 环境要求

| 依赖 | 版本 | 说明 |
|---|---|---|
| Go | 1.25+（以 `go.mod` 为准） | **需要 CGO**，无 gcc 无法编译 |
| C 编译器 | MSYS2 / MinGW-w64 gcc | `mattn/go-sqlite3` 依赖 |
| Python | 3.10+ | 工具配方（`tools/*.yaml`）用 |
| Node | 18+ | 跑前端单测 |

零基础重建环境：运行 `scripts\setup-windows.cmd`，细节见 [docs/dev/01-environment.md](docs/dev/01-environment.md)。

## 常用命令

```bash
go build -o cyberstrike-ai.exe ./cmd/server    # 构建（Windows）
go build -o cyberstrike-ai ./cmd/server        # 构建（Linux/macOS）
go test ./internal/...                         # Go 单测
node --test web/static/js/*.test.cjs           # 前端单测
run-windows.cmd                                # 启动服务（Windows，自带 PATH 修复）
run-windows.cmd --http                         # 纯 HTTP 启动（避免自签证书告警）
```

## 必须知道的坑

- **启动必须在项目根目录**：前端用相对路径 `./web/static` 与 `web/templates/*`，换目录启动会 404。
- **`web/static/vendor/` 是被强制纳入版本控制的**：上游 `.gitignore` 里的 `vendor/` 规则会误伤这个前端目录，但页面运行时需要这些库。不要"顺手修复"这个忽略规则，也不要把这些文件删掉。
- **数据库没有迁移框架**：`internal/database/database.go` 全部用 `CREATE TABLE IF NOT EXISTS`。加字段要自己写兼容逻辑，不能指望自动迁移。
- **Windows 上工具链不完整**：`tools/*.yaml` 里的 70+ 原生二进制（sqlmap / nuclei / ffuf 等）在 Windows 上默认不存在，执行会报"命令不存在"。这是预期行为，不要把工具报错当成代码 bug。
- **`config.yaml` 含密钥，已被 gitignore**：不要提交，也不要删除这条忽略规则。

## 目录地图

| 路径 | 职责 |
|---|---|
| `cmd/server/` | 服务入口（flag 解析 → 加载配置 → 启动 app） |
| `internal/app/app.go` | **总装点**：初始化 DB/认证/MCP/工具/Agent 并注册全部路由。初始化顺序敏感 |
| `internal/handler/` | HTTP 层，按业务拆分；新增 API 主要在这里 |
| `internal/agent/`、`internal/multiagent/` | 单智能体 / Eino ADK 多智能体编排 |
| `internal/mcp/`、`internal/einomcp/` | MCP 服务端、外部 MCP 联邦、Eino↔MCP 桥接 |
| `internal/security/` | 认证、RBAC、工具加载与命令执行器 |
| `internal/workflow/`、`internal/hitl/` | 工作流引擎、人机协同审批 |
| `internal/knowledge/`、`internal/database/` | RAG 知识库、SQLite 持久化 |
| `tools/*.yaml` | 工具配方（改 YAML 即可加工具，支持热重载） |
| `skills/`、`roles/`、`agents/` | 运行时由 Agent 加载的 Skill / 角色 / 子 Agent 定义 |
| `web/` | 纯静态前端，无构建步骤，改完刷新即生效 |

## 开发约定

- **改动前先看** [docs/dev/03-conventions.md](docs/dev/03-conventions.md)（分支、提交、上游同步、测试要求）
- **每次开发结束更新** [docs/dev/04-progress.md](docs/dev/04-progress.md)（进度日志），确保换机器/换 AI 工具后能接上
- **架构性决策记入** [docs/dev/05-decisions.md](docs/dev/05-decisions.md)
- 改完代码至少跑一次 `go build ./cmd/server`；涉及核心逻辑时跑相关包的 `go test`

## 文档索引

| 文件 | 内容 |
|---|---|
| [docs/dev/README.md](docs/dev/README.md) | 文档体系导航与使用方式 |
| [docs/dev/01-environment.md](docs/dev/01-environment.md) | 环境搭建、迁移到新机器的完整步骤、已知环境坑 |
| [docs/dev/02-architecture.md](docs/dev/02-architecture.md) | 架构地图、关键文件与调用链、扩展点 |
| [docs/dev/03-conventions.md](docs/dev/03-conventions.md) | 二开约定：分支模型、上游同步、许可证合规、测试 |
| [docs/dev/04-progress.md](docs/dev/04-progress.md) | 进度日志 |
| [docs/dev/05-decisions.md](docs/dev/05-decisions.md) | 决策记录（ADR） |
| `docs/zh-CN/`、`docs/en-US/` | **上游原生文档**（架构、配置、部署、测试等 30+ 篇），不要改动 |