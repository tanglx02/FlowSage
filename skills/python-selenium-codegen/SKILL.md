---
name: python-selenium-codegen
description: >-
  生成 Python 浏览器自动化脚本：Selenium/Playwright 显式等待、选择器常量表、操作步骤录制与回放、
  断言与截图留证。Use when turning an observed page workflow into a replayable automation script.
  Do not use for HTTP-only clients, and do not generate scripts that bypass authentication or CAPTCHA.
metadata:
  tags: [自动化, Selenium, Playwright, 脚本生成]
---

# 生成浏览器自动化脚本

把人在页面上做过的一串操作，变成可重复执行的脚本。

## 与 CLI 客户端的分工

| | CLI 客户端（F1） | 自动化脚本（F2） |
|---|---|---|
| 通道 | HTTP 接口 | 真实浏览器操作页面 |
| 适用 | 有稳定接口的功能 | 接口缺失、必须点页面、或需要"像人一样"操作 |
| 速度 | 快 | 慢但更接近真实使用 |

**优先判断能否用接口实现**：能走接口的不要生成浏览器脚本——慢且脆。

## 脚本结构（固定）

```
<workflow>_auto.py
├── SELECTORS      选择器常量表（集中定义，便于页面改版后统一修）
├── CONFIG         base_url / 会话 / 超时 / 截图目录
├── setup_driver() 浏览器初始化（含下载目录、窗口大小、无头开关）
├── step_*()       每个业务步骤一个函数
├── verify_*()     每步之后的断言
└── main()         编排 + 异常处理 + 截图留证
```

## 硬性要求

| 要求 | 说明 |
|---|---|
| **显式等待，禁用 sleep** | 用"等待某条件成立"，不要 `time.sleep(3)`。sleep 要么太短不稳、要么太长浪费时间 |
| 选择器集中在常量表 | 页面改版只改一处 |
| 选择器优先级 | `data-*` 属性 > 语义化 id > 可见文本 > CSS 路径（见 `page-structure-extraction`） |
| 每步有断言 | 否则失败时不知道错在哪一步 |
| 关键节点截图 | 失败时自动截图到 `截图目录/<时间戳>_<步骤>.png` |
| 会话复用 | 复用已有 profile 或注入 Cookie，**不实现自动登录** |
| 幂等与可重入 | 脚本中断后能安全重跑；破坏性操作前先确认当前状态 |

## 等待的写法参考

```python
# 好：等待明确的条件
WebDriverWait(driver, 15).until(
    EC.presence_of_element_located((By.CSS_SELECTOR, SELECTORS["alert_table_rows"]))
)

# 不好：靠猜时间
time.sleep(3)
```

动态渲染的页面还要等**接口响应完成**，而不只是元素出现（元素出现时数据可能还没填上）。

## 翻页与循环

从 `page-structure-extraction` 拿到分页控件形态：

- 页码式：循环点击下一页，直到"下一页"不可用，**同时比对首页与末页是否有重叠**（防止死循环）
- 无限滚动：滚动到底并等待新元素出现，设最大次数上限

循环必须有**终止条件与最大轮次**，不要写 `while True`。

## 异常处理

| 情况 | 处理 |
|---|---|
| 元素找不到 | 截图 + 输出当前 URL 与页面标题，便于定位 |
| 会话失效（跳登录页） | 明确提示重新获取会话，停止执行 |
| 出现预期外的弹窗 | 记录并截图，不盲目点掉 |
| 操作不可逆（删除/提交） | 执行前输出将要做什么，--yes 才真正执行 |

## 不要做的事

- 不生成自动登录 / 验证码识别
- 不在脚本里硬编码凭据
- 不用 `nth-child` 长链选择器
- 不对未授权目标生成脚本