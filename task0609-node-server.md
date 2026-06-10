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
DELETE /api/v1/edge/browser-envs/:envId
GET  /api/v1/edge/tasks/:taskId
GET  /api/v1/edge/tasks/:taskId/events
```

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

## 8. 第一验收场景

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
