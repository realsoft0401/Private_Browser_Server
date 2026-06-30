# Server Browser Env Query APIs

这份文档只说明三条中心 browser-env 查询 / 刷新接口：

- `GET /api/v1/browser-envs`
- `GET /api/v1/browser-envs/{envId}`
- `POST /api/v1/browser-envs/{envId}/refresh`

---

## 1. GET /api/v1/browser-envs

### 业务语义

返回 Node Server 当前缓存的 browser-env 列表。

### 它负责什么

- 查询 `server_browser_envs`
- 支持按下面字段过滤：
  - `accountId`
  - `clientId`
  - `userId`
  - `rpaType`
  - `status`

### 它不负责什么

- 不自动调用 Edge
- 不自动刷新 env 缓存
- 不返回 Edge 原子资产正文

### SSE 说明

- 不使用 SSE
- 原因：它是普通中心查询接口，同步 HTTP 已足够表达结果

---

## 2. GET /api/v1/browser-envs/{envId}

### 业务语义

返回中心缓存中的单条 browser-env 聚合摘要。

### 它负责什么

- 查询 `server_browser_envs`
- 返回中心当前主视图

### 它不负责什么

- 不自动调用 Edge
- 不自动刷新缓存
- 不返回 profile / binding / proxy / browser-data 原文

### SSE 说明

- 不使用 SSE
- 原因：它是普通中心查询接口，同步 HTTP 已足够表达结果

---

## 3. POST /api/v1/browser-envs/{envId}/refresh

### 业务语义

让 Node Server 主动向目标 Edge 拉一次该 env 的 detail，并刷新中心缓存。

### 它负责什么

- 校验这条 env 已经存在于中心
- 校验目标节点当前 `healthy + verified`
- 调 Edge `GET /api/v1/edge/browser-envs/{envId}`
- 回写 `server_browser_envs`

### 它不负责什么

- 不自动发现新的 env
- 不创建 server task
- 不发起 run / stop / backup / restore

### SSE 说明

- 不使用 SSE
- 原因：它只是单条 env 的单次同步拉取，同步 HTTP 已足够表达结果

---

## 4. 三条接口的关系

- `GET /browser-envs`
  - 看中心当前 env 列表
- `GET /browser-envs/{envId}`
  - 看中心当前单条 env 摘要
- `POST /browser-envs/{envId}/refresh`
  - 主动拉新这条 env 的中心缓存

它们不会替代：

- `POST /api/v1/browser-envs/{envId}/run`
- `POST /api/v1/edge/browser-envs/{envId}/run`
