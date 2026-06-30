# Server Slot Reconcile 回归测试

这份文档只验证一条主线：

- `POST /api/v1/edge-clients/{clientId}/slot-reconcile`
- `GET /api/v1/server-tasks/{taskId}`
- `GET /api/v1/server-tasks/{taskId}/events`

目标是确认下面 4 件事同时成立：

1. Node Server 能创建中心 task
2. Node Server 能通过 HTTP 向 Client 拉取 `/api/v1/edge/slots`
3. Node Server 能把结果回写到：
   - `edge_clients`
   - `edge_client_slots`
   - `edge_client_slot_logs`
   - `server_tasks`
4. 中心 SSE 事件流能完整输出任务过程

---

## 1. 测试范围

本次测试只验证“中心 slot 对账”。

不验证：

- 平台额度
- 批量 slot 治理
- create-slot / destroy-slot / reinit-slot 的 Node 编排
- browser-env 生命周期

---

## 2. 前置条件

必须先满足：

1. `Private_Browser_Client` 已启动
2. `Private_Browser_Server` 已启动
3. Client 已经完成 bind
4. Node 能查到该 Client，且状态是：
   - `healthStatus=healthy`
   - `discoveryStatus=verified`

推荐变量：

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export CLIENT_BASE="http://192.168.111.119:3300"
export ACCOUNT_ID="906090119"
export CLIENT_ID="9060901190003"
```

如果你的 Client IP 或 ClientID 不同，替换成你的真实值。

---

## 3. 启动前检查

### 3.1 检查 Client 是否在线

```bash
curl -s "$CLIENT_BASE/health" | jq
```

通过标准：

- 返回 `code=1000`
- `data.ok=true`

### 3.2 检查 Node 是否在线

```bash
curl -s "$SERVER_BASE/health" | jq
```

通过标准：

- 返回 `code=1000`

### 3.3 检查 Node 当前是否能看到该节点

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients?accountId=$ACCOUNT_ID" | jq
```

通过标准：

- `items` 中存在目标 `clientId`
- 该节点：
  - `healthStatus=healthy`
  - `discoveryStatus=verified`

如果这里看不到节点，先不要继续。

---

## 4. 场景 A：空 slot 回归

这个场景验证：

- Client 当前没有任何 slot
- Node 发起 `slot_reconcile` 后仍能正常成功
- `edge_client_slots` 保持空表结果

### 4.1 先确认 Client 当前 slot 列表为空

```bash
curl -s "$CLIENT_BASE/api/v1/edge/slots" | jq
```

通过标准：

- 返回：

```json
{
  "code": 1000,
  "message": "success",
  "data": []
}
```

### 4.2 发起 slot-reconcile

```bash
RECONCILE_RESP="$(curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/slot-reconcile" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "manual-regression-empty-slots"
  }')"

echo "$RECONCILE_RESP" | jq
export SERVER_TASK_ID="$(echo "$RECONCILE_RESP" | jq -r '.data.taskId')"
echo "$SERVER_TASK_ID"
```

通过标准：

- 返回 `code=1000`
- 能拿到 `taskId`
- `taskType=slot_reconcile`
- `eventsUrl=/api/v1/server-tasks/{taskId}/events`
- 发起接口只表示中心接单成功，不表示本次 slot 对账已经最终成功

### 4.3 订阅 SSE

```bash
curl -N "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID/events"
```

通过标准：

- 至少依次出现这些阶段：
  - `load_client`
  - `fetch_slots`
  - `replace_slots`
  - `finalize_success`
- 最终事件必须是：
  - `event: task.completed`
  - `status: success`

典型结果：

```text
event: task.progress
data: {"stage":"load_client","status":"pending",...}

event: task.progress
data: {"stage":"fetch_slots","status":"running",...}

event: task.progress
data: {"stage":"replace_slots","status":"running",...}

event: task.completed
data: {"stage":"finalize_success","status":"success",...}
```

### 4.4 查看 task 详情

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID" | jq
```

通过标准：

- `status=success`
- `currentStage=finalize_success`
- `eventsUrl` 正确

### 4.5 查看数据库结果

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,target_slot_count,actual_slot_count,available_slot_count,running_slot_count,slot_exception_status,slot_exception_reason,last_slot_checked_at
FROM edge_clients
WHERE client_id='$CLIENT_ID';

SELECT client_id,slot_id,status,current_env_id,current_run_id,container_name,cdp_port,vnc_port,last_error
FROM edge_client_slots
WHERE client_id='$CLIENT_ID'
ORDER BY slot_id;

SELECT client_id,action,result,message,created_at
FROM edge_client_slot_logs
WHERE client_id='$CLIENT_ID'
ORDER BY id DESC
LIMIT 3;

SELECT id,task_type,status,events_url,error_message,suggestion,created_at,updated_at,finished_at
FROM server_tasks
WHERE id='$SERVER_TASK_ID';
"
```

通过标准：

- `edge_clients.last_slot_checked_at` 已更新
- `actual_slot_count=0`
- `available_slot_count=0`
- `running_slot_count=0`
- `slot_exception_status=normal`
- `edge_client_slots` 查询结果为空
- `edge_client_slot_logs` 至少有一条：
  - `action=slot_reconcile`
  - `result=success`
- `server_tasks.status=success`

---

## 5. 场景 B：有 slot 回归

这个场景验证：

- Client 上存在真实 slot
- Node 对账后会把 slot 明细写入 `edge_client_slots`
- `available_slot_count / running_slot_count` 会跟着变化

### 5.1 在 Client 上准备一个空闲 slot

如果 Client 还没有 slot，先创建：

```bash
curl -s -X POST "$CLIENT_BASE/api/v1/edge/slots" \
  -H "Content-Type: application/json" \
  -d '{
    "slotId": "slot001"
  }' | jq
```

如果返回：

```json
{
  "code": 1003,
  "message": "数据状态冲突"
}
```

表示 `slot001` 已经存在，不是失败，可以直接继续下一步。

### 5.2 确认 Client 端 slot 当前态

```bash
curl -s "$CLIENT_BASE/api/v1/edge/slots" | jq
curl -s "$CLIENT_BASE/api/v1/edge/slots/slot001" | jq
```

通过标准：

- 列表里能看到 `slot001`
- `slot001.status=waiting`

### 5.3 再次发起 slot-reconcile

```bash
RECONCILE_RESP="$(curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/slot-reconcile" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "manual-regression-slot001"
  }')"

echo "$RECONCILE_RESP" | jq
export SERVER_TASK_ID="$(echo "$RECONCILE_RESP" | jq -r '.data.taskId')"
echo "$SERVER_TASK_ID"
```

### 5.4 订阅 SSE

```bash
curl -N "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID/events"
```

通过标准：

- 阶段和空 slot 场景一致
- 最终必须 `task.completed`

### 5.5 查看 Node 数据库中的 slot 明细

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,target_slot_count,actual_slot_count,available_slot_count,running_slot_count,slot_exception_status,slot_exception_reason,last_slot_checked_at
FROM edge_clients
WHERE client_id='$CLIENT_ID';

SELECT client_id,slot_id,status,current_env_id,current_run_id,container_name,cdp_port,vnc_port,last_error
FROM edge_client_slots
WHERE client_id='$CLIENT_ID'
ORDER BY slot_id;

SELECT client_id,action,result,message,created_at
FROM edge_client_slot_logs
WHERE client_id='$CLIENT_ID'
ORDER BY id DESC
LIMIT 3;
"
```

通过标准：

- `actual_slot_count >= 1`
- `edge_client_slots` 至少存在一条：
  - `slot_id=slot001`
- 如果 `slot001.status=waiting`：
  - `available_slot_count >= 1`
  - `running_slot_count = 0`

---

## 6. 可选加强验证：包挂载运行场景

如果你还想继续验证“包已经挂载到 slot 并正在运行”的统计，可以再做这一轮。

这里必须先统一口径：

- `containerStatus=running`
  - 只表示 slot 的基础浏览器容器在线
  - 不代表当前已有配置包挂载在这个 slot 上运行
- Client `slot.status=occupied`
  - 才表示当前已有 browser-env 配置包挂载并正在这个 slot 上运行
- Node `slot.status=running`
  - 是 Node 对 Client `occupied` 的中心归一化口径
  - 这里的 `running` 指“包运行态”
  - 不是指“容器存活态”

所以这一步的真正目标不是“看到容器活着”，而是先让 Client 返回：

- `slot001.status=occupied`
- `currentPackageId` 非空
- `currentRunId` 非空

### 6.1 先在 Client 上让某个 browser-env 占用 `slot001`

这一步走你已经有的正式 Client run 流程，不在本文档里重复展开。

目标结果：

1. Client 自己的 run task 先成功
2. Client 上 `slot001` 不再是 `waiting`
3. Client 上 `slot001.status=occupied`
4. Client 上 `currentPackageId/currentRunId` 已出现
5. 对账后 Node 才应把它收口成 `running`

推荐在发 Node `slot-reconcile` 之前，先补两条 Client 自查：

```bash
curl -s "$CLIENT_BASE/api/v1/edge/slots/slot001" | jq
curl -s "$CLIENT_BASE/api/v1/edge/slots" | jq
```

只有当这里已经看到：

```json
{
  "slotId": "slot001",
  "status": "occupied",
  "currentPackageId": "...",
  "currentRunId": "..."
}
```

才应该继续对 Node 发起 `slot-reconcile`。

如果这里仍然是：

```json
{
  "slotId": "slot001",
  "status": "waiting",
  "containerStatus": "running"
}
```

含义是：

- slot 基础容器还活着
- 但没有任何包挂载到这个 slot 上运行
- 这时 Node 对账结果必须仍然是 `waiting`
- 这不属于 Node 失败，而是 Client 当前事实如此

### 6.2 再次执行 slot-reconcile

```bash
RECONCILE_RESP="$(curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/slot-reconcile" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "manual-regression-slot-running"
  }')"

echo "$RECONCILE_RESP" | jq
export SERVER_TASK_ID="$(echo "$RECONCILE_RESP" | jq -r '.data.taskId')"
curl -N "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID/events"
```

### 6.3 查看数据库

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,target_slot_count,actual_slot_count,available_slot_count,running_slot_count,slot_exception_status
FROM edge_clients
WHERE client_id='$CLIENT_ID';

SELECT client_id,slot_id,status,current_env_id,current_run_id,container_name,last_error
FROM edge_client_slots
WHERE client_id='$CLIENT_ID'
ORDER BY slot_id;
"
```

通过标准：

- `slot001.status=running`
- `current_env_id` 非空
- `current_run_id` 非空
- `running_slot_count >= 1`
- `available_slot_count = 0`

说明：

- 当前 Server 会把 Client 老口径：
  - `occupied -> running`
  - `releasing -> ending`
  做统一归一化
- 这里的 `running` 一律指“包挂载运行态”
- 不是指 slot 基础容器本身是否还在运行

---

## 7. 异常判断

### 7.1 返回 `edge client not found`

含义：

- 这个 `clientId` 在 Node 中不存在

处理：

- 先查：

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients?accountId=$ACCOUNT_ID" | jq
```

### 7.2 返回 `edge client is not healthy and verified`

含义：

- 节点当前不是 `healthy + verified`

处理：

- 先确认 heartbeat 是否持续打进来
- 再看 `GET /api/v1/edge-clients`

### 7.3 SSE 最终是 `task.failed`

重点看：

- `fetch_slots_failed`
- `replace_slots_failed`
- `update_summary_failed`

对应处理方向：

- `fetch_slots_failed`
  - 查 Client 是否在线
  - 查 `GET /api/v1/edge/slots` 是否可访问
- `replace_slots_failed`
  - 查 Node SQLite 是否正常
- `update_summary_failed`
  - 查 `edge_clients` 字段迁移是否已生效

### 7.4 为什么 `containerStatus=running` 但 Node 仍然不是 `running`

这是本轮测试里最容易误解的点。

如果你看到 Client 返回：

```json
{
  "slotId": "slot001",
  "status": "waiting",
  "containerStatus": "running"
}
```

它的真实含义是：

- slot 的基础浏览器容器在线
- 但没有任何 browser-env 包挂载在这个 slot 上运行

所以这时 Node 对账后必须仍然是：

- `edge_client_slots.status=waiting`
- `running_slot_count=0`

这不是测试失败，而是正确结果。

### 7.5 为什么 `target_slot_count=0` 但 `actual_slot_count=1` 仍然是 `normal`

当前这版 Node 还没有接入平台目标 slot 数。

因此当：

- `target_slot_count=0`
- `actual_slot_count=1`

时，当前对账语义仍然是：

- 先按 Client 实际 slot 事实建立中心缓存
- 不因为“平台目标数尚未下发”就自动判成 `slot 异常`

也就是说，在平台额度 / 目标 slot 数正式接入前：

- `target_slot_count=0`
- `slot_exception_status=normal`

是允许出现的，不属于本轮测试失败。

---

## 8. 本轮测试完成标准

这份文档走完以后，至少要确认下面结论成立：

1. Node 能创建 `slot_reconcile` 中心 task
2. Node 的 SSE 接口能正确输出任务过程
3. 空 slot 场景下：
   - task 成功
   - `edge_client_slots` 为空
   - `edge_clients.last_slot_checked_at` 已更新
4. 有 slot 场景下：
   - `edge_client_slots` 能写入明细
   - `actual_slot_count / available_slot_count / running_slot_count` 能同步变化
5. `server_tasks` 和 `edge_client_slot_logs` 都有正式留痕

如果这 5 条都成立，可以认为这次 `slot_reconcile + server-tasks + SSE` 主链已经通过。
