# Node Server 接口设计：`GET /api/v1/edge-clients`

## 1. 功能目标

`GET /api/v1/edge-clients` 用于返回当前主账号下的中心节点列表，并动态补充 `heartbeatStatus`。

它服务于：

- 管理端节点列表页
- 创建 env 前选择目标节点
- 管理员判断哪些节点已经达到 `healthy + verified + online`

## 2. 设计来源

- 用户明确要求节点接入、验证和业务放行必须分层表达，不能把“注册过”直接当成“可用”。
- 因此列表接口必须把 `healthStatus`、`discoveryStatus`、`heartbeatStatus` 同时给出来，方便前端和管理员按统一口径判断。

## 3. 数据来源

- `edge_clients`
- `heartbeatStatus` 基于 `last_heartbeat_at` 动态计算，不单独落库

## 4. 业务边界

- 只读中心缓存
- 不主动探测 Client
- 不触发 verify
- 不刷新 Docker 事实

## 5. 请求与响应

```http
GET /api/v1/edge-clients
```

必须带 Platform Header。

成功返回：

- `items`
- `total`

每个节点重点字段：

- `clientId`
- `name`
- `baseUrl`
- `dockerApiUrl`
- `arch`
- `healthStatus`
- `discoveryStatus`
- `heartbeatStatus`
- `lastError`

## 6. 成功判定

- 能正确按主账号返回中心节点列表
- 每个节点都动态补充了当前 `heartbeatStatus`

## 7. 失败判定

- Platform Header 缺失
- SQLite 查询失败

## 8. 联调验收标准

- 同一主账号下只能看到自己的节点
- `heartbeatStatus` 会随 `lastHeartbeatAt` 变化而变化
- 列表接口不会因为某个 Client 暂时不可达而整页失败
