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
- `discoveryReason`
- `primaryAction`
- `allowedActions`
- `heartbeatStatus`
- `lastCheckedAt`
- `lastError`

## 5. 状态字段解释

节点详情页需要比列表页更明确地区分“身份、健康、在线”三层事实。

`discoveryStatus`：

- `blocked`
  节点当前不允许业务放行；它本身不区分“未 verify”和“身份异常待确认”。
- `verified`
  节点身份连续性已确认。

`discoveryReason`：

- `""`
  没有额外身份异常原因。
- `ip_mismatch`
  当前发现到的 clientIp/baseUrl 与登记地址不一致。
- `device_fact_changed`
  当前探测到的设备事实与原记录差异过大。

`lastDiscoveredAt`：

- 表示最近一次通过 discovery 看见该节点入口的时间。
- 它不表示 verify 时间，也不表示本机健康检查时间。

当前实现补充说明：

- 如果节点记录的 `client_ip` 长期为空，后续 `ip_mismatch` 更依赖 `baseUrl` 这条地址事实；
- 因此详情页看到 `clientIp=""` 时，应理解为“这条节点记录的地址事实还不够完整”，
  后续如果要稳定做 IP 变化治理，建议先把正式 `clientIp` 补齐。

详情页展示时，不要把：

- `heartbeatStatus=online`
- `healthStatus=healthy`

误读成“节点一定可用”；只要 `discoveryStatus != verified`，业务动作都必须继续阻断。

详情页操作建议：

- `blocked + ""`
  主按钮应是 `verify`
  次按钮可以保留 `refresh device-info`
- `blocked + ip_mismatch`
  主按钮应是 `confirm-address-update`
  不应继续显示 `verify`
- `blocked + device_fact_changed`
  不应继续显示 `verify` 或 `confirm-address-update`
  应强调查看 `lastError`、核对宿主事实、必要时重新登记新节点
- `verified + healthy + online`
  才允许被上层 env/create、run、stop 等业务入口引用

如果页面想避免自己重复写状态判断，可以直接使用接口返回的：

- `primaryAction`
- `allowedActions`

把它们当成节点治理按钮白名单。

## 6. 成功判定

- 能按主账号 + clientId 读取到节点
- 能按当前时间动态算出 `heartbeatStatus`

## 7. 失败判定

- `clientId` 不存在
- 节点不属于当前主账号
- SQLite 读取失败

## 8. 适用场景

- 点击节点列表进入详情
- verify 失败后查看 `lastError`
- run/create env 前人工确认节点准入事实
- 判断当前节点是否处于“未绑定发现项已重新绑定”或 `blocked + discoveryReason!= ""` 待确认阶段
