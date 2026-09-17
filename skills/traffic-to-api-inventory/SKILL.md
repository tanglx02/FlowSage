---
name: traffic-to-api-inventory
description: >-
  从抓包流量归纳目标 Web 应用的接口清单：过滤静态资源、端点归并、路径参数泛化、请求/响应结构推断、
  统一响应外壳与分页模式识别、鉴权方式判定。Use when analyzing captured traffic (HAR / DevTools extension / CDP)
  to produce an API inventory. Do not use for writing the client code itself.
metadata:
  tags: [流量分析, API清单, 接口逆向]
---

# 流量 → 接口清单

输入：归一化流量记录（HAR / 扩展回传 / CDP 捕获，结构一致）。
输出：结构化接口清单，作为代码生成（F1 / F2）的唯一输入。

## 执行顺序

**必须按此顺序**，每一步都过滤掉大量噪声，跳步会导致后面被静态资源淹没。

### 1. 过滤静态资源

只保留「可能是接口」的记录，命中以下任一即保留：

- 响应 `Content-Type` 含 `json` / `xml` / `javascript` 且 URL 含 `api` 等接口特征
- 请求方法为 `POST` / `PUT` / `PATCH` / `DELETE`
- URL 路径段含 `api`、`v1`/`v2`、`rest`、`graphql`、`rpc`
- 响应体是 JSON 对象/数组

剔除：`.js` `.css` `.png` `.jpg` `.svg` `.woff` `.ico` `.map` 等扩展名，以及 `Content-Type` 为
`text/css`、`image/*`、`font/*` 的记录。

> 经验值：真实站点 286 条流量里通常只有约 49 条是接口，**先过滤再看**。

### 2. 端点归并

同一逻辑端点会出现多次（分页、重试、不同参数）。归并键：**方法 + 泛化后的路径**。

### 3. 路径参数泛化

路径中出现以下形态的段，替换为 `{param}` 并在清单里记下参数名与来源：

| 形态 | 判定依据 | 示例 |
|---|---|---|
| UUID / 长十六进制 | 形如 `4c456157226143ffa1b4576cf38176c6` | `/alarms/{id}` |
| 纯数字 | 全数字段 | `/users/{id}` |
| 带前缀的标识 | `NO123`、`ORD-2024-001` | `/orders/{orderNo}` |

泛化依据：**同一位置在不同请求中出现过不同的值**。只出现一次的值不要急着泛化，
先标记为"疑似参数"，在清单里注明证据不足。

### 4. 请求结构推断

| 维度 | 做法 |
|---|---|
| 位置 | Query / Body(form) / Body(json) / Path / Header |
| 字段名 | 递归展开 JSON；表单按 `&` 拆分 |
| 类型 | `string` / `number` / `boolean` / `object` / `array`，数组看元素类型 |
| 必填 | 在**所有**同端点请求中都出现 → 标必填；部分出现 → 标可选 |
| 枚举候选 | 同字段取值集合很小（≤10）且重复出现 → 列出候选值 |
| 固定值 | 所有请求里取值恒定（如 `"pushOmsFlag":"0"`）→ 标注"疑似固定" |

### 5. 响应结构推断

- 递归展开 JSON，数组取**所有元素的字段并集**
- 字段路径用点号表示：`data.list[].alarmId`
- 记录字段出现率，低于 100% 的标可选

### 6. 统一外壳识别

很多系统在业务数据外再包一层。识别信号：多个不同端点的响应顶层键**完全一致**且包含
`code`/`status`/`success` 之一。常见形态：

```json
{"code": 200, "data": {...}, "msg": "ok"}
```

识别到后要记下：判定成功的条件（如 `code ∈ {0, 200}`）、失败时错误信息字段、业务数据所在字段。
**这层外壳必须写进清单**，否则生成物会在每个接口上重复解包逻辑。

### 7. 分页模式识别

| 模式 | 请求特征 | 响应特征 |
|---|---|---|
| 页码式 | `pageNo`/`pageNum` + `pageSize`/`limit` | `list`/`records`/`rows` + `total`/`totalCount` |
| 游标式 | `cursor`/`nextToken` | `nextCursor`/`hasMore` |
| offset 式 | `offset` + `limit` | 同上 |

记录：页码从 0 还是 1 开始（看实际请求值）、默认每页条数、总数上限。

### 8. 鉴权方式判定

| 方式 | 判定信号 |
|---|---|
| Cookie 会话 | 请求带 `Cookie`，且登录接口响应 `Set-Cookie` |
| Bearer Token | `Authorization: Bearer xxx` |
| OAuth2 | 存在 `/oauth2/token`、`/oauth2/publicKey` 类端点 → 单独标注取 token 流程 |
| 自定义头 | 恒定的 `X-*` 头（如 `X-BFF-Mode: true`）→ 标注为**业务必带头** |

**自定义头极易漏**：它是业务网关分流的依据，少了它整个客户端全挂。

### 9. 分组

按路径前缀归纳功能模块，例如：

```
/tsgz/nspt-tsgz-alarmquery/api/v1/alarms/*   → 告警查询
/tsgz/nspt-tsgz-system/sysDict/*             → 数据字典
/tsgz/nspt-tsgz-system/attchment/*           → 附件
```

分组名要能让人一眼看懂，用业务语义而非路径原文。

## 输出要求

清单里每条接口至少要有：方法、路径（含泛化参数）、业务名、请求结构、响应结构、
是否分页、鉴权要求、证据（来自哪几条流量）。

**不要编造**：流量里没出现的字段不要猜；推断出的枚举候选要标明"候选"而非"确定"。

## 收尾提醒

- 清单要能直接读出"这个接口是干什么的"，方法名如 `queryAdvPage` 比路径更有信息量
- 发现同一功能存在多个版本路径（`/v1/` 与 `/v2/`）要都保留并标注差异
- 完成后与 `session-auth-handling` 交接会话信息，与 `python-cli-codegen` 交接接口清单