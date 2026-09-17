---
name: page-structure-extraction
description: >-
  从页面 DOM/HTML 提取结构化信息：表单字段与提交目标、表格列与数据源、可点击元素与选择器、
  页面与接口的调用关系。Use when you need stable selectors and page semantics for generated automation
  or CLI views. Do not use for scraping content as data collection.
metadata:
  tags: [DOM, 选择器, 页面结构]
---

# 页面结构提取

给两类产出物供料：自动化脚本需要**稳定选择器**，CLI 视图需要**字段的中文含义**
（接口字段名往往缩写严重，页面标签才是人话）。

## 提取内容

### 1. 表单

| 项 | 说明 |
|---|---|
| 字段清单 | `name`/`id`、类型、是否必填、默认值 |
| 标签文字 | `<label>` 或邻近文本 → **字段的中文含义** |
| 提交目标 | form 的 `action`，或按钮绑定的 JS 调用 → 推断对应的接口 |
| 校验规则 | `pattern`、`maxlength`、`required` |

表单字段与接口字段的**映射关系**最有价值：它把"`h_src_ip` 是什么意思"变成"源 IP"。

### 2. 表格 / 列表

- 列标题（表头文字）→ 对应响应字段的显示名
- 列与响应字段的顺序对应关系（前端常按顺序渲染）
- 分页控件形态（页码 / 无限滚动）→ 决定自动化脚本怎么翻页

### 3. 可点击元素与选择器

为自动化准备**稳定选择器**，按优先级：

| 优先级 | 选择器类型 | 原因 |
|---|---|---|
| 1 | `data-testid` / 自定义稳定属性 | 专为自动化准备 |
| 2 | 语义化 id | 稳定 |
| 3 | 可见文本（Playwright 的 `get_by_text`） | 可读，但文案改动会失效 |
| 4 | CSS 层级路径 | **最后手段**，结构一变就失效 |

**避免**：`nth-child` 长链、纯 class 组合（构建产物里的 class 名常是哈希，一变全废）。

### 4. 页面 ↔ 接口映射

观察"打开某页面时发起了哪些请求"，建立：

```
告警列表页 → queryAdvPage, alarmTypeStats, subtypes
告警详情页 → alarms/{id}, alarms/alarmReasonOptions
```

这张表让生成物能按"页面/功能"组织，而不是按接口罗列。

## 采集方式

| 方式 | 适用 |
|---|---|
| 托管浏览器 / CDP 连接 | 直接执行 JS 取 DOM，最完整 |
| 流量里的 HTML 响应 | 只能拿到初始 HTML，动态渲染的内容取不到 |

动态渲染（Vue/React）的页面**必须用真实浏览器取 DOM**，
不要试图从 HTML 响应里解析——拿到的只是空壳。

## 输出与交接

产出物：选择器常量表 + 字段映射表 + 页面接口映射表。

- 选择器表 → `python-selenium-codegen`
- 字段中文名 → `python-cli-codegen`（用于界面展示）
- 页面接口映射 → `web-app-domain-modeling`（用于划分视图）

## 注意

- 提取是为了**生成程序**，不是采集数据内容；不要把页面内容当数据抓走
- 选择器必须实测可用后再写进生成物，不要"看着像"就写
- 同一元素有多个候选选择器时，全部列出让使用者选