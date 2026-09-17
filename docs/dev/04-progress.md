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

### 2026-09-18 — P1 收口：Web 控制台「流量分析」页（HAR 导入 → 接口清单）

**状态**：已完成，服务已在本机 8080 端口跑起来

**做了什么**

- **DB**：新增 `api_inventories` 表（`CREATE TABLE IF NOT EXISTS`，含 host / 记录数 / 接口数 / payload JSON / 归属用户）与 `internal/database/traffic.go` 的四个访问方法
- **Handler**：新增 `internal/handler/traffic.go`
  - `POST /api/traffic/har`：上传 → `trafficparse.ParseHAR` → `apianalyze.Analyze` → 落库 → 返回完整分析结果（上限 128 MB）
  - `GET /api/traffic/inventories`（按项目过滤 + 分页，**不返回 payload**）
  - `GET /api/traffic/inventories/:id`（返回完整分析结果）
  - `DELETE /api/traffic/inventories/:id`
  - 归属校验：全局范围可见全部，其余只能看到/删除自己的清单
- **RBAC**：`PermissionCatalog` 新增 `traffic:read` / `traffic:write` / `traffic:delete`，并在 `permissionForRequest` 里映射 `/api/traffic`（不加映射会被拦截为"未配置访问权限"403）
- **前端**（纯静态，无构建）：新增导航「流量分析」+ 页面 + `web/static/js/traffic.js` + `web/static/css/traffic.css`
  - 左侧清单列表、右侧详情：统一外壳卡片、鉴权与固定头卡片、按模块过滤的接口表、接口详情（查询参数、请求/响应字段树、分页）、导出 JSON、删除

**涉及文件**

- `internal/database/{traffic.go,database.go}` — 新增表与访问方法
- `internal/handler/{traffic.go,traffic_test.go}` — 新增
- `internal/app/app.go` — 注册 `/api/traffic` 路由
- `internal/security/{rbac.go,rbac_middleware.go}` — 新增权限与路由映射
- `web/templates/index.html`、`web/static/js/{router.js,traffic.js}`、`web/static/css/traffic.css` — 前端
- `docs/dev/06-design.md` — P0/P1 状态更新

**验证方式**

- `go build ./...` 通过；`go test ./internal/handler/ -run TestTraffic` 2 个用例通过
  （覆盖：静态资源过滤、端点归并、路径参数泛化、外壳识别 `code == 200`、分页识别、越权拒绝、删除后 404）
- `go test ./internal/security/...` 中 `TestEveryProtectedRouteHasCatalogPermission` 通过（新路由都有目录内权限）
- 真实 HAR 回归：`flowsage analyze 192.169.20.251.har` → 286 条 → 49 条候选 → **19 个接口**，外壳 19/19，`x-bff-mode: true`
- 服务实测：`run-windows.cmd --http` 启动，`GET /` 200、新页面静态资源 200、导航含入口、`/api/traffic/inventories` 未带 token 返回 401

**遗留 / 下一步**

- 清单暂未支持人工编辑/确认（设计里 P1 提到"可看可编辑"，编辑留到 P2 之前补）
- 上传时未绑定项目（页面暂无项目下拉），`project_id` 目前靠接口预留
- **P2 起点**：接口清单 → 生成 Python CLI 客户端（以 alertctl 为模板），另建议先补扩展/CDP 通道把流量来源补齐

---

### 2026-09-18 — P1 核心：流量解析 + API 分析引擎（已用真实 HAR 验证）

**状态**：引擎完成并验证；Web 层待接

**做了什么**

- 新增 `internal/trafficparse`：HAR 1.2 解析、静态资源过滤、请求头归一化（剔除噪声头）、四种通道共用的归一化记录模型
- 新增 `internal/apianalyze`：九步分析链——过滤 → 端点归并 → 路径参数泛化 → 请求结构 → 响应结构（**递归展开**）→ 统一外壳 → 分页模式 → 鉴权与必带头 → 分组
- `cmd/flowsage` 新增 `analyze` 子命令（内部验证用），支持 `--json` 输出完整清单
- 新增 6 个单测，守住三个曾出错的不变量：数值格式、外壳识别、外壳内的分页识别

**真实数据验证结果**（`192.169.20.251.har`）

| 项 | 结果 |
|---|---|
| 流量过滤 | 286 条 → 49 条接口候选（滤掉 237 条静态资源） |
| 接口识别 | **19 个**（人工整理版只有 9 个） |
| 统一外壳 | `{code, data, msg}`，成功判定 `code == 200`，覆盖率 **19/19** |
| 业务必带头 | `x-bff-mode: true`（缺失会导致全部请求异常） |
| 鉴权 | oauth2，取 token 路径 `/tsgz/oauth2/token` |
| 分页 | `queryAdvPage` → `pageNo/pageSize` + `data.list/data.total` |
| 嵌套结构 | `data.list[]` 元素展开出 **45 个字段**（告警记录的完整结构） |
| 路径参数泛化 | `/alarms/4c4561...` → `/alarms/{id}` |

**过程中修掉的 4 个 bug**

1. `trimFloat` 对整数误用去尾零，把 `200` 变成 `2`、`150` 变成 `15`（影响所有数值字段）
2. 外壳覆盖率把"字段数"当成"接口数"打印，显示成 3/19
3. 分页只看请求侧，把共用查询模板的统计接口误判为分页接口
4. 结构推断不递归，导致藏在 `data` 里的 `list/total` 完全看不到（分页因此全漏）

**涉及文件**

- `internal/trafficparse/{record,har}.go` — 新增
- `internal/apianalyze/{analyze,endpoint,schema,analyze_test}.go` — 新增
- `cmd/flowsage/{main,analyze}.go` — 新增 analyze 子命令

**验证方式**

- `go test ./internal/apianalyze/...` → 6 个用例全部通过
- `go vet` 与 `go build ./...` → 通过
- `flowsage analyze <真实 HAR>` → 输出与上表一致

**遗留 / 下一步**

- **Web 层**：HAR 上传接口 + 接口清单页面（产品交互面，用户要求不用命令行）
- 清单落库（新增 `api_inventories` 表）与人工确认/编辑能力
- P2：基于清单生成 Python CLI 客户端

---

### 2026-09-18 — 定位二次校准：改为 Web 端「Web 应用 → Python 程序」工具，Skills 整体替换

**状态**：P0 完成（文档 + skills）；P1 起待开发

**做了什么**

- **定位校准**（用户澄清）：产品是 **Web 端 Agent**，操作全在 Web 控制台，不用命令行；三个功能点：
  F1 流量 → Python CLI/TUI 客户端、F2 浏览器 → Selenium 式自动化脚本、F3 API 分析
- 重写 [06-design.md](06-design.md)：三功能定义、四种流量通道对比、产出物规范、架构分层、精简后控制台菜单、8 个 skill 清单、P0~P4 分阶段计划
- **Skills 整体替换**：删除全部 22 个渗透 skill（31 个文件），新写 8 个功能向 skill + 索引 README
- 新增 ADR-014 ~ ADR-018：Web 端形态、产品定位、四种流量通道优先级、skills 替换、控制台精简
- **真实数据摸底**：分析 `192.169.20.251.har`（286 条流量）→ 过滤出 **19 个真实接口**
  - 其中 `oauth2/token`、`oauth2/publicKey`、`sysMenu/routes`、`alarmLevelStats`、`getNewestQuestionInfoByType` 等
    **是当初人工整理时遗漏的**（人工版只覆盖 9 个）
  - 这组数据成为 P1 分析引擎的开发与验收样本

**涉及文件**

- `docs/dev/06-design.md` — 整体重写
- `skills/` — 删除 22 个渗透 skill，新增 8 个功能 skill 与 README
- `docs/dev/05-decisions.md` — 新增 ADR-014 ~ ADR-018
- `AGENTS.md` — 更新定位、目录地图（标注 roles/agents 待替换）

**验证方式**

- 本阶段为文档与 skill 编写，无代码改动
- HAR 分析用 PowerShell 完成，接口清单已核对（19 条，含方法/路径/状态/体积）

**遗留 / 下一步**

- **P1（下一步）**：HAR 导入 → 归一化流量 → 接口清单，Web 页面可看可编辑；
  验收标准是用该 HAR 跑出清单，覆盖人工整理的 9 个接口且能多找出遗漏项
- 控制台精简（摘掉渗透路由与菜单）待做
- `roles/`（14 个）与 `agents/`（16 个）仍为渗透向内容，待替换
- 生成物存放位置、生成物差异对比等 2 项待明确（见 06-design §10）

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