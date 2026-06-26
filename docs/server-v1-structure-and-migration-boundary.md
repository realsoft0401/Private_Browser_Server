# Private_Browser_Server V1 项目结构与 old 迁移边界

## 1. 文档目标

这份文档只回答 3 件事：

1. 新的 `Private_Browser_Server` V1 到底应该长成什么结构。
2. 当前新 Server 已有骨架里，哪些应该保留继续扩。
3. `Private_Browser_Server_Old` 里哪些能力可以迁，哪些必须重写，哪些明确不要再带回来。

这份文档服务的是“正式启动 Server 新项目”的设计收口，不直接讨论具体代码实现细节。

> 文档适用范围说明：
> 这份文档描述的是 Server V1 正式目标结构，但其中提到的 bind / push clientId 仍包含当前“Node 发现 Client -> bind -> 写回 Client 本地 JSON”的配套链路。
> 后续如与根目录 `project.md`、根目录 `AGENTS.md` 的最新总口径冲突，以总口径为准。

## 2. 当前判断

先把当前现状说清楚：

- `Private_Browser_Server` 现在已经不是空目录。
- 它已经有一套第一阶段骨架，核心是：
  - `/health`
  - `/swagger`
  - UDP discovery listener
  - discovered 视图
  - bind
  - push clientId 写回
  - 已绑定节点列表 / 详情
- 这套骨架的目录层次已经和 `Private_Browser_Client` 基本一致，这点是对的，应该保留。

但它现在还只是：

```text
Node 先找到 Client
  -> probe
  -> bind
  -> 写回 Client 本地 node-registration.json
```

还没有真正进入你现在要的中心服务阶段：

- 中心节点治理
- 环境包聚合
- Server task 持久化
- run 准入
- 平台额度收口
- 统一业务入口

所以这次不是“推翻重来”，而是：

- 保留当前新 Server 的工程层次
- 把业务域从 bind 第一阶段扩成正式中心层

## 3. 新 Server 的正式定位

`Private_Browser_Server` 后续正式定位必须固定为：

```text
中心调度服务
  -> 管理 Edge Client 节点
  -> 聚合 browser-env 资产视图
  -> 持久化平台级任务事实
  -> 作为前端和平台的统一入口
```

它负责：

- 节点发现、登记、绑定、解绑、验证、健康收口。
- 通过 Edge API 调用 Client 能力。
- 聚合 Edge 的 browser-env 状态。
- 持久化平台级任务事实。
- 执行 run / stop / backup / restore / delete / import-package 的中心编排。
- 收口 `clientId + health_status + discovery_status + 平台额度` 这一套准入逻辑。

它不负责：

- 直接读 Edge SQLite。
- 直接读 Edge 环境包目录。
- 直接读 `browser-data/profile`。
- 通过 SSH 绕过 Client API 操作环境包。
- 在中心侧伪造 Edge 本地状态。

## 4. 推荐正式目录结构

建议新的 Server V1 正式收口成下面这个结构：

```text
Private_Browser_Server/
  main.go
  README.md
  project.md
  agent.md
  go.mod
  go.sum

  Settings/
    config-docker.yaml
    settings.go

  Infrastructures/
    Init.go
    SQLite/
      sqlite.go

  Routes/
    Routes.go

  Models/
    Node/
      node.go
    Bind/
      bind.go
    BrowserEnv/
      browser_env.go
    Task/
      task.go
    Discovery/
      discovery.go

  Dao/
    Node/
      dao.go
    Bind/
      dao.go
    BrowserEnv/
      dao.go
    Task/
      dao.go

  Repository/
    Common/
      sqlite.go
    Node/
      repository.go
      discovery_observation.go
    Bind/
      repository.go
    BrowserEnv/
      repository.go
    Task/
      repository.go

  Service/
    Health/
      health.go
    Discovery/
      listener.go
      memory.go
      probe.go
    EdgeClient/
      client.go
    Node/
      discovered_http.go
      list_http.go
      detail_http.go
      verify_http.go
      unbind_http.go
    Bind/
      bind_http.go
      push_http.go
      service.go
    BrowserEnv/
      create_http.go
      list_http.go
      detail_http.go
      run_http.go
      stop_http.go
      backup_http.go
      restore_http.go
      revalidate_http.go
      import_http.go
      service.go
    Task/
      detail_http.go
      list_http.go
      service.go
    Admission/
      run_admission.go
    Platform/
      quota_client.go

  Pkg/
    HttpResponse/
      HttpResponse.go
      ResponseCode.go

  docs/
    openapi.yaml
    api/
    bind-flow-final.md
    server-v1-structure-and-migration-boundary.md

  public/
    swagger.html

  data/
```

## 5. 各层职责边界

### 5.1 `Node`

`Node` 域只负责节点本身。

它关心的是：

- `clientId`
- `mainAccountId`
- `baseUrl/clientIp`
- `healthStatus`
- `discoveryStatus`
- `identityChanged/ipMismatch`
- 节点是否可调度

它不负责：

- browser-env 业务编排
- Server task 收口
- 平台额度

### 5.2 `Bind`

`Bind` 是节点归属关系服务，不是发现服务，也不是环境包服务。

它只负责：

- bind
- push clientId 写回
- unbind
- bind / push 审计日志

它不负责：

- discovery 缓存
- run 准入
- env 生命周期

补充收口：

- `push clientId` 是 Node 发现 Client、完成 bind 后的写回链路，不是 Client 自注册
- 它不是发现链路，也不是业务放行链路
- 正式长期口径里，节点身份真相、节点归属和业务放行判断始终只以 Server 中心记录为准

### 5.3 `Discovery`

`Discovery` 只负责发现线索和探测。

它只负责：

- UDP beacon 监听
- 平台字段校验
- `/health` / `/api/v1/edge/device-info` 探测
- discovered 内存视图
- 已登记节点的心跳回写

它不负责：

- 自动生成 `clientId`
- 自动绑定账号
- 自动放行业务动作

### 5.4 `BrowserEnv`

`BrowserEnv` 是正式业务资产域。

它负责：

- 调用 Edge 的 `browser-envs/*`
- 保存 `server_browser_envs`
- 给前端返回中心聚合视图
- 在动作前后同步 Edge 当前事实

它不负责：

- 直接读 Edge 本地目录
- 直接读 Edge SQLite
- 替代 Edge 成为资产真相源

### 5.5 `Task`

`Task` 是 Server 侧持久化任务域。

它负责：

- 保存平台级任务
- 绑定 `edgeTaskId`
- 收口最终 `success/failed`
- 保存错误、建议、审计字段

它不负责：

- 成为 Edge 本地 task 的镜像数据库
- 代替 `BrowserEnv` 决定资产事实

### 5.6 `Admission`

`Admission` 专门负责 run 准入。

它负责：

- 校验节点已有 `clientId`
- 校验 `health_status=healthy`
- 校验 `discovery_status=verified`
- 校验平台额度
- 校验当前 `running` 占用数

它不负责：

- 真正发送 run 请求
- 真正维护平台额度来源

### 5.7 `Platform`

`Platform` 只负责中心外部依赖，例如平台额度接口。

V1 可以先保留空骨架或 mock 接口，但边界要先立住。

## 6. 数据表正式边界

Server V1 至少应明确保留 4 类中心表：

### 6.1 `edge_clients`

正式节点表。

这是节点中心身份和节点治理真相源。

负责保存：

- `clientId`
- `mainAccountId`
- `baseUrl`
- `clientIp`
- `os/arch/docker`
- `healthStatus`
- `discoveryStatus`
- `discoveryReason`
- `lastHeartbeatAt`
- `lastCheckedAt`

不负责保存：

- browser-env 原子材料
- Edge 本地目录路径
- 登录态

### 6.2 `edge_client_bind_logs`

绑定和下发留痕表。

负责保存：

- bind 请求
- push clientId 结果
- 覆盖下发
- unbind

### 6.3 `server_browser_envs`

中心聚合环境表。

这是中心缓存和调度视图，不是 Edge 资产真相源。

负责保存：

- `envId`
- `clientId`
- `mainAccountId`
- `userId`
- `rpaType`
- `status`
- `containerStatus`
- `runtimeProtection`
- `lastError`
- `lastSyncedAt`

### 6.4 `server_tasks`

平台级任务表。

负责保存：

- `taskId`
- `clientId`
- `envId`
- `edgeTaskId`
- `type`
- `status`
- `errorMessage`
- `suggestion`
- `createdAt/updatedAt/finishedAt`

## 7. old 里哪些能力可以直接迁

下面这些是 old 里已经证明方向正确、可以作为新 Server 基础的部分。

### 7.1 可以直接继承目录层次和工程组织

- `Settings`
- `Infrastructures`
- `Routes`
- `Models`
- `Dao`
- `Repository`
- `Service`
- `Pkg/HttpResponse`
- `docs/openapi.yaml`
- `public/swagger.html`

理由：

- 它已经和 Client 形成一致层次。
- 后续双项目协作、排障、接口文档维护都会更顺。

### 7.2 可以直接继承 `EdgeClient` 统一 HTTP 客户端思想

可迁移：

- 统一超时
- 统一 Header 注入
- 统一错误映射
- 统一 JSON / multipart 调用

但要保持原则：

- 不自动重试
- 不自动补偿
- 不静默吞掉 Edge 错误

### 7.3 可以直接继承 UDP discovery listener 基础思路

可迁移：

- `listener.go`
- `memory.go`
- `probe.go`
- 报文字段校验

但 discovered 只保留为：

- 发现视图
- 短期内存事实
- 节点接入前线索

discovery 的顺序必须固定为：

```text
Client 先广播 UDP
  -> Node 发现
  -> Node probe
  -> Node bind
  -> Client 留痕
```

不要把 discovered 重新做成正式业务实体。

### 7.4 可以直接继承 `edge_clients` 作为正式节点表

这张表方向是对的。

它应该继续保留为：

- 节点中心身份表
- 节点状态收口表
- 绑定归属表

### 7.5 可以直接继承 bind / push 主线

当前新 Server 已经有：

- bind
- push clientId
- 节点列表
- 节点详情

这条主线不应推翻，应作为正式中心层的第一块基础。

但它的顺序不能反过来理解成“先写回 Client，再让 Node 去发现”。

## 8. old 里哪些能力必须“迁思想，不直接搬代码”

这部分最关键。

### 8.1 `server_browser_envs`

这张表必须保留思想，但不能原样机械搬。

需要重写的原因：

- 现在 Client 的正式生命周期已经收口到 `browser-envs/*`
- 中心侧必须跟 Client 新口径一致
- 旧的字段、旧的状态枚举、旧的接口路径不一定还能成立

所以：

- 保留“中心聚合 env 视图”思想
- 按当前 Client 的正式状态字段重新设计

### 8.2 `server_tasks`

任务表思想必须保留，但收口规则要按现在新的要求重写。

必须按新规则收口：

- 终态只有 `success/failed`
- Edge task 丢失时不能默认成功
- 必须回读 Edge 当前事实再判定

### 8.3 run 准入

old 里的 run admission 文档和思路可以保留，但不能只迁成注释。

需要单独形成新域：

- `Admission/run_admission.go`

因为这条链已经不是普通参数校验，而是正式业务闸门。

### 8.4 节点健康与发现状态

old 中一些状态枚举已经发生过多轮收口。

新的做法应该是：

- 保留 `health_status`
- 保留 `discovery_status`
- 原因放进 `discovery_reason`
- 不再无限增加状态枚举

## 9. old 里哪些能力明确不要再带回来

### 9.1 不带回“绕过 clientId 直接按 baseUrl 做业务调用”

正式业务调用必须围绕：

- `clientId`
- `health_status`
- `discovery_status`

不能退回：

- 前端给一个 `baseUrl`
- Server 直接透传去调 Edge

### 9.2 不带回“直接读 Edge 本地文件”

明确禁止：

- 读 Edge SQLite
- 读 `data/browser-envs`
- 读备份包
- 读 `browser-data/profile`

### 9.3 不带回“把 discovered 当正式已绑定节点”

discovered 只是发现态，不是正式节点。

不能因为：

- 收到 UDP
- `/health` 通了

就默认已经是正式可调度节点。

### 9.4 不带回“任务成功 = Edge 接单成功”

不能把：

- Edge 返回 `taskId`

直接当成：

- Server task 成功

### 9.5 不带回“旧 API 名字和旧路径包袱”

新的正式中心层应该统一围绕：

- `edge-clients/*`
- `browser-envs/*`
- `tasks/*`

不要把历史混乱命名继续往后带。

## 10. 新 Server 的推荐迁移顺序

建议按下面顺序推进，不要同时铺太多域。

### 第一步：保留并稳定现有 Node / Bind / Discovery 骨架

先确保下面这块真正稳定：

- discovery listener
- discovered 视图
- bind
- push clientId
- list/detail
- `/health`
- `/swagger`

### 第二步：补齐正式中心表边界

至少把下面 4 张表定下来：

- `edge_clients`
- `edge_client_bind_logs`
- `server_browser_envs`
- `server_tasks`

### 第三步：补 `BrowserEnv + Task` 域

这是 Server 真正从“节点接入服务”进入“中心调度服务”的分水岭。

优先补：

- env 列表
- env 详情
- run
- stop
- task detail

### 第四步：补 `Admission + Platform` 域

把下面这条链正式做成中心闸门：

```text
clientId
  -> healthy
  -> verified
  -> 平台额度
  -> 当前 running 占用
  -> 才允许 run
```

### 第五步：再扩 backup / restore / revalidate / import-package

这些动作一定要在 `server_tasks` 和 `server_browser_envs` 事实源建立后再接。

## 11. 一句话收口

新的 `Private_Browser_Server` 不是重写一个空项目，也不是把 old 整包搬回来。

正确做法是：

- 保留当前新 Server 已经对的工程层次和 bind/discovery 基础
- 从 old 迁“中心层思想、表边界、EdgeClient 能力、任务与节点治理经验”
- 不再迁回旧路径、旧混用边界和任何绕过 Client API 的做法

这样后面你再开真正的 Server 开发，项目边界会很稳。
