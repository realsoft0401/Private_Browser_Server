# Server Browser Env Run

这份文档只说明中心正式接口：

- `POST /api/v1/browser-envs/{envId}/run`

---

## 1. 业务语义

由 Node Server 发起一次中心 browser-env run。

它不是直接把请求转发给 Edge 就结束，而是要先经过中心准入，再创建中心任务，再由中心收口最终成功或失败。

---

## 2. 它负责什么

- 读取 `server_browser_envs` 中心聚合记录
- 读取 env 对应的 `clientId`
- 执行统一中心 run admission
- 显式把 `slotId` 透传给目标 Client
- 创建 `server_tasks`
- 用中心 SSE 返回过程与终态
- 在成功或失败后回写 `server_browser_envs.lastTaskId / lastError / lastSyncedAt`

---

## 3. 它不负责什么

- 不自动选 slot
- 不直接拉平台 quota
- 不直接修改 `target_slot_count`
- 不跨 Client 自动迁移 env
- 不替代 Edge 自身 `run` 校验

---

## 4. 当前请求体口径

```json
{
  "slotId": "slot001",
  "forceRecreate": false
}
```

当前正式只收这两个字段：

- `slotId`
  - 必填
  - Node 当前阶段不自动选 slot
- `forceRecreate`
  - 选填
  - 只是透传给 Edge 正式 run 协议

明确不允许：

- `clientId`
- `image`
- `proxy`
- `fingerprint`
- 任意 Docker 参数透传

---

## 5. 前置条件

必须同时满足：

1. `server_browser_envs` 能查到这条 env
2. env 已绑定到某个正式 `clientId`
3. 目标节点当前 `healthStatus=healthy`
4. 目标节点当前 `discoveryStatus=verified`
5. `slotExceptionStatus=normal`
6. `availableSlotCount > 0`
7. 有可信 quota 快照，且：
   - `status=valid`
   - `quotaLimit > 0`
   - `quotaAvailableSnapshot > 0`
   - `expiresAt` 未过期

只要任一条件不满足，本次中心 run 必须 failed。

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
- `dispatch_edge_run`
- `edge_task_accepted`
- `edge.<edge_stage>`
- `finalize_success`
- `finalize_admission_failed`
- `finalize_edge_failed`
- `finalize_sync_failed`

### 成功判定

要同时满足：

1. 中心准入通过
2. Edge run 接单成功
3. Edge task 终态为 `success`
4. Node 能再次读取 Edge `browser-env detail`
5. Node 能把新的运行摘要同步回 `server_browser_envs`

### 失败判定

任一环节失败都必须收口成 `failed`，包括：

- 中心准入失败
- Edge run 接单失败
- Edge task detail 查询失败
- Edge task 最终 failed
- Edge task success 但中心无法再次确认 env 事实

---

## 7. SSE 说明

SSE 任务化接口。立即返回 JSON `taskId/eventsUrl`。

发起接口只表示中心接单成功，不表示本次 run 已经最终成功。

调用方必须继续订阅 `/api/v1/server-tasks/{taskId}/events`，观察 `task.progress`、
`task.completed`、`task.failed` 事件，才能确认本次 run 的最终结论。

原因：

- 它天然是长链路
- 包含中心判断 + 远端执行 + 终态确认
- 普通同步 HTTP 不足以表达真实结果

---

## 8. 与相近接口的边界

它不会替代：

- `GET /api/v1/edge-clients/{clientId}/run-quota`
  - 只看准入，不发业务动作
- `POST /api/v1/edge-clients/{clientId}/slot-reconcile`
  - 只刷新 slot 事实，不发业务动作
- `POST /api/v1/edge/browser-envs/{envId}/run`
  - 这是 Edge 本机正式执行接口，不是中心接口
