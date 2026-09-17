# FlowSage 开发文档

这套文档的唯一目标是：**换一台机器、换一个 AI 工具，也能立刻接上开发，不丢上下文。**

## 怎么用

| 你的处境 | 先读 |
|---|---|
| 刚 clone 下来，要跑起来 | [01-environment.md](01-environment.md) |
| **想知道这项目要做什么、怎么做** | **[06-design.md](06-design.md)** |
| 要改代码，先搞清结构 | [02-architecture.md](02-architecture.md) |
| 准备动手，别踩规矩 | [03-conventions.md](03-conventions.md) |
| 不知道上一步做到哪了 | [04-progress.md](04-progress.md) |
| 想改某个设计，先看有没有定过 | [05-decisions.md](05-decisions.md) |

AI 工具请先读仓库根目录的 [`AGENTS.md`](../../AGENTS.md)，它是所有入口的汇总。

## 领域参考项目

`D:\project\python\态势感知告警处理`（Python 版 `alertctl`）：

- `技术功能实现文档.md` —— 站点接口契约、智能处置规则、终端兼容性结论，**需求细节都在这里**
- `.trae/skills/tsgz-alert-cli/` —— 该项目的项目级 skill
- `二区/`、`三区/alertctl/smart.py` —— **已验证的处置规则引擎，本项目复用不重写**

该项目与 FlowSage 是**协作关系**：它提供领域逻辑与既有资产，FlowSage 提供浏览器与 Agent 能力。

## 与上游文档的关系

| 目录 | 归属 | 能否改动 |
|---|---|---|
| `AGENTS.md`、`docs/dev/`、`scripts/`、`.trae/` | **FlowSage 自己的** | 随开发持续更新 |
| `docs/zh-CN/`、`docs/en-US/` | **上游原生文档** | 不要改，跟随上游合并更新 |

上游文档覆盖配置项、部署、安全模型、MCP 联邦、工作流等细节，二开时是重要的参考资料：

- `docs/zh-CN/architecture.md` — 上游完整架构设计
- `docs/zh-CN/configuration.md` — `config.yaml` 全量配置说明
- `docs/zh-CN/developer-guide.md` — 上游开发者指南
- `docs/zh-CN/mcp-federation.md` — 外部 MCP 联邦（接 Kali 工具链的关键）
- `docs/zh-CN/testing.md` — 测试说明

## 维护要求

开发过程中按 `docs/dev/03-conventions.md` 第 8 节同步更新对应文件。
**进度日志是硬性要求**——没有它，换机器就等于从零开始。