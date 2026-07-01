# POST /api/v1/browser-envs/{envId}/revalidate

当前状态：已实现并已回归。

## 功能目标

对处于异常状态的 browser-env 发起受控重新校验，让 Edge Client 重新检查环境包索引、原子材料和运行摘要，并在校验通过后把中心缓存刷新为可信状态。

这条接口解决的是“环境包进入 error 后，不能带病 run/backup/restore/delete，需要先受控校验”的问题。

## 业务边界

- 负责读取 `server_browser_envs` 中的目标环境包
- 负责校验目标 Client 当前 `healthy + verified`
- 负责创建中心 `server_tasks`
- 负责调用目标 Edge `POST /api/v1/edge/browser-envs/{envId}/revalidate`
- 负责轮询 Edge task detail 并通过中心 SSE 收口最终成功或失败
- 不直接修改 Edge 环境包文件
- 不重建登录态、不修复账号内容、不替换 proxy/fingerprint raw
- 不自动重试

## 状态机与前置条件

- Node 侧要求 env 必须存在于 `server_browser_envs`
- Node 侧要求目标 Client 当前可用，即 `health_status=healthy` 且 `discovery_status=verified`
- Edge 侧当前只允许 `status=error` 的环境包执行 revalidate
- 成功后：Node 从 Edge detail 重新同步中心缓存
- 失败后：中心 task 进入 `failed`，env 摘要写入 `lastError`

## SSE 说明

- 本接口使用 SSE
- HTTP 接口立即返回 `taskId/eventsUrl`
- 调用方必须继续订阅 `/api/v1/server-tasks/{taskId}/events`
- SSE 中断后，通过 `GET /api/v1/server-tasks/{taskId}` 查询中心终态
- 接单成功不等于校验成功

## 请求示例

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export ENV_ID="906090001_tk_324867594169356288"

REVALIDATE_RESP="$(curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/revalidate")"
echo "$REVALIDATE_RESP" | jq

export SERVER_TASK_ID="$(echo "$REVALIDATE_RESP" | jq -r '.data.taskId')"
curl -N "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID/events"
```

## 成功接单响应示例

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "taskId": "server-task-1782800005001",
    "taskType": "browser_env_revalidate",
    "resourceType": "browser_env",
    "resourceId": "906090001_tk_324867594169356288",
    "eventsUrl": "/api/v1/server-tasks/server-task-1782800005001/events"
  }
}
```

## 典型 SSE 事件

```text
event: task.progress
data: {"event":"task.progress","taskId":"server-task-1782800005001","taskType":"browser_env_revalidate","resourceType":"browser_env","resourceId":"906090001_tk_324867594169356288","clientId":"9060901190003","envId":"906090001_tk_324867594169356288","stage":"dispatch_edge_revalidate","status":"running","message":"dispatching edge revalidate","timestamp":"2026-07-01T10:00:00+08:00"}

event: task.completed
data: {"event":"task.completed","taskId":"server-task-1782800005001","taskType":"browser_env_revalidate","resourceType":"browser_env","resourceId":"906090001_tk_324867594169356288","clientId":"9060901190003","envId":"906090001_tk_324867594169356288","stage":"finalize_success","status":"success","message":"browser env revalidated","timestamp":"2026-07-01T10:00:02+08:00"}
```

## 验收标准

- env 不存在时返回 `server browser env not found`
- 目标 Client 不健康或未 verified 时拒绝派发
- 正常接单返回 `browser_env_revalidate` 的 `taskId/eventsUrl`
- Edge revalidate 成功后中心 task 进入 `success`
- Edge revalidate 失败后中心 task 进入 `failed`，并保留错误和修复建议
