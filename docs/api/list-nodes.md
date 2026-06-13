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
- `discoveryReason`
- `primaryAction`
- `allowedActions`
- `heartbeatStatus`
- `lastError`

## 6. 状态解释口径

列表页必须把节点当前所处阶段解释清楚，避免把“在线”误读成“可用”。

`discoveryStatus`：

- `blocked`
  表示节点当前不允许业务放行；还需要继续看 `discoveryReason`。
- `verified`
  表示中心已确认这是登记过的那台节点。

`discoveryReason`：

- `""`
  表示当前只是未 verify，或者暂时没有额外身份异常原因。
- `ip_mismatch` / `device_fact_changed`
  表示节点记录还在，但 IP、baseUrl 或设备事实变化过大，必须人工确认。

当前实现补充说明：

- discovery 列表里的 `clientId` 是否为空，取决于它能否匹配到 `edge_clients` 里的正式节点记录；
- 如果正式节点记录被删除，同一 discovery 项会恢复为未绑定状态；
- 如果正式节点记录的 `client_ip` 为空，`ip_mismatch` 判断会更依赖 `baseUrl` 这条地址事实。

`healthStatus`：

- `healthy`
  表示 Client 本机检查、device-info 和 Docker 2375 当前健康。
- `unhealthy`
  表示节点可达，但本机关键能力异常。
- `stale`
  表示中心缓存不可信或动作前校验失败。
- `offline`
  表示中心确认节点不可达。

`heartbeatStatus`：

- `online`
  表示最近心跳足够新鲜。
- `stale`
  表示最近心跳已过期，但还没达到 offline。
- `offline`
  表示长时间未收到有效心跳。

列表页操作口径建议直接按下面映射：

- `discoveryStatus=blocked` 且 `discoveryReason=""`
  只显示 `verify` 与 `refresh device-info`
- `discoveryStatus=blocked` 且 `discoveryReason=ip_mismatch`
  只显示 `confirm-address-update`
- `discoveryStatus=blocked` 且 `discoveryReason=device_fact_changed`
  只显示“查看详情/人工排查”，不要继续给 `verify` 或 `confirm-address-update`
- `discoveryStatus=verified` 且 `healthStatus=healthy` 且 `heartbeatStatus=online`
  才允许进入 create env 和后续生命周期动作

如果前端不想自己重复判断，也可以直接使用接口返回的：

- `primaryAction`
- `allowedActions`

作为按钮渲染依据。

业务上必须同时满足：

- `discoveryStatus=verified`
- `healthStatus=healthy`
- `heartbeatStatus=online`

才允许创建环境包和执行生命周期动作。

## 7. 成功判定

- 能正确按主账号返回中心节点列表
- 每个节点都动态补充了当前 `heartbeatStatus`

## 8. 失败判定

- Platform Header 缺失
- SQLite 查询失败

## 9. 联调验收标准

- 同一主账号下只能看到自己的节点
- `heartbeatStatus` 会随 `lastHeartbeatAt` 变化而变化
- 列表接口不会因为某个 Client 暂时不可达而整页失败
