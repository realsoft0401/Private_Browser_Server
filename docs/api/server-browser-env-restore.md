# Server Browser Env Restore

这份文档只说明中心正式接口：

- `POST /api/v1/browser-envs/{envId}/restore`

---

## 1. 业务语义

由 Node Server 发起一次中心 browser-env restore。

它不是同步恢复结果接口，而是：

- 中心先接单
- 转发到目标 Edge 正式 restore 接口
- 持续观察 Edge task
- 最后再把恢复后的 env 摘要同步回中心缓存

---

## 2. 它负责什么

- 读取 `server_browser_envs` 中心聚合记录
- 校验目标节点当前 `healthy + verified`
- 创建 `server_tasks`
- 发起目标 Edge `POST /api/v1/edge/browser-envs/{envId}/restore`
- 轮询 Edge task 终态
- 在成功或失败后回写 `server_browser_envs.lastTaskId / lastError / lastSyncedAt`

---

## 3. 它不负责什么

- 不上传外部 tgz 包
- 不替代 import-package
- 不自动 run
- 不绕过 Edge 正式 restore 校验

---

## 4. 当前请求体口径

当前正式不收请求体。

```http
POST /api/v1/browser-envs/906090001_tk_324867594169356288/restore
```

当前不允许：

- `clientId`
- `slotId`
- 任意 runtime 参数透传

---

## 5. 前置条件

必须同时满足：

1. `server_browser_envs` 能查到这条 env
2. env 已绑定到某个正式 `clientId`
3. 目标节点当前 `healthStatus=healthy`
4. 目标节点当前 `discoveryStatus=verified`

补充边界：

- restore 是否允许执行，由 Edge 正式 restore 协议决定
- 中心不自己重写备份包路径、目录冲突、身份一致性等资产规则

---

## 6. 状态机与收口

### 中心任务

- 发起成功时，先创建 `server_tasks`
- 任务终态只允许：
  - `success`
  - `failed`

### SSE 阶段

当前最小正式阶段包括：

- `load_server_env`
- `dispatch_edge_restore`
- `edge_task_accepted`
- `edge.<edge_stage>`
- `finalize_success`
- `finalize_edge_failed`
- `finalize_sync_failed`

### 成功判定

要同时满足：

1. Edge restore 接单成功
2. Edge task 终态为 `success`
3. Node 能再次读取 Edge `browser-env detail`
4. Node 能把新的恢复后摘要同步回 `server_browser_envs`

### 失败判定

任一环节失败都必须收口成 `failed`，包括：

- 目标节点不可达
- Edge restore 接单失败
- Edge task detail 查询失败
- Edge task 最终 failed
- Edge task success 但中心无法再次确认 env 事实

---

## 7. SSE 说明

SSE 任务化接口。立即返回 JSON `taskId/eventsUrl`。

发起接口只表示中心接单成功，不表示本次 restore 已经最终成功。

调用方必须继续订阅 `/api/v1/server-tasks/{taskId}/events`，观察 `task.progress`、
`task.completed`、`task.failed` 事件，才能确认本次 restore 的最终结论。

原因：

- restore 是明显多阶段长链路动作
- 包含远端解包恢复、归档删除和中心事实收口
- 普通同步 HTTP 不足以表达真实结果

---

## 8. 与相近接口的边界

它不会替代：

- `POST /api/v1/browser-envs/{envId}/backup`
  - backup 是正向归档动作
- `POST /api/v1/edge/browser-envs/{envId}/restore`
  - 这是 Edge 本机正式执行接口，不是中心接口
- `POST /api/v1/browser-envs/{envId}/refresh`
  - refresh 只同步摘要，不发 restore 动作
