---
name: session-auth-handling
description: >-
  处理目标站点的会话与鉴权：Cookie/Bearer/OAuth2 的获取与复用、失效检测、
  验证码人工介入点、业务必带固定头。Use when生成或调试客户端的登录态与会话逻辑。
  Do not use for bypassing authentication or automating CAPTCHA solving.
metadata:
  tags: [会话, 鉴权, Cookie, OAuth2]
---

# 会话与鉴权处理

三个功能（CLI 生成 / 自动化生成 / 接口分析）都依赖同一套会话处理，因此单独成规。

## 三种鉴权形态

| 形态 | 识别 | 客户端处理 |
|---|---|---|
| Cookie 会话 | 请求带 `Cookie`，登录响应有 `Set-Cookie` | 从流量/浏览器提取整行 Cookie，放进配置或环境变量 |
| Bearer Token | `Authorization: Bearer xxx` | Token 单独存放；注意有效期与刷新 |
| OAuth2 | 存在 `/oauth2/token`、`/oauth2/publicKey` 端点 | 需还原取 token 流程（grant_type、参数、返回结构） |

**OAuth2 要单独对待**：它意味着访问令牌有明确的生命周期，客户端必须实现过期后的重新获取，
而不是像 Cookie 那样一直复用。清单里若出现 `oauth2/token`，必须把它的请求体与响应结构完整写进生成物。

## 验证码：必须人工介入

登录环节若有图形验证码：

- **不做自动识别、不做绕过**——不可靠且不合规
- 正确做法是"浏览器登录 → 提取会话 → 注入客户端"
- 提取通道见 `docs/dev/06-design.md` §4：浏览器扩展 / 托管浏览器 / CDP 连接 / HAR 导入

生成的程序要在会话失效时给出**明确可执行的指引**，而不是抛一个 401 就结束：

```
当前会话已失效。
处理：在 FlowSage 的「流量」页重新提取会话，或用 flowsage browser cookie 重新导出，
然后更新配置中的 cookie（或环境变量 <APP>_COOKIE）。
```

## 失效检测

不要等业务请求失败了才发现会话过期。做法：

1. 启动时调一个**轻量的鉴权接口**（`sysUser/me`、`getVersionInfo` 这类）探活
2. 运行中收到 `401/403` 或业务码表示未登录 → 立即标记失效并停止后续提交
3. 记录会话有效期（站点常在响应里给出 `session_ttl_seconds` 之类的值），到期前提醒

## 业务必带的固定头

流量里恒定出现的 `X-*` 头往往是**网关分流依据**（如 `X-BFF-Mode: true`），
缺失会导致全部请求异常。识别与落地：

- 在接口清单里标注为**全局必带头**
- 在 `session.py` 里统一注入，不要散落在各接口方法里
- 生成配置项允许覆盖（换版本时策略可能变化）

## 凭据存放

| 位置 | 适用 | 说明 |
|---|---|---|
| 环境变量 | **推荐** | 如 `<APP>_COOKIE`，不落盘 |
| 配置文件 | 现场便利 | 必须同时提供"用环境变量"的选项 |

生成物**不得**把凭据硬编码进源码。

## 交接

- 会话获取方式 → 与 Web 控制台的「流量」页对接
- 失效处理逻辑 → 由 `python-cli-codegen` / `python-selenium-codegen` 调用
- 自检入口 → `codegen-verification` 的 `--check` 必须覆盖会话探活