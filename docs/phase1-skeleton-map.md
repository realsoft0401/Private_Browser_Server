# Private_Browser_Server 第一阶段骨架拆解

## 1. 文档目的

前面的正式定案已经完成，这份文档开始回答更落地的问题：

> 新的 `Private_Browser_Server` 第一阶段，到底应该先创建哪些文件，每个文件负责什么，哪些先做，哪些后做。

它服务的是“真正开工建新骨架”这一步。

> 文档适用范围说明：
> 这份文档只描述 Server 重建早期阶段的最小骨架，不等于商业 V1 的完整能力范围。
> 这里出现的 `push clientId`、`node-registration.json` 等内容，都是 Node 发现 Client 后完成绑定并写回留痕的配套链路，不应被理解成 Client 自主发号。

## 2. 第一阶段目标

新的 `Private_Browser_Server` 第一阶段只做节点接入主线的最小骨架，不把旧的 Env、Task、RPA、Dashboard 一起搬回来。

第一阶段目标固定为：

```text
服务能启动
  -> 能查看 /health
  -> 能查看 /swagger
  -> Node 能找到 Client
  -> Node 能 bind
  -> Node 能做身份写回
  -> Client 能留下本地留痕
```

这里的真正目标不是把“push clientId”做成长期主能力，而是：

- 先稳定节点发现、探测、绑定的最小骨架
- 为后续中心节点治理、环境聚合、任务持久化和 run admission 留出清晰扩展位

## 3. 新项目目录骨架

第一阶段建议目录如下：

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

  Dao/
    Node/
      dao.go
    Bind/
      dao.go

  Repository/
    Common/
      sqlite.go
    Node/
      repository.go
    Bind/
      repository.go

  Service/
    Health/
      health.go
    Discovery/
      memory.go
      probe.go
    EdgeClient/
      client.go
    Node/
      discovered_http.go
      list_http.go
      detail_http.go
    Bind/
      bind_http.go
      push_http.go
      service.go

  Pkg/
    HttpResponse/
      HttpResponse.go
      ResponseCode.go

  docs/
    openapi.yaml
    bind-flow-final.md
    phase1-skeleton-map.md

  public/
    swagger.html

  data/
```

## 4. 文件级职责

## 4.1 根入口

### `main.go`

职责：

- 识别项目根目录
- 调用 `Infrastructures.Init`

不负责：

- 任何业务逻辑
- 任何 bind/discovery 细节

### `README.md`

职责：

- 说明这是新的第一阶段 Node Server
- 明确当前只做节点接入主线的最小骨架

## 4.2 配置层

### `Settings/config-docker.yaml`

第一阶段至少要有：

- `server.host`
- `server.port`
- `sqlite.path`
- `discovery.enabled`
- `discovery.listen_address`
- `discovery.port`
- `discovery.magic`
- `discovery.protocol_version`
- `discovery.group`
- `edge.request_timeout_seconds`

### `Settings/settings.go`

职责：

- 读取配置
- 归一化默认值
- 暴露全局 `Settings.Conf`

第一阶段不要塞：

- JWT
- Dashboard
- Env
- RPA

## 4.3 基础设施层

### `Infrastructures/Init.go`

职责：

- 初始化配置
- 初始化 SQLite
- 启动 HTTP 服务

### `Infrastructures/SQLite/sqlite.go`

职责：

- 打开数据库
- 创建最小表结构

第一阶段建议只建两类表：

1. `edge_clients`
2. `edge_client_bind_logs`

## 4.4 路由层

### `Routes/Routes.go`

第一阶段只挂这些入口：

- `GET /`
- `GET /health`
- `GET /swagger`
- `GET /openapi.yaml`
- `GET /api/v1/edge-clients/discovered`
- `POST /api/v1/edge-clients/bind`
- `POST /api/v1/edge-clients/:clientId/push-client-id`
- `GET /api/v1/edge-clients`
- `GET /api/v1/edge-clients/:clientId`

## 4.5 模型层

### `Models/Node/node.go`

负责：

- 正式已绑定节点模型

建议字段：

```text
clientId
accountId
name
clientIp
baseUrl
dockerApiUrl
os
arch
healthStatus
pushStatus
createdAt
updatedAt
```

### `Models/Bind/bind.go`

负责：

- bind 请求体
- push 请求体
- bind 响应体
- push 响应体

## 4.6 Dao 层

### `Dao/Node/dao.go`

职责：

- 定义 `edge_clients` 的底层结构

### `Dao/Bind/dao.go`

职责：

- 定义 `edge_client_bind_logs` 的底层结构

## 4.7 Repository 层

### `Repository/Common/sqlite.go`

职责：

- 暴露统一 DB 连接

### `Repository/Node/repository.go`

职责：

- 创建已绑定节点
- 按 `clientId` 查询
- 按 `accountId` 列表查询
- 按 `clientIp` 查是否已绑定
- 更新 push 状态

### `Repository/Bind/repository.go`

职责：

- 写 bind 日志
- 写 push 日志

## 4.8 Service 层

### `Service/Health/health.go`

职责：

- 输出服务健康
- 输出 sqlite 初始化状态

### `Service/Discovery/memory.go`

职责：

- 第一阶段只保留“当前发现结果”的内存缓存
- 不落正式表

这层正好对应你已经拍板的口径：

- discovered 是过程
- 不需要正式入库

### `Service/Discovery/probe.go`

职责：

- 通过 `clientIp/baseUrl` 探测 Client
- 调 `/health`
- 调 `/api/v1/edge/device-info`
- 整理本机事实

### `Service/EdgeClient/client.go`

职责：

- Node -> Client HTTP 调用封装
- 使用 `X-Edge-API-Key`

第一阶段至少要支持：

- GET `/health`
- GET `/api/v1/edge/device-info`
- POST `/api/v1/edge/node-registration/assign`

### `Service/Node/discovered_http.go`

职责：

- 返回当前 discovered 列表

注意：

- 这里是内存发现视图
- `clientId` 允许为空

### `Service/Node/list_http.go`

职责：

- 返回正式已绑定节点列表

### `Service/Node/detail_http.go`

职责：

- 返回正式已绑定节点详情

### `Service/Bind/service.go`

职责：

- bind 主逻辑
- 根据 `accountId + clientIp` 找当前在线 Client
- 生成 `clientId`
- 写中心节点记录
- 自动 push

### `Service/Bind/bind_http.go`

职责：

- `POST /api/v1/edge-clients/bind`

### `Service/Bind/push_http.go`

职责：

- `POST /api/v1/edge-clients/:clientId/push-client-id`

## 4.9 Pkg 层

### `Pkg/HttpResponse/*`

职责：

- 统一 `code/message/data`

第一阶段推荐保留最少错误码：

- success
- invalid params
- not found
- conflict
- unauthorized
- remote error
- internal error

## 4.10 docs 层

### `docs/openapi.yaml`

第一阶段只写 bind 主线接口，不要提前把 Env/Task 全堆进去。

### `docs/bind-flow-final.md`

作为正式定案依据。

## 5. 开发顺序建议

第一阶段最适合按下面顺序做：

### 第 1 步

先做：

- `main.go`
- `Settings`
- `Infrastructures`
- `Routes`
- `Health`

验收：

- 服务能启动
- `/health` 能通

### 第 2 步

再做：

- SQLite
- `edge_clients`
- `edge_client_bind_logs`

验收：

- 表能建

### 第 3 步

再做：

- `Discovery/memory.go`
- `Discovery/probe.go`
- discovered 接口

验收：

- Node 能看到当前待绑定 Client
- discovered 返回里 `clientId=""`

### 第 4 步

再做：

- `Bind/service.go`
- bind 接口
- `clientId` 生成规则

验收：

- `accountId + clientIp` 能完成 bind
- `edge_clients` 成功入库

### 第 5 步

再做：

- `EdgeClient/client.go`
- `POST /api/v1/edge/node-registration/assign`
- 自动 push

验收：

- bind 后自动 push
- Client 本地生成 `node-registration.json`

## 6. 第一阶段明确不做

为了避免一开始又变重，这些先不做：

- Env 代理
- Task 聚合总线
- Dashboard
- Auth/JWT
- Image policy 完整策略
- 平台额度联动
- RPA/CDP
- 复杂前端页面

## 7. 最终一句话

新的 `Private_Browser_Server` 第一阶段骨架，应该只围绕：

> 能启动、能发现、能 bind、能 push、能让 Client 写本地 JSON

来搭文件，不要一开始把旧项目所有业务域重新搬进来。
