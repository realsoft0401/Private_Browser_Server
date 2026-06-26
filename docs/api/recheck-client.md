# POST /api/v1/edge-clients/{clientId}/recheck

## 功能目标

对某个正式已登记节点发起一次管理员手动重探测，重新校验这台 Client 当前的健康事实、设备事实和发现可信度。

当前业务语义名正式收口为：

- 中文业务名：`会话校验`
- 技术接口名：`recheck`

> 当前文档定位：这是 `Private_Browser_Server` 的正式节点治理接口。
> 它负责“重探测并刷新中心摘要”，不是 bind 接口，也不是 IP 确认接口，更不是 browser-env 生命周期接口。

## 业务边界

- 负责根据 `clientId` 找到当前正式节点
- 负责重新调用 Client `/health`
- 负责重新调用 Client `/api/v1/edge/device-info`
- 负责重新校验 `clientIp / baseUrl / hostname / os / arch / docker` 摘要
- 负责刷新 `health_status / discovery_status / discovery_reason / last_error / last_checked_at`
- 负责确认“当前这条节点会话是否仍然成立”
- 负责记录管理员重探测审计
- 不负责重新 bind
- 不负责生成新的 `clientId`
- 不负责自动确认 IP 漂移
- 不负责自动覆盖 `identity_changed / ip_mismatch`
- 不负责直接放行业务 run

## 业务语义解释

这里的“会话校验”不是重新绑定，也不是重新建档。

它校验的是：

- 这台 Client 现在还活着没有
- 当前地址还能不能访问
- 当前设备事实是不是还是原来那台
- Node Server 里保存的连接信息是不是已经过期

所以这条接口的本质是：

- 重新确认“当前节点会话是否仍然有效”
- 如果有效，就恢复或保持中心可信状态
- 如果无效或冲突，就把问题明确收口到 `probe_failed / ip_mismatch / identity_changed`

## 前置校验

- `clientId` 必填
- 目标节点必须存在于当前 `edge_clients`
- 目标节点必须是当前有效节点
- 请求体可为空；如传 body，必须是合法 JSON

## 状态流转

成功后：

- Server 会刷新这台节点的最新探测摘要
- 如果 `/health + /device-info` 全部通过，且设备事实与当前记录一致：
  - `health_status` 恢复为 `healthy` 或保持 `healthy`
  - `discovery_status` 恢复为 `verified` 或保持 `verified`
  - `discovery_reason` 清空
- 如果 Client 可达，但设备事实与当前记录冲突：
  - `discovery_status = blocked`
  - `discovery_reason = identity_changed` 或 `ip_mismatch`
- 如果 Client 不可达，或关键探测失败：
  - `health_status = unhealthy` 或 `offline`
  - `discovery_status = blocked`
  - `discovery_reason = probe_failed` 或等价失败原因

失败后：

- 如果中心读取节点失败，当前节点状态保持不变
- 如果重探测过程中探测失败，接口本身可以返回失败，同时把失败事实回写到节点摘要和日志

## 请求与响应

### 请求

```http
POST /api/v1/edge-clients/9060901190002/recheck
Content-Type: application/json
```

```json
{
  "source": "manual-recheck"
}
```

也允许空 body：

```http
POST /api/v1/edge-clients/9060901190002/recheck
```

### 成功响应

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "clientId": "9060901190002",
    "status": "rechecked",
    "healthStatus": "healthy",
    "discoveryStatus": "verified",
    "discoveryReason": "",
    "checkedAt": 1782500001
  }
}
```

发现设备事实冲突，但接口仍同步收口成功：

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "clientId": "9060901190002",
    "status": "rechecked",
    "healthStatus": "healthy",
    "discoveryStatus": "blocked",
    "discoveryReason": "identity_changed",
    "checkedAt": 1782500001
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
  "message": "recheck request body 非法"
}
```

Client 不可达或关键探测失败：

```json
{
  "code": 1005,
  "message": "recheck probe failed: GET http://192.168.111.119:3300/health: context deadline exceeded"
}
```

## SSE 说明

- 本接口当前不使用 SSE
- 原因：第一版 `recheck` 只包含一次同步重探测，耗时和阶段数量都还可控，同步 HTTP 足够表达最终收口结果
- 如果后续 `recheck` 扩展成多阶段治理动作，例如包含平台协同、IP 确认、批量修复，再单独升级成 task + SSE

## 任务编排

当前接口不创建 `server_tasks`。

当前第一阶段按同步 HTTP 收口：

1. 读取中心节点
2. 调用 Client `/health`
3. 调用 Client `/api/v1/edge/device-info`
4. 对比当前中心记录与最新设备事实
5. 刷新中心节点摘要
6. 写 `recheck` 审计
7. 返回最终结果

## 成功判定

- 找到了目标节点
- 重探测请求执行完成
- 当前响应能明确返回最终摘要

补充说明：

- `recheck` 的“成功”只表示这次治理动作已经收口，不等于节点一定恢复为 `verified`
- 如果最新事实是 `identity_changed / ip_mismatch / probe_failed`，也可以作为一次成功收口的治理结果返回

## 失败判定

- `clientId` 为空
- 目标节点不存在
- 请求体非法
- 中心读取节点失败
- 与 Client 通信失败，且本次无法形成可信摘要

## 日志字段

- `action=recheck`
- `clientId`
- `accountId`
- `clientIp`
- `baseUrl`
- `result`
- `message`
- `healthStatus`
- `discoveryStatus`
- `discoveryReason`

## 联调验收标准

- 管理员可以对已登记节点发起 `recheck`
- `recheck` 能重新探测 `/health` 与 `/api/v1/edge/device-info`
- 事实一致时，节点可恢复到 `healthy + verified`
- 事实冲突时，节点必须进入 `blocked + identity_changed/ip_mismatch`
- `recheck` 不得自动重绑、不得生成新的 `clientId`
- `recheck` 不得自动确认 IP 变更
