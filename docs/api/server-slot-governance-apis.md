# Server Slot Governance APIs

这份文档只说明两条新的 Node Server slot 治理接口边界：

- `GET /api/v1/edge-clients/{clientId}/slots`
- `POST /api/v1/edge-clients/{clientId}/target-slot-count`

---

## 1. GET /api/v1/edge-clients/{clientId}/slots

### 业务语义

返回 Node Server 当前缓存的 node-slot 明细和 slot 摘要。

### 它负责什么

- 返回 `edge_client_slots` 当前缓存
- 返回 `edge_clients` 上的 slot 摘要字段：
  - `targetSlotCount`
  - `actualSlotCount`
  - `availableSlotCount`
  - `runningSlotCount`
  - `slotExceptionStatus`
  - `slotExceptionReason`

### 它不负责什么

- 不直接去调用 Client `/api/v1/edge/slots`
- 不触发新的 `slot-reconcile`
- 不修改任何中心状态

### 状态口径

- `waiting`
  - 当前没有包挂载在该 slot 上运行
- `running`
  - 表示包挂载运行态
  - 不是单纯底层 slot 容器活着

### SSE 说明

- 不使用 SSE
- 原因：这是普通查询接口，同步 HTTP 已足够表达结果

---

## 2. POST /api/v1/edge-clients/{clientId}/target-slot-count

### 业务语义

管理员手工更新当前节点的中心目标 slot 数。

### 它负责什么

- 更新 `edge_clients.target_slot_count`
- 根据当前 `actual_slot_count` 重新计算：
  - `slot_exception_status`
  - `slot_exception_reason`
- 记录治理留痕

### 它不负责什么

- 不直接调用 Client 创建 slot
- 不直接调用 Client 删除 slot
- 不自动触发 `slot-reconcile`
- 不修改 `actual_slot_count`

### 当前限制

- 当前只允许 `targetSlotCount > 0`
- 它是平台正式下发接口接入前的临时治理入口

### SSE 说明

- 不使用 SSE
- 原因：这是短链路中心治理动作，同步 HTTP 已足够表达结果

---

## 3. 两条接口的关系

- `GET /slots`
  - 看当前中心缓存结果
- `POST /target-slot-count`
  - 改当前中心目标值

它们不会替代：

- `POST /api/v1/edge-clients/{clientId}/slot-reconcile`

三条接口分工应固定为：

- `slot-reconcile`
  - 刷新实际 slot 事实
- `target-slot-count`
  - 更新中心目标值
- `GET /slots`
  - 查询当前中心视图
