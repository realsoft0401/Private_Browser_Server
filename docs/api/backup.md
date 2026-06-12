# Node Server 接口设计：`POST /api/v1/envs/{envId}/backup`

## 1. 功能目标

`POST /api/v1/envs/{envId}/backup` 用于让 `Private_Browser_Server` 代表平台或管理员，向目标 `Private_Browser_Client` 发起一次受控环境包备份动作。

该接口的业务目标不是“下载一份副本”，而是：

- 让目标 Edge Client 把当前环境包归档为受控 `tar.gz`
- 删除该环境包在目标 Edge 上的源运行目录
- 删除该环境包关联的已停止容器
- 保留 Edge 本地 SQLite 索引
- 把环境包主状态推进到 `backed_up`
- 为后续 `restore` 保留唯一可信资产来源

成功后的业务结论应是：

- 当前环境已不再占用原运行目录
- 中心层和 Edge 层都能看到 `status=backed_up`
- 后续不能直接 `run`，必须先 `restore`

## 2. 设计来源

- `Private_Browser_Client` 已经把 `backup` 定义为“归档资产并释放本机运行目录”，而不是旧的临时导出/下载接口。
- 当前中心层已经具备 `create/list/detail/run/stop/del/package` 第一版生命周期代理能力，`backup` 是下一个必须补齐的正式资产动作。
- 用户明确要求后续所有正式接口都必须沉淀成企业级 API 文档资产，不能只靠 `openapi.yaml` 或零散对话维持口径。
- 用户明确要求：一旦失败，必须留下管理员可追溯的错误日志、任务错误和资源摘要错误，不能只在 HTTP 返回里报错后消失。

## 3. 业务边界

### 3.1 这个接口负责什么

- 只负责“让指定 envId 在其原绑定 Edge 上执行 backup”
- 负责创建中心 task、绑定 edge task、刷新中心环境摘要
- 负责在 Edge task 丢失时再次读取 Edge 状态，确认动作是否真的完成
- 负责把失败原因和修复建议留给管理员

### 3.2 这个接口不负责什么

- 不负责下载备份包
- 不负责把备份包搬运到 Node Server 或其它服务器
- 不负责跨节点迁移环境
- 不自动 `stop`
- 不自动 `restore`
- 不自动重试
- 不读取 Edge SQLite、环境目录或备份 tar 文件本体

### 3.3 与相近接口的边界

- `backup`：归档并释放运行目录，保留可恢复资产
- `restore`：从 Edge 本机已登记备份包恢复环境目录
- `import-package`：导入外部标准包，不读取本机 `backupPath`
- `DELETE /api/v1/envs/{envId}/package`：彻底销毁环境资产，不保留恢复入口
- `DELETE /api/v1/envs/{envId}/del`：只删除运行镜像，不处理环境资产

## 4. 请求与响应

## 4.1 请求

```http
POST /api/v1/envs/{envId}/backup
```

路径参数：

- `envId`：要备份的中心环境包 ID

第一版不设计请求体。

原因：

- Edge 侧 backup 当前也不需要业务参数
- 备份路径、文件名、是否覆盖、校验方式都必须由 Edge 本机受控生成
- 不允许前端透传危险文件参数，避免企业级 API 变成文件系统写入口

## 4.2 成功响应

接口本身采用异步任务模式，立即返回中心任务摘要：

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "taskId": "task_1770000000000000000",
    "taskType": "backup_env",
    "status": "pending",
    "clientId": "edge_client_001",
    "envId": "906090119_tk_323407300419129344",
    "edgeTaskId": "edge_task_1770000000000000001",
    "eventsUrl": "http://node-server.example/api/v1/server/tasks/task_1770000000000000000/events",
    "message": "环境包备份任务已创建",
    "createdAt": 1770000000
  }
}
```

说明：

- `taskId` 是中心任务 ID
- `edgeTaskId` 只有在成功创建 Edge task 后才有值
- 调用方应继续通过 task detail 或 SSE 观察最终结果

## 4.3 失败响应

失败仍使用统一错误包装，但必须返回清晰可执行文案。例如：

- `环境包正在运行，请先执行 stop 后再 backup`
- `目标 Edge Client 当前不是 healthy + verified + online，禁止执行 backup`
- `环境包已是 backed_up，请先 restore 后再重新 backup`

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

用于确认环境包当前状态，而不是直接盲发 backup。

必须拒绝的状态：

- `status=running`
- `status=error`
- `status=deleted`
- `status=backed_up`
- `status=archived`

允许进入 backup 的第一版状态：

- `created`
- `stopped`

### 5.3 为什么要先做 Edge 预检

- 让管理员更早拿到明确错误，而不是中心 task 创建后才失败
- 避免“任务已创建但本来就不该执行”的假异步噪音
- 保证企业级 API 的错误语义稳定，不依赖前端自己猜状态机

## 6. 状态流转

第一版建议明确为：

```text
created  -> backed_up   允许
stopped  -> backed_up   允许
running  -> backed_up   禁止，必须先 stop
error    -> backed_up   禁止，必须先修复并 revalidate
backed_up -> backed_up  禁止重复 backup
deleted  -> backed_up   禁止
```

backup 成功后的后续约束：

- 不允许直接 `run`
- 必须先 `restore`

## 7. 任务编排

该接口必须采用中心 task 模式。

## 7.1 中心编排流程

```text
POST /api/v1/envs/{envId}/backup
  -> 查 server_browser_envs
  -> EnsureClientReadyForBusiness
  -> GET Edge env detail 预检
  -> 创建 server_tasks(type=backup_env)
  -> 回写 server_browser_envs.last_task_id
  -> POST Edge /api/v1/edge/browser-envs/{envId}/backup
  -> 绑定 edgeTaskId
  -> 返回中心 taskId/eventsUrl
```

## 7.2 为什么要走中心 task

- backup 属于不可逆的资产动作，必须在中心层可审计
- 不能只依赖 Edge 内存 task
- 前端和管理员必须能在 Node Server 看到统一任务视图

## 7.3 SSE 阶段建议

建议中心 SSE 至少区分这些阶段：

- `server_precheck`
- `edge_precheck`
- `server_task_create`
- `edge_backup_start`
- `edge_task_poll`
- `finalize`

## 8. 成功判定

Node Server 只能按“资产事实”确认 backup 成功，不能因为请求发出去过就默认成功。

## 8.1 明确成功

下面两种情况之一成立，中心 task 才能记 `success`：

1. Edge task 明确终态为 `success`
2. Edge task 丢失，但再次读取 Edge 环境详情后，确认：
   - 环境索引仍存在
   - `status=backed_up`

## 8.2 关键成功事实

backup 的成功事实是：

```text
Edge env detail -> index.status == backed_up
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
- 预检发现 env 状态不允许 backup
- 创建中心 task 失败
- 调 Edge `/backup` 失败
- Edge task 明确 `failed/error`
- Edge task 丢失，且无法确认 `status=backed_up`
- Edge 实际可能成功，但中心缓存回写失败

## 10. 中心缓存收口规则

第一版不建议扩充 `server_browser_envs` 表去保存 `backupPath/checksum/size`。

原因：

- Node Server 当前定位是中心聚合摘要，不复制 Edge 资产明细
- 当前最重要的中心事实是“该 env 是否已经 backed_up”

## 10.1 成功后更新

backup 成功后中心缓存至少更新：

- `status=backed_up`
- `container_status=missing` 或按 Edge detail 实际值
- `monitor_status=unknown`
- `cdp_url=""`
- `web_vnc_url=""`
- `last_task_id=当前中心 taskId`
- `last_error=""`
- `updated_at=Edge detail.updatedAt` 或当前时间

## 10.2 失败后更新

backup 失败后：

- 不伪造 `status=backed_up`
- 保持原主状态
- 更新 `last_task_id`
- 更新 `last_error`

## 11. 错误与日志规范

backup 是资产动作，一旦失败必须给管理员留下足够排障信息。

## 11.1 最少留痕位置

至少同时落到：

- `server_tasks.error_message`
- `server task SSE` 阶段事件
- `server_browser_envs.last_error`
- Node Server 结构化日志

## 11.2 结构化日志字段

至少包含：

- `taskId`
- `taskType=backup_env`
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

- `server_precheck`
- `edge_precheck`
- `server_task_create`
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

错误不能只写“备份失败”，必须包含：

- 失败原因
- 影响范围
- 修复建议

示例：

- `环境包正在运行，Node Server 已拒绝 backup；请先执行 stop，确认环境状态变为 stopped 或 created 后重试`
- `目标节点当前 heartbeatStatus=offline，禁止执行 backup；请先恢复节点连通性`
- `Edge backup task 已丢失，且当前环境状态不是 backed_up；Node Server 无法确认资产是否已归档，请管理员检查目标节点备份包与环境详情`

## 12. 联调验收标准

第一版至少覆盖以下场景：

## 12.1 成功路径

1. 目标 env 当前为 `stopped`
2. 调用 `POST /api/v1/envs/{envId}/backup`
3. 中心返回 `taskId`
4. task 最终收口为 `success`
5. Edge env detail 显示 `status=backed_up`
6. Node Server 环境列表里的该 env 也同步为 `backed_up`

## 12.2 关键失败路径

至少验证：

- env 正在 `running`
- 节点 `heartbeatStatus=offline`
- 节点 `health_status=unhealthy`
- env 已经 `backed_up`
- Edge `/backup` HTTP 调用失败
- Edge task 明确 `failed`
- Edge task 丢失且不能确认 `backed_up`
- 中心缓存回写失败

## 12.3 通过标准

满足以下条件才算接口完成：

- 成功路径可稳定收口
- 失败路径都有明确中文错误和修复建议
- 管理员能在 task detail、SSE、env 摘要、服务日志中看到一致错误事实
- 没有自动 stop、自动 retry、自动 restore 这类越权补救动作

## 13. 当前实现状态

截至 `2026-06-12`，本文件是 `backup` 接口的设计与联调标准文档。

当前代码现状：

- Edge `POST /api/v1/edge/browser-envs/{envId}/backup` 已存在
- Node Server `POST /api/v1/envs/{envId}/backup` 尚未落地
- 本文档用于指导后续 Node Server 的 `backup` 实现、OpenAPI 编写和联调验收
