# 架构地图

上游形态是 **Go 单体 + gin + SQLite + 纯静态前端**（无前端构建步骤）。
本文只覆盖二开需要知道的部分；上游完整设计见 `docs/zh-CN/architecture.md`。

## 启动链

```
cmd/server/main.go
  ├─ flag 解析：-config / --https / --http / --reset-admin-password
  ├─ config.EnsureLocalConfig + config.Load        # config.yaml 首次自动从 example 生成
  ├─ config.EnsureMCPAuth
  ├─ logger.New
  └─ app.New(cfg, log, configPath)                 # → internal/app/app.go
       └─ application.RunWithContext(ctx)          # 监听 SIGINT / SIGTERM
```

`internal/app/app.go` 的 `New()` 是**唯一总装点**，初始化顺序敏感，大致为：

```
multiagent.InitADK
  → gin 引擎 + CORS
  → database.NewDB                              # SQLite
  → security.NewAuthManager + RBAC
  → audit / monitor / HITL 保留策略
  → mcp.NewServerWithStorage
  → security.NewExecutor + RegisterTools        # 加载 tools/*.yaml
  → 注册内置工具（vulnerability / asset / projectFact / vision）
  → ExternalMCPManager                          # 外部 MCP 联邦
  → agent.NewAgent
  → 知识库（knowledge.enabled 为 true 时）
  → 各 Handler
  → 路由注册
```

**加新模块就在这里挂**。初始化顺序写错会导致依赖注入失败。

## 静态前端与路由

- `internal/app/app.go` 中：`router.Static("/static", "./web/static")`、`LoadHTMLGlob("web/templates/*")`、`GET /` 渲染 `index.html`
- **路径是相对的**，所以服务必须在项目根目录启动
- 前端是 40+ 个原生 JS，`index.html` 逐个 `<script src>` 引入，改完刷新即生效，没有打包步骤
- 认证路由挂在 `/api/auth`（`POST /api/auth/login` 登录）
- `POST /api/mcp` 是 MCP 端点；`GET /api/config/tools` 查询工具列表（分页，默认 20 条/页，`page_size` 最大 100）

## 模块职责

| 模块 | 职责 |
|---|---|
| `internal/app` | 总装与路由注册 |
| `internal/handler` | HTTP 层，按业务拆分（约 120 个文件） |
| `internal/agent` | 单智能体（MCP 工具驱动） |
| `internal/multiagent` | Eino ADK run loop / 中间件 / Deep / Plan-Execute / Supervisor |
| `internal/agents` | Markdown 子 Agent 定义解析（`agents/*.md`，front matter） |
| `internal/mcp` | MCP Server、外部 MCP 联邦、内置工具 |
| `internal/einomcp` | Eino ↔ MCP 工具桥接 |
| `internal/workflow` | 工作流引擎（start / agent / tool / condition / hitl / output / end 节点） |
| `internal/security` | 认证、RBAC、Shell 执行器、工具加载 |
| `internal/hitl` | 人机协同审批 |
| `internal/knowledge` | RAG：查询改写 / 向量检索 / 精排 / 后处理 |
| `internal/tooloutput` | 大输出截断落盘 |
| `internal/database` | SQLite 持久化（**无迁移框架**） |
| `internal/c2`、`internal/robot` | 内置 C2、机器人接入 |

依赖方向：`handler → agent/multiagent/mcp`，`multiagent → einomcp → mcp`，`app` 横切全部。

## 扩展点（二开重点）

### 1. 工具配方 `tools/*.yaml`（改 YAML 即可，无需改 Go）

加载链路：

```
config.Load 读 security.tools_dir
  → config.ReloadSecurityToolsFromDir      # 扫描 .yaml / .yml
  → security.Executor.RegisterTools        # 由 parameters 生成 InputSchema
  → ExecuteTool                            # 按 flag / positional / template 拼命令行
```

- 当前共 **138 个工具（92 个启用）**
- 支持热重载（`internal/handler/config.go`）
- 规范见上游 `tools/README.md`
- 执行入口是 `exec.CommandContext(ctx, toolConfig.Command, cmdArgs...)`，
  **直接使用 YAML 里写的命令**，所以命令必须能在 PATH 中解析

### 2. Skill / 角色 / 子 Agent（纯文件，改完即生效）

| 目录 | 内容 | 加载方 |
|---|---|---|
| `skills/` | 标准 SKILL.md 包 | `internal/skillpackage`，经 Eino skill 工具渐进披露 |
| `roles/` | 角色 YAML，限定工具集 | `internal/config` 的 `roles_dir` |
| `agents/` | Markdown 子 Agent 定义 | `internal/agents` |

> 注意：`skills/` 是**运行时给 Agent 用**的目录。开发协作用的 skill 放在 `.trae/skills/`，两者不要混。

### 3. 前端

`web/static/js/*.js` 直接改，无需构建。i18n 文案在 `web/static/i18n/{zh-CN,en-US}.json`。

### 4. 浏览器抓包（FlowSage 方向的现成起点）

`plugins/browser-extension/cyberstrikeai-browser-extension/` **上游已内置**：
Chrome/Edge 扩展，在 DevTools 中捕获 Network 流量并发送到 CyberStrikeAI 做 AI 辅助分析。

```
background/   后台 service worker
panel/        DevTools 面板 UI
popup/        弹窗
lib/          与后端通信的公共逻辑
dist/         打包产物（被 gitignore）
```

另有 `plugins/burp-suite/cyberstrikeai-burp-extension/`（Burp 插件，Java）。

### 5. CLI 入口

`cmd/` 目前有：`server`（主入口）、`mcp-stdio`、`test-config`、`test-external-mcp`、`test-sse-mcp-server`。

新增 CLI 子命令时建议在 `cmd/` 下新建独立目录，复用 `internal/` 的能力，
避免把交互逻辑塞进 `cmd/server`。

## 数据存储

- SQLite，驱动 `mattn/go-sqlite3`（**需要 CGO**）
- WAL 模式 + 连接池 + PASSIVE checkpoint
- 库文件：`data/conversations.db`、`data/knowledge.db`
- 建表：`internal/database/database.go` 的 `initTables()`，**全部 `CREATE TABLE IF NOT EXISTS`**

## 测试

| 类型 | 数量 | 命令 |
|---|---|---|
| Go 单测 | 223 个 `*_test.go` | `go test ./internal/...` |
| 前端单测 | 16 个 `*.test.cjs`（`node:test`） | `node --test web/static/js/*.test.cjs` |

说明见上游 `docs/zh-CN/testing.md`。

## 平台差异处理

上游已用 build tag 处理跨平台：`handler/terminal_stream_{unix,windows}.go`、
`handler/terminal_ws_{unix,windows}.go`、`security/procattr_{unix,windows}.go`。
`creack/pty` 仅 unix 引用，Windows 走 stdout/stderr 管道，**无编译阻塞**。