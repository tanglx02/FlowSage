# 二开约定

## 1. 许可证合规（每个改动都要守）

上游为 Apache-2.0（版权归 Ed1s0nZ）。约束如下：

| 要求 | 做法 |
|---|---|
| 保留许可证与版权声明 | 不要删除或修改 `LICENSE`、`NOTICE` |
| 声明修改（§4b） | **改动上游已有文件时，在文件顶部加一行** `// Modified by FlowSage: <一句话>`（注释符按语言替换） |
| 新增文件 | 无需标注 |
| 不得暗示背书 | 不使用上游作者名义/商标做推广；README 等对外文案中如实说明是衍生项目 |

对外分发（尤其是公开仓库、二进制发布）时，确保 `LICENSE` 与 `NOTICE` 一并携带。

## 2. 上游基线不可动

- `3df1aff`（`chore: import upstream CyberStrikeAI v1.7.17 as pristine baseline`）是**只读基线**，
  永远不要 rebase / amend / 丢弃这个提交
- 需要与上游比对差异时：

  ```bash
  git diff 3df1aff..HEAD --stat          # 我改了哪些文件
  git diff 3df1aff..HEAD -- internal/    # 只看某目录
  ```

## 3. 与上游同步更新

`upstream` remote 已配置指向 `https://github.com/Ed1s0nZ/CyberStrikeAI.git`。

```bash
git fetch upstream --tags
git log --oneline HEAD..upstream/main            # 看看上游新增了什么
git merge upstream/main                          # 在 main 上合并（推荐）
```

冲突处理原则：

- 冲突集中在 `internal/` 与 `web/` 时，**优先接受上游修复**，再把 FlowSage 的改动重新叠加
- `web/static/vendor/` 若出现冲突，保留双方（该目录是被强制纳入的）
- 合并后必须重跑 `go build` + `go test ./internal/...`

## 4. 分支模型

| 分支 | 用途 |
|---|---|
| `main` | 可运行、可构建。任何时刻 clone 下来都能跑起来 |
| `feature/<name>` | 单个功能开发，完成后合回 `main` |
| `fix/<name>` | 缺陷修复 |

小改动可直接在 `main` 提交；涉及跨模块重构或大方向调整时开分支。

## 5. 提交信息

采用 Conventional Commits：

```
feat: 新增浏览器流量捕获 CLI 子命令
fix: 修正 python3 工具在 Windows 上的命令解析
docs: 补充架构地图
chore: 导入上游基线
refactor: 拆分 capture 模块
test: 补充 executor 单测
```

正文用中文说明"为什么改"，与上游 `git log` 风格保持可读性。

## 6. 改动质量门

提交前至少完成：

```bash
go build -o cyberstrike-ai.exe ./cmd/server
```

涉及核心逻辑（`internal/agent`、`internal/multiagent`、`internal/security`、`internal/workflow`、
`internal/database`）时，追加：

```bash
go test ./internal/...
```

改动前端 JS 时：

```bash
node --test web/static/js/*.test.cjs
```

## 7. 文件与目录约定

| 约定 | 说明 |
|---|---|
| 不改 `web/static/vendor/` | 前端运行时依赖，且是被强制的忽略规则例外 |
| 不提交 `config.yaml` | 含密钥，已被 gitignore；也不要删除该忽略规则 |
| 不在 `skills/` 放开发协作文档 | 那是运行时给 Agent 加载的目录，开发 skill 放 `.trae/skills/` |
| 数据库变更 | 无迁移框架，改动 `initTables()` 时自行处理老库兼容（加列用 `ALTER TABLE ... ADD COLUMN` + 错误忽略，或先探测列是否存在） |
| 新增 Go 依赖 | 优先复用已有依赖，新增前确认 `go.mod` 变更是否必要 |
| 换行符 | 保持 LF（仓库已设 `core.autocrlf=input`） |

## 8. 文档同步义务

开发过程中**必须同步更新**，这是"换机器/换 AI 工具能接上"的前提：

| 发生的事 | 更新哪个文件 |
|---|---|
| 完成一个功能 / 修复 | `docs/dev/04-progress.md` 追加一条 |
| 做出架构性选择（选型、方案取舍） | `docs/dev/05-decisions.md` 追加一条 ADR |
| 环境步骤变化（新增依赖、新命令） | `docs/dev/01-environment.md` 与 `scripts/setup-windows.cmd` |
| 目录/模块职责变化 | `docs/dev/02-architecture.md` 与 `AGENTS.md` 的目录地图 |
| 新增约定 | 本文件 |

## 9. 安全提醒

本项目的运行时能力包含 WebShell、C2、后渗透等高风险功能。
**仅在自有系统或已获明确授权的目标上使用**，公开仓库不得附带真实目标数据、
真实凭据、公司内网信息。提交前自查：

- 无真实 IP / 域名 / 账号密码
- 无 `config.yaml`、`.env`、证书私钥
- 无客户或项目名相关敏感字样