# Node Server 接口设计：`GET /api/v1/envs`

## 1. 功能目标

`GET /api/v1/envs` 用于返回当前主账号下的中心环境包聚合列表。

它服务于：

- 前端环境列表页
- 任务关联目标查询
- 管理员按节点、状态、类型查看资产分布

## 2. 数据来源

- `server_browser_envs`

## 3. 业务边界

- 中心列表展示的是聚合摘要，不是 Edge 完整环境实体
- 不主动刷新 Edge 真实状态
- 不返回任何敏感资产内容

## 4. 支持过滤

- `clientId`
- `rpaType`
- `status`
- `page`
- `pageSize`

说明：

- `rpaType` 查询支持 `tiktok/facebook/instagram/youtube/twitter` 这类别名
- 实际列表里的 `rpaType` 通常返回 Edge 归一化后的短码，例如 `tk/fb/ins/yt/x`

## 5. 请求与响应

```http
GET /api/v1/envs
```

必须带 Platform Header。

成功返回：

- `total`
- `page`
- `pageSize`
- `items`

每个环境包重点字段：

- `envId`
- `clientId`
- `rpaType`
- `name`
- `status`
- `containerStatus`
- `monitorStatus`
- `cdpUrl`
- `webVncUrl`
- `lastTaskId`
- `lastError`

## 6. 成功判定

- 能按主账号返回中心环境包列表
- 过滤条件和分页生效

## 7. 失败判定

- Platform Header 缺失
- SQLite 查询失败

## 8. 联调验收标准

- `rpaType=tiktok` 能查到 `tk` 环境包
- `status/clientId` 过滤结果正确
- 列表不返回 profile、proxy 明文、fingerprint raw、browser-data 资产
