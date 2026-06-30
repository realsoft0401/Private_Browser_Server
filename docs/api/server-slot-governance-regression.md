# Server Slot Governance 回归测试

这份文档验证两条新的 Node Server 治理接口：

- `GET /api/v1/edge-clients/{clientId}/slots`
- `POST /api/v1/edge-clients/{clientId}/target-slot-count`

目标是确认：

1. Node 能返回当前缓存的 `edge_client_slots`
2. Node 能返回 `target/actual/available/running` 这些 slot 摘要
3. 手工修改 `target_slot_count` 后，`slot_exception_status` 能立即跟着变化

---

## 1. 前置条件

推荐变量：

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export ACCOUNT_ID="906090119"
export CLIENT_ID="9060901190003"
```

必须先满足：

1. Node Server 已启动
2. Client 已绑定
3. 节点状态是：
   - `healthStatus=healthy`
   - `discoveryStatus=verified`
4. 最好已经至少跑过一次 `slot-reconcile`

---

## 2. 启动前检查

### 2.1 检查节点是否存在

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients?accountId=$ACCOUNT_ID" | jq
```

通过标准：

- `items` 中能看到目标 `clientId`

### 2.2 检查当前是否已有 slot 缓存

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,slot_id,status,current_env_id,current_run_id,container_name
FROM edge_client_slots
WHERE client_id='$CLIENT_ID'
ORDER BY slot_id;
"
```

通过标准：

- 至少已有 1 条缓存结果

如果这里为空，先回去执行一次：

- `POST /api/v1/edge-clients/{clientId}/slot-reconcile`

---

## 3. 测试 `GET /api/v1/edge-clients/{clientId}/slots`

### 3.1 查询中心缓存的 slot 明细

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/slots" | jq
```

通过标准：

- 返回 `code=1000`
- `data.clientId` 正确
- `data.items` 至少存在 1 条
- `data.total >= 1`

示例关注点：

- `targetSlotCount`
- `actualSlotCount`
- `availableSlotCount`
- `runningSlotCount`
- `slotExceptionStatus`
- `items[].slotId`
- `items[].status`
- `items[].currentEnvId`
- `items[].currentRunId`

### 3.2 核对数据库

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,target_slot_count,actual_slot_count,available_slot_count,running_slot_count,slot_exception_status,slot_exception_reason,last_slot_checked_at
FROM edge_clients
WHERE client_id='$CLIENT_ID';

SELECT client_id,slot_id,status,current_env_id,current_run_id,container_name,cdp_port,vnc_port,last_error
FROM edge_client_slots
WHERE client_id='$CLIENT_ID'
ORDER BY slot_id;
"
```

通过标准：

- HTTP 返回和 SQLite 结果一致
- `total` 和 `edge_client_slots` 行数一致

---

## 4. 测试 `POST /api/v1/edge-clients/{clientId}/target-slot-count`

这条接口当前是“平台正式下发链路接入前”的临时管理员治理入口。

它只改中心目标 slot 数，不会自动去创建 / 删除 Client 本机 slot。

### 4.1 先记下当前值

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/slots" | jq '.data | {
  targetSlotCount,
  actualSlotCount,
  availableSlotCount,
  runningSlotCount,
  slotExceptionStatus,
  slotExceptionReason,
  total
}'
```

### 4.2 把目标 slot 数设成一个“大于当前实际值”的数字

如果当前 `actualSlotCount=1`，就设成 `2`：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/target-slot-count" \
  -H "Content-Type: application/json" \
  -d '{
    "targetSlotCount": 2,
    "source": "manual-regression-target-slot"
  }' | jq
```

通过标准：

- 返回 `code=1000`
- `targetSlotCount=2`
- `actualSlotCount` 保持原值
- `slotExceptionStatus=exception`
- `slotExceptionReason=target_slot_count=2 actual_slot_count=1`

### 4.3 再查一次 slot 缓存摘要

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/slots" | jq '.data | {
  targetSlotCount,
  actualSlotCount,
  availableSlotCount,
  runningSlotCount,
  slotExceptionStatus,
  slotExceptionReason,
  total
}'
```

通过标准：

- `targetSlotCount` 已更新成新值
- `slotExceptionStatus=exception`

### 4.4 查数据库

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,target_slot_count,actual_slot_count,available_slot_count,running_slot_count,slot_exception_status,slot_exception_reason,last_slot_checked_at
FROM edge_clients
WHERE client_id='$CLIENT_ID';

SELECT client_id,action,result,message,created_at
FROM edge_client_slot_logs
WHERE client_id='$CLIENT_ID'
ORDER BY id DESC
LIMIT 5;
"
```

通过标准：

- `edge_clients.target_slot_count` 已更新
- `slot_exception_status=exception`
- `edge_client_slot_logs` 至少新增一条：
  - `action=update_target_slot_count`
  - `result=success`

---

## 5. 测试完成后恢复现场

为了不把节点一直留在异常态，测试结束后应把目标 slot 数改回与你当前实际值一致的数字。

例如当前 `actualSlotCount=1`，就恢复成 `1`：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/target-slot-count" \
  -H "Content-Type: application/json" \
  -d '{
    "targetSlotCount": 1,
    "source": "manual-restore-target-slot"
  }' | jq
```

恢复后再查：

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/slots" | jq '.data | {
  targetSlotCount,
  actualSlotCount,
  availableSlotCount,
  runningSlotCount,
  slotExceptionStatus,
  slotExceptionReason,
  total
}'
```

通过标准：

- `targetSlotCount=1`
- `actualSlotCount=1`
- `slotExceptionStatus=normal`
- `slotExceptionReason=""`

---

## 6. 异常判断

### 6.1 返回 `edge client not found`

含义：

- 这个 `clientId` 在 Node 中不存在

处理：

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients?accountId=$ACCOUNT_ID" | jq
```

### 6.2 返回 `targetSlotCount 必须大于 0`

含义：

- 当前临时治理口径下，不允许把目标 slot 数设成 `0`

说明：

- 这条接口当前只支持“人工治理一个明确目标值”
- 不承担“清空平台目标数”的语义

### 6.3 为什么这里只改了目标数，实际 slot 没变

这是正确结果。

`POST /target-slot-count` 当前只改中心治理字段：

- `target_slot_count`
- `slot_exception_status`
- `slot_exception_reason`

它不会直接让 Client：

- create slot
- destroy slot
- reinit slot

真正的本机 slot 变化仍然要靠：

- Client 自己的 slot 管理接口
- 或后续正式平台链路 / Node 编排链路

---

## 7. 本轮完成标准

这份文档走完以后，至少要确认：

1. `GET /slots` 能稳定返回中心 slot 缓存
2. `POST /target-slot-count` 能更新中心目标值
3. 目标值与实际值不一致时，能立刻进入 `slotExceptionStatus=exception`
4. 目标值恢复一致后，能立刻回到 `slotExceptionStatus=normal`
5. `edge_client_slot_logs` 中有对应治理留痕

如果这 5 条都成立，可以认为 Node 的“slot 查询 + 目标数治理”这条链已经打通。
