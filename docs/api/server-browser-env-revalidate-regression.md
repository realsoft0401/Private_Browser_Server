# POST /api/v1/browser-envs/{envId}/revalidate 回归测试

## 测试目标

验证中心 revalidate 链路能正确创建中心任务、转发 Edge revalidate，并通过 `/api/v1/server-tasks/{taskId}/events` 收口最终结果。

revalidate 只用于异常环境包。正常 `created/stopped/running/backed_up` 环境不应该为了测试强行 revalidate；如果 Edge 返回“不需要 revalidate”，这是正确的失败收口，不是接口未接通。

## 前置环境

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export ENV_ID="318275706305908736_tk_319725200528642048"
```

## 1. 不存在 env 的失败校验

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/not_exist_env/revalidate" | jq
```

通过标准：

- `code=1004`
- `message=server browser env not found`
- 不创建中心 task

## 2. 正式发起 revalidate

只有当目标 env 在 Edge 侧处于 `status=error` 时，才建议执行本步骤。

```bash
REVALIDATE_RESP="$(curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/revalidate")"
echo "$REVALIDATE_RESP" | jq

export SERVER_TASK_ID="$(echo "$REVALIDATE_RESP" | jq -r '.data.taskId')"
echo "$SERVER_TASK_ID"
```

通过标准：

- `code=1000`
- `data.taskType=browser_env_revalidate`
- `data.resourceType=browser_env`
- `data.resourceId=$ENV_ID`
- `data.eventsUrl=/api/v1/server-tasks/{taskId}/events`

## 3. 订阅中心 SSE

```bash
curl -N "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID/events"
```

通过标准：

- 能看到 `task.progress`
- 最终只能是 `task.completed` 或 `task.failed`
- 如果 Edge 侧 env 不是 `error`，最终 `task.failed` 是合理结果
- 如果 Edge 校验通过，最终 `task.completed`，中心缓存随后被刷新

## 4. 查询中心 task 终态

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID" | jq
```

通过标准：

- `code=1000`
- `data.taskType=browser_env_revalidate`
- `data.status` 与 SSE 最终事件一致
- 失败时 `error/suggestion` 有排障信息

## 5. 查询任务列表确认可回看

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks?taskType=browser_env_revalidate&page=1&pageSize=20" | jq
```

通过标准：

- 能看到刚刚创建的 revalidate 任务
- 说明 `GET /api/v1/server-tasks` 可以承担任务历史回看入口

## 结论

revalidate 的成功标准不是“HTTP 接单返回 1000”，而是中心 SSE 或 task detail 最终进入可信终态。接单成功只说明 Node 已经创建中心任务并开始后台编排。
