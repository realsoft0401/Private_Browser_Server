# Private_Browser_Server V1 数据库表设计

## 1. 文档目标

这份文档只回答一件事：

- `Private_Browser_Server` 当前重建阶段的本地索引库到底要建哪些正式表，每张表保存什么，不保存什么。

它服务的是后续：

- `Models`
- `Dao`
- `Repository`
- SQLite migrate
- OpenAPI / 管理端字段口径

统一收口。

## 2. 总体原则

> 存储路线说明：
> 当前文档描述的是 Server 重建阶段和单机开发阶段的本地索引库口径，因此落点是 SQLite。
> 研发架构书与 BP 的商业化正式口径仍然是中心服务使用 MySQL。
> 二者不是互相冲突，而是阶段不同：
> SQLite 用于当前重建阶段快速收口中心索引与接口骨架，商业 V1 落地时应把正式中心持久化迁到 MySQL。

先把最重要的原则写死：

### 2.1 Server SQLite 是中心控制面索引库，不是 Edge 资产库

Server 本地 SQLite 只保存：

- 节点中心身份
- 节点状态摘要
- browser-env 聚合缓存
- 平台任务事实
- 平台额度快照
- 绑定与解绑审计

它不保存：

- Edge `profile.json`
- Edge `binding.json`
- `proxy` 明文
- `fingerprint raw`
- `browser-data/profile`
- Cookies / Local Storage / IndexedDB
- Edge SQLite 全量镜像

### 2.2 真相源不能混

必须固定：

- 节点中心身份真相源：`edge_clients`
- Edge 本机环境资产真相源：Client 自己的 SQLite + 环境包目录
- 中心环境聚合视图：`server_browser_envs`
- 平台任务真相源：`server_tasks`
- 平台额度真相源：PlatformServer
- 平台额度本地缓存：Node Server SQLite

### 2.3 不建立第二套 slot 真相源，但要建立中心 slot 关系缓存

V1 不把 slot 真相从 Client 搬到 Node Server，也不让 Node Server 脱离 Client 自己发明 slot 状态。

但这不等于中心完全不建 slot 相关表。

当前正式口径是：

- slot 真相源继续留在 Client
- Node Server 允许建立 node-slot 关系子表和 slot 审计表
- 这些表只保存中心缓存、治理关系和排障摘要，不替代 Client 本机正式事实

Server 正式保存：

- `server_browser_envs.current_slot_id`
- `server_tasks` 动作里的 `slotId`
- `edge_clients` 上的 slot 摘要字段
- `edge_client_slots` 当前 node-slot 关系子表
- `edge_client_slot_logs` slot 资源动作与异常审计

### 2.4 正式表按“当前关系表 / 历史审计表”分开处理

原因：

- 节点解绑后历史任务还要可追
- 环境包删除后中心审计还要可追
- 平台排障要看历史状态变化

正式规则：

- `edge_clients`
- `server_browser_envs`

这类“当前正式对象表”允许软删除或保留当前有效标记。

- `edge_client_bind_logs`
- `edge_client_slot_logs`
- `server_tasks`

这类“历史审计表”天然就是历史保留表，不谈软删除。

- `edge_client_slots`

这类“当前关系子表”不做软删除；解绑、缩容或关系失效时直接物理删除，历史统一看日志。

注意：

- 软删除不代表还能继续业务放行
- 业务准入只能看未删除有效记录

## 3. V1 正式表清单

建议 V1 正式保留 7 张中心表：

1. `edge_clients`
2. `edge_client_bind_logs`
3. `edge_client_slots`
4. `edge_client_slot_logs`
5. `server_browser_envs`
6. `server_tasks`
7. `client_run_quotas`

## 3.1 这 7 张表的关系边界

为了避免后面把 Node 库重新做成第二套 Client，本节把表关系边界写死。

- `edge_clients`
  - 节点中心主表
  - 保存当前正式节点关系和 slot 摘要

- `edge_client_bind_logs`
  - 节点绑定治理历史
  - 不反向承载当前节点主状态

- `edge_client_slots`
  - node-slot 当前关系子表
  - 只保存当前 `client_id` 下正式存在的 slot 关系

- `edge_client_slot_logs`
  - slot 资源动作与异常历史
  - 不反向承载当前 slot 主状态

- `server_browser_envs`
  - browser-env 中心聚合表
  - 通过 `current_slot_id` 引用 slot，不复制 slot 状态机

- `server_tasks`
  - 中心任务事实表
  - 允许记录 `slot_reconcile`，但不替代 slot 关系表和 slot 日志表

- `client_run_quotas`
  - 平台额度缓存表
  - 不替代 slot 资源事实

## 4. `edge_clients`

## 4.1 表定位

`edge_clients` 是正式节点表。

这张表是：

- Client 中心身份表
- 节点绑定归属表
- 节点治理状态表
- 节点级 slot 摘要表

它不是：

- discovered 临时视图表
- slot 明细表
- browser-env 资产表

## 4.2 最小正式字段

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
target_slot_count
actual_slot_count
available_slot_count
running_slot_count
slot_exception_status
slot_exception_reason
last_slot_checked_at
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

## 4.3 字段解释

### 身份字段

- `id`
  - 对外统一叫 `clientId`
  - 规则固定为 `mainAccountId + 4位设备序号`
  - 是中心正式节点身份

- `main_account_id`
  - 当前归属主账号
  - 同一时刻一个 `clientId` 只能归属一个主账号

- `client_sequence`
  - 账号下设备序号
  - 按 `MAX(client_sequence)+1` 分配
  - 不回收

### 地址与设备事实

- `name`
  - 节点显示名，默认可等于 hostname 或 clientId

- `base_url`
  - Edge 服务入口

- `client_ip`
  - 当前内网接入 IP

- `docker_api_url`
  - 该 Client 暴露给中心理解的 Docker API 地址摘要

- `os`
- `arch`
- `cpu_cores`
- `memory_total_mb`
- `docker_version`
  - 都属于设备事实摘要

### 节点治理状态

- `health_status`
  - 只允许：
    - `healthy`
    - `unhealthy`
    - `offline`
    - `stale`

- `discovery_status`
  - 只允许：
    - `blocked`
    - `verified`

- `discovery_reason`
  - 保存被拦住或异常原因
  - 例如：
    - `not_bound`
    - `ip_mismatch`
    - `identity_changed`
    - `probe_failed`
    - `stale`

- `push_status`
  - 保存最近一次下发 `clientId` 到 Client 的结果
  - 例如：
    - `pending`
    - `success`
    - `failed`

### slot 摘要字段

- `target_slot_count`
  - 当前中心要求这台 Client 应持有多少个 slot
  - 来源是 Platform / Node 中心配置

- `actual_slot_count`
  - 最近一次 slot 对账时，Client 正式 slot API 返回的有效 slot 总数

- `available_slot_count`
  - 最近一次 slot 对账时，处于 `waiting` 状态的 slot 数
  - run 前快速校验优先看它

- `running_slot_count`
  - 最近一次 slot 对账时，处于 `running` 状态的 slot 数

- `slot_exception_status`
  - slot 资源层是否异常
  - 只允许：
    - `normal`
    - `exception`

- `slot_exception_reason`
  - slot 异常原因摘要
  - 例如：
    - `slot_count_mismatch`
    - `slot_state_mismatch`
    - `slot_sync_failed`

- `last_slot_checked_at`
  - 最近一次完成 slot 对账任务的时间

### 发现与心跳

- `last_discovered_at`
  - 最近一次通过 discovery 看到它的时间

- `last_heartbeat_at`
  - Server 真正收到心跳的时间

- `last_heartbeat_reported_at`
  - Client 自报心跳时间

- `last_heartbeat_source`
  - `udp` 或 `http`

- `last_checked_at`
  - 最近一次完成 `/health + /device-info` 探测时间

- `last_error`
  - 最近一次节点治理错误摘要

### 审计字段

- `created_by_user_id`
- `created_by_username`
- `created_at`
- `updated_at`
- `deleted_at`

## 4.4 不应该进入这张表的内容

不要塞：

- browser-env 状态
- slot 明细状态机
- `slot_id`
- `container_id`
- `cdp_port`
- `vnc_port`
- 平台额度
- Edge 文件路径
- profile / proxy / fingerprint 详情

## 4.5 索引建议

- `UNIQUE(base_url) WHERE deleted_at = 0`
- `INDEX(main_account_id, deleted_at)`
- `INDEX(health_status, discovery_status, deleted_at)`
- `INDEX(client_ip, deleted_at)`
- `INDEX(slot_exception_status, deleted_at)`

## 5. `edge_client_bind_logs`

## 5.1 表定位

这张表是节点绑定审计表。

它负责记录：

- bind
- push clientId
- unbind
- clear registration
- 覆盖下发

## 5.2 最小正式字段

```text
id
client_id
main_account_id
client_ip
action
result
message
operator_user_id
operator_username
created_at
```

## 5.3 字段解释

- `action`
  - 例如：
    - `bind`
    - `push_client_id`
    - `unbind`
    - `clear_registration`

- `result`
  - 例如：
    - `success`
    - `failed`

- `message`
  - 错误或结果摘要

## 5.4 这张表为什么单独存在

因为：

- `edge_clients` 只适合放当前节点状态
- 绑定链路必须有据可查
- push 失败不能只留在日志文件里

## 6. `edge_client_slots`

## 6.1 表定位

`edge_client_slots` 是 Node Server 的 node-slot 关系子表。

它是：

- 当前已绑定 `client_id` 下的 slot 明细关系表
- slot 对账任务的中心缓存落点
- run 选 slot、治理 slot、排障 slot 的明细依据

它不是：

- Client 本机 slot 真相源
- slot 永久历史表
- 平台授权表

## 6.2 关键边界

- slot 正式状态固定沿用 Client 当前运行模型：
  - `waiting`
  - `loading`
  - `running`
  - `ending`
- 这张表属于“当前绑定关系下”的正式关系表
- unbind 时必须删除
- rebind 后不恢复旧关系，而是等待重新初始化和 slot 对账重建
- 这张表允许保存运行摘要，但最终事实仍以 Client 本机为准

## 6.3 最小正式字段

```text
id
client_id
slot_id
status
current_env_id
current_run_id
container_id
container_name
cdp_port
vnc_port
last_error
last_synced_at
created_at
updated_at
```

## 6.4 字段解释

- `id`
  - 自增主键
  - 只服务数据库内部引用

- `client_id`
  - 当前属于哪个中心节点

- `slot_id`
  - slot 编号
  - 当前正式口径统一为：
    - `slot001`
    - `slot002`
    - ...
  - 只在当前 `client_id` 范围内唯一，不做全局唯一

- `status`
  - 当前 slot 状态
  - 只允许：
    - `waiting`
    - `loading`
    - `running`
    - `ending`

- `current_env_id`
  - 当前占用该 slot 的 env
  - `waiting` 时允许为空

- `current_run_id`
  - 当前运行链路标识
  - `waiting` 时允许为空

- `container_id`
- `container_name`
- `cdp_port`
- `vnc_port`
  - 最近一次从 Client 同步到的运行摘要
  - 方便中心排障和展示

- `last_error`
  - 最近一次 slot 层错误摘要

- `last_synced_at`
  - 最近一次由 slot 对账任务刷新这条关系的时间

- `created_at`
- `updated_at`
  - 中心缓存自身审计字段

## 6.5 使用原则

- 这张表采用当前 `client_id + slot_id` 业务唯一约束
- 不做软删除
- 解绑、缩容或关系重建时直接物理删除
- 历史变化统一看日志表，不看这张表残留尸体

## 6.6 索引建议

- `UNIQUE(client_id, slot_id)`
- `INDEX(client_id, status)`
- `INDEX(current_env_id)`
- `INDEX(last_synced_at)`

## 7. `edge_client_slot_logs`

## 7.1 表定位

这张表是 slot 资源治理与异常审计表。

它负责记录：

- create-slot
- destroy-slot
- reinit-slot
- assign-slot
- release-slot
- slot-sync
- slot-exception
- slot-recover

它不是：

- bind / unbind 审计表
- browser-env 生命周期任务表

## 7.2 最小正式字段

```text
id
client_id
slot_id
action
result
env_id
run_id
message
operator_user_id
operator_username
created_at
```

## 7.3 字段解释

- `action`
  - 例如：
    - `create_slot`
    - `destroy_slot`
    - `reinit_slot`
    - `assign_slot`
    - `release_slot`
    - `sync_slot`
    - `slot_exception`
    - `slot_recover`

- `result`
  - 只建议：
    - `success`
    - `failed`

- `env_id`
- `run_id`
  - 与当前 slot 动作有关时保存引用

- `message`
  - 动作或异常摘要
  - 例如目标 slot 数、实际 slot 数、失败原因、管理员提示

## 7.4 为什么单独存在

因为：

- slot 已经是独立资源对象
- slot 动作不能继续混进 bind 日志
- slot 异常不能只留在运行日志或 task 错误里

## 8. `server_browser_envs`

## 8.1 表定位

`server_browser_envs` 是中心环境聚合视图表。

它是：

- 列表缓存
- 调度视图
- 前端主状态来源

它不是：

- Edge 资产真相源
- 环境包原子文件存储表

## 8.2 主键与绑定原则

- 主键固定 `env_id`
- 当前口径下，一个 env 固定绑定一台 Client
- V1 不做自动跨节点迁移

## 8.3 最小正式字段

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

## 8.4 字段解释

- `env_id`
  - 环境唯一标识

- `main_account_id`
  - 中心主账号归属

- `client_id`
  - 当前绑定节点

- `user_id`
  - 业务用户标识

- `rpa_type`
  - 如 `tk`

- `name`
  - 展示名

- `status`
  - 中心主状态
  - 前端和调度优先看这个

- `container_status`
  - Edge Docker 事实摘要

- `runtime_status`
  - 中心运行态摘要
  - 例如可表达 `running/stopped/backed_up/error`

- `current_slot_id`
  - 当前 env 绑定到哪个 slot
  - 只作为引用字段，不建立中心 slot 主表

- `cdp_url`
- `web_vnc_url`
  - Edge 返回的内网监控地址摘要

- `last_task_id`
  - 最近一次平台动作 task

- `last_error`
  - 最近一次错误摘要

- `last_synced_at`
  - 最近一次与 Edge 同步时间

## 8.5 不应该进入这张表的内容

不要塞：

- `profile.json` 全量内容
- `binding.json` 全量内容
- 代理明文
- 指纹原文
- 浏览器登录态
- slot 完整状态机

## 8.6 索引建议

- `INDEX(main_account_id, deleted_at)`
- `INDEX(client_id, status, deleted_at)`
- `INDEX(user_id, deleted_at)`
- `INDEX(last_synced_at)`

## 9. `server_tasks`

## 9.1 表定位

`server_tasks` 是平台级持久任务表。

它负责：

- 保存中心 taskId
- 绑定 `edgeTaskId`
- 保存最终结果
- 保存错误与建议

它不是：

- Edge task 全量镜像表
- SSE 事件明细全量仓库

## 9.2 最小正式字段

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

## 9.3 字段解释

- `type`
  - 例如：
    - `create_env`
    - `run_env`
    - `stop_env`
    - `backup_env`
    - `restore_env`
    - `revalidate_env`
    - `import_env_package`
    - `delete_env_package`
    - `slot_reconcile`

- `resource_type`
  - 例如：
    - `browser_env`
    - `edge_client`
    - `docker_image`

- `resource_id`
  - 与 `resource_type` 配套的资源主键

- `status`
  - 中间态允许：
    - `pending`
    - `running`
  - 正式终态只允许：
    - `success`
    - `failed`
  - 这里的成功只表示中心任务动作完成
  - 如果是 `slot_reconcile`，即使发现 `slot 异常`，任务本身也可以是 `success`

- `edge_task_id`
  - Edge 返回的任务 id

- `events_url`
  - Edge 事件订阅入口摘要
  - 对 `slot_reconcile` 这类中心任务，也允许保存中心 SSE 入口摘要

- `error_message`
- `suggestion`
  - 管理员排障字段

## 9.4 不应该进入这张表的内容

不要塞：

- Edge SSE 全量长日志
- proxy 明文
- fingerprint raw
- browser-data 路径详情

## 9.5 索引建议

- `INDEX(main_account_id, created_at)`
- `INDEX(env_id, created_at)`
- `INDEX(client_id, created_at)`
- `INDEX(status, updated_at)`

## 10. `client_run_quotas`

## 10.1 表定位

这张表是平台额度快照表。

它是：

- 最近一次可信平台额度缓存
- run 准入参考快照
- 管理员排障依据

它不是：

- 平台真相源
- 最终商业授权源

## 10.2 最小正式字段

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

## 10.3 字段解释

- `client_id`
  - 对应哪个中心节点

- `quota_limit`
  - 平台给的并发上限

- `quota_used_snapshot`
  - 当次抓取或校验时看到的已占用数摘要

- `quota_available_snapshot`
  - 可用数摘要

- `fetched_at`
  - 这份快照的抓取时间

- `expires_at`
  - 这份快照什么时候失去可信度

- `status`
  - 这份快照当前是否可信
  - 例如：
    - `valid`
    - `expired`
    - `untrusted`

- `last_error`
  - 最近一次平台额度同步错误摘要

## 10.4 使用原则

- run 时优先实时查平台
- 平台查成功，刷新这张表
- 平台不可达、额度不存在、额度为 `0`、已过期或不可信时，V1 默认拒绝放行

## 11. 明确不建的表

V1 明确不建：

- `discovered_clients`
- `edge_task_events`

原因：

- `slot` 真相留在 Client，本地 Node 只保留关系缓存和审计
- discovered 只是过程视图
- 事件流不是中心长期事实源

## 12. 一句话收口

Server V1 SQLite 的正确定位是：

- 用最少但足够的正式中心表，收住节点、slot 关系缓存、slot 审计、环境、任务、额度和绑定审计
- 不把 Edge 本地资产、slot 真相和敏感材料整套搬进中心库
- 允许中心保存 slot 摘要和 node-slot 关系缓存，但最终 slot 正式事实仍以 Client 为准

这样后面扩功能，数据库不会先变成第二套 Edge 系统。

## 13. SQLite 建表草案

这一节不是最终 migrate 文件，但要求已经足够接近正式 SQL。

原则：

- 先把字段和约束收准
- 具体列顺序、默认值微调可以放到迁移实现阶段
- 只要与本节冲突，以本节为准，不要再回到旧口径

### 13.1 `edge_clients`

```sql
CREATE TABLE edge_clients (
  id TEXT PRIMARY KEY,
  main_account_id TEXT NOT NULL DEFAULT '',
  client_sequence INTEGER NOT NULL DEFAULT 0,
  name TEXT NOT NULL DEFAULT '',
  base_url TEXT NOT NULL DEFAULT '',
  client_ip TEXT NOT NULL DEFAULT '',
  docker_api_url TEXT NOT NULL DEFAULT '',
  os TEXT NOT NULL DEFAULT '',
  arch TEXT NOT NULL DEFAULT '',
  cpu_cores INTEGER NOT NULL DEFAULT 0,
  memory_total_mb INTEGER NOT NULL DEFAULT 0,
  docker_version TEXT NOT NULL DEFAULT '',
  health_status TEXT NOT NULL DEFAULT 'stale',
  discovery_status TEXT NOT NULL DEFAULT 'blocked',
  discovery_reason TEXT NOT NULL DEFAULT 'not_bound',
  push_status TEXT NOT NULL DEFAULT 'pending',
  target_slot_count INTEGER NOT NULL DEFAULT 0,
  actual_slot_count INTEGER NOT NULL DEFAULT 0,
  available_slot_count INTEGER NOT NULL DEFAULT 0,
  running_slot_count INTEGER NOT NULL DEFAULT 0,
  slot_exception_status TEXT NOT NULL DEFAULT 'normal',
  slot_exception_reason TEXT NOT NULL DEFAULT '',
  last_slot_checked_at INTEGER NOT NULL DEFAULT 0,
  last_discovered_at INTEGER NOT NULL DEFAULT 0,
  last_heartbeat_at INTEGER NOT NULL DEFAULT 0,
  last_heartbeat_reported_at INTEGER NOT NULL DEFAULT 0,
  last_heartbeat_source TEXT NOT NULL DEFAULT '',
  last_checked_at INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  created_by_user_id TEXT NOT NULL DEFAULT '',
  created_by_username TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0,
  deleted_at INTEGER NOT NULL DEFAULT 0
);
```

### 13.2 `edge_client_bind_logs`

```sql
CREATE TABLE edge_client_bind_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  client_id TEXT NOT NULL DEFAULT '',
  main_account_id TEXT NOT NULL DEFAULT '',
  client_ip TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT '',
  result TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  operator_user_id TEXT NOT NULL DEFAULT '',
  operator_username TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0
);
```

### 13.3 `edge_client_slots`

```sql
CREATE TABLE edge_client_slots (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  client_id TEXT NOT NULL,
  slot_id TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'waiting',
  current_env_id TEXT NOT NULL DEFAULT '',
  current_run_id TEXT NOT NULL DEFAULT '',
  container_id TEXT NOT NULL DEFAULT '',
  container_name TEXT NOT NULL DEFAULT '',
  cdp_port INTEGER NOT NULL DEFAULT 0,
  vnc_port INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  last_synced_at INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0
);
```

### 13.4 `edge_client_slot_logs`

```sql
CREATE TABLE edge_client_slot_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  client_id TEXT NOT NULL DEFAULT '',
  slot_id TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT '',
  result TEXT NOT NULL DEFAULT '',
  env_id TEXT NOT NULL DEFAULT '',
  run_id TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  operator_user_id TEXT NOT NULL DEFAULT '',
  operator_username TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0
);
```

### 13.5 `server_browser_envs`

```sql
CREATE TABLE server_browser_envs (
  env_id TEXT PRIMARY KEY,
  main_account_id TEXT NOT NULL DEFAULT '',
  client_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  rpa_type TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  container_status TEXT NOT NULL DEFAULT '',
  runtime_status TEXT NOT NULL DEFAULT '',
  current_slot_id TEXT NOT NULL DEFAULT '',
  cdp_url TEXT NOT NULL DEFAULT '',
  web_vnc_url TEXT NOT NULL DEFAULT '',
  last_task_id TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  last_synced_at INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0,
  deleted_at INTEGER NOT NULL DEFAULT 0
);
```

### 13.6 `server_tasks`

```sql
CREATE TABLE server_tasks (
  id TEXT PRIMARY KEY,
  main_account_id TEXT NOT NULL DEFAULT '',
  operator_user_id TEXT NOT NULL DEFAULT '',
  operator_username TEXT NOT NULL DEFAULT '',
  client_id TEXT NOT NULL DEFAULT '',
  env_id TEXT NOT NULL DEFAULT '',
  type TEXT NOT NULL DEFAULT '',
  resource_type TEXT NOT NULL DEFAULT '',
  resource_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  edge_task_id TEXT NOT NULL DEFAULT '',
  events_url TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  suggestion TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT 0,
  updated_at INTEGER NOT NULL DEFAULT 0,
  finished_at INTEGER NOT NULL DEFAULT 0
);
```

### 13.7 `client_run_quotas`

```sql
CREATE TABLE client_run_quotas (
  client_id TEXT PRIMARY KEY,
  quota_limit INTEGER NOT NULL DEFAULT 0,
  quota_used_snapshot INTEGER NOT NULL DEFAULT 0,
  quota_available_snapshot INTEGER NOT NULL DEFAULT 0,
  fetched_at INTEGER NOT NULL DEFAULT 0,
  expires_at INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'untrusted',
  last_error TEXT NOT NULL DEFAULT ''
);
```

## 14. 约束与索引最终收口

### 14.1 枚举口径

实现阶段即使 SQLite 不强制写 `CHECK`，代码层和 migrate 注释也必须按这里执行。

`edge_clients.health_status`

- `healthy`
- `unhealthy`
- `offline`
- `stale`

`edge_clients.discovery_status`

- `blocked`
- `verified`

`edge_clients.push_status`

- `pending`
- `success`
- `failed`

`edge_clients.slot_exception_status`

- `normal`
- `exception`

`edge_client_slots.status`

- `waiting`
- `loading`
- `running`
- `ending`

`edge_client_slot_logs.result`

- `success`
- `failed`

`server_tasks.status`

- `pending`
- `running`
- `success`
- `failed`

`client_run_quotas.status`

- `valid`
- `expired`
- `untrusted`

### 14.2 唯一约束

`edge_clients`

- `PRIMARY KEY(id)`
- `UNIQUE(base_url) WHERE deleted_at = 0`

`edge_client_slots`

- `PRIMARY KEY(id)`
- `UNIQUE(client_id, slot_id)`

`server_browser_envs`

- `PRIMARY KEY(env_id)`

`server_tasks`

- `PRIMARY KEY(id)`

`client_run_quotas`

- `PRIMARY KEY(client_id)`

### 14.3 推荐索引

`edge_clients`

- `INDEX(main_account_id, deleted_at)`
- `INDEX(client_ip, deleted_at)`
- `INDEX(health_status, discovery_status, deleted_at)`
- `INDEX(slot_exception_status, deleted_at)`

`edge_client_bind_logs`

- `INDEX(client_id, created_at)`
- `INDEX(main_account_id, created_at)`

`edge_client_slots`

- `INDEX(client_id, status)`
- `INDEX(current_env_id)`
- `INDEX(last_synced_at)`

`edge_client_slot_logs`

- `INDEX(client_id, created_at)`
- `INDEX(slot_id, created_at)`
- `INDEX(env_id, created_at)`

`server_browser_envs`

- `INDEX(main_account_id, deleted_at)`
- `INDEX(client_id, status, deleted_at)`
- `INDEX(user_id, deleted_at)`
- `INDEX(last_synced_at)`

`server_tasks`

- `INDEX(main_account_id, created_at)`
- `INDEX(client_id, created_at)`
- `INDEX(env_id, created_at)`
- `INDEX(status, updated_at)`

### 14.4 字段级硬规则

这些规则不一定全靠 SQL `CHECK` 完成，但后续实现必须遵守。

1. `edge_clients.target_slot_count / actual_slot_count / available_slot_count / running_slot_count` 不能为负数。
2. `edge_client_slots.status=waiting` 时，`current_env_id` 和 `current_run_id` 应为空字符串。
3. `edge_client_slots` 不做软删除，解绑和缩容直接物理删除。
4. `edge_client_slots.slot_id` 只在当前 `client_id` 范围内唯一，不做全局唯一。
5. `server_tasks` 中 `slot_reconcile` 的 `success` 只表示对账任务完成，不等于 slot 资源层一定正常。
6. `client_run_quotas` 只是平台额度缓存，不得反向覆盖 `edge_clients.target_slot_count`。

## 15. 最终落地建议

今天数据库口径先收住到这里，后续代码实现顺序建议固定为：

1. 先按本文件建立 migrate 草案。
2. 再落 `Models / Dao / Repository`。
3. 再写 slot 对账 task 与 SSE。
4. 最后再接 run 前准入判断。

## 16. slot 对账 task + SSE 与数据库关系

这一节只回答：

- slot 对账任务为什么必须进 `server_tasks`
- SSE 事件为什么不单独落库
- slot 对账完成后到底更新哪些表

### 16.1 slot 对账任务的数据库落点

`slot_reconcile` 是中心任务，不是 slot 日志本身。

因此：

- 任务主事实落到 `server_tasks`
- slot 明细刷新落到 `edge_client_slots`
- 节点 slot 摘要刷新落到 `edge_clients`
- slot 动作和异常审计落到 `edge_client_slot_logs`

不要把这四层混成一张表。

### 16.2 为什么 `slot_reconcile` 要进 `server_tasks`

因为它已经被收口成：

- 多阶段
- 可能较长
- 需要 SSE
- 会产生明确任务结果

所以它必须具备：

- `taskId`
- `status`
- `eventsUrl`
- `error_message`
- `suggestion`

这些字段正好属于 `server_tasks`，不应再单独发明第二套 task 表。

### 16.3 `slot_reconcile` 任务建议写法

`server_tasks.type`

- `slot_reconcile`

`server_tasks.resource_type`

- `edge_client`

`server_tasks.resource_id`

- 当前 `clientId`

`server_tasks.client_id`

- 当前对账针对的 `clientId`

`server_tasks.env_id`

- 保持空字符串

### 16.4 SSE 事件为什么不落库

当前口径仍然保持：

- SSE 事件流不是中心长期事实源
- 中心长期事实只保留最终 task 结果
- 中间阶段事件只用于实时观察

因此：

- 不新增 `task_events`
- 不新增 `slot_reconcile_events`
- 不把每一条 SSE 事件写入 SQLite

如果后续确实需要长期保存关键阶段，只能另做“阶段摘要字段”或“异常摘要日志”，不要把完整事件流整套入库。

### 16.5 `slot_reconcile` 完成后的写表顺序

建议固定为：

1. `server_tasks`
   - 创建一条 `pending`
2. 拉 Client 当前 slot 明细
3. 全量刷新 `edge_client_slots`
4. 重算并刷新 `edge_clients` slot 摘要
5. 如有异常，写 `edge_client_slot_logs`
6. 回写 `server_tasks`
   - `success` 或 `failed`

注意：

- `slot_reconcile` 的 `success` 只表示对账任务完成
- 即使发现 `slot 异常`，任务本身也可以是 `success`
- 异常结论继续看 `edge_clients.slot_exception_status`

## 17. 动作对表影响矩阵

这一节专门收口：

- 哪个动作改哪几张表
- 哪些是直接修改
- 哪些只是引用
- 哪些必须删除重建

### 17.1 bind

直接修改：

- `edge_clients`
- `edge_client_bind_logs`
- `server_tasks`（如果 bind 以后任务化）

后置动作：

- bind 成功后应触发 slot 对账
- slot 对账再去建立 `edge_client_slots` 和刷新 `edge_clients` slot 摘要

### 17.2 unbind

直接修改：

- `edge_clients`
  - 删除当前有效绑定结果或标记解绑失效
- `edge_client_slots`
  - 直接物理删除当前 `client_id` 下全部关系
- `edge_client_bind_logs`
  - 写 `unbind`

不修改：

- `server_browser_envs` 历史聚合记录不在这里直接删历史
- `server_tasks` 历史任务不回滚

### 17.3 rebind

直接修改：

- `edge_clients`
  - 建立新的 `clientId`
- `edge_client_bind_logs`
  - 写新的 `bind`

后置动作：

1. 清空 Client 当前全部 slot
2. 按 `target_slot_count` 重新初始化空白 slot
3. 触发 `slot_reconcile`
4. 由 `slot_reconcile` 重建：
   - `edge_client_slots`
   - `edge_clients` slot 摘要

关键规则：

- rebind 不恢复旧 slot 关系
- rebind 后 slot 从 `slot001` 开始重新编号

### 17.4 recheck

直接修改：

- `edge_clients`
  - 刷新节点治理状态
- `edge_client_bind_logs`
  - 写 `recheck`

默认不直接修改：

- `edge_client_slots`

但如果后续规则要求“recheck 成功后自动触发 slot 对账”，那修改 `edge_client_slots` 的动作也应通过后置 `slot_reconcile` 完成，而不是在 `recheck` 内部直接改。

### 17.5 confirm-address-update

直接修改：

- `edge_clients`
  - 更新 `client_ip / base_url / docker_api_url`
- `edge_client_bind_logs`
  - 写 `confirm_address_update`

后置动作：

- 成功后应触发 `slot_reconcile`
- 用新的地址重新收口：
  - `edge_client_slots`
  - `edge_clients` slot 摘要

### 17.6 create-slot / destroy-slot / reinit-slot

这些动作属于 slot 资源治理动作。

直接修改：

- `edge_client_slot_logs`
  - 记录动作审计
- `server_tasks`
  - 如果以后任务化，要记录任务结果

动作完成后不建议直接手写中心缓存结果，而建议：

- 统一触发 `slot_reconcile`
- 由 `slot_reconcile` 重新刷新：
  - `edge_client_slots`
  - `edge_clients`

这样可以避免业务动作和中心缓存双写打架。

### 17.7 run / stop / ending

这些动作优先修改的是：

- `server_tasks`
- `server_browser_envs`

其中 slot 相关关系：

- `server_browser_envs.current_slot_id`
  - 保存 env 当前引用的 slot

如果动作成功后需要刷新 slot 明细，原则上也建议通过：

- 轻量 slot 刷新
  或
- `slot_reconcile`

来统一收口，不要在多个动作里各自手写 `edge_client_slots` 的最终事实。

## 18. 明天继续前的数据库结论

到今天为止，数据库口径已经正式收成：

1. Node Server 允许建立中心 slot 关系缓存，但不夺走 Client 的 slot 真相源地位。
2. `edge_clients` 只放节点级 slot 摘要。
3. `edge_client_slots` 只放当前绑定关系下的 slot 明细缓存。
4. `edge_client_slot_logs` 单独记录 slot 动作与异常。
5. `slot_reconcile` 必须进 `server_tasks`，但 SSE 全量事件不入库。
6. rebind 后 slot 不恢复旧关系，而走清空、重初始化、再对账重建。
