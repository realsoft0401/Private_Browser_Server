# GET /api/v1/server-tasks

## 功能目标

查询 Node Server 持久化的中心任务列表，用于管理员查看最近生命周期动作、失败任务和任务审计摘要。

这条接口解决的是“没有 taskId 时怎么回看任务历史”的问题。它不替代单任务详情，也不替代 SSE 事件流。

## 业务边界

- 负责读取 `server_tasks` 主表中的任务摘要
- 支持按 `clientId/envId/resourceId/taskType/status` 过滤
- 支持 `page/pageSize` 分页，`pageSize` 最大 100
- 不访问 Edge Client
- 不补发 SSE 事件明细
- 不触发任务重试或生命周期动作

## 状态机与前置条件

- 该接口只读中心 SQLite，不要求目标 Client 在线
- 如果某个 Client 已经 offline，历史任务仍然可以查询
- 如果任务仍处于 `pending/running`，列表只返回当前主摘要；具体阶段应继续看 detail 或 events

## SSE 说明

- 本接口不使用 SSE
- 原因：列表查询可以同步返回明确结果，没有多阶段长过程
- 每条任务会返回 `eventsUrl`
- 如需观察单个任务过程，继续请求 `GET /api/v1/server-tasks/{taskId}/events`

## 请求示例

```bash
export SERVER_BASE="http://127.0.0.1:3400"

curl -s "$SERVER_BASE/api/v1/server-tasks?page=1&pageSize=20" | jq
```

按环境包过滤：

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks?envId=$ENV_ID&page=1&pageSize=20" | jq
```

按失败任务过滤：

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks?status=failed&page=1&pageSize=20" | jq
```

## 成功响应示例

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "items": [
      {
        "taskId": "server-task-1782800001001",
        "taskType": "browser_env_run",
        "resourceType": "browser_env",
        "resourceId": "906090001_tk_324867594169356288",
        "status": "success",
        "currentStage": "",
        "message": "",
        "eventsUrl": "/api/v1/server-tasks/server-task-1782800001001/events",
        "createdAt": "2026-06-30T10:00:00+08:00",
        "updatedAt": "2026-06-30T10:00:20+08:00",
        "finishedAt": "2026-06-30T10:00:20+08:00"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20
  }
}
```

## 验收标准

- 不带过滤条件能返回最近任务列表
- `clientId/envId/resourceId/taskType/status` 任一过滤条件都能正常收窄结果
- `pageSize` 超过 100 时服务端自动收紧为 100
- 列表接口返回 `application/json`，不返回 `text/event-stream`
- `GET /api/v1/server-tasks/{taskId}` 和 `/events` 继续保持原行为
