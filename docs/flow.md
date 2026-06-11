# Private_Browser_Server 流程文档

## 定位

Node Server 是节点管理服务，负责管理 Edge Client、聚合环境包摘要、发起 RPA/CDP/生命周期动作。

Node Server 不负责平台账号登录，也不直接读 Client SQLite 或环境包目录。

## Client 接入流程

```text
1. Client 通过 UDP discovery 广播本机入口。
2. Node Server 监听 UDP，写入 discovered 内存缓存。
3. 如果发现项匹配已注册 edge_clients，实时回写 last_heartbeat_at。
4. Client 也可以主动调用 `POST /api/v1/server/edge-clients/heartbeat` 上报正式心跳。
5. 前端调用 GET /api/v1/edge-clients/discovered 查看发现项。
6. 未绑定 Client 调用 POST /api/v1/edge-clients 注册。
7. 调用 POST /api/v1/edge-clients/{clientId}/device-info/refresh 刷新 Docker 设备事实。
8. 调用 POST /api/v1/edge-clients/{clientId}/verify 完成 verified。
9. 后续业务动作必须先通过 EnsureClientReadyForBusiness。
```

## 推荐 API 顺序

```text
GET  /api/v1/edge-clients/discovered
POST /api/v1/edge-clients/probe-docker
POST /api/v1/edge-clients
POST /api/v1/edge-clients/{clientId}/device-info/refresh
POST /api/v1/edge-clients/{clientId}/verify
POST /api/v1/envs
POST /api/v1/envs/{envId}/run
POST /api/v1/envs/{envId}/stop
```

## 业务放行规则

业务动作统一前置条件：

```text
healthStatus == healthy
discoveryStatus == verified
heartbeatStatus == online
arch in [amd64, arm64]
baseUrl 非空
dockerApiUrl 非空
lastError 为空
```

不满足时必须拒绝，并提示下一步修复 API。

## 当前实现状态

已实现：

- UDP discovery。
- discovered 返回 clientId。
- last_heartbeat_at 实时回写。
- `POST /api/v1/server/edge-clients/heartbeat` 正式心跳落库。
- heartbeatStatus 动态计算。
- probe-docker。
- device-info/refresh。
- verify。
- EnsureClientReadyForBusiness。

待实现：

- RPA/CDP 动作入口。
