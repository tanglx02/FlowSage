# FlowSage Skills

本目录是 **FlowSage Agent 的运行时 Skill 库**，由 Agent 在处理任务时按需加载。
内容已按「Web 应用 → Python 程序」的产品定位整体替换，不含渗透测试相关内容。

## Skill 清单

| Skill | 用途 | 服务于 |
|---|---|---|
| [traffic-to-api-inventory](traffic-to-api-inventory/SKILL.md) | 从抓包流量归纳接口清单（过滤、归并、结构推断、外壳/分页/鉴权识别） | **F3** |
| [web-api-recon](web-api-recon/SKILL.md) | 补全流量里没出现的接口（菜单、前端 JS、命名规律） | **F3** |
| [web-app-domain-modeling](web-app-domain-modeling/SKILL.md) | 接口清单 → 实体/视图/操作的领域模型 | F1 前置 |
| [python-cli-codegen](python-cli-codegen/SKILL.md) | 生成 Python CLI/TUI 客户端（分层固定，对齐 alertctl 规范） | **F1** |
| [page-structure-extraction](page-structure-extraction/SKILL.md) | 从 DOM 提取表单、表格、稳定选择器、页面↔接口映射 | F1 / F2 前置 |
| [python-selenium-codegen](python-selenium-codegen/SKILL.md) | 生成浏览器自动化脚本（显式等待、选择器表、断言、截图） | **F2** |
| [session-auth-handling](session-auth-handling/SKILL.md) | Cookie/Bearer/OAuth2 会话处理、失效检测、验证码人工介入 | 三者公共 |
| [codegen-verification](codegen-verification/SKILL.md) | 生成物的四道验证关卡 | 三者收尾 |

> F1 = 流量 → CLI/TUI 程序，F2 = 浏览器 → 自动化程序，F3 = API 分析。
> 定义见 [docs/dev/06-design.md](../../docs/dev/06-design.md) §3。

## 推荐执行链路

```
流量接入
   ↓
traffic-to-api-inventory ──→ web-api-recon（补全）──→ 人工确认
   ↓
web-app-domain-modeling
   ↓
   ├─→ python-cli-codegen ──┐
   └─→ page-structure-extraction ─→ python-selenium-codegen ─┤
                                                             ↓
                                              session-auth-handling（贯穿）
                                                             ↓
                                                 codegen-verification
```

## 编写约定

- frontmatter 必填 `name`（与目录同名）与 `description`（含"做什么 + 何时使用 + 不适用什么"）
- 正文写给 **Agent** 看，是可执行的操作指导，不是给人读的说明文档
- 涉及产出的，要写清**输出形态与交接对象**（下一个 skill 需要什么）
- 硬性约束（不许猜、不许编造、不许自动登录）必须显式写出

## 已废弃

原上游的 22 个渗透测试 skill（`web-attack-methods`、`post-exploitation`、`cloud-attack-methods` 等）
已删除。需要查阅时从 git 历史取回：

```bash
git show 3df1aff:skills/web-attack-methods/SKILL.md
```