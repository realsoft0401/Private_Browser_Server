# Private_Browser_Server 项目计划

> **文档版本**：v0.1  
> **创建日期**：2026-06-03  
> **阶段目标**：Server V1.0 最小闭环

## 1. 项目定位

`Private_Browser_Server` 是私有浏览器云的中心调度层。它承接商业化入口能力，包括用户认证、节点管理、环境包聚合、任务编排、审计日志和最小 Dashboard。

当前已经完成并验证的是运行层和边缘控制层：

- `Private_Browser_Edge_AMD64`
- `Private_Browser_Edge_ARM`
- `Private_Browser_Client`

因此 Server V1 的核心任务是把这些已经存在的单节点能力统一到中心入口，而不是重新实现浏览器容器控制。

## 2. V1.0 目标

V1.0 要证明：

```text
一个中心 Server
  可以管理多个 Edge 节点
  可以根据节点健康和架构选择节点
  可以代理环境包创建 / 启动 / 停止
  可以聚合任务和环境包状态
  可以保留用户、节点、任务和审计记录
```

V1.0 完成时，前端、Apifox 或客户集成方不再直接调用 Edge 创建环境包，而是统一调用 Server API。

## 3. V1.0 必须完成范围

| 顺序 | 子系统 | 必须能力 | 验收标准 |
|------|--------|----------|----------|
| 1 | Settings / MySQL / Repository | 配置读取、MySQL 连接、Repository 基础方法 | 服务启动后可自动检查数据库连接和基础表 |
| 2 | Auth | 用户注册、登录、JWT、角色字段 | 管理员可创建用户；普通用户只能访问自己的资源 |
| 3 | Node | 节点注册、UDP 自动发现、手动加入、Docker 2375 探测、架构归一化、心跳接收 | Server 可通过 UDP beacon 自动发现 Client，也可手动加入；短时不可确认可标记 `stale`，确认失联后变为 `offline`，恢复探测后再回到 `healthy` |
| 4 | EdgeClient | Server 到 Edge 的 HTTP 客户端 | 统一超时、错误映射和 API Key Header；不做自动重试 |
| 5 | Env | 创建、启动、停止、详情、列表 | Server 可代理 Edge 完成单节点环境包生命周期 |
| 6 | Task | Server 任务表、Edge taskId 绑定、状态刷新 | run/stop/pull-image 等耗时动作在 Server 可查询最终状态 |
| 7 | ImagePolicy | 按节点 `arch` 选择镜像 | `unknown` 架构禁止自动创建环境包 |
| 8 | Dashboard | 最小统计 | 可查看节点数、环境包数、running 数、失败任务数 |

任务职责边界：

- Edge / Client task 是边缘节点本机短期执行观察，主要服务 SSE 实时进度和内网排障，不做长期持久化。
- Server task 是平台级持久任务事实，负责绑定 userId、nodeId、envId、edgeTaskId、最终状态、错误原因和完成时间。
- 前端、Dashboard、审计和历史查询应以 Server task 为准。
- Server 调 Edge 创建任务后，必须把 Edge `taskId` 记录到 Server 任务表，并由 Server 聚合 Edge 最终结果。
- 不要让 Client SQLite 再扩展出一套长期任务历史，避免 Client task 和 Server task 形成双事实源。

Server 访问 Edge 边界：

- Server 只能通过 Edge / Client HTTP API 获取状态、下发生命周期动作和接收任务结果。
- Server 不直接连接、挂载或读取 Edge SQLite；Edge SQLite 只作为 Client 本机环境包索引和运行摘要。
- Server 不直接扫描 Edge `data/browser-envs`、备份包目录或 `browser-data/profile`，也不读取登录态文件、代理明文或指纹 raw。
- Server 不通过 SSH 到 Edge 节点绕过 API 翻环境包文件、修目录、改配置或搬运环境包；备份、恢复、导入、删除和排障修复都应通过 Client API 或后续受控 artifact API 完成。
- 后期如果要采集宿主机环境变量、服务日志、系统诊断或部署状态，应优先设计成 Edge 受控诊断接口，由 Edge 本机采集、白名单过滤、脱敏后返回，Server 只保存诊断结果。
- SSH 可以保留为独立运维或救援通道，但不能把它升级成读取 Edge SQLite、`browser-data/profile`、代理明文、指纹 raw 或登录态文件的业务数据源。
- Server 自己只保存中心聚合索引、状态摘要、任务事实和审计记录，避免和 Client 本地事实形成双事实源。
- Edge Client 的本地 SQLite、环境包目录和生命周期 API 是单个环境包在当前节点上的资产事实源；Server 的聚合表只是中心缓存和调度视图。
- Server 执行 `run/stop/backup/restore/revalidate/delete/import-package` 前必须调用 Edge API 校验当前状态，动作完成后以 Edge 返回结果刷新中心缓存。
- Server 创建环境包时必须明确指定目标 `nodeId/control_nodes.id`；指定在哪台 Client，就固定在哪台 Client 上创建、运行、停止、备份、恢复和删除。
- Server 可以校验指定节点是否 healthy、架构是否已识别、镜像策略是否可用、Docker 是否可达，但不能在未指定节点时自动选择机器，也不能在后续生命周期里自动换节点。
- 只有节点 `health_status=healthy` 且 `discovery_status=verified` 时，Server 才允许创建环境包和执行 run/stop/backup/restore/revalidate/delete/import-package 等生命周期动作。
- `unhealthy` 节点不允许带病执行任何环境包生命周期动作，包括 run、stop、backup、restore、revalidate、delete、import-package；需要先修复节点并重新探测恢复到 `healthy + verified`。
- V1 前期不实现批量生命周期动作，不提供批量 run/stop/backup/restore/revalidate/delete/import-package。多选 UI 如果后续出现，也只能拆成多个独立 Server task，逐个校验、逐个执行、逐个记录成功或失败。
- 后期可在客户节点性能足够强时评估受控批量能力，但必须具备节点容量评估、并发上限、队列调度和资源保护；批量入口仍必须逐个环境包校验节点健康、环境包状态、配置一致性和网络指纹要求。
- 当前 Server 不实现定时自动生命周期调度，没有定时 run、定时 stop、定时 backup、定时 delete 或无人值守自动恢复需求；所有生命周期动作必须来自用户、管理员或明确业务 API 主动请求。
- Server 可以接收 Client 状态同步、心跳和任务进度，但不能把这些后台机制扩展成自动启动、自动停止、自动备份或自动删除环境包的计划任务。
- 指定节点处于 `unhealthy/offline/stale/identity_changed/discovered`，或架构为 `unknown`、Docker 不可达、镜像策略不可用时，Server 必须拒绝创建和使用，并返回明确原因与修复方向。
- `stale` 表示 Server 对该节点或环境包的中心缓存不可信，不能作为创建或运行放行状态；必须重新探测恢复为 healthy/verified 后才能继续。
- `server_browser_envs.node_id` 是环境包绑定的中心节点身份；历史任务、环境包聚合和审计都应围绕该 `nodeId` 追踪。
- Edge 失联、心跳超时、校验失败或中心缓存与 Edge 返回不一致时，Server 统一把中心缓存标记为 `stale`，具体原因写入错误说明或同步摘要字段。
- Server 不允许因为某个 Edge `stale/offline` 就自动把环境包调度到另一台 Client 运行；当前商业口径要求环境包只能在同一台服务器恢复和运行。
- 跨服务器转移会影响宿主硬件指纹、CPU 架构、浏览器平台事实、镜像契约和网络环境。后期只有在核心环境指纹比对能力完成，并确认源/目标服务器兼容后，才允许显式账号转移。
- 核心环境指纹比对至少应覆盖：内部架构枚举、浏览器平台事实、image contract、Chromium 大版本、fingerprintEngineVersion、launchArgsVersion、WebRTC 策略、屏幕/语言/UA 兼容性、代理和网络指纹运行保护要求。
- 核心环境指纹比对只服务未来显式账号转移，不参与当前 `identityHash`，也不能让 Server V1 自动跨服务器调度环境包。

节点发现边界：

- Server 支持 UDP discovery 自动发现 Client，也支持管理员手动填写 Client 地址加入节点。
- Client UDP beacon 只用于发现服务入口，不承载业务动作，不传环境包状态、用户、proxy 明文、fingerprint raw、Cookies、Local Storage、IndexedDB、Session Storage、Login Data 或备份包路径。
- Server 不能抓到 UDP 就处理，必须先校验 `discoveryMagic`、`service=Private_Browser_Client`、`discoveryGroup` 和 `protocolVersion`。不匹配当前平台或当前内网发现域的 UDP 必须直接丢弃。
- `discoveryMagic/service/discoveryGroup` 用于识别本平台 discovery 报文，clientIp/baseUrl 用于识别具体 Client 和去重；这些字段不是用户权限，也不能替代 HTTP 探测和节点鉴权。
- Client 不再生成额外 `clientId`。在独立内网管理模式下，Server 以 UDP 来源 IP 和 HTTP 探测确认后的 `base_url/client_ip` 作为自动发现去重依据。
- clientIp/baseUrl 是 Client 的内网接入地址和发现去重依据；`nodeId` 或 `control_nodes.id` 是 Server 分配的中心身份，用于平台管理、权限、任务、环境包聚合和审计。
- Client 不生成、不保存、不上报 `nodeId`；Server 在节点落库后维护 `nodeId -> clientIp/baseUrl` 的映射。
- Server 如果发现同一 clientIp/baseUrl 对应的 hostname、os、arch、dockerInfo 等设备事实明显变化，应标记 `identity_changed` 或等价状态，禁止自动覆盖节点事实，必须由管理员确认。
- 如果已登记节点仍能通过原心跳、HTTP 探测或管理连接证明同一个 Client 还在线，但新 UDP beacon 的 clientIp 与 Server 记录不一致，Server 应标记 `identity_changed`，记录 `ip_mismatch` 原因，并提示管理员手动更新节点 IP。
- IP 不一致时，Server 不能自动覆盖原 `client_ip/base_url`，也不能自动创建新节点；管理员确认后，才能把原 `nodeId/control_nodes.id` 绑定到新的 clientIp/baseUrl。
- 管理员手动确认更新 IP 后，原 `nodeId/control_nodes.id` 必须保持不变，只更新 `client_ip/base_url` 和发现/健康摘要；历史任务、环境包聚合、审计记录仍绑定原 `nodeId`。
- IP 更新完成后，Server 必须重新执行 `/health`、`/api/v1/edge/device-info`、Docker 2375 探测和架构归一化。只有设备事实仍匹配原节点，才可以把 discovery 状态恢复为 `verified`。
- 如果更新后发现 `arch`、Docker 环境、hostname、环境包列表或设备能力与原节点差异过大，Server 不能直接恢复 `verified`，应继续保持 `identity_changed` 并要求管理员确认。
- Server 收到 UDP beacon 后，必须再通过 Client HTTP API 完成 `/health`、`/api/v1/edge/device-info` 或等价探测，确认服务可达、设备能力、Docker 状态和架构归一化后，才允许写入或更新 `control_nodes`。
- `discovered` 只是节点发现线索，不是可用状态；处于 `discovered` 的节点不能创建环境包，也不能执行 run/stop/backup/restore/delete/import-package。
- 节点进入 `verified` 前，Server 必须确认 UDP 报文匹配 `discoveryMagic/service/discoveryGroup/protocolVersion`，Client `/health` 可达，`/api/v1/edge/device-info` 可达，Docker 2375 可达，架构已归一化为 `amd64` 或 `arm64`，clientIp/baseUrl 与记录一致，不存在 `ip_mismatch/identity_changed/stale/unhealthy/offline`，且镜像策略能按该架构选出可用镜像。
- `verified` 只代表身份与能力验证通过，不单独代表业务可用；只有 `health_status=healthy` 且 `discovery_status=verified` 时，Server 才能对节点下发环境包创建和生命周期动作。
- `health_status=unhealthy` 时，Server 不能把 stop/delete 当成例外下发给 Edge。当前口径是节点不带病工作，所有环境包生命周期动作统一禁止，避免在 Docker、磁盘、SQLite、镜像或设备事实异常时扩大损坏。
- Client `/health` 只返回本机视角 `healthy/unhealthy` 和 checks 明细，不能也不应该返回中心节点的 `offline/stale`。
- Server 的 `health_status` 由 Server 根据 Client API 访问结果、心跳、Docker 2375 探测、架构识别和缓存一致性推导：`healthy` 表示 Client 可达且本机 checks 正常；`unhealthy` 表示 Client 可达但本机能力异常；`offline` 表示 Client 已确认失联；`stale` 表示中心缓存不可信或短时无法确认，必须重新探测。
- 节点心跳和探测阈值必须可配置，建议默认 `heartbeat_interval_seconds=15`、`stale_after_seconds=30`、`offline_after_seconds=90`、`failure_threshold=3`。
- Server 超过 `stale_after_seconds` 没有可靠心跳或探测结果，或动作前校验出现短时超时/缓存冲突时，应先标记 `stale` 并拒绝创建和生命周期动作。
- Server 只有在超过 `offline_after_seconds` 仍无法访问 Client，或连续探测失败达到 `failure_threshold` 后，才把节点标记为 `offline`。
- 节点从 `stale/offline` 恢复时，必须重新完成 `/health`、`/api/v1/edge/device-info`、Docker 2375 探测、架构归一化和 discovery 状态确认；如果 Client 可达但本机 checks 异常，应恢复为 `unhealthy`，不能直接恢复 `healthy`。
- 手动加入与 UDP 自动发现必须复用同一套 HTTP 探测、去重、健康检查和落库流程。
- 去重优先使用独立内网 clientIp 和 `base_url`，避免同一台 Client 因重复广播生成多个节点。
- UDP discovery 只在独立内网模式下启用；未来如果进入共享内网或公网，需要增加 beacon 签名、预共享 token、mTLS 或等价节点鉴权。

镜像职责边界：

- 浏览器运行镜像选择归 Server 管理。
- Server 负责维护 ImagePolicy，根据节点 `arch` 决定下发给 Edge 的 `runtime.image`。
- Edge 只负责在本机 Docker 拉取、检查和运行 Server 已经确定的镜像。
- 前端不应绕过 Server 直接决定商业运行镜像。
- 节点架构为 `unknown` 时，Server 必须拒绝自动创建环境包，要求先完成设备探测。

## 4. V1.0 暂缓范围

| 能力 | 暂缓原因 | 计划阶段 |
|------|----------|----------|
| 自动跨节点迁移 | 当前禁止自动跨服务器运行同一环境包；涉及硬件指纹、CPU 架构、浏览器平台事实和账号连续性风险 | 暂不排期，待核心环境指纹比对能力完成后再评估显式账号转移 |
| 计费系统 | 需要真实套餐、客户和用量口径稳定 | V1.2 |
| Server 集群化 | 单 Server 足够完成第一阶段验证 | V2.0 |
| Marketplace / Webhook | 依赖生态和第三方集成验证 | V3.0 |
| 原始 CDP 命令网关 | 安全风险高，应先做安全原子动作 | V1.5 |

## 5. 核心数据模型

### 5.1 users

保存商业用户认证与权限信息。密码必须使用 bcrypt 或等价算法哈希，不得保存明文密码。

关键字段：

```text
id              雪花 ID，商业用户编号
username        登录名
password_hash   密码哈希
role            admin / user
status          active / disabled
created_at
updated_at
```

### 5.2 control_nodes

保存 Edge 节点的接入信息、设备能力和健康状态。节点 ID 采用 `userId + 4 位设备序号`，由 Server 生成。

节点可以通过两种方式进入 Server：

- UDP discovery 自动发现：Server 监听独立内网 UDP beacon，收到后主动调用 Client HTTP API 探测并登记。
- 手动加入：管理员填写 Client `base_url` 或 IP/端口，Server 走同样的 HTTP 探测、去重和落库流程。

UDP beacon 不是节点事实源，只是发现线索。节点事实必须来自 Client HTTP API 探测结果、Docker 2375 能力探测和后续心跳。

关键字段：

```text
id                    对外节点设备号
user_id               节点归属用户
device_sequence       用户下设备序号
name                  节点名称
client_ip             Client 独立内网 IP，用于 UDP 自动发现去重
base_url              Edge Service 地址
docker_api_url        Docker Engine HTTP API 地址
api_key_hash          Edge 心跳和调用鉴权用 API Key 哈希
discovery_source      manual / udp
last_discovered_at    最近一次 UDP beacon 或手动探测时间
os                    操作系统
arch                  amd64 / arm64 / unknown
cpu_cores
memory_total_mb
docker_version
health_status         healthy / unhealthy / offline
discovery_status      discovered / verified / identity_changed
discovery_reason      empty / ip_mismatch / device_fact_changed / manual_update_required
last_heartbeat_at
created_at
updated_at
```

`health_status` 是 Server 的中心判断结果，不是 Client `/health` 原样透传字段。Client `/health` 只能证明本机当前 checks 是 `healthy` 或 `unhealthy`；`offline` 来自 Server 访问不到 Client，`stale` 来自 Server 缓存或校验结果不可信。

### 5.3 server_browser_envs

保存所有 Edge 环境包的中心聚合索引。这里只保存索引和状态摘要，不保存 profile、proxy 明文、fingerprint raw 或 browser-data。

该表的数据来源必须是 Edge API 响应、Edge 任务结果或 Server 自己的调度记录，不能通过读取 Edge SQLite 或扫描 Edge 环境包目录生成。Edge 本地文件和 SQLite 由 Client 维护一致性；Server 只聚合 API 层已经确认的事实。

该表不是环境包本地资产事实源。状态值如果变成 `stale`，表示 Server 当前缓存已过期或不可信，可能原因包括 Edge 失联、心跳超时、状态校验失败或中心缓存与 Edge 返回不一致。`stale` 只表达中心缓存可信度下降，不能替代 Edge 的真实生命周期状态；恢复可信状态必须重新调用 Edge API 校验并刷新。

Server 下发生命周期动作前，不只校验节点 `healthy + verified`，还必须通过 Edge API 校验目标环境包自身状态。只要环境包配置异常，`profile.json`、`binding.json`、`proxy/`、`fingerprint/`、`browser-data/profile` 等原子必需材料缺失、不可解析、关键字段非法或校验失败，状态为 `error/deleted`，运行目录 missing 且不是受控 `backed_up`，Server 都必须拒绝普通 run/stop/backup/restore/import-package。`status=error` 的重新准入必须走 Edge `revalidate` 或等价受控校验接口，成功后也只能恢复到 `created/stopped + runtimeProtection=pending`。

环境包异常时不能带病操作，也不能把 stop/delete 当作例外。应先通过 Client 受控诊断或配置修复接口恢复环境包配置一致性，再重新校验并执行正常生命周期动作。

配置修复由 Server 发起、Client 在本机通过受控接口执行。Server 不能直接改 Edge SQLite 或环境包文件。Client 只能修复索引摘要、缺失或过期的运行态字段、本机端口重新分配、container 运行摘要、非身份类配置格式问题，以及能从现有环境包文件一致推导出来的元数据。

受控修复禁止改 `envId/userId/rpaType`、`identityHash`、`browser-data/profile` 登录态内容、proxy 明文来源、fingerprint raw、核心身份字段和 binding 身份字段，也不能重建登录态或替换账号环境。修复完成后必须重新校验，校验通过且节点仍为 `healthy + verified` 后才允许生命周期动作。

关键字段：

```text
env_id
user_id
node_id
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

### 5.4 tasks

保存 Server 任务和 Edge 任务的绑定关系，用于前端查询长时间动作状态。

关键字段：

```text
id
user_id
node_id
env_id
type                  create_env / run_env / stop_env / pull_image / backup_env
status                pending / running / success / failed
edge_task_id
events_url
error_message
created_at
updated_at
finished_at
```

`pending/running` 只作为执行中暂态，Server task 的终态只有 `success/failed`，不增加 `unknown/stale/manual_check_required/canceled` 这类终态。

Client task 只是 Edge 进程内短期观察；Client 重启、SSE 中断或 Edge `taskId` 查不到时，Server 必须重新调用 Client 环境包状态接口校验事实。能确认动作完成则收敛为 `success`；无法确认、状态冲突、Client 失联、配置异常或资产动作不可信时，统一收敛为 `failed` 并写清原因。

Server 不能因为 Edge task 丢失就默认成功，也不能自动重放 backup/restore/delete/import-package 等资产动作；需要重试时必须由用户或管理员重新发起新的 Server task。

所有任务失败后都不自动重试，包括 run、stop、backup、restore、delete、import-package、proxy update、proxy-mode update 和 pull-image。失败就是失败，必须先修复节点、网络指纹、代理、镜像、端口或环境包配置，再由用户或管理员重新发起新任务。

EdgeClient 只负责统一超时、错误映射和请求结果记录，不能在底层悄悄重放请求；资产类动作、配置变更和镜像拉取都必须保持“一次请求一次结果”的任务语义。

### 5.5 image_policies

保存不同 CPU 架构与环境下的镜像选择规则。

关键字段：

```text
id
arch                  amd64 / arm64
channel               stable / dev
image
tag
enabled
created_at
updated_at
```

## 6. API 规划

### 6.1 Auth

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
GET  /api/v1/auth/me
```

### 6.2 Node

```text
POST /api/v1/nodes/probe-docker
POST /api/v1/nodes
GET  /api/v1/nodes
GET  /api/v1/nodes/:id
POST /api/v1/nodes/:id/device-info/refresh
POST /api/v1/server/nodes/heartbeat
```

### 6.3 Env

```text
POST /api/v1/envs
GET  /api/v1/envs
GET  /api/v1/envs/:envId
POST /api/v1/envs/:envId/run
POST /api/v1/envs/:envId/stop
```

### 6.4 Task

```text
GET /api/v1/server/tasks
GET /api/v1/server/tasks/:taskId
GET /api/v1/server/tasks/:taskId/events
```

### 6.5 Dashboard

```text
GET /api/v1/server/dashboard
```

## 7. 开发顺序

第一轮开发按下面顺序推进：

```text
1. 项目骨架、配置、MySQL 连接
2. Repository 基础层
3. users + JWT + RBAC
4. control_nodes 注册、UDP discovery、手动加入、探测、心跳
5. EdgeClient 封装和错误映射
6. server_browser_envs 聚合索引
7. Env 创建 / 启动 / 停止 / 详情 / 列表
8. Task 表 + Edge SSE 结果同步
9. 最小 Dashboard 统计
10. Apifox / OpenAPI 文档与端到端验收脚本
```

## 8. 第一阶段验收场景

V1 第一阶段必须跑通以下场景：

1. 管理员注册并登录 Server。
2. Server 通过 UDP discovery 自动发现 Edge，或管理员手动填写 Client 地址。
3. Server 调用 Edge HTTP API 完成设备探测、架构归一化、去重和登记。
4. Server 调用 Docker 探测，保存 `os`、`arch`、CPU、内存、Docker 版本。
5. Edge 使用 API Key 上报心跳。
6. Server 创建环境包时明确指定 `nodeId`，并校验该节点 `health_status=healthy`、`discovery_status=verified`、架构已识别、镜像策略可用、Docker 可达。
7. Server 调用 Edge `/api/v1/edge/browser-envs`。
8. Server 启动环境包，并把 Edge taskId 绑定到 Server task。
9. 前端或 Apifox 查询 Server task，看到最终成功或失败。
10. Server 查询环境包详情，返回 CDP 和 WebVNC 地址。
11. Server 停止环境包，Dashboard 统计同步变化。

CDP / VNC / WebVNC 地址口径：

- V1 内网管理模式下，Server 从 Edge 获取并保存的是 Edge 独立内网可访问地址。
- 这些地址用于 Server、内网管理端和运维工具监控/连接容器设备。
- 这些地址不是公网地址，也不是外部客户浏览器最终访问地址。
- 如果后续需要面向外部客户开放浏览器画面，应由 Server、网关或反向代理重新包装访问地址。

运行可用性口径：

- Server 聚合 Edge 运行结果时，不能只看容器是否 running。
- 浏览器服务、代理出口、timezone 和网络指纹具有原子性；Edge 返回 timezone/代理出口探测失败或超时时，Server task 必须按失败收口，中心环境状态应聚合为 `error` 或等价待排查状态。`container_status=running` 只能作为 Docker 排障事实展示，不能包装成业务 `running` 成功。
- Dashboard、任务详情和环境包详情应保留网络指纹保护结果，避免用户误以为容器启动成功就可以正常使用。
- 后续如果增加 `usable/available/riskStatus`，必须由 Server 基于 Edge 的网络指纹保护状态进行聚合。
- 代理配置修改同样遵守这条原子性：配置落盘、配置版本更新、容器重建、timezone/代理出口验证都不是最终可用；只有运行态完成网络指纹确认后，Server 才能把环境标记为可用。`identityHash` 只做 `envId/userId/rpaType` 一致性摘要，不因代理配置变化而改变。

## 9. 商业化并行准备

Server V1 开发期间同步准备 POC 材料：

- 标准 Demo：创建环境包、启动、WebVNC 查看、备份恢复。
- POC 报告模板：启动成功率、代理探测成功率、平均内存、磁盘增长、故障次数。
- 目标客户名单：先准备 10-20 个跨境电商或社媒营销团队做访谈。
- 销售边界：不承诺不封号，不承诺自动发帖/关注/私信等平台业务脚本。

## 10. 不能退回的原则

- 不要把中心用户体系加回 `Private_Browser_Client`。
- 不要让 Server 直接访问 Edge 的 SQLite、环境包目录、备份包目录或 browser-data。
- 不要通过 SSH 到 Edge 节点绕过 API 翻环境包文件、修改配置或搬运环境包；所有边缘状态读取和生命周期动作都必须通过 Edge API 或受控 artifact API。
- 宿主机环境变量、系统日志和部署状态如果后期确实需要，必须做成受控诊断接口，不要变成任意文件读取。
- 不要因为某个 Edge `stale/offline` 就自动把环境包调度到另一台服务器运行；跨服务器账号转移必须等核心环境指纹比对能力完成，并由用户或上层业务显式触发。
- 不要保存 proxy 明文、fingerprint raw、Cookies 或 Local Storage。
- 不要让前端决定镜像字符串；镜像由 Server `ImagePolicy` 根据节点架构选择。
- 不要在节点架构为 `unknown` 时自动创建环境包。
- 不要把 UDP beacon 当成节点事实源；自动发现后必须通过 Client HTTP API 探测确认才能落库或更新节点。
- 不要处理没有匹配 `discoveryMagic/service/discoveryGroup` 的 UDP 报文；Server 只识别本平台、本发现域的 Client beacon。
- 不要为了演示快而绕过 JWT、API Key、任务表和审计边界。
