# Private_Browser_Server 项目计划

> **文档版本**：v0.1  
> **创建日期**：2026-06-03  
> **阶段目标**：Node Server V1.0 最小闭环

> **当前执行清单**：见 [task0609-node-server.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/task0609-node-server.md)

## 1. 项目定位

`Private_Browser_Server` 当前定位为节点管理与自动化调度 Server。它不是最终客户登录系统，不负责客户账号密码、套餐、计费或业务订单。

它承接的是节点侧中心能力：节点管理、设备能力探测、环境包聚合、生命周期任务编排、RPA/CDP 操作数据来源、自动化任务下发、任务结果摘要和审计日志。

2026-06-09 最新 demo 口径：

- Node Server V1 可以部署在 RK3528 4G+64G 这类轻量 ARM 控制设备上，只承担控制面职责。
- 浏览器运行面全部放在 x86 Client 服务器上，由 `Private_Browser_Client` 管理 Docker、Chromium、VNC/CDP、代理和 RPA 实际执行。
- Node Server 不运行浏览器容器，不把本机 Docker 当浏览器运行节点，不跑 Chromium/VNC/Clash。
- Node Server + 前端静态资源 + SQLite 是 RK3528 的推荐部署形态。
- V1 demo 阶段只做用户上下文透传、节点管理、生命周期代理、任务和状态摘要；PlatformServer 的 slot、机位、Redis 商业授权闭环统一放入 V2，不进入 Node Server V1。

当前已经完成并验证的是运行层和边缘控制层：

- `Private_Browser_Edge_AMD64`
- `Private_Browser_Edge_ARM`
- `Private_Browser_Client`

因此 Server V1 的核心任务是把这些已经存在的单节点能力统一到节点管理入口，并为后续 RPA/CDP 自动化调度提供中心任务来源，而不是重新实现浏览器容器控制，也不是先实现最终客户登录系统。

## 2. V1.0 目标

V1.0 要证明：

```text
一个Node Server
  可以管理多个 Edge Client
  可以校验节点健康、架构和镜像策略
  可以代理环境包创建 / 启动 / 停止
  可以聚合任务和环境包状态
  可以下发受控 RPA / CDP 操作数据
  可以保留节点、环境包、任务、操作结果和审计记录
```

V1.0 完成时，上层业务平台、管理端、Apifox 或自动化系统不再直接调用 Edge 创建环境包或下发 CDP/RPA 动作，而是统一调用 Node Server API。

## 3. V1.0 必须完成范围

| 顺序 | 子系统 | 必须能力 | 验收标准 |
|------|--------|----------|----------|
| 1 | Settings / SQLite / Repository | 配置读取、本地 SQLite 连接、Repository 基础方法 | 服务启动后可自动检查 SQLite 连接和基础表 |
| 2 | Node | 节点注册、UDP 自动发现、手动加入、Docker 2375 探测、架构归一化、心跳接收、verified 状态机 | Server 可通过 UDP beacon 自动发现 Client，也可手动加入；只有 `healthy + verified` 才能承接业务动作 |
| 3 | EdgeClient | Server 到 Edge 的 HTTP 客户端 | 统一超时、错误映射和节点凭证 Header；不做自动重试 |
| 4 | ImagePolicy | 按节点 `arch` 选择镜像 | `unknown` 架构禁止自动创建环境包 |
| 5 | Env | 创建、启动、停止、详情、列表 | Server 可代理 Edge 完成单节点环境包生命周期 |
| 6 | Task | Server 任务表、Edge taskId 绑定、状态刷新 | run/stop/pull-image 等耗时动作在 Server 可查询最终状态 |
| 7 | RPA / CDP Task | 保存受控操作数据、下发到指定节点和环境包、记录结果摘要 | 操作来源在 Server，执行在 Edge，结果可追踪、可审计 |
| 8 | Audit / Dashboard | 审计日志、最小统计 | 可查看节点数、环境包数、running 数、失败任务数、自动化任务结果 |

Auth / JWT / 用户登录如果后续保留，只作为 Node Server 的管理员保护或上层业务平台调用保护；它不是当前 V1 主线，也不是最终客户账号密码数据库。

V1 demo 的上层用户上下文来自 PlatformServer：

- 前端先调用 PlatformServer `POST /api/v1/auth/login` 登录。
- 前端再调用 PlatformServer `GET /api/v1/auth/me` 获取当前用户、主账号和推荐的 Node Server Header。
- 前端调用 Node Server API 时携带用户上下文 Header。
- Node Server V1 先按内网 demo 信任这些 Header，用于任务归属、审计、环境包创建人和操作人记录。
- Node Server V1 不强制回调 PlatformServer 校验 token；后续 V1.1/V2 再接 `POST /api/v1/auth/verify-token` 或等价服务端校验。

统一 Header：

```text
X-Main-Account-Id: 906090119
X-Platform-User-Id: user_xxx
X-Platform-Username: owner_906090119
X-Platform-Role: owner
```

数据库职责边界：

- Node Server 使用本地 SQLite 作为节点管理和自动化调度数据库，保存 `edge_clients`、`server_browser_envs`、`tasks`、`automation_actions`、`image_policies` 和审计摘要。
- SQLite 只保存节点侧调度事实和状态摘要，不保存最终客户账号密码、套餐、计费、业务订单或跨客户业务数据。
- 平台管理端使用 MySQL，负责客户、套餐、计费、业务订单、跨节点业务视图和更高层权限。
- Node Server 可以向平台管理端同步摘要或接受上层调度请求，但不能把自己升级成平台客户数据库。
- 代码层已经把 Node Server 自身配置收紧为 `SQLiteConfig`，避免实现时继续沿用旧 MySQL 口径。
- Node Server SQLite 可以保存从 Header 解析出的 `main_account_id`、`platform_user_id`、`platform_username`、`platform_role` 作为任务和环境包聚合审计字段，但这些字段不是 Node Server 自己的登录用户表。

任务职责边界：

- Edge / Client task 是边缘节点本机短期执行观察，主要服务 SSE 实时进度和内网排障，不做长期持久化。
- Server task 是节点调度层持久任务事实，负责绑定 clientId、envId、edgeTaskId、自动化操作 ID、最终状态、错误原因和完成时间。
- 管理端、Dashboard、审计和历史查询应以 Server task 为准。
- Server 调 Edge 创建任务后，必须把 Edge `taskId` 记录到 Server 任务表，并由 Server 聚合 Edge 最终结果。
- 不要让 Client SQLite 再扩展出一套长期任务历史，避免 Client task 和 Server task 形成双事实源。

Server 访问 Edge 边界：

- Server 只能通过 Edge / Client HTTP API 获取状态、下发生命周期动作和接收任务结果。
- Server 不直接连接、挂载或读取 Edge SQLite；Edge SQLite 只作为 Client 本机环境包索引和运行摘要。
- Server 不直接扫描 Edge `data/browser-envs`、备份包目录或 `browser-data/profile`，也不读取登录态文件、代理明文或指纹 raw。
- Server 不通过 SSH 到 Edge Client绕过 API 翻环境包文件、修目录、改配置或搬运环境包；备份、恢复、导入、删除和排障修复都应通过 Client API 或后续受控 artifact API 完成。
- 后期如果要采集宿主机环境变量、服务日志、系统诊断或部署状态，应优先设计成 Edge 受控诊断接口，由 Edge 本机采集、白名单过滤、脱敏后返回，Server 只保存诊断结果。
- SSH 可以保留为独立运维或救援通道，但不能把它升级成读取 Edge SQLite、`browser-data/profile`、代理明文、指纹 raw 或登录态文件的业务数据源。
- Server 自己只保存中心聚合索引、状态摘要、任务事实和审计记录，避免和 Client 本地事实形成双事实源。
- Edge Client 的本地 SQLite、环境包目录和生命周期 API 是单个环境包在当前节点上的资产事实源；Server 的聚合表只是中心缓存和调度视图。
- Server 执行 `run/stop/backup/restore/revalidate/delete/import-package` 前必须调用 Edge API 校验当前状态，动作完成后以 Edge 返回结果刷新中心缓存。
- Server 创建环境包时必须明确指定目标 `clientId/edge_clients.id`；指定在哪台 Client，就固定在哪台 Client 上创建、运行、停止、备份、恢复和删除。
- V1 demo 中，创建环境包和任务时应记录 Platform Header 中的主账号和操作人信息；这些字段只服务归属与审计，不让 Node Server 变成客户账号数据库。
- Server 可以校验指定节点是否 healthy、架构是否已识别、镜像策略是否可用、Docker 是否可达，但不能在未指定节点时自动选择机器，也不能在后续生命周期里自动换节点。
- 只有节点 `health_status=healthy` 且 `discovery_status=verified` 时，Server 才允许创建环境包和执行 run/stop/backup/restore/revalidate/delete/import-package 等生命周期动作。
- `unhealthy` 节点不允许带病执行任何环境包生命周期动作，包括 run、stop、backup、restore、revalidate、delete、import-package；需要先修复节点并重新探测恢复到 `healthy + verified`。
- V1 前期不实现批量生命周期动作，不提供批量 run/stop/backup/restore/revalidate/delete/import-package。多选 UI 如果后续出现，也只能拆成多个独立 Server task，逐个校验、逐个执行、逐个记录成功或失败。
- 后期可在客户节点性能足够强时评估受控批量能力，但必须具备节点容量评估、并发上限、队列调度和资源保护；批量入口仍必须逐个环境包校验节点健康、环境包状态、配置一致性和网络指纹要求。
- 当前 Server 不实现定时自动生命周期调度，没有定时 run、定时 stop、定时 backup、定时 delete 或无人值守自动恢复需求；所有生命周期动作必须来自用户、管理员或明确业务 API 主动请求。
- Server 可以接收 Client 状态同步、心跳和任务进度，但不能把这些后台机制扩展成自动启动、自动停止、自动备份或自动删除环境包的计划任务。
- 指定节点处于 `unhealthy/offline/stale/identity_changed/discovered`，或架构为 `unknown`、Docker 不可达、镜像策略不可用时，Server 必须拒绝创建和使用，并返回明确原因与修复方向。
- `stale` 表示 Server 对该节点或环境包的中心缓存不可信，不能作为创建或运行放行状态；必须重新探测恢复为 healthy/verified 后才能继续。
- `server_browser_envs.client_id` 是环境包绑定的中心 Client 身份，数据库内部暂保留列名；对外 API 和业务语言统一叫 `clientId`。历史任务、环境包聚合和审计都应围绕该 `clientId` 追踪。
- Edge 失联、心跳超时、校验失败或中心缓存与 Edge 返回不一致时，Server 统一把中心缓存标记为 `stale`，具体原因写入错误说明或同步摘要字段。
- Server 不允许因为某个 Edge `stale/offline` 就自动把环境包调度到另一台 Client 运行；当前商业口径要求环境包只能在同一台服务器恢复和运行。
- 跨服务器转移会影响宿主硬件指纹、CPU 架构、浏览器平台事实、镜像契约和网络环境。后期只有在核心环境指纹比对能力完成，并确认源/目标服务器兼容后，才允许显式账号转移。
- 核心环境指纹比对至少应覆盖：内部架构枚举、浏览器平台事实、image contract、Chromium 大版本、fingerprintEngineVersion、launchArgsVersion、WebRTC 策略、屏幕/语言/UA 兼容性、代理和网络指纹运行保护要求。
- 核心环境指纹比对只服务未来显式账号转移，不参与当前 `identityHash`，也不能让 Server V1 自动跨服务器调度环境包。

RPA / CDP 操作边界：

- Node Server 是后续 RPA / CDP 操作数据的来源和下发入口，负责保存受控操作模板、步骤、参数摘要、目标 clientId/envId、任务状态、结果摘要和审计记录。
- Edge / Client 负责在本机浏览器容器内执行受控动作，返回任务状态和必要结果摘要。
- Server 不能保存 Cookies、Local Storage、IndexedDB、Session Storage、Login Data、`browser-data/profile`、proxy 明文或 fingerprint raw。
- 原始 CDP 命令网关风险高，V1 不开放任意透传。V1 只允许白名单化、可审计、可失败收口的原子动作。
- 自动化任务失败后不自动重试；必须修复节点、环境包、代理、网络指纹或操作参数后，由管理员或上层业务重新发起新任务。

节点发现边界：

- Server 支持 UDP discovery 自动发现 Client，也支持管理员手动填写 Client 地址加入节点。
- Client UDP beacon 只用于发现服务入口，不承载业务动作，不传环境包状态、用户、proxy 明文、fingerprint raw、Cookies、Local Storage、IndexedDB、Session Storage、Login Data 或备份包路径。
- Server 不能抓到 UDP 就处理，必须先校验 `discoveryMagic`、`service=Private_Browser_Client`、`discoveryGroup` 和 `protocolVersion`。不匹配当前平台或当前内网发现域的 UDP 必须直接丢弃。
- `discoveryMagic/service/discoveryGroup` 用于识别本平台 discovery 报文，clientIp/baseUrl 用于识别具体 Client 和去重；这些字段不是用户权限，也不能替代 HTTP 探测和节点鉴权。
- Client Edge 不自生成 `clientId`。在独立内网管理模式下，Server 以 UDP 来源 IP 和 HTTP 探测确认后的 `base_url/client_ip` 作为自动发现去重依据。
- clientIp/baseUrl 是 Client 的内网接入地址和发现去重依据；`clientId` 或内部 `edge_clients.id` 是 Server 分配的中心身份，用于平台管理、权限、任务、环境包聚合和审计。
- Client Edge 不生成、不保存、不上报 `clientId`；Server 在节点落库后维护 `clientId -> clientIp/baseUrl` 的映射。
- 三层服务统一口径：商业设备唯一 ID 统一叫 `clientId`，由 Node Server 分配并维护，底层保存为 `edge_clients.id`，不是 Client Edge 自生成的 `device_unique_id`。
- Node Server 可以按 `mainAccountId + 4 位设备序号` 生成对外Client 设备号，例如 `9060901190001`。这个设备号用于 PlatformServer 商业机位归属、状态上报和审计。
- Client 只提供 IP、baseUrl、hostname、os、arch、Docker 信息等本机事实；Node Server 探测确认后生成或绑定 `edge_clients.id`。
- 如果设备 IP/baseUrl 变化或设备重置，Node Server 不能自动创建新商业设备 ID 代替旧 ID，必须标记 `ip_mismatch/identity_changed/manual_update_required`，等待管理员确认后保持原 `edge_clients.id` 不变并更新接入地址。
- Server 如果发现同一 clientIp/baseUrl 对应的 hostname、os、arch、dockerInfo 等设备事实明显变化，应标记 `identity_changed` 或等价状态，禁止自动覆盖节点事实，必须由管理员确认。
- 如果已登记节点仍能通过原心跳、HTTP 探测或管理连接证明同一个 Client 还在线，但新 UDP beacon 的 clientIp 与 Server 记录不一致，Server 应标记 `identity_changed`，记录 `ip_mismatch` 原因，并提示管理员手动更新节点 IP。
- IP 不一致时，Server 不能自动覆盖原 `client_ip/base_url`，也不能自动创建新节点；管理员确认后，才能把原 `clientId/edge_clients.id` 绑定到新的 clientIp/baseUrl。
- 管理员手动确认更新 IP 后，原 `clientId/edge_clients.id` 必须保持不变，只更新 `client_ip/base_url` 和发现/健康摘要；历史任务、环境包聚合、审计记录仍绑定原 `clientId`。
- IP 更新完成后，Server 必须重新执行 `/health`、`/api/v1/edge/device-info`、Docker 2375 探测和架构归一化。只有设备事实仍匹配原节点，才可以把 discovery 状态恢复为 `verified`。
- 如果更新后发现 `arch`、Docker 环境、hostname、环境包列表或设备能力与原节点差异过大，Server 不能直接恢复 `verified`，应继续保持 `identity_changed` 并要求管理员确认。
- Server 收到 UDP beacon 后，必须再通过 Client HTTP API 完成 `/health`、`/api/v1/edge/device-info` 或等价探测，确认服务可达、设备能力、Docker 状态和架构归一化后，才允许写入或更新 `edge_clients`。
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
| 最终客户登录、套餐、计费和业务订单 | 属于上层业务平台，不属于当前 Node Server 主线 | 暂缓 |
| Server 集群化 | 单 Server 足够完成第一阶段验证 | V2.0 |
| Marketplace / Webhook | 依赖生态和第三方集成验证 | V3.0 |
| 原始 CDP 命令网关 | 安全风险高，应先做受控 RPA/CDP 原子动作 | V1.5 |

## 5. 核心数据模型

### 5.1 operators（可选，暂缓）

当前 Node Server V1 不以最终客户登录系统为目标，因此不优先实现客户账号密码数据库。

如果后续需要给 Node Server 增加管理员保护，可设计 `operators` 或 `admin_accounts`，只服务Node Server 运维登录、上层平台调用授权和审计归属。它不保存最终客户业务账号，也不承接套餐、计费和业务订单。

关键字段：

```text
id              管理员或上层调用方编号
username        管理员登录名或调用方名称
password_hash   密码哈希
role            admin / operator / service
status          active / disabled
created_at
updated_at
```

### 5.2 edge_clients

保存 Client 的接入信息、设备能力和健康状态。clientId 由 Node Server 生成，代表中心侧 Client 身份。

`edge_clients.id` 是 SQLite 内部保存字段；对外 API、Swagger、PlatformServer 上报和业务文档统一叫 `clientId`。Client Edge 不生成该 ID；PlatformServer 不直接生成或覆盖该 ID。Node Server 负责生成、保存、下发或在受控接口中返回该 ID，并在向 PlatformServer 上报商业运行摘要时使用 `clientId`。

节点可以通过两种方式进入 Server：

- UDP discovery 自动发现：Server 监听独立内网 UDP beacon，收到后主动调用 Client HTTP API 探测并登记。
- 手动加入：管理员填写 Client `base_url` 或 IP/端口，Server 走同样的 HTTP 探测、去重和落库流程。

UDP beacon 不是节点事实源，只是发现线索。节点事实必须来自 Client HTTP API 探测结果、Docker 2375 能力探测和后续心跳。

关键字段：

```text
id                    对外Client 设备号
node_sequence         节点序号
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
client_id
main_account_id       来自 Platform Header，用于 demo 归属和审计
created_by_user_id    来自 Platform Header，记录创建人
created_by_username   来自 Platform Header，便于排障展示
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

保存 Server 任务和 Edge 任务的绑定关系，用于管理端、上层业务平台或自动化系统查询长时间动作状态。

关键字段：

```text
id
client_id
env_id
main_account_id       来自 Platform Header，用于任务归属
operator_user_id      来自 Platform Header，记录操作人
operator_username     来自 Platform Header，便于审计展示
operator_role         来自 Platform Header，V1 demo 只记录不做复杂 RBAC
type                  create_env / run_env / stop_env / pull_image / backup_env / rpa_action / cdp_action
status                pending / running / success / failed
edge_task_id
automation_action_id
events_url
error_message
created_at
updated_at
finished_at
```

`pending/running` 只作为执行中暂态，Server task 的终态只有 `success/failed`，不增加 `unknown/stale/manual_check_required/canceled` 这类终态。

Client task 只是 Edge 进程内短期观察；Client 重启、SSE 中断或 Edge `taskId` 查不到时，Server 必须重新调用 Client 环境包状态接口校验事实。能确认动作完成则收敛为 `success`；无法确认、状态冲突、Client 失联、配置异常或资产动作不可信时，统一收敛为 `failed` 并写清原因。

Server 不能因为 Edge task 丢失就默认成功，也不能自动重放 backup/restore/delete/import-package 等资产动作；需要重试时必须由管理员或上层业务重新发起新的 Server task。

所有任务失败后都不自动重试，包括 run、stop、backup、restore、delete、import-package、proxy update、proxy-mode update、RPA/CDP action 和 pull-image。失败就是失败，必须先修复节点、网络指纹、代理、镜像、端口、环境包配置或操作参数，再由管理员或上层业务重新发起新任务。

EdgeClient 只负责统一超时、错误映射和请求结果记录，不能在底层悄悄重放请求；资产类动作、配置变更和镜像拉取都必须保持“一次请求一次结果”的任务语义。

### 5.5 automation_actions

保存 Node Server 发起的受控 RPA / CDP 操作数据。该表保存的是操作意图、步骤、目标和结果摘要，不保存浏览器登录态或敏感运行资产。

关键字段：

```text
id
client_id
env_id
type                  rpa / cdp
action_name           open_url / click / type / check_login / evaluate_safe_script 等白名单动作
payload_json          受控参数，不允许 proxy 明文、fingerprint raw、Cookies、Local Storage 等敏感内容
status                pending / running / success / failed
result_summary_json   结果摘要和错误原因，不保存登录态实体
server_task_id
created_at
updated_at
finished_at
```

维护原则：

- `payload_json` 必须是白名单动作的结构化参数，不能成为任意 CDP 透传后门。
- `result_summary_json` 只能保存业务可用的摘要，例如页面标题、检测结果、错误码、截图引用 ID；不能保存 Cookies、Local Storage、IndexedDB、Session Storage 或 Login Data。
- 每个自动化动作都必须绑定 `client_id + env_id + server_task_id`，方便排障和审计。

### 5.6 image_policies

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

### 6.0 Platform User Context Header

V1 demo 所有需要归属和审计的业务接口，都应读取以下 Header：

```text
X-Main-Account-Id
X-Platform-User-Id
X-Platform-Username
X-Platform-Role
```

读取原则：

- Header 缺失时，demo 阶段可以返回明确错误，要求前端先登录 PlatformServer 并调用 `auth/me`。
- Node Server 只记录这些字段，不在 V1 demo 中校验用户密码、套餐、slot 或机位。
- Header 不能替代节点健康、环境包状态、Docker、镜像和网络指纹校验。
- 后续接入 PlatformServer `verify-token` 后，Header 只作为展示或兼容字段，可信用户上下文以服务端校验结果为准。

### 6.1 Operator / Access Guard（可选，暂缓）

```text
POST /api/v1/operators/login     // 可选，仅用于 Node Server 管理员保护
GET  /api/v1/operators/me        // 可选，不作为最终客户账号系统
```

### 6.2 Node

```text
POST /api/v1/edge-clients/probe-docker
POST /api/v1/edge-clients
GET  /api/v1/edge-clients
GET  /api/v1/edge-clients/:clientId
POST /api/v1/edge-clients/:clientId/device-info/refresh
POST /api/v1/server/edge-clients/heartbeat
```

### 6.3 Env

```text
POST /api/v1/envs
GET  /api/v1/envs
GET  /api/v1/envs/:envId
POST /api/v1/envs/:envId/run
POST /api/v1/envs/:envId/stop
```

Env 创建、run、stop 请求在 V1 demo 中需要携带 Platform Header。Node Server 负责把 Header 中的主账号和用户信息写入 `server_browser_envs`、`tasks` 和审计摘要。

### 6.4 Task

```text
GET /api/v1/server/tasks
GET /api/v1/server/tasks/:taskId
GET /api/v1/server/tasks/:taskId/events
```

### 6.5 RPA / CDP Action

```text
POST /api/v1/automation/actions
GET  /api/v1/automation/actions
GET  /api/v1/automation/actions/:clientId
```

### 6.6 Dashboard

```text
GET /api/v1/server/dashboard
```

## 7. 开发顺序

第一轮开发按下面顺序推进：

```text
1. 项目骨架、配置、SQLite 连接
2. Platform Header 解析中间件和用户上下文模型
3. Repository 基础层
4. edge_clients 注册、UDP discovery、手动加入、探测、心跳和 verified 状态机
5. EdgeClient 封装和错误映射
6. ImagePolicy 按节点架构选择镜像
7. server_browser_envs 聚合索引，写入 mainAccount/operator 审计字段
8. Env 创建 / 启动 / 停止 / 详情 / 列表
9. Task 表 + Edge SSE 结果同步，写入 mainAccount/operator 审计字段
10. automation_actions 受控 RPA/CDP 操作数据、下发和结果摘要
11. 最小 Dashboard / Audit 统计
12. Apifox / OpenAPI 文档与端到端验收脚本
```

## 8. 第一阶段验收场景

V1 第一阶段必须跑通以下场景：

1. 用户先在 PlatformServer 登录，并通过 `auth/me` 获取 Node Server Header。
2. 前端或 Apifox 调用 Node Server API 时带上 Platform Header。
3. Server 通过 UDP discovery 自动发现 Edge，或管理员手动填写 Client 地址。
4. Server 调用 Edge HTTP API 完成设备探测、架构归一化、去重和登记。
5. Server 调用 Docker 探测，保存 `os`、`arch`、CPU、内存、Docker 版本。
6. 节点进入 `health_status=healthy + discovery_status=verified`。
7. Edge 使用节点凭证上报心跳。
8. Server 创建环境包时明确指定 `clientId`，记录 Header 中的主账号和操作人，并校验该 Client `health_status=healthy`、`discovery_status=verified`、架构已识别、镜像策略可用、Docker 可达。
9. Server 调用 Edge `/api/v1/edge/browser-envs`。
10. Server 启动环境包，并把 Edge taskId 绑定到 Server task。
11. 管理端或 Apifox 查询 Server task，看到最终成功或失败。
12. Server 查询环境包详情，返回 CDP 和 WebVNC 地址摘要。
13. Server 下发一个受控 RPA/CDP 测试动作，并记录动作结果摘要。
14. Server 停止环境包，Dashboard 统计同步变化。

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

- 不要把最终客户用户体系加回 `Private_Browser_Client`。
- 不要让 Server 直接访问 Edge 的 SQLite、环境包目录、备份包目录或 browser-data。
- 不要通过 SSH 到 Edge Client绕过 API 翻环境包文件、修改配置或搬运环境包；所有边缘状态读取和生命周期动作都必须通过 Edge API 或受控 artifact API。
- 宿主机环境变量、系统日志和部署状态如果后期确实需要，必须做成受控诊断接口，不要变成任意文件读取。
- 不要因为某个 Edge `stale/offline` 就自动把环境包调度到另一台服务器运行；跨服务器账号转移必须等核心环境指纹比对能力完成，并由用户或上层业务显式触发。
- 不要保存 proxy 明文、fingerprint raw、Cookies 或 Local Storage。
- 不要让前端决定镜像字符串；镜像由 Server `ImagePolicy` 根据节点架构选择。
- 不要在节点架构为 `unknown` 时自动创建环境包。
- 不要把 UDP beacon 当成节点事实源；自动发现后必须通过 Client HTTP API 探测确认才能落库或更新节点。
- 不要处理没有匹配 `discoveryMagic/service/discoveryGroup` 的 UDP 报文；Server 只识别本平台、本发现域的 Client beacon。
- 不要为了演示快而绕过节点凭证、任务表和审计边界。
