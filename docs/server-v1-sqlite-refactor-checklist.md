# Private_Browser_Server V1 SQLite 与数据层改造清单

## 1. 文档目标

这份文档只服务一件事：

- 把已经定案的数据库设计，翻译成 `Private_Browser_Server` 当前代码层面的具体改造清单。

它回答的是：

- 现在缺哪些表
- 现有表哪些字段不够
- `Models / Dao / Repository / SQLite migrate` 应该各改什么

## 2. 当前现状

当前新 Server 代码里只有最小第一阶段骨架：

- `Models/Node`
- `Models/Bind`
- `Dao/Node`
- `Dao/Bind`
- `Repository/Node`
- `Repository/Bind`
- SQLite 只建：
  - `edge_clients`
  - `edge_client_bind_logs`

这和现在已经定案的 V1 正式数据库结构相比，还缺：

- `server_browser_envs`
- `server_tasks`
- `client_run_quotas`

同时，当前 `edge_clients` 和 `edge_client_bind_logs` 也还不完整。

## 3. 当前 SQLite migrate 缺口

当前 [sqlite.go](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/Infrastructures/SQLite/sqlite.go) 的 `migrate()` 只建了：

- 极简 `edge_clients`
- 极简 `edge_client_bind_logs`

### 3.1 `edge_clients` 当前缺失字段

当前只有：

- `client_id`
- `account_id`
- `name`
- `client_ip`
- `base_url`
- `docker_api_url`
- `os`
- `arch`
- `health_status`
- `push_status`
- `api_key_hash`
- `created_at`
- `updated_at`

按正式定案还缺：

- `main_account_id`
- `client_sequence`
- `cpu_cores`
- `memory_total_mb`
- `docker_version`
- `discovery_status`
- `discovery_reason`
- `last_discovered_at`
- `last_heartbeat_at`
- `last_heartbeat_reported_at`
- `last_heartbeat_source`
- `last_checked_at`
- `last_error`
- `created_by_user_id`
- `created_by_username`
- `deleted_at`

### 3.2 `edge_client_bind_logs` 当前缺失字段

当前只有：

- `client_id`
- `account_id`
- `client_ip`
- `action`
- `result`
- `message`
- `created_at`

按正式定案建议补：

- `main_account_id`
- `operator_user_id`
- `operator_username`

如果短期不补操作者字段，后面至少要留出迁移位。

### 3.3 当前完全缺失的正式表

必须新增：

- `server_browser_envs`
- `server_tasks`
- `client_run_quotas`

## 4. `Models` 层改造清单

## 4.1 `Models/Node/node.go`

当前模型仍是 bind 第一阶段视角，字段不够完整。

需要补到正式节点模型：

- `MainAccountID`
- `ClientSequence`
- `CPUCores`
- `MemoryTotalMB`
- `DockerVersion`
- `DiscoveryStatus`
- `DiscoveryReason`
- `LastDiscoveredAt`
- `LastHeartbeatAt`
- `LastHeartbeatReportedAt`
- `LastHeartbeatSource`
- `LastCheckedAt`
- `LastError`
- `CreatedByUserID`
- `CreatedByUsername`
- `DeletedAt`

同时建议统一：

- `AccountID` 改成 `MainAccountID`

避免后面节点表和任务表的账号语义不一致。

## 4.2 新增 `Models/BrowserEnv/browser_env.go`

新增中心环境聚合模型。

至少需要：

- `EnvID`
- `MainAccountID`
- `ClientID`
- `UserID`
- `RPAType`
- `Name`
- `Status`
- `ContainerStatus`
- `RuntimeStatus`
- `CurrentSlotID`
- `CDPURL`
- `WebVNCURL`
- `LastTaskID`
- `LastError`
- `LastSyncedAt`
- `CreatedAt`
- `UpdatedAt`
- `DeletedAt`

## 4.3 新增 `Models/Task/task.go`

新增平台级持久任务模型。

至少需要：

- `ID`
- `MainAccountID`
- `OperatorUserID`
- `OperatorUsername`
- `ClientID`
- `EnvID`
- `Type`
- `ResourceType`
- `ResourceID`
- `Status`
- `EdgeTaskID`
- `EventsURL`
- `ErrorMessage`
- `Suggestion`
- `CreatedAt`
- `UpdatedAt`
- `FinishedAt`

## 4.4 新增 `Models/Quota/quota.go`

新增额度快照模型。

至少需要：

- `ClientID`
- `QuotaLimit`
- `QuotaUsedSnapshot`
- `QuotaAvailableSnapshot`
- `FetchedAt`
- `ExpiresAt`
- `Status`
- `LastError`

## 5. `Dao` 层改造清单

## 5.1 `Dao/Node/dao.go`

当前 Row 字段远少于正式节点表。

需要补齐到和 `edge_clients` 正式字段一致。

同时建议收口：

- `AccountID` 改成 `MainAccountID`

## 5.2 `Dao/Bind/dao.go`

需要补齐：

- `MainAccountID`
- `OperatorUserID`
- `OperatorUsername`

## 5.3 新增 `Dao/BrowserEnv/dao.go`

和 `server_browser_envs` 一一对应。

## 5.4 新增 `Dao/Task/dao.go`

和 `server_tasks` 一一对应。

## 5.5 新增 `Dao/Quota/dao.go`

和 `client_run_quotas` 一一对应。

## 6. `Repository` 层改造清单

## 6.1 `Repository/Node/repository.go`

当前能力还只够：

- create
- get by client id
- list by account id
- update push status

后续至少要补：

- `GetByBaseURL`
- `GetByClientIP`
- `AllocateNextSequence`
- `UpdateDiscoveryObservation`
- `UpdateHeartbeat`
- `UpdateProbeResult`
- `SoftDelete`
- `UpdateAddress`

### 关键改造点

当前 `generateClientID()` 还在 `Service/Bind/service.go` 里按 `len(nodes)+1` 计算。

后续应迁到 `Repository/Node`：

- 提供 `AllocateNextSequence(mainAccountID)` 或等价方法
- 内部按 `MAX(client_sequence)+1` 分配

## 6.2 `Repository/Bind/repository.go`

需要补支持：

- 写 `bind`
- 写 `push_client_id`
- 写 `unbind`
- 写 `clear_registration`

并允许记录操作者摘要。

## 6.3 新增 `Repository/BrowserEnv/repository.go`

至少要支持：

- `Create`
- `Upsert`
- `GetByEnvID`
- `ListByMainAccountID`
- `ListByClientID`
- `UpdateRuntimeSummary`
- `UpdateLastTask`
- `UpdateLastError`
- `SoftDelete`

## 6.4 新增 `Repository/Task/repository.go`

至少要支持：

- `Create`
- `GetByID`
- `List`
- `UpdateRunning`
- `UpdateSuccess`
- `UpdateFailed`

### 关键改造点

必须从一开始就按正式终态写：

- `success`
- `failed`

不要把旧的 `canceled` 再带进来。

## 6.5 新增 `Repository/Quota/repository.go`

至少要支持：

- `UpsertSnapshot`
- `GetByClientID`
- `MarkExpired`
- `UpdateLastError`

## 7. `Infrastructures/SQLite/sqlite.go` 改造顺序

建议不要直接大改一坨 SQL，而是按下面顺序推进。

### 第一步：补齐 `edge_clients`

把现有 `edge_clients` 扩成正式字段。

注意：

- 旧列名 `account_id` 如果保留，要么迁移成 `main_account_id`
- 要么统一兼容映射

更推荐：

- 直接收口成 `main_account_id`

### 第二步：补齐 `edge_client_bind_logs`

先把操作者字段补上。

### 第三步：新增 `server_browser_envs`

这是 Server 从“bind 主线”进入“正式中心业务层”的第一张关键表。

### 第四步：新增 `server_tasks`

这是正式动作编排必须依赖的表。

### 第五步：新增 `client_run_quotas`

这是后续 run admission 的本地额度快照表。

## 8. 路由与服务层暂时不要做的事

在这份清单阶段，还不要急着：

- 直接开写 `BrowserEnv` 所有 HTTP 接口
- 直接开写 `Task` 所有 HTTP 接口
- 直接接平台额度接口

先把：

- SQLite 表
- Model
- Dao
- Repository

的边界打稳。

## 9. 推荐落地顺序

建议按下面顺序做：

1. 改 `edge_clients` 正式字段
2. 改 `Models/Node`、`Dao/Node`、`Repository/Node`
3. 改 `edge_client_bind_logs`
4. 新增 `Models/BrowserEnv`、`Dao/BrowserEnv`、`Repository/BrowserEnv`
5. 新增 `Models/Task`、`Dao/Task`、`Repository/Task`
6. 新增 `Models/Quota`、`Dao/Quota`、`Repository/Quota`
7. 最后再进入 `Service/BrowserEnv`、`Service/Task`、`Service/Admission`

## 10. 一句话收口

当前新 Server 的问题不是“没有结构”，而是“数据层还停在 bind 第一阶段”。

所以现在最正确的动作，不是立刻扩很多 API，而是先把 SQLite、Model、Dao、Repository 升级到 V1 正式中心层结构。
