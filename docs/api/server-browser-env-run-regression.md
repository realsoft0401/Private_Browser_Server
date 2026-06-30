# Server Browser Env Run Regression

这份文档用于回归：

- `POST /api/v1/browser-envs/{envId}/run`

## 1. 测试目标

确认下面 4 件事：

1. 中心 run 接口必须要求显式 `slotId`
2. 中心 run 会先走 run admission
3. 中心 run 接单后会返回 `server taskId/eventsUrl`
4. 中心 run 的最终结果必须看中心 SSE，不是看接单响应

## 2. 当前限制

这条回归依赖：

1. `server_browser_envs` 已经存在目标 env 记录
2. 目标 env 绑定到正式 `clientId`
3. 目标 `clientId` 当前 `healthy + verified`
4. 目标 `clientId` 已有可用 slot
5. 目标 `clientId` 已写入有效 quota 快照

如果当前中心库里还没有 `server_browser_envs` 记录，这轮只能先测失败路径，不能测 happy path。

## 3. 基础变量

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export ENV_ID="906090001_tk_324867594169356288"
export SLOT_ID="slot001"
```

## 4. 失败路径：缺少 slotId

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/run" \
  -H "Content-Type: application/json" \
  -d '{}' | jq
```

预期：

- `code=1002`
- 错误信息明确指出 `slotId` 不能为空或请求体非法

## 5. 失败路径：中心 env 不存在

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/not_exist_env/run" \
  -H "Content-Type: application/json" \
  -d '{
    "slotId": "slot001",
    "forceRecreate": false
  }' | jq
```

预期：

- `code=1004`
- `message=server browser env not found`

## 6. Happy path：中心接单

只有当前中心库里已经存在目标 env 记录时再执行：

```bash
RUN_RESP="$(curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/run" \
  -H "Content-Type: application/json" \
  -d "{
    \"slotId\": \"$SLOT_ID\",
    \"forceRecreate\": false
  }")"

echo "$RUN_RESP" | jq
export SERVER_TASK_ID="$(echo "$RUN_RESP" | jq -r '.data.taskId')"
echo "$SERVER_TASK_ID"
```

预期：

- `code=1000`
- `data.taskType=browser_env_run`
- `data.resourceType=browser_env`
- `data.resourceId=$ENV_ID`
- `data.eventsUrl` 指向 `/api/v1/server-tasks/{taskId}/events`

## 7. 订阅中心 SSE

```bash
curl -N "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID/events"
```

至少要看到：

- `load_server_env`
- `dispatch_edge_run`
- `edge_task_accepted`

最终必须收口到：

- `task.completed`
  或
- `task.failed`

## 8. 结果解释

### 如果在 admission 阶段失败

常见原因：

- 节点不是 `healthy`
- 节点不是 `verified`
- `slotExceptionStatus=exception`
- `availableSlotCount=0`
- quota 缺失、过期或已用尽

这时预期：

- SSE 最终应是 `task.failed`
- `suggestion` 应明确指向修复方向

### 如果 Edge 接单后失败

常见原因：

- Edge 上 env 资产异常
- slot 当前不可用
- Docker / runtime image / runtime protection 校验失败

这时预期：

- 中心 SSE 最终应是 `task.failed`
- 失败信息来自 Edge task detail

### 如果 Edge success 但中心 sync 失败

这时预期：

- 中心仍按 `task.failed` 收口
- 因为当前规则要求：中心无法再次确认 env 事实时，不能把 run 记成成功
