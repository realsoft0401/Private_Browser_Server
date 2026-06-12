# Node Server 接口设计：`GET /api/v1/server/tasks/{taskId}/events`

## 1. 功能目标

`GET /api/v1/server/tasks/{taskId}/events` 用于返回中心任务的 SSE 事件流。

它的核心价值是：

- 把 Node Server 自己的前置编排阶段展示出来
- 在必要时回退代理 Edge task SSE
- 给前端和管理员一个实时观察通道

## 2. 设计来源

- `run` 这类动作存在中心镜像预检和拉镜像阶段，只透传 Edge SSE 无法覆盖完整过程。
- 用户要求企业级 API 不只是“最后成功/失败”，还要能看到过程。

## 3. 业务边界

### 3.1 负责什么

- 优先输出中心任务事件流
- 若当前 task 没有本地事件流，则回退代理 Edge SSE

### 3.2 不负责什么

- 不替代 `server_tasks` 持久事实
- 连接中断不等于任务失败

## 4. 请求与响应

```http
GET /api/v1/server/tasks/{taskId}/events
Accept: text/event-stream
```

必须带 Platform Header。

返回是 `text/event-stream`，不是统一 JSON 包装。

## 5. 典型阶段

当前建议关注：

- `queued`
- `image_check`
- `pulling_image`
- `edge_run`
- `finalize`
- `heartbeat`

## 6. 事件来源规则

- `run_env` 这类有中心前置编排的任务，优先走 Node Server 本地事件流
- 没有本地事件流且已绑定 `edgeTaskId` 的任务，会回退代理 Edge SSE
- 本地事件流不存在且 `edgeTaskId` 也为空时，当前接口应明确失败

## 7. 联调验收标准

- run 任务能看到中心阶段事件
- 没有中心阶段事件的任务能回退代理 Edge SSE
- SSE 断开后 task detail 仍可查询最终事实
