# Server Browser Env Delete Package Regression

这份文档用于回归：

- `DELETE /api/v1/browser-envs/{envId}/package`

## 1. 测试目标

确认下面 5 件事：

1. 中心 package delete 是 task + SSE，不是同步最终结果
2. 中心会把 delete 委派到目标 Edge 正式 package delete 接口
3. 中心 SSE 能看到 Edge 删除阶段
4. 删除成功后中心缓存这条 env 会消失
5. `server_tasks` 审计仍然保留

## 2. 基础变量

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export CLIENT_BASE="http://192.168.111.119:3300"
export ACCOUNT_ID="906090119"
export ENV_ID="906090001_tk_330198837593378816"
```

## 3. 前置条件

执行 happy path 之前，目标 env 需要是非运行态：

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs/$ENV_ID" | jq '.data.status,.data.containerStatus'
```

建议预期：

- `created`
- `stopped`
- `backed_up`

## 4. 发起中心 package delete

```bash
DELETE_RESP="$(curl -s -X DELETE "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/package")"
echo "$DELETE_RESP" | jq
export SERVER_TASK_ID="$(echo "$DELETE_RESP" | jq -r '.data.taskId')"
echo "$SERVER_TASK_ID"
```

预期：

- `code=1000`
- `data.taskType=browser_env_delete_package`
- `data.resourceType=browser_env`
- `data.resourceId=$ENV_ID`

## 5. 订阅中心 SSE

```bash
curl -N "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID/events"
```

至少要看到：

- `load_server_env`
- `dispatch_edge_delete_package`
- `edge_task_accepted`

最终必须收口到：

- `task.completed`
  或
- `task.failed`

## 6. 核对中心缓存是否移除

```bash
curl -i -s "$SERVER_BASE/api/v1/browser-envs/$ENV_ID"
curl -s "$SERVER_BASE/api/v1/browser-envs?accountId=$ACCOUNT_ID" | jq
```

成功后预期：

- detail 返回 `code=1004`
- 列表里不再出现该 env

## 7. 核对 Client 事实

```bash
curl -i -s "$CLIENT_BASE/api/v1/edge/browser-envs/$ENV_ID"
```

成功后预期：

- Client 返回“环境包不存在”或等价 not found 结果

## 8. 核对中心任务审计

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT id,task_type,resource_id,status,error_message,created_at,finished_at
FROM server_tasks
WHERE resource_id='$ENV_ID'
ORDER BY created_at DESC
LIMIT 4;

SELECT count(*) AS env_rows
FROM server_browser_envs
WHERE env_id='$ENV_ID';
"
```

成功后预期：

- 最新一条 `task_type=browser_env_delete_package`
- `status=success`
- `env_rows=0`
