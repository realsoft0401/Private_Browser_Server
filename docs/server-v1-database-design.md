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

### 2.3 不建立第二套 slot 真相源

V1 不建立中心 `slots` 主表。

slot 真相继续留在 Client。

Server 只保存：

- `server_browser_envs.current_slot_id`
- `server_tasks` 动作里的 `slotId`

### 2.4 所有正式表默认支持软删除或历史保留

原因：

- 节点解绑后历史任务还要可追
- 环境包删除后中心审计还要可追
- 平台排障要看历史状态变化

但注意：

- 软删除不代表还能继续业务放行
- 业务准入只能看未删除有效记录

## 3. V1 正式表清单

建议 V1 正式保留 5 张中心表：

1. `edge_clients`
2. `edge_client_bind_logs`
3. `server_browser_envs`
4. `server_tasks`
5. `client_run_quotas`

## 4. `edge_clients`

## 4.1 表定位

`edge_clients` 是正式节点表。

这张表是：

- Client 中心身份表
- 节点绑定归属表
- 节点治理状态表

它不是：

- discovered 临时视图表
- slot 表
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
- slot 状态
- 平台额度
- Edge 文件路径
- profile / proxy / fingerprint 详情

## 4.5 索引建议

- `UNIQUE(base_url) WHERE deleted_at = 0`
- `INDEX(main_account_id, deleted_at)`
- `INDEX(health_status, discovery_status, deleted_at)`
- `INDEX(client_ip, deleted_at)`

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

## 6. `server_browser_envs`

## 6.1 表定位

`server_browser_envs` 是中心环境聚合视图表。

它是：

- 列表缓存
- 调度视图
- 前端主状态来源

它不是：

- Edge 资产真相源
- 环境包原子文件存储表

## 6.2 主键与绑定原则

- 主键固定 `env_id`
- 当前口径下，一个 env 固定绑定一台 Client
- V1 不做自动跨节点迁移

## 6.3 最小正式字段

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

## 6.4 字段解释

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

## 6.5 不应该进入这张表的内容

不要塞：

- `profile.json` 全量内容
- `binding.json` 全量内容
- 代理明文
- 指纹原文
- 浏览器登录态
- slot 完整状态机

## 6.6 索引建议

- `INDEX(main_account_id, deleted_at)`
- `INDEX(client_id, status, deleted_at)`
- `INDEX(user_id, deleted_at)`
- `INDEX(last_synced_at)`

## 7. `server_tasks`

## 7.1 表定位

`server_tasks` 是平台级持久任务表。

它负责：

- 保存中心 taskId
- 绑定 `edgeTaskId`
- 保存最终结果
- 保存错误与建议

它不是：

- Edge task 全量镜像表
- SSE 事件明细全量仓库

## 7.2 最小正式字段

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

## 7.3 字段解释

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

- `edge_task_id`
  - Edge 返回的任务 id

- `events_url`
  - Edge 事件订阅入口摘要

- `error_message`
- `suggestion`
  - 管理员排障字段

## 7.4 不应该进入这张表的内容

不要塞：

- Edge SSE 全量长日志
- proxy 明文
- fingerprint raw
- browser-data 路径详情

## 7.5 索引建议

- `INDEX(main_account_id, created_at)`
- `INDEX(env_id, created_at)`
- `INDEX(client_id, created_at)`
- `INDEX(status, updated_at)`

## 8. `client_run_quotas`

## 8.1 表定位

这张表是平台额度快照表。

它是：

- 最近一次可信平台额度缓存
- run 准入参考快照
- 管理员排障依据

它不是：

- 平台真相源
- 最终商业授权源

## 8.2 最小正式字段

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

## 8.3 字段解释

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

## 8.4 使用原则

- run 时优先实时查平台
- 平台查成功，刷新这张表
- 平台不可达、额度不存在、额度为 `0`、已过期或不可信时，V1 默认拒绝放行

## 9. 明确不建的表

V1 明确不建：

- `server_slots`
- `discovered_clients`
- `edge_task_events`

原因：

- `slot` 真相留在 Client
- discovered 只是过程视图
- 事件流不是中心长期事实源

## 10. 一句话收口

Server V1 SQLite 的正确定位是：

- 用最少的正式中心表，收住节点、环境、任务、额度和绑定审计
- 不把 Edge 本地资产、slot 真相和敏感材料搬进中心库

这样后面扩功能，数据库不会先变成第二套 Edge 系统。
