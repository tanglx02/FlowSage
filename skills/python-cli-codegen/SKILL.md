---
name: python-cli-codegen
description: >-
  把接口清单生成可维护的 Python CLI/TUI 客户端：httpx 会话层、归一化模型、数据源分层、
  Textual 全屏界面与纯文本保底模式、TOML 配置。Use when generating the command-line client for a target web app.
  Do not use for Selenium/browser automation scripts, and do not invent interfaces absent from the inventory.
metadata:
  tags: [代码生成, Python, CLI, TUI]
---

# 生成 Python CLI 客户端

目标：让原本只能在网页上点的操作，变成命令行可交互的程序。
**分层必须严格照搬模板**，不要每次自创结构——可维护性来自一致性。

## 目录结构（固定）

```
<app>ctl/
├── cli.py        入口、参数解析、自检
├── config.py     TOML 配置加载与校验
├── models.py     归一化模型与枚举
├── session.py    HTTP 会话、Cookie 复用、统一拆包
├── sources/
│   ├── base.py   数据源抽象（接口）
│   ├── web.py    真实站点实现（由接口清单生成）
│   └── mock.py   演示数据源（离线可跑）
├── tui.py        Textual 全屏界面
└── plain.py      纯文本交互（终端兼容保底）
config.example.toml
requirements.txt
```

## 生成顺序

1. **config.py + config.example.toml**：`base_url`、`verify_tls`、`timeout`、`cookie`、
   `page_size`、以及每个接口的路径常量。路径**必须可配置**，换站点/换版本时只改配置。
2. **models.py**：从响应结构反推领域模型。字段名保留站点原义（如 `alarmId`），
   但**在归一化层转成可读字段**（`id` / `title` / `severity`），界面层只认归一化模型。
3. **session.py**：统一处理
   - 自签名证书 `verify=False`、超时、UA/Accept 头
   - **统一外壳拆包**（清单里识别到的 `{code,data,msg}` 层）：成功返回 `data`，失败抛业务异常
   - `401/403` 或业务码表示未登录 → 抛 `SessionExpired`，界面提示重新获取会话
   - 业务必带的固定头（如 `X-BFF-Mode: true`）
4. **sources/web.py**：一个接口一个方法。方法名用**业务语义**（`fetch_alerts`），
   不要用路径原文（`queryAdvPage`）当方法名，路径放进常量。
5. **sources/mock.py**：造可离线跑的样例数据。**必须有**——它让程序在无网络、
   无凭据时也能演示与自测，也是验证生成物的手段。
6. **plain.py + tui.py**：先文本模式再全屏。文本模式不依赖任何终端特性，是保底方案。

## 硬性要求

| 要求 | 原因 |
|---|---|
| 分页统一由 `sources` 层处理，界面不碰 | 界面层不应理解站点分页协议 |
| 所有请求走 `session.py` | 鉴权、拆包、错误处理只有一处实现 |
| 批量操作逐条提交且单条失败不中断整批 | 站点是否支持批量提交需实测确认，先按最保守方式实现 |
| 提交类操作前先校验必要前置条件（如附件文件存在） | 避免"每条都失败一轮" |
| 中文直接可输入，不依赖终端特性 | Windows 终端对中文输入支持差，提供剪贴板粘贴与外部编辑器回填 |

## 配置与凭据

- 配置优先级：**环境变量 > 配置文件 > 内置默认**
- 凭据**推荐走环境变量**（如 `<APP>_COOKIE`），避免明文落盘
- 配置文件读取要兼容**记事本另存产生的 BOM**（`tomllib` 遇 BOM 会直接报错）

## requirements.txt

只列真实需要的包，默认最小集：

```
httpx>=0.27
textual>=0.80
```

避免引入需要编译的依赖——现场机器常常不能联网装东西。

## 交付前自检

```bash
python -m <app>ctl --check          # 连通性 + 会话有效性 + 关键接口可用
python -m <app>ctl --mock           # 离线演示，不产生任何真实请求
```

两项都必须能跑通。自检逻辑见 `codegen-verification`。

## 不要做的事

- 不要生成自动登录（图形验证码必须人工）
- 不要凭猜测补接口——清单里没有的不生成
- 不要把站点原始字段名直接暴露给界面层