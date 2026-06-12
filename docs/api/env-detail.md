# Node Server 接口设计：`GET /api/v1/envs/{envId}`

## 1. 功能目标

`GET /api/v1/envs/{envId}` 用于返回单个中心环境包聚合详情，让管理员或平台前端查看当前中心已确认的状态与连接摘要。

## 2. 数据来源

- `server_browser_envs`

## 3. 业务边界

- 当前第一版只返回 SQLite 聚合事实
- 不主动刷新 Edge detail
- 不返回 profile 明细、proxy 明文、fingerprint raw、browser-data 资产

## 4. 请求与响应

```http
GET /api/v1/envs/{envId}
```

必须带 Platform Header。

返回重点字段：

- `envId`
- `clientId`
- `rpaType`
- `status`
- `containerStatus`
- `monitorStatus`
- `cdpUrl`
- `webVncUrl`
- `lastTaskId`
- `lastError`

## 5. 成功判定

- 能按主账号 + envId 读取到中心环境包缓存

## 6. 失败判定

- `envId` 不存在
- 环境包不属于当前主账号
- SQLite 读取失败

## 7. 适用场景

- 列表页进入单 env
- 生命周期动作前确认当前中心状态
- 排查 `lastTaskId/lastError`
