# GET /api/v1/server-tasks/{taskId}

## 功能目标

查询当前 Node Server 进程内的 server task 摘要，给前端刷新页面、管理员排障和 Node 自己的中心任务观察使用。

这条接口的定位是“任务摘要查询”，不是事件流本身。真正多阶段过程仍以 SSE 为准。

## 业务边界

- 负责返回当前进程内 server task 摘要
- 负责返回任务终态、当前阶段、错误信息、时间戳
- 负责返回 `eventsUrl`
- 不替代 `GET /api/v1/server-tasks/{taskId}/events`
- 不伪装成最终事件流接口

## SSE 说明

- 本接口本身不用 SSE
- 但返回值里应包含 `eventsUrl`
- 如果任务仍在执行中，调用方应继续订阅 `GET /api/v1/server-tasks/{taskId}/events`

## 成功响应示例

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "taskId": "server-task-1782800001001",
    "taskType": "browser_env_run",
    "resourceType": "browser_env",
    "resourceId": "906090001_tk_324867594169356288",
    "status": "running",
    "currentStage": "dispatch_edge_run",
    "message": "run admission passed, dispatching edge run",
    "eventsUrl": "/api/v1/server-tasks/server-task-1782800001001/events",
    "createdAt": "2026-06-30T10:00:00+08:00",
    "updatedAt": "2026-06-30T10:00:03+08:00",
    "finishedAt": "",
    "error": "",
    "suggestion": ""
  }
}
```
