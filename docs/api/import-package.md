# Node Server 接口设计：`POST /api/v1/envs/import-package`

## 1. 功能目标

`POST /api/v1/envs/import-package` 用于让 `Private_Browser_Server` 代理目标 Edge 导入外部标准环境包，并把导入结果纳入中心资产视图。

该接口的业务目标不是“随便上传一个目录压缩包”，而是：

- 让调用方把一个符合平台标准协议的单环境 `.tar.gz` 包上传给 Node Server
- Node Server 把该包导入明确指定的目标 `clientId`
- Edge 保留原 `envId/userId/rpaType`
- Edge 重新分配当前服务器的 `envSequence/CDP/VNC/containerName/containerId/containerStatus/monitorStatus`
- 导入完成后只恢复到 `created`，等待后续显式 `run`

成功后的业务结论应是：

- Edge 已导入标准包
- 原 `envId/userId/rpaType` 被保留
- 本机运行资源已按当前服务器重新分配
- 环境进入可后续 `run` 的 `created` 态

## 2. 设计来源

- 用户要求导入必须保留环境身份，但运行摘要必须按当前服务器重新分配。
- `import-package` 不是 `restore`，因为 `restore` 只认本机已登记备份包；`import-package` 处理的是外部标准包。
- 用户明确要求正式资产动作必须留下中心 task、错误留痕和管理员排障信息，不能只靠 Edge HTTP 同步响应。
- `browser-data/profile` 可能很大，因此 Node Server 不能把导入设计成一个无边界的“长时间卡住前端连接”的黑盒接口。

## 3. 业务边界

### 3.1 这个接口负责什么

- 校验目标 `clientId`
- 校验目标节点必须 `healthy + verified + online`
- 接收一个标准单环境 `.tar.gz` 包
- 创建中心 task，记录导入归属、目标节点和后续审计信息
- 把标准包受控转交给目标 Edge 执行导入
- 导入成功后写入或刷新中心环境聚合记录

### 3.2 这个接口不负责什么

- 不自动 `run`
- 不自动 `pull-image`
- 不自动创建运行容器
- 不跨节点随意迁移已存在环境
- 不自动覆盖同名 `envId`
- 不做批量导入
- 不替代 `rebuild-index`

补充约束：

- 如果中心层已存在同 `envId` 的活跃记录（`created/running/stopped/backed_up/error` 等），必须拒绝导入
- 如果中心层只保留一条 `status=deleted` 的历史记录，允许在同一个 `clientId` 上重新导入并复活中心聚合
- 如果 `deleted` 历史记录绑定的是另一台 `clientId`，当前版本仍然拒绝导入，避免把 `import-package` 变成隐式跨节点迁移

### 3.3 与相近接口的边界

- `import-package`：上传外部标准包导入本机，保留身份、重分配本机资源
- `restore`：只从本机 `backupPath` 恢复，不能拿外部包替代
- `backup`：生成标准包并释放源目录，不负责把包导回系统
- `rebuild-index`：灾难恢复工具，只重建本机已存在目录索引，不接收上传文件

## 4. 请求与响应

## 4.1 请求

```http
POST /api/v1/envs/import-package
Content-Type: multipart/form-data
```

建议第一版请求字段：

- `clientId`
  - 目标中心节点 ID
- `file`
  - 标准环境包 `.tar.gz`

建议示例：

```bash
curl -X POST http://node-server.example/api/v1/envs/import-package \
  -H 'X-Main-Account-Id: 906090119' \
  -H 'X-Platform-User-Id: user_1780995561009325000_000001' \
  -F 'clientId=9060901190001' \
  -F 'file=@906090119_tk_323407300419129344-backup.tar.gz'
```

### 4.2 为什么必须带 `clientId`

- `server_browser_envs` 还不存在，Node Server 不能从 env 记录反推节点
- 平台口径已经明确：创建、导入、运行都必须由调用方显式指定目标节点
- Node Server 不允许在未指定节点时自动选择机器

### 4.3 包协议最低要求

导入包至少必须满足：

- 单环境包：一个请求只处理一个 `envId`
- tar 第一层就是唯一 `envId/` 根目录
- 必须包含 `profile.json`
- 必须包含 `binding.json`
- 必须包含 `proxy/`
- 必须包含 `fingerprint/`
- 必须包含 `browser-data/profile`
- `profile.package.checksums` 必须可校验

以下情况必须直接拒绝：

- 多外壳目录
- 散文件直接落在 tar 根目录
- 一个 tar 内有多个环境目录
- `envId` 与 profile/binding 身份不一致
- 原子材料缺失

## 4.4 成功响应

第一版建议采用中心 task 模式。

考虑到导入包可能很大，Node Server 不应把“上传完成后再同步等待 Edge 全量导入结束”作为唯一交互方式；更推荐的口径是：

1. Node Server 先完成最小请求校验与上传文件受控落盘
2. 创建中心 task
3. 后台把临时包转交给 Edge 导入
4. 立即返回中心任务摘要

建议成功响应：

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "taskId": "task_1770000000000000000",
    "taskType": "import_env_package",
    "status": "pending",
    "clientId": "9060901190001",
    "envId": "906090119_tk_323407300419129344",
    "eventsUrl": "http://node-server.example/api/v1/server/tasks/task_1770000000000000000/events",
    "message": "环境包导入任务已创建",
    "createdAt": 1770000000
  }
}
```

说明：

- `taskId` 是中心任务 ID
- 如果第一版实现里 Node Server 能在预解析上传包时拿到 `envId`，应尽早写入中心 task
- 若上传后才解析出 `envId`，则需要在后台导入阶段回写到任务与环境摘要
- 前端应继续通过 task detail 或 SSE 观察最终结果

## 4.5 失败响应

失败仍使用统一错误包装，但必须返回清晰可执行文案。例如：

- `目标 Edge Client 当前不是 healthy + verified + online，禁止执行 import-package`
- `导入包不是平台标准单环境包；请使用 Client backup 生成标准原子包，或重新打包完整 envId 环境目录`
- `目标节点已存在相同 envId，禁止覆盖导入；如确需替换，请先由管理员明确处理原环境`
- `中心已存在相同 envId 的 deleted 历史，但它绑定的是另一台 client；当前版本不允许借 import-package 自动跨节点迁移`

## 5. 前置校验

Node Server 在真正创建中心 task 或转发上传包前，必须完成下面这组校验。

## 5.1 节点校验

1. 指定明确 `clientId`
2. `clientId` 必须属于当前 `mainAccountId`
3. `clientId` 必须通过 `EnsureClientReadyForBusiness`

只有当节点满足以下条件时才允许继续：

- `health_status=healthy`
- `discovery_status=verified`
- `heartbeatStatus=online`

## 5.2 上传包预校验

Node Server 应至少完成这些预校验：

- multipart 中必须存在 `file`
- 文件扩展名或 MIME 不能明显违背 `.tar.gz`
- 上传大小不能超过平台受控上限
- 至少能解析出单环境结构与基础身份字段
- 如果中心已存在同 `envId` 的记录，必须进一步区分它是活跃记录还是 `deleted` tombstone；只有同 `clientId` 的 `deleted` 记录允许继续

## 5.3 Edge 侧导入前置条件

Node Server 需要以文档约束的方式明确依赖 Edge 校验：

- Edge 必须继续校验单根目录
- Edge 必须继续校验 checksums
- Edge 必须拒绝本机已存在同名 `envId`
- Edge 必须重新分配当前服务器的 `envSequence/CDP/VNC`
- Edge 不自动拉镜像、不自动创建容器、不自动启动浏览器

## 6. 状态流转

第一版建议明确为：

```text
上传标准包
  -> Edge 校验通过
  -> 在目标节点导入成功
  -> 中心 env 写入/刷新为 created
```

导入成功后的后续约束：

- 不自动 `run`
- 不自动 `pull-image`
- `runtimeProtection/proxyRuntime` 只能回到待重新验证口径

## 7. 任务编排

建议采用中心 task 模式。

## 7.1 推荐中心编排流程

```text
POST /api/v1/envs/import-package
  -> 校验 Platform Header 和 clientId
  -> EnsureClientReadyForBusiness
  -> 接收 multipart 上传并写入 Node 临时文件
  -> 预解析包结构，读取 envId/userId/rpaType
  -> 创建 server_tasks(type=import_env_package)
  -> 后台把包转发给 Edge /api/v1/edge/browser-envs/import-package
  -> 根据 Edge 返回结果写入/刷新 server_browser_envs
  -> 收口中心 task success/failed
```

## 7.2 为什么建议中心 task

- 导入包可能很大，不能把整个动作都压在前端同步等待上
- import-package 属于正式资产导入动作，必须中心可审计
- 失败后必须能留下任务事实、环境摘要错误和日志，而不是只在一次上传响应里消失

## 7.3 第一版是否需要 `edgeTaskId`

当前 Edge `import-package` 是同步 multipart 接口，不返回 Edge task。

因此第一版更合理的 Node Server 口径是：

- 中心层仍然创建 `server task`
- 但不绑定 `edgeTaskId`
- Node Server 在后台同步等待 Edge 导入结果，然后把中心 task 直接收口为 `success/failed`

这样既能保留平台级任务事实，又不需要伪造一个不存在的 Edge task。

## 8. 成功判定

Node Server 只能按“导入后的资产事实”确认成功，不能因为上传完成或 Edge 回过 200 就默认成功。

## 8.1 明确成功

下面这些条件同时成立，中心 task 才能记 `success`：

1. Edge `/api/v1/edge/browser-envs/import-package` 返回成功
2. Edge 返回体中的导入结果为 `status=created`
3. 中心成功写入或刷新 `server_browser_envs`

## 8.2 关键成功事实

import-package 的成功事实是：

```text
Edge import result -> status == created
```

而不是：

- “上传已完成”
- “Node 临时文件已经落盘”
- “Edge 曾经回过 200”

## 9. 中心缓存收口

导入成功后，中心层至少应写入或刷新：

- `envId`
- `mainAccountId`
- `clientId`
- `status=created`
- `rpaType`
- `name`
- `container_status=unknown`
- `monitor_status=unknown`
- `cdpUrl`
- `webVncUrl`
- `lastTaskId`
- `lastError=""`

当前实现口径：

- `cdpUrl/webVncUrl` 会按目标 Edge `baseUrl + 已分配端口/envId` 预生成可访问地址
- 这些地址只代表当前节点上的连接入口，不代表浏览器容器已经 running
- 调用方仍需以后续 `run` 和运行态校验结果判断环境是否真正可用

### 9.1 为什么导入后还是 `created`

- `import-package` 只恢复资产和本机资源分配
- 它不创建容器、不启动浏览器
- 真正可运行前还要由后续 `run` 去做镜像前置检查和运行态验证

## 10. 失败判定

下面这些都必须统一收口为中心 `failed`：

- 节点不 ready
- 上传包为空或超过限制
- 导入包不合法
- Edge 返回包结构/校验失败
- Edge 本机已存在相同 `envId`
- Edge import-package 失败
- 中心缓存回写失败
- 上传临时文件或 staging 清理失败
- 这类清理失败不能把任务改成 success；应明确标为 failed 并留下清理建议

## 11. 错误与日志规范

import-package 是资产导入动作，一旦失败必须给管理员留下足够排障信息。

## 11.1 最少留痕位置

至少同时落到：

- `server_tasks.error_message`
- `server task SSE` 阶段事件
- `server_browser_envs.last_error`
- Node Server 结构化日志

## 11.2 结构化日志字段

至少包含：

- `taskId`
- `taskType=import_env_package`
- `mainAccountId`
- `clientId`
- `envId`
- `stage`
- `status`
- `errorSource`
- `message`
- `error`
- `suggestion`
- `occurredAt`

建议补充：

- `packageName`
- `packageSize`
- `packageChecksum`
- `nodeTempPath`
- `httpStatus`
- `edgeCode`
- `edgeMessage`

## 11.3 `errorSource` 建议枚举

- `server_precheck`
- `upload_receive`
- `package_parse`
- `task_create`
- `edge_http`
- `edge_validate`
- `snapshot_sync_failed`
- `cleanup_failed`

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

错误不能只写“导入失败”，必须包含：

- 失败原因
- 影响范围
- 修复建议

示例：

- `导入包不是平台标准单环境包；检测到 tar 根目录结构不符合要求，不能导入为可运行环境；请使用 Client backup 生成标准原子包或重新打包完整 envId 目录`
- `目标节点已存在相同 envId，禁止覆盖导入；请先确认原环境是否需要删除、备份或迁移`
- `导入已失败且 Node 临时 staging 清理失败；当前任务不会自动重试，请管理员检查节点磁盘与残留临时目录后重新发起`

## 12. 联调验收标准

第一版至少覆盖以下场景：

## 12.1 成功路径

1. 指定一个 `healthy + verified + online` 的 `clientId`
2. 上传一个标准单环境 `.tar.gz` 包
3. 中心返回 `taskId`
4. task 最终收口为 `success`
5. Edge 导入结果为 `status=created`
6. Node Server 环境列表里出现该 env，并绑定到指定 `clientId`

## 12.2 关键失败路径

至少验证：

- `clientId` 不存在
- 节点 `heartbeatStatus=offline`
- 节点 `health_status=unhealthy`
- tar 内多个环境目录
- 缺少 `profile.json` / `binding.json` / `browser-data/profile`
- checksums 不匹配
- 目标节点已存在同名 `envId`
- Edge import-package 失败
- 中心缓存回写失败
- staging 清理失败

## 12.3 通过标准

满足以下条件才算接口完成：

- 成功路径可稳定收口
- 失败路径都有明确中文错误和修复建议
- 管理员能在 task detail、SSE、env 摘要、服务日志中看到一致错误事实
- 没有自动 run、自动 pull-image、自动覆盖同名 envId、自动重试这类越权补救动作

## 13. 当前实现状态

截至 `2026-06-12`，本文件是 `import-package` 接口的正式设计、实现与联调标准文档。

当前代码现状：

- Edge `POST /api/v1/edge/browser-envs/import-package` 已存在
- Node Server `POST /api/v1/envs/import-package` 第一版已落地
- 当前实现采用“中心 task + 本地 SSE + 后台同步 Edge multipart 导入 + 直接中心收口”模式，不绑定 `edgeTaskId`
- 本文档继续作为后续 OpenAPI 对齐和联调验收标准
