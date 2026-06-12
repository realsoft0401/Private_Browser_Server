# Node Server 接口设计：`POST /api/v1/envs`

## 1. 功能目标

`POST /api/v1/envs` 用于让 `Private_Browser_Server` 代表平台或管理员，在指定 `Edge Client` 上创建一份新的浏览器环境包，并把该环境登记为中心可见资产。

该接口的成功结果不只是“Edge 创建了一个目录”，而是：

- 目标 Edge 已创建环境包资产
- Node Server 已写入中心聚合记录 `server_browser_envs`
- 该环境已绑定到明确的 `clientId`
- 后续 `run/stop/backup/restore/delete` 都围绕这条中心记录继续执行

## 2. 设计来源

- 当前项目已经明确“环境包属于某个主账号，并固定绑定某个中心节点”。
- 用户要求 `imagePolicy` 必须由中心层受控，不能让普通前端绕过后端直接指定镜像。
- Edge 是本地资产事实源，但企业级前端和调度系统不能直接盯 Edge，每次都需要一个稳定的中心聚合入口。

## 3. 业务边界

### 3.1 这个接口负责什么

- 校验 `clientId` 是否可用
- 校验目标节点是否 `healthy + verified + online`
- 根据目标节点架构和 `imagePolicy` 解析最终 `runtime.image`
- 调用 Edge 创建环境包
- 写入 `server_browser_envs`

### 3.2 这个接口不负责什么

- 不自动 `run`
- 不扫描 Edge 目录确认是否已有同名目录
- 不直接读 Edge SQLite
- 不保存 profile 明文、proxy 明文、fingerprint raw、browser-data
- Edge 创建成功但中心写库失败时，不自动回滚删 Edge 资产

## 4. 请求与响应

## 4.1 请求

```http
POST /api/v1/envs
```

请求体核心字段：

- `clientId`
- `name`
- `rpaType`
- `runtime.imagePolicy`
- `runtime.startupUrl`
- `runtime.shmSize`
- `environment`
- `proxy`
- `fingerprint`
- `metadata`

## 4.2 成功响应

当前是同步接口，成功后直接返回：

- `envId`
- `mainAccountId`
- `clientId`
- `status=created`
- `ports`
- `identityHash`
- `cdpUrl`
- `webVncUrl`
- `env`

## 4.3 失败响应

常见失败应包括：

- `clientId` 不存在或不属于当前主账号
- 节点未通过 `EnsureClientReadyForBusiness`
- `imagePolicy` 不能解析为当前节点架构允许的镜像
- Edge 创建失败
- 中心写库失败

## 5. 前置校验

创建前必须按这个顺序执行：

1. 校验 Platform Header
2. 校验请求体格式和必填字段
3. 用 `clientId` 调 `EnsureClientReadyForBusiness`
4. 根据 `client.Arch` 解析 `imagePolicy`
5. 组装 Edge 创建请求
6. 调用 Edge `/api/v1/edge/browser-envs`

## 6. 状态流转

成功创建后的中心主状态固定为：

```text
created
```

创建接口本身不负责推进到：

- `running`
- `stopped`
- `backed_up`
- `deleted`

这些状态必须由后续生命周期动作推进。

## 7. 中心缓存写入规则

成功创建后，Node Server 必须写入一条新的 `server_browser_envs`：

- `env_id`
- `main_account_id`
- `client_id`
- `rpa_type`
- `name`
- `status=created`
- `container_status=unknown`
- `monitor_status=unknown`
- `cdp_url`
- `web_vnc_url`
- `last_task_id=""`
- `last_error=""`

## 8. 错误与日志规范

虽然当前不是 task 接口，但失败仍然必须留下服务日志。

建议日志字段：

- `mainAccountId`
- `clientId`
- `imagePolicy`
- `resolvedImage`
- `stage`
- `errorSource`
- `error`
- `suggestion`

建议 `stage`：

- `server_precheck`
- `image_policy_resolve`
- `edge_create`
- `center_cache_create`

## 9. 联调验收标准

至少覆盖：

- `amd64` 节点创建成功
- `arm64` 节点创建成功
- `imagePolicy` 与节点架构不匹配
- 节点不是 `verified`
- Edge 创建成功但中心写库失败

## 10. 当前实现状态

截至 `2026-06-12`：

- 接口已实现
- 当前为同步创建
- 后续如果需要更复杂的创建前编排，再考虑升级为 task 模式
