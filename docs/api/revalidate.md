# Node Server 接口设计：`POST /api/v1/envs/{envId}/revalidate`

## 1. 功能目标

`POST /api/v1/envs/{envId}/revalidate` 用于让 `Private_Browser_Server` 在管理员完成排查后，代表平台向目标 `Private_Browser_Client` 发起一次受控重新准入动作。

它的目标不是运行环境，而是恢复“准入资格”：

- 重新校验 `profile.json`、`binding.json`、`proxy/`、`fingerprint/`、`browser-data/profile` 等原子材料
- 重新确认 Docker 身份事实与本机端口可用性
- 把异常环境从 `status=error` 恢复到允许继续生命周期的 `created` 或 `stopped`
- 把 `runtimeProtection/proxyRuntime` 重置回待重新验证状态

成功后的业务结论应是：

- Edge 已完成重新准入校验
- 中心层和 Edge 层都不再停留在 `status=error`
- 当前不会自动启动容器、自动拉镜像或自动执行网络指纹验证
- 调用方下一步可根据实际状态显式执行 `run`

## 2. 设计来源

- 用户已经明确：`status=error` 不能被 `run/stop/backup/proxy update` 等普通动作隐式恢复。
- 必须存在一个独立、受控、可审计的重新准入动作。
- Client 侧 `revalidate` 已经实现为正式 Edge task/SSE 接口，因此 Node Server 不能把它伪装成同步接口。
- 用户明确要求所有正式接口都要补齐企业级任务、状态机、错误留痕和管理员排障文档。

## 3. 业务边界

### 3.1 这个接口负责什么

- 只负责“让指定 envId 在其原绑定 Edge 上执行 revalidate”
- 负责创建中心 task、调用 Edge `revalidate`、绑定 `edgeTaskId`
- 负责在 Edge task 丢失时，再次读取 Edge 环境详情确认是否已恢复到正常准入状态
- 负责把失败原因和修复建议留给管理员

### 3.2 这个接口不负责什么

- 不启动容器
- 不拉镜像
- 不替代 `restore`
- 不替代 `run`
- 不修复登录态内容本身
- 不跨节点恢复
- 不自动重试

### 3.3 与相近接口的边界

- `revalidate`：只恢复异常环境的准入资格，不负责真正启动
- `restore`：只从本机备份包恢复目录，不校验 error 环境重新准入
- `run`：启动浏览器容器并完成运行态验证，不负责解除 `status=error`
- `backup`：归档并释放目录，不负责修复异常环境

## 4. 请求与响应

## 4.1 请求

```http
POST /api/v1/envs/{envId}/revalidate
```

路径参数：

- `envId`：要重新准入的中心环境包 ID

第一版不设计请求体。

原因：

- Edge 侧 revalidate 当前不需要业务参数
- 重新准入只能基于现有环境包原子材料和 Docker 事实受控执行
- 不允许前端透传修复开关，避免 revalidate 退回成不受控“顺手修环境”接口

## 4.2 成功响应

接口本身采用异步任务模式，立即返回中心任务摘要：

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "taskId": "task_1770000000000000000",
    "taskType": "revalidate_env",
    "status": "pending",
    "clientId": "edge_client_001",
    "envId": "906090119_tk_323407300419129344",
    "edgeTaskId": "task_1770000000000000000_12345",
    "eventsUrl": "http://node-server.example/api/v1/server/tasks/task_1770000000000000000/events",
    "message": "环境包重新校验任务已创建",
    "createdAt": 1770000000
  }
}
```

说明：

- `taskId` 是中心任务 ID
- `edgeTaskId` 是 Edge revalidate task ID
- 调用方应继续通过 task detail 或 SSE 观察最终结果

## 4.3 失败响应

失败仍使用统一错误包装，但必须返回清晰可执行文案。例如：

- `只有 status=error 的环境包允许 revalidate；当前状态为 created`
- `目标 Edge Client 当前不是 healthy + verified + online，禁止执行 revalidate`
- `环境包当前状态为 backed_up，请先 restore 后再 revalidate`

## 5. 前置校验

Node Server 在真正创建中心 task 前，必须完成下面这组校验。

## 5.1 中心层校验

1. 根据 `envId` 查询 `server_browser_envs`
2. 确认该环境属于当前 `mainAccountId`
3. 读取其绑定的 `clientId`
4. 调用 `EnsureClientReadyForBusiness`

只有当节点满足以下条件时才允许继续：

- `health_status=healthy`
- `discovery_status=verified`
- `heartbeatStatus=online`

## 5.2 Edge 预检

中心层还必须先调用一次：

```text
GET /api/v1/edge/browser-envs/{envId}
```

用于确认环境包当前状态，而不是直接盲发 revalidate。

允许进入 revalidate 的第一版状态：

- `error`

必须拒绝的状态：

- `created`
- `stopped`
- `running`
- `backed_up`
- `archived`
- `deleted`

### 5.3 为什么要先做 Edge 预检

- 让管理员更早拿到明确错误，而不是中心 task 创建后才失败
- 避免“本来不需要 revalidate 的环境”被包成假异步任务
- 保证企业级 API 的错误语义稳定，不依赖前端自己猜哪些状态能重新准入

## 6. 状态流转

第一版建议明确为：

```text
error     -> created   允许
error     -> stopped   允许
created   -> created   禁止，已经是正常准入状态
stopped   -> stopped   禁止，已经是正常准入状态
running   -> running   禁止，必须先管理员排查运行态问题
backed_up -> created   禁止，必须先 restore
deleted   -> deleted   禁止
```

revalidate 成功后的后续约束：

- 不自动 `run`
- `runtimeProtection/proxyRuntime` 只回到待验证状态，不证明最终网络指纹可用

## 7. 任务编排

该接口必须采用中心 task + Edge task 绑定模式。

## 7.1 中心编排流程

```text
POST /api/v1/envs/{envId}/revalidate
  -> 查 server_browser_envs
  -> EnsureClientReadyForBusiness
  -> GET Edge env detail 预检
  -> 创建 server_tasks(type=revalidate_env)
  -> 回写 server_browser_envs.last_task_id
  -> POST Edge /api/v1/edge/browser-envs/{envId}/revalidate
  -> 保存 edgeTaskId 到 server_tasks
  -> 返回中心 taskId/eventsUrl
  -> 后续由 task detail / SSE 驱动终态收口
```

## 7.2 为什么要走中心 task

- revalidate 是正式异常恢复动作，必须在中心层可审计
- Client 侧 task 只是短期内存事实，不能替代平台任务
- 前端和管理员必须能在 Node Server 看到统一任务视图和最终结论

## 7.3 SSE 阶段建议

当前实现会先走中心 task 启动，再复用 Edge task 事件流。

至少要能看到：

- `queued`
- `running`
- `done` / `error`

如果后续中心层再补本地前置阶段，也应继续挂在同一条中心任务流上。

## 8. 成功判定

Node Server 只能按“重新准入事实”确认 revalidate 成功，不能因为 Edge task 曾经存在就默认成功。

## 8.1 明确成功

下面两种情况之一成立，中心 task 才能记 `success`：

1. Edge task 明确终态为 `success`
2. Edge task 丢失，但再次读取 Edge 环境详情后，确认：
   - `status=created`，或
   - `status=stopped`

## 8.2 关键成功事实

revalidate 的成功事实是：

```text
Edge env detail -> index.status == created/stopped
```

而不是：

- “Edge 曾经回过 200”
- “taskId 曾经创建过”
- “SSE 曾经跑到某一步”

## 9. 失败判定

下面这些都必须统一收口为中心 `failed`：

- 中心 env 不存在
- 节点不 ready
- Edge detail 预检失败
- 预检发现 env 状态不允许 revalidate
- 创建中心 task 失败
- 调 Edge `/revalidate` 失败
- Edge task 明确 `failed/error`
- Edge task 丢失，且无法确认 `status=created/stopped`
- Edge 实际可能成功，但中心缓存回写失败

## 10. 中心缓存收口规则

revalidate 成功后，中心缓存要以 Edge detail 当前事实为准，而不是写死某一个状态。

原因：

- Client revalidate 成功后可能恢复到 `created`，也可能恢复到 `stopped`
- Node Server 只保存聚合摘要，不复制 Client 全量运行态与 runtimeProtection 细节

## 10.1 成功后更新

revalidate 成功后中心缓存至少更新：

- `status=created` 或 `status=stopped`
- `container_status` 以 Edge detail 为准
- `monitor_status` 以 Edge detail 为准
- `cdp_url/web_vnc_url` 以 Edge detail 刷新结果为准
- `last_task_id=当前中心 taskId`
- `last_error=""`

## 10.2 失败后更新

revalidate 失败后：

- 不伪造正常准入状态
- 保持原主状态
- 更新 `last_task_id`
- 更新 `last_error`

## 11. 错误与日志规范

revalidate 是异常恢复动作，一旦失败必须给管理员留下足够排障信息。

## 11.1 最少留痕位置

至少同时落到：

- `server_tasks.error_message`
- `server task SSE` / Edge 代理 SSE 阶段事件
- `server_browser_envs.last_error`
- Node Server 结构化日志

## 11.2 结构化日志字段

至少包含：

- `taskId`
- `taskType=revalidate_env`
- `mainAccountId`
- `clientId`
- `envId`
- `edgeTaskId`
- `stage`
- `status`
- `errorSource`
- `message`
- `error`
- `suggestion`
- `occurredAt`

建议补充：

- `nodeBaseUrl`
- `envStatus`
- `containerStatus`
- `httpStatus`
- `edgeCode`
- `edgeMessage`

## 11.3 `errorSource` 建议枚举

- `edge_precheck`
- `task_create`
- `edge_http`
- `edge_task_failed`
- `edge_task_missing`
- `state_confirm_failed`
- `snapshot_sync_failed`

## 11.4 日志脱敏要求

禁止记录：

- 代理明文
- fingerprint raw
- browser-data 内容
- Cookies
- Local Storage
- IndexedDB
- Session Storage
- Login Data

## 11.5 错误文案要求

错误不能只写“重新校验失败”，必须包含：

- 失败原因
- 影响范围
- 修复建议

示例：

- `只有 status=error 的环境包允许 revalidate；当前状态为 stopped，说明环境已恢复正常准入，不需要重复调用`
- `环境包当前状态为 backed_up，请先 restore 后再 revalidate`
- `Edge revalidate task 已丢失，且当前环境状态仍不是 created/stopped；Node Server 无法确认重新准入是否完成，请管理员检查 Edge 服务日志与环境包详情`

## 12. 联调验收标准

第一版至少覆盖以下场景：

## 12.1 成功路径

1. 目标 env 当前为 `error`
2. 调用 `POST /api/v1/envs/{envId}/revalidate`
3. 中心返回 `taskId` 和 `edgeTaskId`
4. task 最终收口为 `success`
5. Edge env detail 显示 `status=created` 或 `status=stopped`
6. Node Server 环境列表里的该 env 也同步为对应正常准入状态

## 12.2 关键失败路径

至少验证：

- env 当前不是 `error`
- env 当前为 `backed_up`
- 节点 `heartbeatStatus=offline`
- 节点 `health_status=unhealthy`
- Edge `/revalidate` HTTP 调用失败
- Edge task 明确 `failed`
- Edge task 丢失且不能确认 `created/stopped`
- 中心缓存回写失败

## 12.3 通过标准

满足以下条件才算接口完成：

- 成功路径可稳定收口
- 失败路径都有明确中文错误和修复建议
- 管理员能在 task detail、SSE、env 摘要、服务日志中看到一致错误事实
- 没有自动 run、自动拉镜像、自动重试这类越权补救动作

## 13. 当前实现状态

截至 `2026-06-12`，本文件是 `revalidate` 接口的设计、实现与联调标准文档。

当前代码现状：

- Edge `POST /api/v1/edge/browser-envs/{envId}/revalidate` 已存在
- Node Server `POST /api/v1/envs/{envId}/revalidate` 第一版已落地
- 当前实现采用“中心 task + Edge task 绑定 + task detail/SSE 收口”模式
- 本文档继续作为后续 `import-package` 对齐的参考样板
