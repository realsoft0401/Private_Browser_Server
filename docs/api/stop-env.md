# Node Server 接口设计：`POST /api/v1/envs/{envId}/stop`

## 1. 功能目标

`POST /api/v1/envs/{envId}/stop` 用于让 `Private_Browser_Server` 发起一次中心环境停止任务，并把最终结果沉淀为中心 task 事实。

## 2. 设计来源

- 用户明确要求 stop 也必须遵守统一节点放行门槛，不能因为它是“收尾动作”就绕过 `healthy + verified + online`。
- 用户要求所有失败都不能自动重试，也不能只依赖 Edge 短期 task。

## 3. 业务边界

### 3.1 负责什么

- 校验 env 与 client
- 创建中心 task
- 调 Edge stop
- 绑定 `edgeTaskId`
- 在 task detail 阶段收口最终状态

### 3.2 不负责什么

- 不直接改容器状态
- 不因为 stop 是收尾动作就放宽节点门槛
- 不自动重试 stop

## 4. 请求与响应

## 4.1 请求

```http
POST /api/v1/envs/{envId}/stop
```

可选请求体：

- `timeoutSeconds`

## 4.2 响应

立即返回中心任务摘要：

- `taskId`
- `taskType=stop_env`
- `clientId`
- `envId`
- `edgeTaskId`
- `eventsUrl`

## 5. 前置校验

1. 根据 `envId` 读取中心 env
2. 调 `EnsureClientReadyForBusiness`
3. 允许 stop 的具体环境状态以后续实现为准，但第一版仍以统一业务放行门槛为前提

## 6. 任务编排

```text
stop request
  -> 创建中心 task(stop_env)
  -> 调 Edge /stop
  -> 绑定 edgeTaskId
  -> task detail / SSE 观察
  -> finalize
```

## 7. 成功判定

下面任一成立即可记 `success`：

1. Edge task 明确 `success`
2. Edge task 丢失，但重新读取 env 后能确认停止事实，例如：
   - `status=stopped`
   - `status=created`
   - `status=backed_up`
   - `containerStatus=exited`
   - `containerStatus=missing`

## 8. 失败判定

- 节点不 ready
- Edge stop 调用失败
- Edge task 明确 failed
- Edge task 丢失且无法确认停止事实

## 9. 中心缓存收口

成功后按 Edge env detail 刷新：

- `status`
- `container_status`
- `monitor_status`
- `cdp_url`
- `web_vnc_url`
- `last_task_id`
- `last_error`

## 10. 错误与日志规范

至少要有：

- `server_tasks.error_message`
- `env.last_error`
- task SSE
- 服务端结构化日志

建议日志字段：

- `taskId`
- `taskType=stop_env`
- `clientId`
- `envId`
- `stage`
- `errorSource`
- `error`
- `suggestion`

## 11. 联调验收标准

- 正常 stop 成功
- 节点 offline 时拒绝 stop
- Edge task failed
- Edge task 丢失但可确认停止
- Edge task 丢失且不可确认停止
