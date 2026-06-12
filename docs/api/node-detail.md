# Node Server 接口设计：`GET /api/v1/edge-clients/{clientId}`

## 1. 功能目标

`GET /api/v1/edge-clients/{clientId}` 用于返回单个中心节点详情，帮助管理员查看该节点当前的接入事实、心跳摘要、设备能力和错误信息。

## 2. 数据来源

- `edge_clients`
- `heartbeatStatus` 动态计算

## 3. 业务边界

- 不主动 refresh
- 不主动 verify
- 不直接扫 Edge 实时状态

## 4. 请求与响应

```http
GET /api/v1/edge-clients/{clientId}
```

必须带 Platform Header。

返回重点：

- `clientId`
- `mainAccountId`
- `baseUrl`
- `dockerApiUrl`
- `arch`
- `healthStatus`
- `discoveryStatus`
- `heartbeatStatus`
- `lastCheckedAt`
- `lastError`

## 5. 成功判定

- 能按主账号 + clientId 读取到节点
- 能按当前时间动态算出 `heartbeatStatus`

## 6. 失败判定

- `clientId` 不存在
- 节点不属于当前主账号
- SQLite 读取失败

## 7. 适用场景

- 点击节点列表进入详情
- verify 失败后查看 `lastError`
- run/create env 前人工确认节点准入事实
