# Private_Browser_Server V1 歧义点与建议收口

> 阅读说明：
> 这份文档用于记录进入正式中心层前仍需拍板的设计点。
> 其中涉及 `node-registration.json`、`push clientId`、`X-Edge-API-Key` 的内容，都应优先按“过渡兼容链路”理解，而不是长期正式主链路。

## 1. 文档目标

这份文档只做一件事：

- 把当前 `Private_Browser_Server` 进入正式中心层之前，真正还模糊、会影响实现的点逐条挑出来。

每条都按下面格式收：

- 现在为什么模糊
- 如果不先定，会影响哪里
- 建议怎么收口

## 2. 当前最该先拍板的歧义

下面先给出当前推荐定案版。

如果后续你明确推翻某一条，再单独回改；在此之前，后续设计和实现先按这些口径推进。

## 2.1 `discovered` 到底是不是永久数据

### 当前模糊点

现在有两种口径同时存在：

- 一种说法是：`discovered` 只是内存过程视图，不落正式表。
- 另一种实现迹象是：Server 后续又需要根据 discovered 做心跳、排障和管理员观察。

### 影响范围

如果这题不先定，会直接影响：

- 是否需要 `discovery_observations` 之类的表
- `/api/v1/edge-clients/discovered` 重启后是否还应保留历史
- 管理员看到的是瞬时线索，还是可追踪线索

### 建议收口

建议定成：

- `discovered` 不是正式节点实体。
- `discovered` 默认只保留内存态，用于接入前观察。
- 正式持久化只落到：
  - `edge_clients.last_discovered_at`
  - `edge_clients.last_heartbeat_at`
  - `edge_clients.discovery_reason`

也就是：

- 不单独建 `discovered` 正式表
- 但已绑定节点的最近发现/心跳事实要回写到 `edge_clients`

## 2.2 bind 输入到底只用 `accountId + clientIp`，还是要允许 `baseUrl`

### 当前模糊点

第一阶段已经定成：

- `accountId + clientIp`

但进入正式中心层后，又会遇到：

- 管理员手动加入节点
- Client IP 变化
- `baseUrl` 比 `clientIp` 更直接可探测

### 影响范围

如果不定，后面会影响：

- bind API 请求体
- 手动加节点流程
- IP 漂移后的修复流程

### 建议收口

建议定成：

- 第一阶段 bind 主输入仍然保留 `accountId + clientIp`
- 但正式 V1 的手动接入流程允许额外传 `baseUrl`
- Server 内部统一做法是：
  - 优先使用调用方明确给的 `baseUrl`
  - 否则用 `clientIp` 组装 `http://{clientIp}:3300`

也就是：

- 外部主语义仍然是“绑定这台 Client”
- 内部探测入口允许 `clientIp` 和 `baseUrl` 两种来源

## 2.3 `discovery_status` 正式枚举到底有哪些

### 当前模糊点

现在文档里同时出现过这些状态：

- `discovered`
- `blocked`
- `verified`
- `identity_changed`

但另一些地方又要求：

- 正式节点只保留 `blocked / verified`
- 具体原因放在 `discovery_reason`

### 影响范围

这题不定会直接影响：

- `edge_clients.discovery_status` 枚举
- 前端筛选
- 风险状态展示
- 数据库迁移

### 建议收口

建议正式定成：

- `edge_clients.discovery_status` 只保留：
  - `blocked`
  - `verified`

其它所有中间或异常语义统一放到：

- `discovery_reason`

例如：

- `not_bound`
- `ip_mismatch`
- `identity_changed`
- `probe_failed`
- `stale`

补充说明：

- `discovered` 只是 discovered 视图里的过程状态，不进入正式节点表枚举

### 当前定案

- 正式通过。
- `edge_clients.discovery_status` 只保留 `blocked / verified`。
- 其它原因全部进入 `discovery_reason`。

## 2.4 bind 成功后，节点是否直接进入 `verified`

### 当前模糊点

现在有两种可能：

1. bind 只是绑定，不等于 verified，还需要单独 verify。
2. bind 过程中已经做了 `/health + /device-info` 探测，那就可以直接 verified。

### 影响范围

这题不定会影响：

- 是否还需要 `POST /api/v1/edge-clients/{clientId}/verify`
- 前端绑定后的提示
- 后续 env 是否能立即使用

### 建议收口

建议定成：

- bind 成功且探测通过后，节点直接进入 `verified`
- 不再保留单独 verify 作为正常接入步骤

单独 verify 只保留为未来异常修复或管理员重验能力，如果要有，也不作为主线必须步骤。

### 当前定案

- 正式通过。
- 正常主线里 bind 成功且探测通过后，节点直接进入 `verified`。
- `verify` 不作为常规接入步骤。

## 2.5 `clientId` 序号生成规则是否足够稳定

### 当前模糊点

当前实现是：

- 取当前账号下节点数量 `len(nodes)+1`

这在第一阶段能跑，但正式中心层会遇到：

- 解绑后再绑
- 删除旧节点
- 并发 bind

### 影响范围

会影响：

- `clientId` 是否可能重复
- 设备序号是否回退
- 审计稳定性

### 建议收口

建议定成：

- `clientId = mainAccountId + 4位设备序号` 继续保留
- 设备序号不再按“当前数量”计算
- 统一按 `MAX(client_sequence) + 1` 分配
- 即使解绑或软删除，也不回收旧序号

这样最稳。

### 当前定案

- 正式通过。
- `clientId = mainAccountId + 4位设备序号` 保持不变。
- 设备序号按 `MAX(client_sequence) + 1` 分配。
- 不回收旧序号。

## 2.6 解绑后 Node 侧到底要不要调用 Client 清空本地 JSON

### 当前模糊点

第一阶段文档已经定过：

- 解绑后 Client 本地 `node-registration.json` 要清空

但现在 Server 侧还没有完整的解绑链路设计。

### 影响范围

会影响：

- 是否需要新的 Edge API，例如 `POST /api/v1/edge/node-registration/clear`
- unbind 的任务编排
- bind / unbind 审计

### 建议收口

建议定成：

- unbind 是 Server 正式动作
- Server 解除中心归属后，必须立即调用 Client 清空本地 `node-registration.json`
- 如果清空失败：
  - 中心 unbind 仍然成立
  - 但要记录 `clearRegistrationStatus=failed`
  - 允许管理员补清

### 当前定案

- 正式通过。
- unbind 后，Server 必须尝试清空 Client 本地 `node-registration.json`。
- 清空失败不回滚中心 unbind，但必须留痕并允许补清。

## 2.7 `edge_clients` 表字段现在还不够完整，正式最小字段到底有哪些

### 当前模糊点

现在新 Server 代码里的节点字段偏少，只够 bind 第一阶段。

但正式中心层需要的又明显更多：

- `discovery_status`
- `discovery_reason`
- `last_heartbeat_at`
- `last_checked_at`
- `last_error`
- `created_by_user_id`

### 影响范围

这题不定，后面会导致：

- Model / Dao / Repository 一轮一轮改
- OpenAPI 和 DB 反复打架

### 建议收口

建议先把 `edge_clients` 最小正式字段固定为：

- `id`
- `main_account_id`
- `client_sequence`
- `name`
- `base_url`
- `client_ip`
- `docker_api_url`
- `os`
- `arch`
- `cpu_cores`
- `memory_total_mb`
- `docker_version`
- `health_status`
- `discovery_status`
- `discovery_reason`
- `push_status`
- `last_discovered_at`
- `last_heartbeat_at`
- `last_heartbeat_reported_at`
- `last_heartbeat_source`
- `last_checked_at`
- `last_error`
- `created_by_user_id`
- `created_by_username`
- `created_at`
- `updated_at`
- `deleted_at`

其中：

- `push_status` 不能只停留在内存或响应体，要正式入库

### 当前定案

- 正式通过。
- `edge_clients` 最小正式字段固定为：

```text
id
main_account_id
client_sequence
name
base_url
client_ip
docker_api_url
os
arch
cpu_cores
memory_total_mb
docker_version
health_status
discovery_status
discovery_reason
push_status
last_discovered_at
last_heartbeat_at
last_heartbeat_reported_at
last_heartbeat_source
last_checked_at
last_error
created_by_user_id
created_by_username
created_at
updated_at
deleted_at
```

- 额外收口：
  - `push_status` 正式入库
  - `discovery_status` 仍只允许 `blocked / verified`
  - `health_status` 继续使用 `healthy / unhealthy / offline / stale`

## 2.8 `server_browser_envs` 以什么为主键，最小字段到底有哪些

### 当前模糊点

现在 old 里这张表偏轻，新文档里又要求它承担更多聚合事实。

特别是这些点还没完全定死：

- 主键只用 `env_id` 是否足够
- 是否必须保存 `user_id`
- 是否需要 `current_slot_id`
- 是否要保存 `runtime_protection`

### 影响范围

会影响：

- Server env 列表接口
- run / stop / backup / restore 前校验
- 前端主状态展示

### 建议收口

建议定成：

- 主键继续使用 `env_id`
- 因为当前业务已经明确一个 env 固定绑定一台 Client，不做自动跨节点迁移

最小正式字段建议至少有：

- `env_id`
- `main_account_id`
- `client_id`
- `user_id`
- `rpa_type`
- `name`
- `status`
- `container_status`
- `runtime_status`
- `current_slot_id`
- `cdp_url`
- `web_vnc_url`
- `last_task_id`
- `last_error`
- `last_synced_at`
- `created_at`
- `updated_at`
- `deleted_at`

### 当前定案

- 正式通过。
- `server_browser_envs` 主键继续使用 `env_id`。
- 当前商业口径下，一个 env 固定绑定一台 Client，不做自动跨节点迁移。
- `server_browser_envs` 最小正式字段固定为：

```text
env_id
main_account_id
client_id
user_id
rpa_type
name
status
container_status
runtime_status
current_slot_id
cdp_url
web_vnc_url
last_task_id
last_error
last_synced_at
created_at
updated_at
deleted_at
```

- 额外收口：
  - 这张表是中心聚合缓存和调度视图，不是 Edge 资产真相源
  - `status` 是前端和中心调度的主状态
  - `container_status` / `runtime_status` 只是辅助排障和运行态摘要

## 2.9 `server_tasks` 现在缺不缺字段

### 当前模糊点

当前 old 的 `server_tasks` 更像最小 task 表，但现在新规则已经更严格：

- 需要绑定 `edgeTaskId`
- 需要保留 `eventsUrl`
- 需要保留 `suggestion`
- 需要明确 `resourceType/resourceId`

### 影响范围

这题不定会直接影响：

- 任务详情 API
- 审计页面
- 失败排障信息

### 建议收口

建议 `server_tasks` 最小正式字段至少有：

- `id`
- `main_account_id`
- `operator_user_id`
- `operator_username`
- `client_id`
- `env_id`
- `type`
- `resource_type`
- `resource_id`
- `status`
- `edge_task_id`
- `events_url`
- `error_message`
- `suggestion`
- `created_at`
- `updated_at`
- `finished_at`

并且终态固定只有：

- `success`
- `failed`

### 当前定案

- 正式通过。
- `server_tasks` 最小正式字段固定为：

```text
id
main_account_id
operator_user_id
operator_username
client_id
env_id
type
resource_type
resource_id
status
edge_task_id
events_url
error_message
suggestion
created_at
updated_at
finished_at
```

- 额外收口：
  - `status` 终态只有 `success / failed`
  - 中间态允许保留 `pending / running`
  - 不再保留 `canceled` 作为 V1 正式终态
  - `resource_type/resource_id` 必须入库，避免后续只靠 `env_id` 或 `client_id` 猜任务目标

## 2.10 run 准入里的“平台额度”要不要本地持久化

### 当前模糊点

现在只明确了：

- run 前必须检查平台额度

但还没定：

- 额度是每次临时调平台
- 还是 Node 本地缓存一份
- 本地缓存要不要落 SQLite

### 影响范围

会影响：

- `Platform/` 域设计
- run 接口性能
- 平台不可达时的收口

### 建议收口

建议定成：

- 平台额度以平台返回为准
- Node Server 本地允许缓存最近一次可信额度
- 缓存应落 SQLite，而不是只放内存

原因：

- Node 重启后还能保留最近一次平台事实
- 便于管理员排障
- 但 run 时仍优先实时校验平台

### 当前定案

- 正式通过。
- 平台额度要在 Node Server 本地 SQLite 持久化一份“最近一次可信额度快照”。
- 这份快照不是平台真相源，只是中心准入与管理员排障缓存。
- run 时优先实时校验平台。
- 平台不可达、额度不存在、额度为 `0`、额度已过期或额度状态不可信时，V1 默认拒绝放行。

#### 当前补充收口

后续额度快照最小字段建议固定为：

```text
client_id
quota_limit
quota_used_snapshot
quota_available_snapshot
fetched_at
expires_at
status
last_error
```

其中：

- `status` 只表示这份快照当前是否可信，不替代平台最终真相
- `expires_at` 是 run 准入判断缓存是否还能参考的关键字段

## 2.11 Server 是否要管理 slot 表

### 当前模糊点

现在新架构已经明确：

- slot 是 Client 本机资源位
- package 是 package
- 容器是容器

但 Server 要不要自己再建一张 slot 中心表，还没明确写死。

### 影响范围

会影响：

- 中心模型复杂度
- 与 Client slot 状态的一致性

### 建议收口

建议定成：

- V1 Server 不单独建立中心 slot 主表
- slot 只作为：
  - `server_browser_envs.current_slot_id`
  - 任务参数
  - Edge 返回事实的一部分

理由：

- 当前 slot 真相源就在 Client
- V1 不值得再造第二套 slot 中心事实源

### 当前定案

- 正式通过。
- V1 不单独建立中心 slot 主表。
- slot 真相继续留在 Client。
- Server 只保存：
  - `server_browser_envs.current_slot_id`
  - `server_tasks` 中动作使用的 `slotId`
  - Edge 返回的 slot 运行事实摘要

#### 当前补充收口

- 需要查看 slot 列表、slot 状态或 slot 详情时，Server 应实时调用 Client `/api/v1/edge/slots/*`
- 不在中心侧复制一套 `waiting / loading / occupied / releasing` 主状态表
- 后续如果真的进入“中心资源池后台”阶段，再评估是否把 slot 升级成中心实体

## 2.12 OpenAPI 命名空间要不要现在统一

### 当前模糊点

现在已有和建议中的路径存在差异，例如：

- `/api/v1/server/edge-clients/heartbeat`
- `/api/v1/edge-clients/discovered`

这说明命名空间还没完全统一。

### 影响范围

这会影响：

- Swagger
- 前端 SDK
- 后续 API 文档

### 建议收口

建议定成：

- 节点与发现统一走 `/api/v1/edge-clients/*`
- 环境包统一走 `/api/v1/browser-envs/*`
- 任务统一走 `/api/v1/tasks/*`

并且：

- 去掉单独的 `/api/v1/server/*` 前缀
- 让中心服务 API 自己就是默认语义，不再额外套 `server`

### 当前定案

- 正式通过。
- 节点走 `/api/v1/edge-clients/*`
- 环境走 `/api/v1/browser-envs/*`
- 任务走 `/api/v1/tasks/*`
- 不再新增 `/api/v1/server/*`

## 3. 建议优先级

建议按下面顺序拍板：

### P0 立刻先定

- 2.3 `discovery_status` 正式枚举
- 2.4 bind 成功后是否直接 verified
- 2.5 `clientId` 序号生成规则
- 2.6 unbind 后是否强制清空 Client 本地 JSON
- 2.12 API 命名空间统一

### P1 紧接着定

- 2.7 `edge_clients` 最小正式字段
- 2.8 `server_browser_envs` 最小正式字段
- 2.9 `server_tasks` 最小正式字段
- 2.10 平台额度缓存方式

### P2 可以边做边细化

- 2.1 discovered 是否需要额外审计持久化
- 2.2 bind 输入是否扩成 `clientIp + baseUrl`
- 2.11 Server 是否需要中心 slot 表

## 4. 一句话收口

当前新 Server 其实已经不是“没有结构”，而是“结构已有、中心层规则还没完全拍板”。

所以最正确的推进方式不是马上写很多代码，而是先把上面这 12 个问题收紧。
