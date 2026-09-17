# 进度日志

> **每次开发结束追加一条**，倒序排列（最新在最上面）。
> 目的：换机器、换 AI 工具后，读最近几条就能接上上下文。

## 记录格式

```markdown
### YYYY-MM-DD — <一句话主题>

**状态**：已完成 / 进行中 / 阻塞

**做了什么**
- ...

**涉及文件**
- `path/to/file` — 说明

**验证方式**
- 执行了哪些命令、结果如何

**遗留 / 下一步**
- ...
```

---

### 2026-09-17 — 项目准备：环境就绪 + 仓库初始化

**状态**：已完成

**做了什么**

- 确认上游 CyberStrikeAI v1.7.17 为 Apache-2.0 开源，可二开
- 摸清架构：Go 单体 + gin + SQLite + 纯静态前端，138 个工具（92 启用）
- 搭建完整运行环境（详见 `docs/dev/01-environment.md`）：
  - 创建 `venv`，安装 `requirements.txt` 全部依赖
  - 修复 Windows 上 `python3` 指向微软商店坏存根的问题（venv 内生成 `python3.exe` 副本）
  - 生成 `config.yaml` 并配置 AI 通道，实测接口连通
  - 新增 `run-windows.cmd` 启动器（把 `venv\Scripts` 与 Git `usr\bin` 前置到 PATH）
- 服务实测启动成功：登录成功、138 个工具全部加载
- 目录扁平化：`CyberStrikeAI-1.7.17\CyberStrikeAI-1.7.17\` → `FlowSage\`
- git 初始化：上游纯净基线 `3df1aff` + `upstream` remote
- 建立文档体系（本目录）与项目级 skill 包（`.trae/skills/flowsage-dev/`）

**涉及文件**

- `run-windows.cmd` — 新增，Windows 启动器
- `scripts/setup-windows.cmd` — 新增，环境一键重建
- `AGENTS.md` — 新增，AI 协作事实源
- `docs/dev/*` — 新增，开发文档体系
- `.trae/skills/flowsage-dev/` — 新增，项目级 skill
- `config.yaml` — 新建（已 gitignore，含 API Key）

**验证方式**

- `go build -o cyberstrike-ai.exe ./cmd/server` → 成功，产物 133 MB
- 服务启动后 `POST /api/auth/login` → 登录成功
- `GET /api/config/tools?page=1&page_size=100` → `total: 138`
- `venv\Scripts\python3.exe -c "import requests, impacket"` → 正常

**遗留 / 下一步**

- 等待确定 FlowSage 的详细设计（浏览器抓包 + CLI 交互式 Agent）
- 候选起点：`plugins/browser-extension/`（上游已有 DevTools 抓包扩展）、`cmd/`（新增 CLI 入口）
- Windows 上 70+ 原生渗透工具缺失，若需本机跑通需另装或用 Kali 侧 MCP 联邦