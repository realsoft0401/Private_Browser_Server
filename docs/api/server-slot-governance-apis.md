# Server Slot Governance APIs

这份文档说明 Node Server slot 治理接口边界：

- `GET /api/v1/edge-clients/{clientId}/slots`
- `POST /api/v1/edge-clients/{clientId}/slots`
- `DELETE /api/v1/edge-clients/{clientId}/slots/{slotId}`
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

## 2. POST /api/v1/edge-clients/{clientId}/slots

### 业务语义

通过 Node Server 在目标 Client 上新增一个真实 slot。

### 它负责什么

- 校验 `slotId` 必须是 `slot001` 这类三位编号
- 校验节点必须是 `healthy + verified`
- 调用 Client `POST /api/v1/edge/slots`
- 成功后重新读取 Client slots
- 全量刷新 `edge_client_slots`
- 把 `targetSlotCount` 同步为动作后的真实 `actualSlotCount`

### 它不负责什么

- 不批量扩容
- 不自动生成 slotId
- 不创建 Browser Env
- 不触发 Browser Env run

### 状态与前置条件

- 允许：节点 `healthStatus=healthy` 且 `discoveryStatus=verified`
- 拒绝：节点 `discovered/stale/offline/unhealthy/identity_changed`
- 拒绝：`slotId` 不符合 `slot[0-9]{3}`
- 拒绝：Client 已存在同名 slot

### SSE 说明

- 不使用 SSE
- 原因：新增单个 slot 是短链路同步动作
- 成功返回时表示 Client 已创建，且 Node slot 缓存已经刷新

### 请求示例

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/slots" \
  -H "Content-Type: application/json" \
  -d '{
    "slotId": "slot004",
    "source": "manual-admin-create-slot"
  }' | jq
```

### 成功响应

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "clientId": "9060901190003",
    "slotId": "slot004",
    "action": "create_slot",
    "result": "success",
    "targetSlotCount": 4,
    "actualSlotCount": 4,
    "availableSlotCount": 4,
    "runningSlotCount": 0,
    "slotExceptionStatus": "normal",
    "slotExceptionReason": "",
    "updatedAt": 1782959000
  }
}
```

---

## 3. DELETE /api/v1/edge-clients/{clientId}/slots/{slotId}

### 业务语义

通过 Node Server 删除目标 Client 上的一个真实 slot。

### 它负责什么

- 校验 `slotId` 必须是 `slot001` 这类三位编号
- 校验节点必须是 `healthy + verified`
- 调用 Client `DELETE /api/v1/edge/slots/{slotId}`
- 默认 `force=false`，只删除 waiting slot
- 成功后重新读取 Client slots
- 全量刷新 `edge_client_slots`
- 把 `targetSlotCount` 同步为动作后的真实 `actualSlotCount`

### 它不负责什么

- 不自动 stop Browser Env
- 不删除 running slot 上挂载的账号环境
- 不批量缩容
- 不绕过 Client 的状态校验

### 状态与前置条件

- 允许：节点 `healthy + verified`
- 允许：目标 slot 在 Client 上是 `waiting`
- 拒绝：目标 slot 是 `loading/running/ending`
- 拒绝：目标 slot 不存在

### SSE 说明

- 不使用 SSE
- 原因：删除单个 waiting slot 是短链路同步动作
- 成功返回时表示 Client 已删除，且 Node slot 缓存已经刷新

### 请求示例

```bash
curl -s -X DELETE "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/slots/slot004" \
  -H "Content-Type: application/json" \
  -d '{
    "force": false,
    "source": "manual-admin-delete-slot"
  }' | jq
```

### 成功响应

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "clientId": "9060901190003",
    "slotId": "slot004",
    "action": "delete_slot",
    "result": "success",
    "targetSlotCount": 3,
    "actualSlotCount": 3,
    "availableSlotCount": 3,
    "runningSlotCount": 0,
    "slotExceptionStatus": "normal",
    "slotExceptionReason": "",
    "updatedAt": 1782959300
  }
}
```

---

## 4. POST /api/v1/edge-clients/{clientId}/target-slot-count

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

## 5. 接口关系

- `GET /slots`
  - 看当前中心缓存结果
- `POST /slots`
  - 在 Client 创建真实 slot，并把目标数同步为新实际数
- `DELETE /slots/{slotId}`
  - 在 Client 删除真实 waiting slot，并把目标数同步为新实际数
- `POST /target-slot-count`
  - 改当前中心目标值

它们不会替代：

- `POST /api/v1/edge-clients/{clientId}/slot-reconcile`

三条接口分工应固定为：

- `slot-reconcile`
  - 刷新实际 slot 事实
- `target-slot-count`
  - 更新中心目标值
- `POST /slots` / `DELETE /slots/{slotId}`
  - 修改 Client 真实 slot 资源，并刷新中心缓存
- `GET /slots`
  - 查询当前中心视图
