# POST /api/v1/edge-clients/{clientId}/confirm-address-update

## 功能目标

在“会话校验”已经发现某个已登记节点存在 `ip_mismatch` 或等价地址漂移问题后，由管理员明确确认这仍然是同一台 Client，并把 Node Server 中心记录更新到新的 `clientIp / baseUrl`。

> 当前文档定位：这是 `Private_Browser_Server` 的正式节点治理接口。
> 它负责“人工确认并更新节点地址”，不是 bind 接口，不是会话校验接口，也不是 browser-env 生命周期接口。

## 业务边界

- 负责根据 `clientId` 找到当前正式节点
- 负责接收管理员确认后的新 `clientIp / baseUrl`
- 负责把中心节点记录更新到新的地址
- 负责在更新后重新探测 `/health`
- 负责在更新后重新探测 `/api/v1/edge/device-info`
- 负责再次校验 `hostname / os / arch / docker` 摘要是否仍匹配原节点
- 负责刷新 `health_status / discovery_status / discovery_reason / last_error / last_checked_at`
- 负责记录管理员地址确认审计
- 不负责重新 bind
- 不负责生成新的 `clientId`
- 不负责自动从会话校验直接跳到地址更新
- 不负责忽略设备事实冲突
- 不负责直接放行业务 run

## 前置校验

- `clientId` 必填
- 目标节点必须存在于当前 `edge_clients`
- 目标节点必须是当前有效节点
- 请求体必须是合法 JSON
- 至少提供一个明确的新地址：
  - `clientIp`
  - 或 `baseUrl`
- 如果同时提供 `clientIp` 和 `baseUrl`，两者必须能指向同一台 Client

## 请求建议口径

建议正式请求体收口为：

```json
{
  "clientIp": "192.168.111.120",
  "baseUrl": "http://192.168.111.120:3300",
  "source": "manual-confirm-address-update"
}
```

解释：

- `clientIp` 是新的独立内网地址
- `baseUrl` 是新的 Edge HTTP 入口
- `source` 只做审计留痕

## 状态流转

成功后：

- 中心节点记录更新到新的 `clientIp / baseUrl`
- Server 立即重新执行 `/health + /device-info` 校验
- 如果设备事实仍匹配原节点：
  - `discovery_status = verified`
  - `discovery_reason = ""`
  - `health_status = healthy` 或按最新探测结果收口

部分成功后：

- 中心地址已更新
- 但重探测后发现设备事实冲突
- 这时：
  - 地址更新动作本身可以视为成功收口
  - 但节点状态必须进入：
    - `discovery_status = blocked`
    - `discovery_reason = identity_changed`

失败后：

- 如果新地址不可达，原中心地址保持不变
- 如果新地址可达但无法形成可信设备摘要，原中心地址保持不变
- 如果中心更新失败，原记录保持不变

## 请求与响应

### 请求

```http
POST /api/v1/edge-clients/9060901190002/confirm-address-update
Content-Type: application/json
```

```json
{
  "clientIp": "192.168.111.120",
  "baseUrl": "http://192.168.111.120:3300",
  "source": "manual-confirm-address-update"
}
```

### 成功响应

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "clientId": "9060901190002",
    "oldClientIp": "192.168.111.119",
    "newClientIp": "192.168.111.120",
    "oldBaseUrl": "http://192.168.111.119:3300",
    "newBaseUrl": "http://192.168.111.120:3300",
    "healthStatus": "healthy",
    "discoveryStatus": "verified",
    "discoveryReason": "",
    "updatedAt": 1782501001
  }
}
```

地址更新成功，但新地址上的设备事实冲突：

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "clientId": "9060901190002",
    "oldClientIp": "192.168.111.119",
    "newClientIp": "192.168.111.120",
    "oldBaseUrl": "http://192.168.111.119:3300",
    "newBaseUrl": "http://192.168.111.120:3300",
    "healthStatus": "healthy",
    "discoveryStatus": "blocked",
    "discoveryReason": "identity_changed",
    "updatedAt": 1782501001
  }
}
```

### 失败响应

目标节点不存在：

```json
{
  "code": 1005,
  "message": "edge client not found"
}
```

请求体非法：

```json
{
  "code": 1002,
  "message": "confirm-address-update request body 非法"
}
```

新地址不可达：

```json
{
  "code": 1005,
  "message": "confirm-address-update probe failed: GET http://192.168.111.120:3300/health: context deadline exceeded"
}
```

## SSE 说明

- 本接口当前不使用 SSE
- 原因：第一版地址确认更新只包含一次人工确认后的更新和重探测，同步 HTTP 足够表达最终收口结果
- 如果后续要串平台确认、多节点协同或批量迁移，再单独升级成 task + SSE

## 任务编排

当前接口不创建 `server_tasks`。

当前第一阶段按同步 HTTP 收口：

1. 读取中心节点
2. 校验请求里的新 `clientIp / baseUrl`
3. 对新地址执行 `/health`
4. 对新地址执行 `/api/v1/edge/device-info`
5. 校验设备事实是否仍匹配原节点
6. 只有校验通过或形成明确治理结论后，才更新中心地址
7. 写 `confirm-address-update` 审计
8. 返回最终结果

## 成功判定

- 找到了目标节点
- 管理员给出了明确的新地址
- 这次地址确认动作已经形成最终收口结果

补充说明：

- “成功”表示这次人工治理动作已经完成，不等于节点一定恢复为 `verified`
- 如果新地址可达，但设备事实变成 `identity_changed`，也可以作为一次成功收口的治理结果返回

## 失败判定

- `clientId` 为空
- 目标节点不存在
- 请求体非法
- 新地址信息缺失
- 新地址探测失败，无法形成可信节点事实
- 中心更新失败

## 日志字段

- `action=confirm_address_update`
- `clientId`
- `accountId`
- `oldClientIp`
- `newClientIp`
- `oldBaseUrl`
- `newBaseUrl`
- `result`
- `message`
- `healthStatus`
- `discoveryStatus`
- `discoveryReason`

## 联调验收标准

- 管理员可以对已登记节点发起地址确认更新
- 新地址必须重新通过 `/health` 与 `/api/v1/edge/device-info`
- 事实一致时，节点可恢复到 `healthy + verified`
- 事实冲突时，节点必须进入 `blocked + identity_changed`
- `confirm-address-update` 不得重新 bind、不得生成新的 `clientId`
- `confirm-address-update` 不得跳过新地址探测直接改库

## 与会话校验的关系

- `会话校验 / recheck`
  - 负责发现问题
  - 负责把问题收口成 `probe_failed / ip_mismatch / identity_changed`

- `地址确认更新 / confirm-address-update`
  - 负责在管理员确认后修正中心地址
  - 负责把原 `clientId` 继续绑定到新的 `clientIp / baseUrl`

这两个动作不能混成一个：

- 不能让 `recheck` 自动改地址
- 也不能让 `confirm-address-update` 跳过“新地址探测”直接落库
