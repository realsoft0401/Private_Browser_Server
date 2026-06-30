# Server Browser Env Stop Regression

这份文档用于回归：

- `POST /api/v1/browser-envs/{envId}/stop`

## 1. 测试目标

确认下面 5 件事：

1. 中心 stop 是同步 HTTP，不是 SSE 接单
2. 中心 stop 会在内部创建 `server_task` 审计事实
3. 目标 Edge stop 成功后，中心会重新同步 env 摘要
4. 中心 `server_browser_envs` 会收口到 `stopped/missing`
5. 再查 Client detail 与中心 detail，结果要一致

## 2. 基础变量

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export CLIENT_BASE="http://192.168.111.119:3300"
export ENV_ID="906090001_tk_330198837593378816"
```

## 3. 前置条件

执行 stop happy path 之前，目标 env 应该已经在运行：

```bash
curl -s "$CLIENT_BASE/api/v1/edge/browser-envs/$ENV_ID" | jq '.data.index.status,.data.index.containerStatus'
curl -s "$SERVER_BASE/api/v1/browser-envs/$ENV_ID" | jq '.data.status,.data.containerStatus'
```

预期：

- Client 侧应是 `running`
- Server 侧应是 `running`

如果当前已经是 `stopped`，先按 run 文档重新跑起来再做这轮 stop 回归。

## 4. 失败路径：中心 env 不存在

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/not_exist_env/stop" \
  -H "Content-Type: application/json" \
  -d '{"timeoutSeconds":10}' | jq
```

预期：

- `code=1004`
- `message=server browser env not found`

## 5. Happy path：中心 stop

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/stop" \
  -H "Content-Type: application/json" \
  -d '{"timeoutSeconds":10}' | jq
```

预期：

- `code=1000`
- `data.envId=$ENV_ID`
- `data.status=stopped`
- `data.containerStatus=missing`
- `data.stoppedAt` 为非 0 时间戳

这一步就是最终 HTTP 结果，不需要再去订阅 SSE。

## 6. 核对 Client detail

```bash
curl -s "$CLIENT_BASE/api/v1/edge/browser-envs/$ENV_ID" | jq '{
  status: .data.index.status,
  containerStatus: .data.index.containerStatus,
  updatedAt: .data.index.updatedAt,
  stoppedAt: .data.container.stoppedAt
}'
```

预期：

- `status=stopped`
- `containerStatus=missing`

## 7. 核对中心 detail

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs/$ENV_ID" | jq
```

预期：

- `data.status=stopped`
- `data.containerStatus=missing`
- `data.runtimeStatus=stopped`
- `data.currentSlotId=''`
- `data.cdpUrl=''`
- `data.webVncUrl=''`

## 8. 核对中心 task 审计

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT id,task_type,resource_id,status,error_message,created_at,finished_at
FROM server_tasks
WHERE resource_id='$ENV_ID'
ORDER BY created_at DESC
LIMIT 3;
"
```

预期：

- 最新一条 `task_type=browser_env_stop`
- `status=success`
- `error_message=''`

## 9. 核对中心 env 缓存

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT env_id,client_id,status,container_status,runtime_status,current_slot_id,last_error,updated_at
FROM server_browser_envs
WHERE env_id='$ENV_ID';
"
```

预期：

- `status=stopped`
- `container_status=missing`
- `runtime_status=stopped`
- `current_slot_id=''`
- `last_error=''`

## 10. 结果解释

### 如果中心 stop 直接失败

常见原因：

- 中心 env 不存在
- 目标节点不是 `healthy`
- 目标节点不是 `verified`
- Edge stop 返回错误

这时预期：

- HTTP 直接返回失败
- 同时 `server_tasks` 最新一条 stop 审计应为 `failed`

### 如果 Edge stop 成功但中心摘要没同步上

这时预期：

- HTTP 仍应返回失败
- 因为当前规则要求：中心无法再次确认停止后事实时，不能把 stop 记成成功
