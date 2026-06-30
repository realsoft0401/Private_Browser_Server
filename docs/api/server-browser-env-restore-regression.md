# Server Browser Env Restore Regression

这份文档用于回归：

- `POST /api/v1/browser-envs/{envId}/restore`

## 1. 测试目标

确认下面 5 件事：

1. 中心 restore 是 task + SSE，不是同步最终结果
2. 中心会把 restore 委派到目标 Edge 正式 restore 接口
3. 中心 SSE 能看到 Edge 任务阶段
4. restore 成功后中心 env 收口到可再次运行的非运行态
5. 中心 `server_tasks` 与 `server_browser_envs` 审计同步正确

## 2. 基础变量

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export CLIENT_BASE="http://192.168.111.119:3300"
export ENV_ID="906090001_tk_330198837593378816"
```

## 3. 前置条件

执行 happy path 之前，目标 env 需要已经是 `backed_up`：

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs/$ENV_ID" | jq '.data.status,.data.containerStatus'
```

建议预期：

- `status=backed_up`

如果当前还没备份，先按 backup 文档跑一遍。

## 4. 发起中心 restore

```bash
RESTORE_RESP="$(curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/restore")"
echo "$RESTORE_RESP" | jq
export SERVER_TASK_ID="$(echo "$RESTORE_RESP" | jq -r '.data.taskId')"
echo "$SERVER_TASK_ID"
```

预期：

- `code=1000`
- `data.taskType=browser_env_restore`
- `data.resourceType=browser_env`
- `data.resourceId=$ENV_ID`
- `data.eventsUrl` 指向 `/api/v1/server-tasks/{taskId}/events`

## 5. 订阅中心 SSE

```bash
curl -N "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID/events"
```

至少要看到：

- `load_server_env`
- `dispatch_edge_restore`
- `edge_task_accepted`

最终必须收口到：

- `task.completed`
  或
- `task.failed`

## 6. 核对中心 detail

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs/$ENV_ID" | jq
```

成功后预期：

- `data.status=created`
- `data.containerStatus=missing`
- `data.runtimeStatus=created`
- `data.currentSlotId=''`

## 7. 核对 Client detail

```bash
curl -s "$CLIENT_BASE/api/v1/edge/browser-envs/$ENV_ID" | jq '.data.index.status,.data.index.containerStatus'
```

成功后预期：

- `created`
- `missing`

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

成功后预期：

- 最新一条 `task_type=browser_env_restore`
- `status=success`
- `error_message=''`
