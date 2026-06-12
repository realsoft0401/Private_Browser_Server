# Node Server 接口设计：`POST /api/v1/server/edge-clients/heartbeat`

## 1. 功能目标

`POST /api/v1/server/edge-clients/heartbeat` 用于接收 `Private_Browser_Client` 主动上报的正式心跳，并把该心跳沉淀为中心节点事实。

成功后的关键结果是：

- `last_heartbeat_at` 记录为 Node Server 实际收到请求的时间
- `last_heartbeat_reported_at` 记录 Client 自报时间
- `heartbeatStatus` 后续可以基于 `last_heartbeat_at` 动态计算为 `online/stale/offline`

## 2. 设计来源

- 之前只有 UDP discovery 被动回写时，`last_heartbeat_at` 很容易混入“广播命中时间”和“业务心跳时间”。
- 用户明确要求 `last_heartbeat_at` 应表达更可信的中心接收时间，而 Client 自报时钟要保留但不能直接作为业务放行依据。

## 3. 业务边界

### 3.1 负责什么

- 校验 discovery 域识别字段
- 通过 `baseUrl/clientIp/sourceIp` 查找已注册节点
- 更新正式心跳事实
- 返回当前节点的心跳摘要

### 3.2 不负责什么

- 不依赖 Platform Header
- 不自动创建节点
- 不自动 verify
- 不自动修改 `healthStatus/discoveryStatus`
- 不接收任何敏感资产数据

## 4. 请求与响应

```http
POST /api/v1/server/edge-clients/heartbeat
```

请求体最小字段：

- `discoveryMagic`
- `protocolVersion`
- `service`
- `discoveryGroup`
- `baseUrl`
- `clientIp`
- `lastHeartbeatAt`

成功返回：

- `clientId`
- `mainAccountId`
- `baseUrl`
- `clientIp`
- `lastHeartbeatAt`
- `lastHeartbeatReportedAt`
- `heartbeatStatus`
- `updatedAt`

## 5. 前置校验

必须通过：

1. `discoveryMagic` 与当前发现域一致
2. `protocolVersion` 一致
3. `service` 必须是 `Private_Browser_Client`
4. `discoveryGroup` 一致
5. `baseUrl` 与 `clientIp` 不能同时为空
6. 必须能匹配到已注册节点

## 6. 成功判定

满足以下条件视为成功：

- 找到目标节点
- 正式心跳写库成功

## 7. 失败判定

- discovery 域字段不匹配
- `baseUrl/clientIp` 缺失且无法匹配
- 目标节点不存在
- 写库失败

## 8. 错误与日志规范

至少应记录：

- `baseUrl`
- `clientIp`
- `sourceIp`
- `discoveryGroup`
- `error`
- `suggestion`

常见建议文案：

- `找不到已注册的 Edge Client，请先通过注册或 verify 流程绑定 baseUrl/clientIp`

## 9. 相关接口

- [list-discovered-clients.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/list-discovered-clients.md)
- [verify-node.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/verify-node.md)
