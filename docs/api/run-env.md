# Node Server 接口设计：`POST /api/v1/envs/{envId}/run`

## 1. 功能目标

`POST /api/v1/envs/{envId}/run` 用于让 `Private_Browser_Server` 发起一次中心环境启动任务。

这个接口的目标不是简单地“把请求转发给 Edge”，而是让 Node Server 负责：

- 校验目标环境和节点是否允许启动
- 读取 Edge 环境详情，确认最终 `runtime.image`
- 先检查镜像是否已存在
- 缺镜像时先拉镜像
- 再触发 Edge run
- 给前端和管理员提供中心 task + SSE 可观察过程

## 2. 设计来源

- 用户要求 run 不能静默依赖 Edge 自己去选镜像或偷偷失败。
- 用户要求前端不只拿到一个 taskId，还要能看到 `image_check / pulling_image / edge_run` 这种中心层阶段。
- 用户要求资产动作失败不自动重试，因此 Node Server 必须把预检、调用和失败原因编排清楚。

## 3. 业务边界

### 3.1 这个接口负责什么

- 创建中心 task
- 在后台执行镜像预检与必要的 pull-image
- 启动 Edge run
- 维护中心 SSE 事件流
- 最终把 success/failed 收口到中心 task

### 3.2 这个接口不负责什么

- 不自动切换到别的节点
- 不自动更改环境里的 `runtime.image`
- 不自动重试失败的 pull-image 或 run
- 不直接读 Edge SQLite 或目录

## 4. 请求与响应

## 4.1 请求

```http
POST /api/v1/envs/{envId}/run
```

可选请求体：

- `forceRecreate`

## 4.2 成功响应

立即返回中心任务摘要：

- `taskId`
- `taskType=run_env`
- `status`
- `clientId`
- `envId`
- `eventsUrl`

## 5. 前置校验

执行前必须完成：

1. 根据 `envId` 读取 `server_browser_envs`
2. 调 `EnsureClientReadyForBusiness`
3. 读取 Edge env detail
4. 校验当前环境状态允许 run
5. 读取 `profile.runtime.image`
6. 检查该镜像是否已存在于目标 Edge

## 6. 任务编排

推荐按当前实现理解：

```text
run request
  -> 创建中心 task(run_env)
  -> server_precheck
  -> image_check
  -> missing? pull_image
  -> edge_run
  -> edge_task_poll
  -> finalize
```

## 7. 成功判定

下面两种情况之一成立，中心 task 才能记 `success`：

1. Edge task 明确 `success`
2. Edge task 丢失，但再次读取 Edge env detail 后，确认 `status=running`

## 8. 失败判定

统一记 `failed` 的情况：

- 目标 env 不存在
- 节点不 ready
- Edge env detail 不可读
- `runtime.image` 为空或非法
- 镜像预检失败
- pull-image 失败
- Edge run 失败
- Edge task 丢失且无法确认 `running`
- 中心任务或缓存回写失败

## 9. 中心缓存收口

run 成功后中心缓存应至少刷新：

- `status=running`
- `container_status` 按 Edge detail
- `monitor_status` 按 Edge detail
- `cdp_url`
- `web_vnc_url`
- `last_task_id`
- `last_error=""`

run 失败后：

- 不伪造 `running`
- 保留 `last_task_id`
- 回写 `last_error`

## 10. 错误与日志规范

必须能给管理员留下：

- 中心 task 错误
- SSE 阶段事件
- env `last_error`
- 结构化日志

建议日志字段：

- `taskId`
- `taskType=run_env`
- `clientId`
- `envId`
- `stage`
- `image`
- `errorSource`
- `error`
- `suggestion`

## 11. 联调验收标准

至少覆盖：

- 镜像已存在，直接 run 成功
- 镜像不存在，先 pull 再 run 成功
- pull-image 失败
- Edge run 失败
- Edge task 丢失但 env 已 `running`
- Edge task 丢失且 env 未 `running`

## 12. 当前实现状态

截至 `2026-06-12`：

- 接口已实现
- 已具备镜像预检、必要拉镜像、中心 task 与 SSE 编排能力
