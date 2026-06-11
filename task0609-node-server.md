# task0609-node-server

## 1. 当前目标

开始整理 `Private_Browser_Server` 的 V1 demo 需求。

当前阶段第一目标不是做完整商业授权系统，而是跑通：

```text
PlatformServer 登录
  -> 前端携带 Platform Header 调用 Node Server
  -> Node Server 管理 x86 Client Edge
  -> Node Server 下发环境包生命周期和 RPA/CDP 动作
  -> Node Server 保存任务、状态摘要和审计
```

## 2. 部署定位

V1 demo 部署口径：

- Node Server 可以运行在 RK3528 4G+64G 轻量控制设备。
- Node Server 只做控制面。
- Node Server 可以同时托管前端静态资源。
- Node Server 使用本地 SQLite。
- x86 服务器部署 `Private_Browser_Client`。
- x86 Client 负责 Docker、Chromium、VNC/CDP、代理、RPA 实际执行。

Node Server 不做：

- 不运行浏览器容器。
- 不把本机 Docker 当浏览器运行节点。
- 不跑 Chromium。
- 不跑 VNC。
- 不跑 Clash/TUN。
- 不保存浏览器真实登录态。
- 不直接读取 Client SQLite 或环境包目录。

## 3. V1 demo 与 PlatformServer 的关系

V1 demo 中，最终客户登录在 PlatformServer。

Node Server 不做客户登录数据库，不做 JWT，不做套餐，不做 slot，不做机位限制。

前端流程：

```text
1. 前端调用 PlatformServer POST /api/v1/auth/login。
2. 前端调用 PlatformServer GET /api/v1/auth/me。
3. 前端拿到当前用户和推荐 Node Server Header。
4. 前端调用 Node Server API 时携带 Header。
5. Node Server 读取 Header，写入任务归属和审计。
```

统一 Header：

```text
X-Main-Account-Id
X-Platform-User-Id
X-Platform-Username
X-Platform-Role
```

V1 demo 规则：

- Node Server 先按内网 demo 信任 Header。
- Header 只用于归属和审计。
- Header 不能替代节点健康、环境包状态、Docker、镜像、网络指纹校验。
- Header 缺失时，Node Server 应返回明确错误，提示先登录 PlatformServer 并调用 `auth/me`。
- 后续 V1.1/V2 再接 PlatformServer `verify-token`，由 Node Server 服务端校验 token。

## 4. V1 demo 必须能力

### 4.1 基础服务

- 监听端口 `3400`。
- 使用本地 SQLite。
- 提供 `/health`。
- 提供 Swagger / OpenAPI。
- 可以托管前端静态资源。
- 所有接口返回统一 JSON 响应。
- 错误信息必须说明原因和修复方向。

### 4.2 Platform Header 解析

新增 Node Server 用户上下文模型：

```text
mainAccountId
platformUserId
platformUsername
platformRole
```

所有业务接口都能读取该上下文。

需要写入的地方：

- `server_browser_envs`
- `tasks`
- `automation_actions`
- 审计摘要

### 4.3 Client 节点管理

Node Server 管理 x86 `Private_Browser_Client`。

必须能力：

- 手动加入 Client。
- 后续支持 UDP discovery 自动发现 Client。
- 调用 Client `/health`。
- 调用 Client `/api/v1/edge/device-info` 或等价设备信息接口。
- 探测 Docker 2375。
- 归一化 CPU 架构：`amd64 / arm64 / unknown`。
- 生成并保存 `clientId`。
- `clientId` 在 SQLite 内部保存为 `edge_clients.id`，是三层统一设备唯一 ID。

设备号规则：

```text
clientId = mainAccountId + 4 位设备序号
示例：
9060901190001
9060901190002
```

V1 demo 先允许手动加入 Client 时传入 `mainAccountId`，Node Server 根据该主账号生成设备号。

节点状态：

```text
discovery_status:
  discovered
  verified
  identity_changed

health_status:
  healthy
  unhealthy
  stale
  offline
```

业务放行条件：

```text
discovery_status = verified
health_status = healthy
arch != unknown
Docker 可达
镜像策略可用
```

### 4.4 EdgeClient

Node Server 到 Client Edge 的 HTTP 客户端必须统一封装。

要求：

- 统一 baseUrl。
- 统一超时。
- 统一错误映射。
- 统一记录请求结果。
- 资产动作不自动重试。
- 不能绕过 API 读 Client SQLite 或文件目录。

调用范围：

```text
GET  /health
GET  /api/v1/edge/device-info
POST /api/v1/edge/browser-envs
GET  /api/v1/edge/browser-envs
GET  /api/v1/edge/browser-envs/:envId
POST /api/v1/edge/browser-envs/:envId/run
POST /api/v1/edge/browser-envs/:envId/stop
POST /api/v1/edge/browser-envs/:envId/backup
POST /api/v1/edge/browser-envs/:envId/restore
POST /api/v1/edge/browser-envs/:envId/revalidate
POST /api/v1/edge/browser-envs/import-package
DELETE /api/v1/edge/browser-envs/:envId/del
DELETE /api/v1/edge/browser-envs/:envId/package
GET  /api/v1/edge/tasks/:taskId
GET  /api/v1/edge/tasks/:taskId/events
```

NodeServer 当前运行时代码尚未下发删除动作；后续扩展时必须区分：
`/del` 只删除环境包关联 Docker 镜像，`/package` 才彻底删除环境包资产。根路径 `DELETE /api/v1/edge/browser-envs/:envId` 不作为新开发接口。

### 4.5 环境包聚合

Node Server 保存环境包中心聚合摘要，不保存完整 profile。

`server_browser_envs` 必须保存：

```text
env_id
client_id
main_account_id
created_by_user_id
created_by_username
rpa_type
name
status
container_status
monitor_status
cdp_url
web_vnc_url
last_task_id
last_error
created_at
updated_at
deleted_at
```

原则：

- Edge / Client 是单个环境包本地资产事实源。
- Node Server 只保存 API 返回后的中心摘要。
- 不保存 profile 详情。
- 不保存 proxy 明文。
- 不保存 fingerprint raw。
- 不保存 browser-data/profile。
- 不保存 Cookies、Local Storage、IndexedDB、Session Storage、Login Data。

### 4.6 生命周期代理

Node Server 提供统一生命周期 API：

```text
POST /api/v1/envs
GET  /api/v1/envs
GET  /api/v1/envs/:envId
POST /api/v1/envs/:envId/run
POST /api/v1/envs/:envId/stop
```

V1 demo 先实现创建、列表、详情、run、stop。

backup / restore / import / delete 可以按实现进度分阶段补，但文档边界先保留。

生命周期动作要求：

- 必须指定目标 `clientId`。
- 不能自动换节点。
- 节点不是 `healthy + verified` 时拒绝。
- 环境包状态异常时拒绝。
- run/stop 必须创建 Server task。
- Edge taskId 必须绑定到 Server task。
- Server task 最终状态只有 `success / failed`。
- Edge task 丢失不能默认成功。
- 动作失败不自动重试。

### 4.7 Server Task

`tasks` 保存 Node Server 任务事实。

关键字段：

```text
id
client_id
env_id
main_account_id
operator_user_id
operator_username
operator_role
type
status
edge_task_id
events_url
error_message
created_at
updated_at
finished_at
```

任务类型：

```text
create_env
run_env
stop_env
pull_image
backup_env
restore_env
delete_env
rpa_action
cdp_action
```

任务终态：

```text
success
failed
```

执行中状态：

```text
pending
running
```

### 4.8 RPA / CDP 动作

V1 demo 先做受控动作，不开放任意 CDP 透传。

可以规划的白名单动作：

```text
open_url
click
type
wait
evaluate_safe_script
check_login
take_screenshot
```

`automation_actions` 保存：

```text
id
client_id
env_id
main_account_id
operator_user_id
operator_username
type
action_name
payload_json
status
result_summary_json
server_task_id
created_at
updated_at
finished_at
```

禁止保存：

- Cookies。
- Local Storage。
- IndexedDB。
- Session Storage。
- Login Data。
- proxy 明文。
- fingerprint raw。
- browser-data/profile。

### 4.9 Dashboard / Audit

V1 demo 最小统计：

- 节点总数。
- healthy 节点数。
- verified 节点数。
- 环境包总数。
- running 环境包数。
- failed task 数。
- 最近任务列表。

Dashboard 只读，不做生命周期动作。

## 5. V1 demo 暂缓能力

- 最终客户登录。
- JWT。
- Platform token 服务端校验。
- slot。
- 机位限制。
- Redis 机位校准。
- 自动跨节点迁移。
- 批量生命周期动作。
- 定时 run / stop / backup。
- 原始 CDP 任意透传。
- Server 集群化。
- 公网节点鉴权。

## 6. 最小 API 清单

### 6.1 System

```text
GET /health
GET /swagger
GET /openapi.yaml
```

### 6.2 Edge Clients

```text
POST /api/v1/edge-clients/probe-docker
POST /api/v1/edge-clients
GET  /api/v1/edge-clients
GET  /api/v1/edge-clients/:clientId
POST /api/v1/edge-clients/:clientId/device-info/refresh
```

### 6.3 Envs

```text
POST /api/v1/envs
GET  /api/v1/envs
GET  /api/v1/envs/:envId
POST /api/v1/envs/:envId/run
POST /api/v1/envs/:envId/stop
```

### 6.4 Tasks

```text
GET /api/v1/tasks
GET /api/v1/tasks/:taskId
GET /api/v1/tasks/:taskId/events
```

### 6.5 Automation

```text
POST /api/v1/automation/actions
GET  /api/v1/automation/actions
GET  /api/v1/automation/actions/:clientId
```

### 6.6 Dashboard

```text
GET /api/v1/dashboard
```

## 7. 开发顺序

1. `[done]` 确认端口 3400、配置、SQLite 路径和 Swagger。
2. `[done]` 实现 Platform Header 解析中间件。
3. `[done]` 实现 SQLite 基础表和 Repository。
4. `[done]` 实现 Node 注册、列表、详情、探测。
5. `[done]` 实现 EdgeClient。
6. 实现 ImagePolicy 最小配置。
7. 实现 Env 创建、列表、详情。
8. 实现 run / stop + Server task。
9. 实现 Task 查询。
10. 实现最小 RPA/CDP 白名单动作。
11. 实现 Dashboard。
12. 做端到端测试：Platform 登录 -> Header -> Node Server -> x86 Client -> 浏览器容器。

## 9. 2026-06-09 已完成实现与验证

### 9.1 基础服务

- `Settings` 已从 `mysql` 配置收紧为 `sqlite` 配置。
- `Rom.Init()` 已真实打开 SQLite，并创建最小控制面表。
- `/health` 已返回 SQLite 路径和 `romInitialized=true`。
- 本地验证端口：`3400`。

SQLite 已建表：

```text
edge_clients
server_browser_envs
server_tasks
```

### 9.2 Platform Header

已新增业务接口中间件，要求：

```text
X-Main-Account-Id
X-Platform-User-Id
```

可选记录：

```text
X-Platform-Username
X-Platform-Role
```

缺少 Header 时拒绝写入，避免节点、环境包和任务产生无归属数据。

### 9.3 Node 接口

已实现：

```text
GET  /api/v1/edge-clients/discovered
POST /api/v1/edge-clients/probe-docker
POST /api/v1/edge-clients
GET  /api/v1/edge-clients
GET  /api/v1/edge-clients/:clientId
POST /api/v1/edge-clients/:clientId/device-info/refresh
```

`GET /api/v1/edge-clients/discovered` 只返回 UDP beacon 缓存，用于验证自动发现链路。
它不自动创建节点，不把节点标记为 `verified`，也不跳过 Client HTTP / Docker 2375 探测。

clientId 生成规则：

```text
clientId = mainAccountId + 4 位节点序号
示例：9060900010001
```

注册节点时只保存接入信息和平台归属：

```text
baseUrl
clientIp
dockerApiUrl
mainAccountId
createdByUserId
createdByUsername
```

设备能力必须通过 Docker 2375 探测后写入，不在注册时伪造。

### 9.4 已完成测试

- `go test ./...` 通过。
- `/health` 返回 `romInitialized=true`。
- 无 Platform Header 调用 `/api/v1/edge-clients` 返回 `1006`，提示缺少上下文。
- 带 Platform Header 注册节点成功。
- Edge Client 列表和详情查询成功。
- 重复 `baseUrl` 注册返回冲突。
- Docker 2375 不可达时返回明确修复方向。
- `192.168.10.119:2375` 探测成功，结果：

```text
os: Ubuntu 24.04.4 LTS
rawArch: x86_64
arch: amd64
cpuCores: 4
memoryTotalMb: 31815
dockerVersion: 29.4.3
dockerApiVersion: 1.54
```

### 9.5 UDP discovery 测试

已完成 Node Server UDP discovery 最小实现：

```text
UDP listen: 0.0.0.0:43000
GET /api/v1/edge-clients/discovered
```

实现边界：

- 只监听、校验、缓存 Client beacon。
- 不自动创建 `edge_clients`。
- 不自动把节点标记为 `verified`。
- 不跳过 Client `/health`、`/api/v1/edge/device-info`、Docker 2375 探测。
- 只接受匹配 `discoveryMagic/service/discoveryGroup/protocolVersion` 的报文。

实测结果：

- 本地 Node Server 能收到 Mac 本机 Client beacon：

```text
sourceIp: 192.168.10.220
baseUrl: http://192.168.10.220:3300
service: private-browser-client
group: default
```

- `192.168.10.119` 的 Client 容器当前是 Docker `bridge` 网络，自动广播 `255.255.255.255:43000` 没有被 Mac/RK 侧 Node Server 收到。
- 通过 Docker exec 在 119 Client 容器内向 Node Server 单播 UDP 测试包，可以被 Node Server 收到：

```text
sourceIp: 192.168.10.119
baseUrl: http://192.168.10.119:3300
```

结论：

- UDP 协议链路可用。
- Server 监听和协议校验可用。
- 119 自动发现失败点在容器广播网络模式，不是 Node Server 解析问题。

后续运行建议：

- 如果 Client 容器需要被同网段 Server 自动发现，优先评估 `--network host`。
- 如果继续使用 Docker `bridge`，应把 Client 的 `discovery.broadcast_address` 配成 Node Server 的内网 IP，让它走 UDP 单播，而不是 `255.255.255.255` 广播。
- 无论哪种方式，UDP 发现后仍必须走 HTTP 探测和 Docker 2375 探测，才能进入 `verified`。

### 9.6 192.168.10.119 Client 重部署记录

2026-06-10 已将 119 的 `private-browser-client` 从 Docker `bridge` 网络重新部署为 `host` 网络。

部署目标：

- 保留 `/Business/data` 数据目录。
- 不删除正在运行的浏览器环境容器。
- Client HTTP 仍暴露在 `192.168.10.119:3300`。
- Docker API 改为从 Client 容器内访问 `http://127.0.0.1:2375`。
- UDP discovery 使用宿主机网络栈发送，使 Node Server 能自动收到 119 beacon。

关键路径：

```text
宿主机数据目录: /Business/data
host 网络配置文件: /Business/data/config-docker-host.yaml
容器内配置挂载: /app/Settings/config-docker.yaml
容器内数据挂载: /app/data
```

最终容器状态：

```text
container: private-browser-client
image: crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_edge_server:0.1.8-amd64
network: host
data bind: /Business/data:/app/data
config bind: /Business/data/config-docker-host.yaml:/app/Settings/config-docker.yaml:ro
```

验证结果：

```text
GET http://192.168.10.119:3300/health
status: healthy
dockerApi: http://127.0.0.1:2375
deviceArch: amd64
dockerVersion: 29.4.3
```

Node Server UDP discovered 列表已能自动收到：

```text
sourceIp: 192.168.10.119
baseUrl: http://192.168.10.119:3300
hostname: 192.168.10.119
service: private-browser-client
```

注意：

- 这次只替换 Client 服务容器，没有删除浏览器环境容器。
- 现有浏览器环境 `906090119_tk_322589886567682048` 仍保持 running。
- 如果后续节点不允许使用 `host` 网络，就需要把 Client discovery 改为 UDP 单播到 Node Server IP。

### 9.7 Client 接入与 verified 放行流程

本节记录下一阶段要实现的标准流程。目标是把“发现 Client”“注册 Client”“验证 Client”“允许业务动作”分清楚，避免前端或 Node Server 因为 UDP 能看到机器就直接 run。

核心原则：

- `discovered` 只代表 Node Server 收到 UDP beacon。
- `clientId` 只由 Node Server 分配，Client 自己不生成、不保存。
- `last_heartbeat_at` 代表 Node Server 最近一次确认收到该 Client 心跳的服务端时间。
- `last_heartbeat_reported_at` 代表 Client 在 UDP beacon 或 HTTP heartbeat 中自报的时间，只做排障辅助。
- `heartbeatStatus=online/stale/offline` 只根据 `last_heartbeat_at` 动态计算，不直接信任 Client 自报时钟。
- `healthStatus=healthy/unhealthy` 来自 Client `/health`、`/api/v1/edge/device-info` 和 Docker 2375 探测。
- `discoveryStatus=verified` 才代表该 Client 已完成接入验证。
- 业务动作必须同时满足 `healthStatus=healthy`、`discoveryStatus=verified`、`heartbeatStatus=online`。

#### 9.7.1 自动发现阶段

第一步：Client 通过 UDP 广播。

```text
Client -> UDP 255.255.255.255:43000
payload:
  discoveryMagic
  protocolVersion
  service
  discoveryGroup
  clientIp
  baseUrl
  hostname
  mode
  version
  startedAt
  lastHeartbeatAt
  capabilities
```

第二步：Node Server 收包校验。

```text
校验 discoveryMagic/service/discoveryGroup/protocolVersion
不匹配直接丢弃
匹配则写入内存 discovered 缓存
```

第三步：如果 UDP 来源能匹配已注册 `edge_clients`，实时回写心跳。

```text
匹配条件:
  baseUrl == edge_clients.base_url
  或 payload.clientIp == edge_clients.client_ip
  或 sourceIp == edge_clients.client_ip

写入:
  edge_clients.last_heartbeat_at = Server 收到 UDP 的时间
  edge_clients.last_heartbeat_reported_at = payload.lastHeartbeatAt
  edge_clients.last_heartbeat_source = udp
  edge_clients.updated_at = Server 收到 UDP 的时间
```

注意：

- 这一步不创建 Client。
- 这一步不改 `healthStatus`。
- 这一步不改 `discoveryStatus`。
- 这一步不让节点进入 `verified`。

第四步：Client 也可以主动调用正式 HTTP 心跳接口。

```http
POST /api/v1/server/edge-clients/heartbeat
Content-Type: application/json

{
  "discoveryMagic": "PRIVATE_BROWSER_CLIENT_DISCOVERY",
  "protocolVersion": 1,
  "service": "Private_Browser_Client",
  "discoveryGroup": "default",
  "baseUrl": "http://192.168.10.119:3300",
  "clientIp": "192.168.10.119",
  "lastHeartbeatAt": 1781163000
}
```

写入规则：

```text
edge_clients.last_heartbeat_at = Server 实际收到 HTTP 心跳的时间
edge_clients.last_heartbeat_reported_at = 请求体里的 lastHeartbeatAt
edge_clients.last_heartbeat_source = http
```

注意：

- HTTP heartbeat 同样不创建 Client。
- HTTP heartbeat 同样不把节点自动改成 healthy 或 verified。
- 仍然要先注册、refresh、verify，业务动作才允许放行。

#### 9.7.2 前端查看发现列表

第一步：商业前端或 PlatformServer 带 Platform Header 调用 Node Server。

```http
GET /api/v1/edge-clients/discovered
X-Main-Account-Id: 906090001
X-Platform-User-Id: user_1780995561009325000_000001
X-Platform-Username: user_906090001
X-Platform-Role: owner
```

返回判断：

```text
clientId 为空:
  只是发现到，还没有注册绑定

clientId 有值:
  已经注册过，可以进入详情/验证/管理流程
```

#### 9.7.3 未绑定 Client 的注册流程

如果 discovered 返回 `clientId=""`，管理员或 owner 可以手动注册。

第一步：可选，先探测 Docker 2375，不写数据库。

```http
POST /api/v1/edge-clients/probe-docker
Content-Type: application/json

{
  "dockerApiUrl": "http://192.168.10.119:2375"
}
```

第二步：手动注册 Client。

```http
POST /api/v1/edge-clients
Content-Type: application/json

{
  "name": "x86-client-119",
  "baseUrl": "http://192.168.10.119:3300",
  "clientIp": "192.168.10.119",
  "dockerApiUrl": "http://192.168.10.119:2375"
}
```

注册成功后：

```text
生成 clientId: {mainAccountId}{sequence}
写入 edge_clients
healthStatus = stale
discoveryStatus = manual
heartbeatStatus 由 last_heartbeat_at 动态计算
```

注意：

- 注册只是绑定中心索引，不代表可运行业务。
- 注册后必须继续执行设备刷新和 verified 验证。

#### 9.7.4 设备能力刷新流程

注册后先刷新设备能力。

```http
POST /api/v1/edge-clients/{clientId}/device-info/refresh
```

Node Server 执行：

```text
调用 Docker 2375:
  GET http://ClientIP:2375/_ping
  GET http://ClientIP:2375/info
  GET http://ClientIP:2375/version

写入:
  os
  arch
  cpu_cores
  memory_total_mb
  docker_version
  healthStatus = healthy 或 unhealthy
  last_checked_at
  last_error
```

注意：

- 当前 refresh 只确认 Docker 设备事实。
- refresh 成功不等于 `verified`。
- refresh 失败必须写 `last_error`，并保持不可业务放行。

#### 9.7.5 verified 验证流程

下一阶段新增接口：

```http
POST /api/v1/edge-clients/{clientId}/verify
```

调用顺序必须固定：

```text
1. 读取 edge_clients，确认 clientId 属于当前 mainAccountId。
2. 检查 heartbeatStatus。
   - offline/stale 直接失败，不继续业务验证。
3. 调用 Client HTTP:
   GET {baseUrl}/health
4. 调用 Client HTTP:
   GET {baseUrl}/api/v1/edge/device-info
5. 调用 Docker 2375:
   GET {dockerApiUrl}/_ping
   GET {dockerApiUrl}/info
   GET {dockerApiUrl}/version
6. 归一化架构。
   - x86_64/amd64 => amd64
   - aarch64/arm64 => arm64
   - 其它 => unknown
7. 对比 Client 返回的 deviceInfo 和 Docker 2375 事实。
8. 全部通过后写入:
   healthStatus = healthy
   discoveryStatus = verified
   last_checked_at = now
   last_error = ""
9. 任一失败写入:
   healthStatus = unhealthy 或 stale
   discoveryStatus 保持 manual/discovered，不进入 verified
   last_error = 明确失败原因和修复建议
```

verify 成功响应应包含：

```json
{
  "clientId": "9060900010001",
  "healthStatus": "healthy",
  "discoveryStatus": "verified",
  "heartbeatStatus": "online",
  "baseUrl": "http://192.168.10.119:3300",
  "dockerApiUrl": "http://192.168.10.119:2375",
  "arch": "amd64"
}
```

#### 9.7.6 业务动作前置校验

后续所有业务动作都必须先走统一校验函数。

适用动作：

```text
POST /api/v1/envs
POST /api/v1/envs/{envId}/run
POST /api/v1/envs/{envId}/stop
未来 backup/restore/delete/import-package/proxy update/RPA/CDP 动作
```

统一放行条件：

```text
healthStatus == healthy
discoveryStatus == verified
heartbeatStatus == online
arch in [amd64, arm64]
baseUrl 非空
dockerApiUrl 非空
```

拒绝条件：

```text
heartbeatStatus = stale/offline
healthStatus = unhealthy/stale/offline
discoveryStatus != verified
arch = unknown
baseUrl 为空
dockerApiUrl 为空
last_error 仍表示关键异常
```

拒绝响应必须告诉管理员：

```text
失败原因
影响范围
修复方式
下一步应调用哪个 API
```

例如：

```text
Client UDP 心跳已超过 90 秒，当前 heartbeatStatus=offline。
Node Server 不能确认该 Client 仍在线，因此拒绝 run。
请先检查 Client 容器是否运行、UDP discovery 是否可达，然后调用:
GET /api/v1/edge-clients/discovered
POST /api/v1/edge-clients/{clientId}/verify
```

#### 9.7.7 当前已实现与待实现

已实现：

- UDP discovery 监听。
- discovered 返回 `clientId`，可区分已绑定/未绑定。
- 已注册 Client 的 `last_heartbeat_at` 实时回写。
- `POST /api/v1/server/edge-clients/heartbeat` 正式心跳落库。
- `heartbeatStatus=online/stale/offline` 动态计算。
- `probe-docker`。
- `device-info/refresh`。
- `POST /api/v1/edge-clients/{clientId}/verify`。
- `EnsureClientReadyForBusiness` 业务动作前置校验函数。
- `POST /api/v1/envs` 已实现代理 Edge 创建环境包并写入中心聚合索引。
- `POST /api/v1/envs/{envId}/run` 已实现中心 task + 镜像预检 + Edge run 编排。
- `POST /api/v1/envs/{envId}/stop` 已实现中心 task 和 Edge stop 绑定。

待实现：

- `backup/restore/revalidate/import-package/del/package` 的 Server 侧生命周期代理。
- verify 失败场景的更多破坏性测试。

#### 9.7.8 状态流转表

`healthStatus`、`discoveryStatus`、`heartbeatStatus` 三个状态不能混用。

```text
阶段: UDP 发现但未注册
edge_clients: 无记录
discovery 列表: clientId=""
healthStatus: 无
discoveryStatus: 无
heartbeatStatus: 无
允许业务动作: 否

阶段: 已注册但未验证
edge_clients: 有记录
discovery 列表: clientId 有值
healthStatus: stale
discoveryStatus: manual
heartbeatStatus: online/stale/offline 动态计算
允许业务动作: 否

阶段: Docker 设备刷新成功
edge_clients: 有记录
healthStatus: healthy
discoveryStatus: manual
heartbeatStatus: online/stale/offline 动态计算
允许业务动作: 否

阶段: verify 成功
edge_clients: 有记录
healthStatus: healthy
discoveryStatus: verified
heartbeatStatus: online
允许业务动作: 是

阶段: UDP 心跳超时
edge_clients: 有记录
healthStatus: 保持上次探测结果
discoveryStatus: 保持上次验证结果
heartbeatStatus: stale/offline
允许业务动作: 否

阶段: verify 失败
edge_clients: 有记录
healthStatus: unhealthy 或 stale
discoveryStatus: 不进入 verified
heartbeatStatus: 按 last_heartbeat_at 动态计算
允许业务动作: 否
```

不要因为 UDP 心跳在线就把 `healthStatus` 改为 `healthy`。

不要因为 Docker 2375 可达就把 `discoveryStatus` 改为 `verified`。

不要因为曾经 `verified` 就忽略 `heartbeatStatus=offline`。

#### 9.7.9 verify 接口请求与响应设计

接口：

```http
POST /api/v1/edge-clients/{clientId}/verify
X-Main-Account-Id: 906090001
X-Platform-User-Id: user_1780995561009325000_000001
X-Platform-Username: user_906090001
X-Platform-Role: owner
```

请求体 V1 可以为空：

```json
{}
```

后续如果需要强制重新探测，可增加：

```json
{
  "force": true
}
```

成功响应：

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "client": {
      "clientId": "9060900010001",
      "healthStatus": "healthy",
      "discoveryStatus": "verified",
      "heartbeatStatus": "online",
      "lastHeartbeatAt": 1781064708,
      "lastCheckedAt": 1781064709,
      "baseUrl": "http://192.168.10.119:3300",
      "dockerApiUrl": "http://192.168.10.119:2375",
      "arch": "amd64"
    },
    "checks": {
      "heartbeat": {
        "status": "passed",
        "message": "UDP 心跳在线"
      },
      "clientHealth": {
        "status": "passed",
        "message": "Client /health healthy"
      },
      "clientDeviceInfo": {
        "status": "passed",
        "message": "Client device-info 可用"
      },
      "docker2375": {
        "status": "passed",
        "message": "Docker 2375 可用"
      },
      "arch": {
        "status": "passed",
        "message": "架构已归一化为 amd64"
      }
    }
  }
}
```

失败响应也应返回 `checks`，方便前端直接展示排障路径：

```json
{
  "code": 1005,
  "message": "Client 验证失败: UDP 心跳已离线，拒绝进入 verified",
  "data": {
    "clientId": "9060900010001",
    "healthStatus": "stale",
    "discoveryStatus": "manual",
    "heartbeatStatus": "offline",
    "nextAction": "请先确认 Client 容器运行、UDP discovery 可达，再重新调用 verify",
    "checks": {
      "heartbeat": {
        "status": "failed",
        "message": "lastHeartbeatAt 超过 offline_after_seconds"
      }
    }
  }
}
```

#### 9.7.10 verify 失败场景与写库规则

```text
失败: clientId 不属于当前 mainAccountId
HTTP: 404 或 1004
写库: 不写
说明: 避免跨主账号探测或泄露 Client 存在性

失败: heartbeatStatus=offline
HTTP: 1005
写库:
  healthStatus = stale
  discoveryStatus 保持原值，不进入 verified
  last_error = UDP 心跳离线 + 修复建议
说明: Node Server 不能确认 Client 当前在线

失败: heartbeatStatus=stale
HTTP: 1005
写库:
  healthStatus = stale
  discoveryStatus 保持原值
  last_error = UDP 心跳过期 + 修复建议
说明: 避免短时断联时误放行业务动作

失败: Client /health 不可达
HTTP: 1005
写库:
  healthStatus = offline 或 stale
  discoveryStatus 保持原值
  last_checked_at = now
  last_error = Client HTTP 不可达 + baseUrl + 修复建议
说明: baseUrl、端口、防火墙、Client 进程都需要排查

失败: Client /health 返回 unhealthy
HTTP: 1005
写库:
  healthStatus = unhealthy
  discoveryStatus 保持原值
  last_checked_at = now
  last_error = health checks 失败项摘要
说明: Client 可达但本机 Docker/SQLite/配置等关键检查异常

失败: /api/v1/edge/device-info 不可达或解析失败
HTTP: 1005
写库:
  healthStatus = unhealthy
  discoveryStatus 保持原值
  last_checked_at = now
  last_error = device-info 失败 + 修复建议
说明: 设备事实不可信，不能 verified

失败: Docker 2375 不可达
HTTP: 1005
写库:
  healthStatus = unhealthy
  discoveryStatus 保持原值
  last_checked_at = now
  last_error = Docker 2375 不可达 + dockerApiUrl + 修复建议
说明: Node Server 无法确认宿主 Docker 能力

失败: arch=unknown
HTTP: 1005
写库:
  healthStatus = unhealthy
  discoveryStatus 保持原值
  last_checked_at = now
  last_error = 架构无法归一化
说明: 镜像策略无法选择，不能业务放行

成功: 全部检查通过
HTTP: 1000
写库:
  healthStatus = healthy
  discoveryStatus = verified
  os/arch/cpu/memory/docker_version 更新为最新探测事实
  last_checked_at = now
  last_error = ""
说明: 节点可以参与后续环境包创建和生命周期动作
```

#### 9.7.11 代码实现拆分建议

实现 `verify` 时不要把所有逻辑堆在 HTTP handler。

建议拆分：

```text
Routes:
  POST /api/v1/edge-clients/:clientId/verify

Service/Node:
  VerifyClient(ctx, platformCtx, clientId)
  buildVerifyChecks(...)
  ensureHeartbeatOnline(...)
  verifyClientHealth(...)
  verifyClientDeviceInfo(...)
  verifyDocker2375(...)
  normalizeArch(...)
  applyVerifySuccess(...)
  applyVerifyFailure(...)

Repository/Node:
  GetByID(...)
  UpdateVerifyResult(...)
```

`VerifyClient` 的职责：

- 编排检查顺序。
- 构造 `checks`。
- 决定成功/失败语义。
- 调 Repository 写最终结果。

Repository 的职责：

- 只写数据库字段。
- 不拼中文错误。
- 不调用 HTTP。
- 不判断业务状态机。

#### 9.7.12 下一步编码顺序

建议按这个顺序写代码：

```text
1. 增加 verify 路由和 Swagger。
2. 增加 verify 请求/响应结构体。
3. 增加 checks 结构体。
4. 增加 heartbeatStatus 前置检查。
5. 增加 Client /health 调用。
6. 增加 Client /api/v1/edge/device-info 调用。
7. 复用现有 Docker 2375 probe。
8. 增加 Repository.UpdateVerifyResult。
9. verify 成功写 healthy + verified。
10. verify 失败写 last_error。
11. 所有 env/run/stop 前先调用统一放行校验。
12. Swagger 测一遍。
13. curl 测成功路径、heartbeat offline、Docker 2375 不通、Client HTTP 不通四类场景。
```

## 8. 第一验收场景

### 9.8 API 文档工具选型

用户确认 Swagger 无法精准表达当前系统里的状态流转和业务调用顺序，因此 API 文档工具按阶段选型：

```text
开发测试阶段:
  Swagger UI + Scalar

内部/客户演示阶段:
  Scalar

正式商业文档:
  Mintlify

嵌入自己后台:
  Scalar API Reference
```

记录位置：

```text
docs/api-docs.md
```

后续原则：

- Swagger/OpenAPI 只负责接口契约和调试。
- Scalar 负责更现代的 API Reference。
- Mintlify 负责正式商业文档站。
- 状态机、调用顺序、失败恢复、verified 放行规则必须写入流程文档和状态文档，不能只依赖 Swagger。

```text
1. PlatformServer 创建主账号和 owner。
2. owner 登录 PlatformServer。
3. 前端调用 auth/me，拿到 Node Server Header。
4. 前端用 Header 调用 Node Server。
5. Node Server 手动加入一台 x86 Client。
6. Node Server 探测 Client health/device-info/Docker。
7. Node Server 生成 edge_clients.id。
8. 节点进入 healthy + verified。
9. Node Server 在该节点创建环境包。
10. Node Server run 环境包。
11. Node Server 查询 Server task，看到 success/failed。
12. Node Server 查询环境详情，拿到 CDP/WebVNC 地址摘要。
13. Node Server 下发一个受控 RPA/CDP 测试动作。
14. Node Server stop 环境包。
15. Dashboard 统计同步变化。
```
