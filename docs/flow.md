# Private_Browser_Server 流程文档

正式节点治理设计以 [node-governance.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/node-governance.md) 为准。
本文保留为接入、恢复和业务放行流程速查表。

## 定位

Node Server 是节点管理服务，负责管理 Edge Client、聚合环境包摘要、发起 RPA/CDP/生命周期动作。

Node Server 不负责平台账号登录，也不直接读 Client SQLite 或环境包目录。

## Client 接入流程

```text
1. Client 通过 UDP discovery 广播本机入口。
2. Node Server 监听 UDP，写入 discovered 内存缓存。
3. 如果发现项匹配已注册 edge_clients，实时回写 last_heartbeat_at / last_discovered_at。
   如果发现到的地址与已登记节点明显不一致，则应进入 blocked/ip_mismatch，而不是自动覆盖原节点地址。
4. Client 也可以主动调用 `POST /api/v1/server/edge-clients/heartbeat` 上报正式心跳。
5. 前端调用 GET /api/v1/edge-clients/discovered 查看发现项。
6. 未绑定 Client 调用 POST /api/v1/edge-clients 注册。
7. 调用 POST /api/v1/edge-clients/{clientId}/device-info/refresh 刷新 Docker 设备事实。
8. 调用 POST /api/v1/edge-clients/{clientId}/verify 完成 verified。
9. 后续业务动作必须先通过 EnsureClientReadyForBusiness。
```

如果节点在运行中被发现地址变化，则治理顺序改为：

```text
1. heartbeat/discovery 把节点标成 blocked + ip_mismatch。
2. 列表/详情页不再允许继续 verify。
3. 管理员确认“这还是原节点，只是地址变了”。
4. 调用 POST /api/v1/edge-clients/{clientId}/confirm-address-update。
5. Server 更新 baseUrl/clientIp/dockerApiUrl，并立即重跑完整探测。
6. 全部通过后恢复 verified；如果设备事实变化过大，则转为 blocked + device_fact_changed。
```

## 推荐 API 顺序

```text
GET  /api/v1/edge-clients/discovered
POST /api/v1/edge-clients/probe-docker
POST /api/v1/edge-clients
POST /api/v1/edge-clients/{clientId}/device-info/refresh
POST /api/v1/edge-clients/{clientId}/verify
POST /api/v1/edge-clients/{clientId}/confirm-address-update
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
- discovered 对已登记节点会补 clientId；未绑定发现项返回 `clientId=""`。
- last_heartbeat_at 实时回写。
- last_discovered_at 实时回写。
- `POST /api/v1/server/edge-clients/heartbeat` 正式心跳落库。
- heartbeatStatus 动态计算。
- `ip_mismatch -> blocked` 最小链路已落地。
- probe-docker。
- device-info/refresh。
- verify。
- EnsureClientReadyForBusiness。

待实现：

- RPA/CDP 动作入口。
