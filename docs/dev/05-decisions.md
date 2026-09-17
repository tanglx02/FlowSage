# 决策记录（ADR）

> 记录**架构性与方向性的选择**，包含被否决的方案与理由。
> 目的：避免换机器/换 AI 工具后重复讨论已经定过的事。

## 记录格式

```markdown
### ADR-NNN — <决策标题>

- **日期**：YYYY-MM-DD
- **状态**：已采纳 / 已废弃 / 被 ADR-XXX 取代
- **背景**：为什么要做这个决定
- **决策**：选了什么
- **理由**：为什么选它
- **被否决的方案**：考虑过但没选的，以及原因
- **影响**：这个决定的后果
```

---

### ADR-001 — 以上游 v1.7.17 纯净快照作为基线

- **日期**：2026-09-17
- **状态**：已采纳
- **背景**：二开需要长期跟进上游更新，同时要能清楚区分"上游代码"与"我们的改动"
- **决策**：把未改动的上游快照作为仓库第一个提交 `3df1aff`，所有改动叠加在其之上；配置 `upstream` remote
- **理由**：保留完整上游历史锚点，可与上游做三方合并；`git diff 3df1aff..HEAD` 能精确列出我们的改动
- **被否决的方案**：直接 fork 上游仓库（会带入完整历史，仓库体积大且克隆慢；且上游历史对二开无实际价值）
- **影响**：不得 rebase / amend / 丢弃 `3df1aff`；上游更新用 `git merge upstream/main` 而非重新导入

### ADR-002 — 项目命名为 FlowSage

- **日期**：2026-09-17
- **状态**：已采纳
- **背景**：项目从"Web 控制台式渗透测试平台"转向"浏览器流量抓包 + CLI 交互式安全 Agent"，需要新名称
- **决策**：命名 **FlowSage**（Flow = 流量，Sage = 智者/智能体）
- **理由**：语义贴合"抓包 + AI 分析"；GitHub 上无同名仓库冲突；CLI 命令 `flowsage` 简短易记
- **被否决的方案**：WebTap（同类命名过多）、Pincer（与 Go 配置库重名）、TraceClaw（Claw 易与 Claude 混淆）
- **影响**：仓库 `tanglx02/FlowSage`；对外文案统一使用该名称；构建产物建议命名 `flowsage`

### ADR-003 — 仓库公开

- **日期**：2026-09-17
- **状态**：已采纳
- **背景**：用户选择公开
- **决策**：GitHub 仓库设为 **public**
- **理由**：用户明确要求
- **影响**：必须严守 `docs/dev/03-conventions.md` 第 1、9 节——保留 LICENSE/NOTICE、标注修改、不提交任何真实凭据与目标数据；含 C2/WebShell 等能力，需在 README 明确"仅限授权测试"

### ADR-004 — Windows 的 python3 用 venv 内解释器副本解决

- **日期**：2026-09-17
- **状态**：已采纳
- **背景**：`tools/*.yaml` 中 16 个配方以 `python3` 为命令；Windows 的 `python3` 指向微软商店应用执行别名，未安装商店版 Python 时不可用
- **决策**：在 `venv\Scripts\` 内复制 `python.exe` 为 `python3.exe`，并由启动器把该目录前置进 PATH
- **理由**：venv 内解释器按所在目录解析 `pyvenv.cfg`，副本仍属 venv 环境，能用到全部依赖；不污染系统目录，可随 venv 一起删除
- **被否决的方案**：
  - 在系统 Python 目录放 `python3.cmd`（`.cmd` 经 cmd.exe 转发会破坏含 `%`、`&`、`<>` 的内联 Python 脚本）
  - 修改系统 PATH 或在 WindowsApps 目录做手脚（侵入系统，且涉及系统设置）
- **影响**：所有工具执行必须通过 `run-windows.cmd` 启动（或自行确保 `venv\Scripts` 在 PATH 中）

### ADR-005 — `web/static/vendor/` 强制纳入版本控制

- **日期**：2026-09-17
- **状态**：已采纳
- **背景**：上游 `.gitignore` 的 `vendor/` 规则（Go 约定）同时匹配了前端目录 `web/static/vendor/`，导致 xterm / marked / cytoscape 等运行时依赖不会被提交
- **决策**：用 `git add -f web/static/vendor` 强制纳入，并在 `AGENTS.md` 中标注为"已知坑"
- **理由**：前端无构建步骤，这些库直接由 `index.html` 引用，缺失会导致页面功能异常；保证 clone 后开箱可用
- **被否决的方案**：修改 `.gitignore` 规则（会偏离上游文件，增加后续合并冲突）
- **影响**：该目录内容不要删除、不要"顺手修复"忽略规则；上游合并时若冲突保留双方

### ADR-006 — 换行符统一为 LF（`core.autocrlf=input`）

- **日期**：2026-09-17
- **状态**：已采纳
- **背景**：开发在 Windows，可能迁移到 Linux；Windows 默认 `autocrlf=true` 会在检出时转 CRLF，破坏 `*.sh` 的解释器行
- **决策**：仓库设 `core.autocrlf=input`，以 LF 提交
- **理由**：跨平台一致；避免 `run.sh` 之类脚本在 Linux 上出现 `^M` 报错
- **影响**：新机器克隆后若 git 报告大量改动，先检查本机 `core.autocrlf` 设置

### ADR-007 — Windows 使用 `run-windows.cmd` 而非上游 `run.sh`

- **日期**：2026-09-17
- **状态**：已采纳
- **背景**：上游 `run.sh` 是 bash 脚本，且在执行环境检查时（早于 venv 激活）就要求 `python3` 可用，在 Windows 上必然失败
- **决策**：新增 `run-windows.cmd`，负责 PATH 修复（`venv\Scripts` + Git `usr\bin`）后启动二进制
- **理由**：不改上游文件即可支持 Windows；PATH 修复同时解决 `python3` 与 `sh`（`tools/exec.yaml`）的解析问题
- **被否决的方案**：修改 `run.sh` 使其兼容 Windows（偏离上游文件，增加合并负担）
- **影响**：Windows 上一律用 `run-windows.cmd`；Linux/macOS 继续用 `run.sh`

### ADR-008 — 文档以 `AGENTS.md` 为单一事实源，各 IDE 只放薄入口

- **日期**：2026-09-17
- **状态**：已采纳
- **背景**：用户需要迁移到其它机器与其它 AI IDE 后无缝衔接，若为每个工具各写一份文档会产生多处重复、难以同步
- **决策**：`AGENTS.md` 作为唯一事实源；`.trae/rules/`、`.cursor/rules/`、`CLAUDE.md` 只写"指针"指向它；细节拆到 `docs/dev/*`
- **理由**：避免文档漂移；任何 IDE 只要读到入口就能找到完整上下文
- **影响**：新增约定只改 `AGENTS.md` 或 `docs/dev/*`；入口文件仅在 IDE 有特殊机制要求时才调整