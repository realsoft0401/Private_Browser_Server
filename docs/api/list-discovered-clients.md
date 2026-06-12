# Node Server 接口设计：`GET /api/v1/edge-clients/discovered`

## 1. 功能目标

`GET /api/v1/edge-clients/discovered` 用于展示 UDP discovery 当前收到的 Edge Client beacon，并把发现项尽可能关联到已注册节点。

它的价值是让管理员先看到“谁在广播”，再决定是否注册或排查，而不是让 UDP 命中自动进库。

## 2. 设计来源

- 用户明确要求 UDP discovery 只负责发现，不自动落库。
- 但管理员仍需要看到“现在有哪些 Client 在广播”。

## 3. 数据来源

- 当前 discovery listener 内存缓存
- 当前主账号下的 `edge_clients`

## 4. 业务边界

### 3.1 负责什么

- 展示当前发现项
- 尝试按 `baseUrl/clientIp/sourceIp` 匹配已注册节点
- 命中时回写 UDP 心跳事实

### 3.2 不负责什么

- 不自动创建节点
- 不自动 verify
- 不因为发现项出现就允许业务动作

## 5. 请求与响应

```http
GET /api/v1/edge-clients/discovered
```

必须带 Platform Header。

成功返回：

- `items`
- `total`

每个发现项重点字段：

- `clientId`
- `sourceIp`
- `sourcePort`
- `firstSeenAt`
- `lastSeenAt`
- `receiveCount`
- `payload.baseUrl`
- `payload.clientIp`
- `payload.capabilities`

## 6. 成功判定

- 能正常返回 discovery 缓存
- 已绑定发现项能补充出 `clientId`
- 命中已注册节点时可同步更新心跳事实

## 7. 失败判定

- Platform Header 缺失
- 查询已注册节点失败
- 回写命中节点的 UDP 心跳事实失败

## 8. 风险控制

- 未绑定发现项必须跳过写库
- 不能因为 IP 相似就错误归属

## 9. 联调验收标准

- 未注册 Client 只显示发现项，不生成节点
- 已注册 Client 能正确补充 `clientId`
- `payload` 只包含 discovery 摘要，不包含敏感资产数据
