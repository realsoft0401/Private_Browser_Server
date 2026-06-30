# Server Browser Env Delete Package

这份文档只说明中心正式接口：

- `DELETE /api/v1/browser-envs/{envId}/package`

---

## 1. 业务语义

由 Node Server 发起一次中心 browser-env package delete。

它不是同步最终结果接口，而是：

- 中心先接单
- 转发到目标 Edge 正式 `package delete`
- 持续观察 Edge task
- Edge 删除成功后，把中心 `server_browser_envs` 这条缓存记录移除

---

## 2. 它负责什么

- 读取 `server_browser_envs` 中心聚合记录
- 校验目标节点当前 `healthy + verified`
- 创建 `server_tasks`
- 发起目标 Edge `DELETE /api/v1/edge/browser-envs/{envId}/package`
- 轮询 Edge task 终态
- 成功后删除中心 `server_browser_envs` 当前缓存

---

## 3. 它不负责什么

- 不自动 stop
- 不自动 backup
- 不删除 Docker 镜像
- 不保留一条可继续业务使用的中心 env 主记录

---

## 4. 当前请求体口径

当前正式不收请求体。

```http
DELETE /api/v1/browser-envs/906090001_tk_324867594169356288/package
```

---

## 5. 前置条件

必须同时满足：

1. `server_browser_envs` 能查到这条 env
2. env 已绑定到某个正式 `clientId`
3. 目标节点当前 `healthStatus=healthy`
4. 目标节点当前 `discoveryStatus=verified`

补充边界：

- env 是否允许 delete，由 Edge 正式 package delete 协议决定
- 中心不在这里偷偷 stop，不重写边缘资产删除规则

---

## 6. 状态机与收口

### 中心任务

- 发起成功时，先创建 `server_tasks`
- 任务终态只允许：
  - `success`
  - `failed`

### SSE 阶段

当前最小正式阶段包括：

- `load_server_env`
- `dispatch_edge_delete_package`
- `edge_task_accepted`
- `edge.<edge_stage>`
- `finalize_success`
- `finalize_edge_failed`
- `finalize_cache_delete_failed`

### 成功判定

要同时满足：

1. Edge package delete 接单成功
2. Edge task 终态为 `success`
3. 中心 `server_browser_envs` 当前缓存删除成功

### 失败判定

任一环节失败都必须收口成 `failed`，包括：

- 目标节点不可达
- Edge package delete 接单失败
- Edge task detail 查询失败
- Edge task 最终 failed
- Edge task success 但中心缓存删除失败

---

## 7. SSE 说明

SSE 任务化接口。立即返回 JSON `taskId/eventsUrl`。

发起接口只表示中心接单成功，不表示本次 package delete 已经最终成功。

调用方必须继续订阅 `/api/v1/server-tasks/{taskId}/events`，观察 `task.progress`、
`task.completed`、`task.failed` 事件，才能确认本次 delete 的最终结论。

---

## 8. 与相近接口的边界

它不会替代：

- `POST /api/v1/browser-envs/{envId}/backup`
  - backup 是保留资产后释放运行目录
- `DELETE /api/v1/edge/browser-envs/{envId}/package`
  - 这是 Edge 本机正式执行接口，不是中心接口
- `DELETE /api/v1/browser-envs/{envId}/del`
  - `del` 只应处理镜像清理，不该删除环境资产；但当前中心侧尚未开放，因为 Client 还没有正式实现该能力
