# Server Run Quota APIs

这份文档只说明两条新的 Node Server run quota / admission 治理接口边界：

- `GET /api/v1/edge-clients/{clientId}/run-quota`
- `POST /api/v1/edge-clients/{clientId}/run-quota/refresh`

---

## 1. GET /api/v1/edge-clients/{clientId}/run-quota

### 业务语义

返回 Node Server 当前缓存的最近一次平台 run quota 快照，并附带当前 run admission 判断。

### 它负责什么

- 返回 `client_run_quotas` 当前缓存
- 结合 `edge_clients` 当前摘要字段做一次中心 run admission 判断：
  - `healthStatus`
  - `discoveryStatus`
  - `slotExceptionStatus`
  - `availableSlotCount`

### 它不负责什么

- 不直接去平台拉最新额度
- 不直接发起 browser-env run
- 不改 Client 任何状态

### 当前 run admission 阻断口径

- `missing_client_identity`
- `client_not_healthy`
- `client_not_verified`
- `slot_exception`
- `no_available_slot`
- `missing_run_quota`
- `run_quota_not_valid`
- `quota_limit_zero`
- `quota_exhausted`
- `run_quota_expired`

### SSE 说明

- 不使用 SSE
- 原因：这是普通同步查询接口，同步 HTTP 已足够表达结果

---

## 2. POST /api/v1/edge-clients/{clientId}/run-quota/refresh

### 业务语义

管理员手工更新当前节点的中心 run quota 快照。

### 它负责什么

- 更新 `client_run_quotas`
- 立即返回新的 quota 快照
- 立即返回新的 run admission 判断
- 记录治理留痕

### 它不负责什么

- 不直接发起 browser-env run
- 不自动修改 `target_slot_count`
- 不修改 `edge_client_slots`

### 当前阶段定位

- 这是平台正式 quota API 接入前的临时治理入口
- 后续平台接口就绪后，应改成 Node 拉平台，再落本地快照
- 当前人工写入只是为了先把 run admission 主链打通

### SSE 说明

- 不使用 SSE
- 原因：这是短链路治理动作，同步 HTTP 已足够表达结果

---

## 3. 两条接口的关系

- `GET /run-quota`
  - 看当前中心缓存结果 + 当前准入判断
- `POST /run-quota/refresh`
  - 刷新当前中心额度快照

它们不会替代：

- `POST /api/v1/edge-clients/{clientId}/slot-reconcile`
- `POST /api/v1/edge-clients/{clientId}/target-slot-count`

三组能力分工应固定为：

- `slot-reconcile`
  - 刷新实际 slot 事实
- `target-slot-count`
  - 更新中心目标 slot 数
- `run-quota`
  - 更新中心平台额度快照

---

## 4. 当前中心 run admission 组合口径

当前 Node 允许进入未来 `browser-env run` 的前提是同时满足：

1. 已有中心 `clientId`
2. `healthStatus=healthy`
3. `discoveryStatus=verified`
4. `slotExceptionStatus=normal`
5. `availableSlotCount > 0`
6. 存在可信额度快照，且：
   - `status=valid`
   - `quotaLimit > 0`
   - `quotaAvailableSnapshot > 0`
   - `expiresAt` 未过期

只要任一条件不满足，当前中心口径就必须返回 `admission.allowed=false`。
