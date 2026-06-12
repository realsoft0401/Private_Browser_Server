# Node Server 接口设计：`GET /api/v1/server/tasks`

## 1. 功能目标

`GET /api/v1/server/tasks` 用于返回当前主账号下的中心任务列表。

它的重点是：

- 提供平台级任务历史查询
- 让管理员从中心视角查看 env/client 对应的任务流
- 作为 task detail 的列表入口

## 2. 数据来源

- `server_tasks`

## 3. 业务边界

- 列表接口不主动刷新大量 Edge task
- 实时状态刷新主要依赖 [task-detail.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/task-detail.md)

## 4. 支持过滤

- `clientId`
- `envId`
- `type`
- `status`
- `page`
- `pageSize`

说明：

- 列表项里的真实持久化字段名是 `type`
- `taskType` 只出现在 `StartTaskResponse` 这类“创建任务成功后的即时摘要”里

## 5. 请求与响应

```http
GET /api/v1/server/tasks
```

必须带 Platform Header。

成功返回：

- `total`
- `page`
- `pageSize`
- `items`

每个任务重点字段：

- `taskId`
- `type`
- `status`
- `clientId`
- `envId`
- `edgeTaskId`
- `errorMessage`
- `createdAt`
- `updatedAt`

## 6. 成功判定

- 能按主账号返回中心任务列表
- 过滤条件和分页生效

## 7. 失败判定

- Platform Header 缺失
- SQLite 查询失败

## 8. 联调验收标准

- `type=run_env` 只能返回 run 任务
- `status=running` 过滤结果正确
- 列表接口不会因为某个 Edge task 已丢失而主动失败
