# GET /api/v1/server-tasks/{taskId}/events

## 功能目标

订阅 Node Server 统一的中心任务 SSE 事件流。

只要某个中心业务接口说明“立即返回 `taskId/eventsUrl`”，调用方就应该通过这条统一入口继续订阅任务过程，而不是把接单响应误解成最终结果。

## 业务边界

- 负责返回 `text/event-stream`
- 负责先补发当前任务已记录的历史事件
- 负责继续推送后续事件
- 不负责创建 task
- 不负责替代 `GET /api/v1/server-tasks/{taskId}`

## SSE 说明

SSE 事件流接口，返回 `text/event-stream`。

典型事件包括：

- `task.progress`
- `task.completed`
- `task.failed`

## 事件示例

```text
event: task.progress
data: {"event":"task.progress","taskId":"server-task-1782800001001","taskType":"browser_env_run","resourceType":"browser_env","resourceId":"906090001_tk_324867594169356288","clientId":"9060901190003","envId":"906090001_tk_324867594169356288","slotId":"slot001","stage":"dispatch_edge_run","status":"running","message":"run admission passed, dispatching edge run","timestamp":"2026-06-30T10:00:03+08:00"}

event: task.completed
data: {"event":"task.completed","taskId":"server-task-1782800001001","taskType":"browser_env_run","resourceType":"browser_env","resourceId":"906090001_tk_324867594169356288","clientId":"9060901190003","envId":"906090001_tk_324867594169356288","slotId":"slot001","stage":"finalize_success","status":"success","message":"browser env run completed","timestamp":"2026-06-30T10:00:20+08:00"}
```
