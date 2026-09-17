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

### 2026-09-18 — MVP-1 完成：浏览器托管 + 会话接管

**状态**：已完成（失效检测留到 MVP-2）

**做了什么**

- 引入 `github.com/chromedp/chromedp v0.16.0`（连带把 `go.mod` 的 Go 版本从 1.25.0 抬到 **1.26**，已按约定给 go.mod 加修改标注）
- 新增 `internal/browser/`：
  - 浏览器探测（Chrome → Edge → 便携版），覆盖 Windows / Linux / macOS
  - 独立实例管理：`--user-data-dir=tmp/browser/profile`，不影响日常浏览器、登录态跨重启保留
  - 进程脱离：Windows 先试 `CREATE_BREAKAWAY_FROM_JOB` 失败降级，Unix 用 `Setsid`；状态存 `tmp/browser/state.json`
  - CDP Cookie 提取（`Network.getCookies`），附最早过期时间计算
- 新增 `cmd/flowsage/` CLI：`browser list | open | status | cookie | close`
- 新增 alertctl 适配器：`--export-alertctl` 原地改写 `config.toml` 的 cookie 行，自动备份、保留 BOM

**涉及文件**

- `internal/browser/{browser,instance,cookie,detach_windows,detach_unix}.go` — 新增
- `cmd/flowsage/{main,browser,alertctl}.go` — 新增
- `go.mod` / `go.sum` — 新增依赖（带修改标注）
- `.gitignore` — 追加 `/flowsage`、`/flowsage.exe`
- `AGENTS.md`、`docs/dev/{01-environment,06-design}.md` — 同步命令、Go 版本、MVP 状态

**验证方式**

- `go build -o flowsage.exe ./cmd/flowsage` → 成功
- `flowsage browser list` → 正确探测到 Chrome（用户在 `%LOCALAPPDATA%` 的安装）与 Edge，Chrome 优先
- `flowsage browser open https://www.baidu.com` → 独立 profile 启动，调试端口就绪
- `flowsage browser cookie --url https://www.baidu.com` → **读到 8 条真实 Cookie**（含域名、过期时间），单行 Cookie 头生成正确
- `--export-alertctl <config.toml 副本>` → 第 12 行 cookie 被正确替换，其余行未动，`.bak` 已生成

**遗留 / 下一步**

- ⚠️ **两道待你在本机确认**：
  1. **浏览器脱离 CLI 存活**：在普通终端执行 `flowsage browser open <url>`，退出命令行后确认浏览器仍在。
     本次自动化验证环境把整条命令树放在 kill-on-close 的 Job 里，子进程一律被连带清理
     （早先用纯 PowerShell `Start-Process` 启动服务时现象相同），故无法在此环境验证。
  2. **企业策略是否禁用 CDP**：内网机器上确认 `--remote-debugging-port` 不被拦截。
- 会话失效检测（定期探测 `sysUser/me` 之类轻量接口后告警）留到 MVP-2 一并做
- 下一步：MVP-2 流量观测（CDP Network 全量捕获 → 落库 → 导出 HAR）

---

### 2026-09-18 — 确定二开方向，产出设计说明（尚未写代码）

**状态**：已完成

**做了什么**

- 通读领域参考项目 `D:\project\python\态势感知告警处理`（技术功能实现文档 + tsgz-alert-cli skill + 双站点代码布局）
- 确定 FlowSage 定位：**由浏览器驱动的安全运营 Agent**（托管浏览器 / 会话接管 / 流量观测 / Selenium 级页面操控 / 接口理解 / 任务执行）
- 与用户敲定四项关键决策（技术栈、运行环境、话术来源、能力范围），全部记入 ADR-009 ~ ADR-013
- 产出 [`06-design.md`](06-design.md)：能力清单、架构图、规则引擎集成契约、MVP-1~4 分阶段计划、风险与待明确项
- 环境探测结论：本机无 Chrome，**有 Edge 153**（Chromium 内核，CDP 可用）；Go 模块缓存中尚无浏览器自动化库
- 补充 [`07-portable-browser.md`](07-portable-browser.md)：三来源（官方 Chromium 快照 / Chrome for Testing / npmmirror 国内镜像）的**分系统下载地址全部实测通过**，含自检步骤与常见问题
- 按用户澄清修正定位：**FlowSage 是通用能力层**，态势感知告警处置只是首个落地场景，不是项目边界

**涉及文件**

- `docs/dev/06-design.md` — 新增，需求与架构的单一来源
- `docs/dev/07-portable-browser.md` — 新增，便携浏览器下载与自检
- `docs/dev/05-decisions.md` — 新增 ADR-009 ~ ADR-013
- `AGENTS.md` — 更新项目定位与文档索引，补充领域参考项目说明
- `docs/dev/README.md` — 索引补充设计说明与浏览器下载
- `.gitignore` — 新增 `/browser/`（按约定带 `Modified by FlowSage` 标注）

**验证方式**

- 本阶段为设计，无代码改动，未执行构建/测试

**遗留 / 下一步**

- **待实施 MVP-1**：浏览器托管 + 会话接管（独立 profile、人工登录、自动提取 Cookie、失效检测），验收标准是能替代"手工复制 Cookie"
- 需引入第一个新依赖：`github.com/chromedp/chromedp`
- 待实测：Edge 的 CDP 是否被企业策略限制
- `06-design.md` 第 9 节"待明确"四项需后续讨论（流量保留策略、接口清单产出形态、验证码交互、多站点并行）

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
- 安装 `gh` CLI（2.101.0）并完成 GitHub 授权（账号 `tanglx02`），配置 git 凭据助手
- **创建公开仓库并推送**：https://github.com/tanglx02/FlowSage （默认分支 `main`）
- 建立文档体系（本目录）与项目级 skill 包（`.trae/skills/flowsage-dev/`）

**涉及文件**

- `run-windows.cmd` — 新增，Windows 启动器
- `scripts/setup-windows.cmd` — 新增，环境一键重建
- `AGENTS.md` — 新增，AI 协作事实源
- `docs/dev/*` — 新增，开发文档体系
- `.trae/skills/flowsage-dev/` — 新增，项目级 skill
- `.trae/rules/`、`.cursor/rules/`、`CLAUDE.md` — 新增，各 AI IDE 薄入口
- `NOTICE` — 新增，声明衍生作品与修改标注约定
- `config.yaml` — 新建（已 gitignore，含 API Key）

**验证方式**

- `go build -o cyberstrike-ai.exe ./cmd/server` → 成功，产物 133 MB
- 服务启动后 `POST /api/auth/login` → 登录成功
- `GET /api/config/tools?page=1&page_size=100` → `total: 138`
- `venv\Scripts\python3.exe -c "import requests, impacket"` → 正常
- `scripts\setup-windows.cmd` → 幂等执行通过
- `gh repo view tanglx02/FlowSage` → `PUBLIC`，两个提交均已推送

**遗留 / 下一步**

- 等待确定 FlowSage 的详细设计（浏览器抓包 + CLI 交互式 Agent）
- 候选起点：`plugins/browser-extension/`（上游已有 DevTools 抓包扩展）、`cmd/`（新增 CLI 入口）
- Windows 上 70+ 原生渗透工具缺失，若需本机跑通需另装或用 Kali 侧 MCP 联邦
- 公开仓库尚无 README（当前用的是上游 README），需补一份说明"这是衍生项目 + 目标形态"的对外说明