# Server Browser Env Delete Image

这份文档只说明中心正式接口：

- `DELETE /api/v1/browser-envs/{envId}/del`

---

## 1. 业务语义

由 Node Server 发起一次中心 browser-env `/del`。

它只做镜像清理：

- 读取当前中心 env 聚合记录
- 调用目标 Edge 正式 `/del`
- 返回镜像删除结果

它不会删除环境包资产，也不会从中心列表移除这条 env。

---

## 2. 它负责什么

- 读取 `server_browser_envs` 中心聚合记录
- 校验目标节点当前 `healthy + verified`
- 同步调用目标 Edge `DELETE /api/v1/edge/browser-envs/{envId}/del`
- 回写 `server_browser_envs.lastError / lastSyncedAt`

---

## 3. 它不负责什么

- 不创建 `server_tasks`
- 不要求调用方订阅 SSE
- 不删除中心 env 缓存
- 不删除 Edge 环境目录
- 不自动 stop

---

## 4. 当前请求体口径

当前正式不收请求体。

```http
DELETE /api/v1/browser-envs/906090001_tk_324867594169356288/del
```

---

## 5. 前置条件

必须同时满足：

1. `server_browser_envs` 能查到这条 env
2. env 已绑定到某个正式 `clientId`
3. 目标节点当前 `healthStatus=healthy`
4. 目标节点当前 `discoveryStatus=verified`

补充边界：

- env 是否允许 `/del`，由 Edge 正式 `/del` 协议决定
- 中心不在这里重写边缘的运行态、备份态或镜像占用校验

---

## 6. 状态机与收口

### 中心 env 缓存

成功后：

- `server_browser_envs.status` 不变
- `server_browser_envs.runtimeStatus` 不变
- `server_browser_envs.lastError` 清空
- `server_browser_envs.lastSyncedAt` 更新时间

失败后：

- 保留原主状态
- `lastError` 记录最近一次 `/del` 失败信息

### 成功判定

要同时满足：

1. Edge `/del` 同步返回成功
2. Node 能成功回写中心 env 摘要的最近错误与同步时间

### 失败判定

任一环节失败都必须直接同步失败，包括：

- 中心 env 不存在
- 目标节点不可达
- Edge `/del` 返回失败
- 中心回写 `lastSyncedAt / lastError` 失败

---

## 7. SSE 说明

- 本接口不使用 SSE
- 原因：当前 `/del` 是镜像清理同步动作，普通 HTTP 足够表达最终结果

---

## 8. 与相近接口的边界

它不会替代：

- `DELETE /api/v1/browser-envs/{envId}/package`
  - package delete 是彻底删环境资产
- `DELETE /api/v1/edge/browser-envs/{envId}/del`
  - 这是 Edge 本机正式执行接口，不是中心接口
