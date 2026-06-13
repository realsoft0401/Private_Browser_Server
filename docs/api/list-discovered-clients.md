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

如果发现项和已注册节点存在地址或设备事实冲突，这个接口后续也应该成为管理员发现
`blocked + ip_mismatch` 的主要入口之一。

### 3.2 不负责什么

- 不自动创建节点
- 不自动 verify
- 不因为发现项出现就允许业务动作
- 不自动覆盖已登记节点的 `baseUrl/clientIp`
- 不自动把疑似换 IP 的节点恢复成 `verified`

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

## 6. 状态解释口径

这个接口返回的是“发现线索”，不是正式节点状态。

要特别区分：

- 发现项存在
  只表示“某个 Edge 服务正在广播”
- 发现项带 `clientId`
  只表示“它看起来能关联到某条已注册节点记录”
- 发现项 `clientId=""`
  表示当前 discovery 项没有匹配到任何已登记节点，这是“未绑定发现项”的正常结果
- 节点真正可用
  仍然要求正式节点满足 `verified + healthy + online`

如果后续发现项显示的 `payload.clientIp/baseUrl` 与已登记节点不一致，应把它理解为：

- 可能只是节点换了 IP
- 也可能指向了另一台机器

中心不能因为 discovery 命中就自动接受这个变化，而应进入 `blocked + ip_mismatch` 的人工确认流程。

当前实现补充说明：

- 匹配优先级是 `baseUrl -> payload.clientIp -> sourceIp`
- 如果 `edge_clients` 中已登记节点记录被删除，下一轮 discovery 会重新回到 `clientId=""`
- 如果已登记节点本身缺少可比较的地址事实，例如 `client_ip` 历史上为空，
  则仅靠“新的 clientIp 变化”不一定总能立即打出 `ip_mismatch`

## 7. 成功判定

- 能正常返回 discovery 缓存
- 已绑定发现项能补充出 `clientId`
- 命中已注册节点时可同步更新心跳事实

## 8. 失败判定

- Platform Header 缺失
- 查询已注册节点失败
- 回写命中节点的 UDP 心跳事实失败

## 9. 风险控制

- 未绑定发现项必须跳过写库
- 不能因为 IP 相似就错误归属
- 不能因为新的 discovery 地址可达，就自动覆盖旧 `baseUrl/clientIp`

## 10. 联调验收标准

- 未注册 Client 只显示发现项，不生成节点
- 已注册 Client 能正确补充 `clientId`
- 临时删除已登记节点记录后，同一 discovery 项应恢复为 `clientId=""`
- `payload` 只包含 discovery 摘要，不包含敏感资产数据
