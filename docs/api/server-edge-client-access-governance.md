# Server Edge Client Access Governance APIs

这份文档收口 Node Server 的节点接入、绑定、心跳和会话校验接口。

它只说明节点治理主链，不说明平台额度、slot 商业约束和 browser-env 生命周期动作。

## 1. 总体链路

正式顺序固定为：

1. Client 先独立启动，并通过 UDP beacon 暴露最小服务摘要。
2. Node Server 监听 UDP，校验 `discoveryMagic/service/discoveryGroup/protocolVersion`。
3. Node Server 通过 Client HTTP API 探测 `/health` 和 `/api/v1/edge/device-info`。
4. 管理员或平台在 Node Server 上发起 bind。
5. Node Server 分配中心 `clientId`，写入 `edge_clients`。
6. Node Server 再把 `clientId/accountId/nodeServerBaseUrl` 写回 Client 本地 `node-registration.json`。
7. Client 后续按写回的 Node 地址上报 heartbeat。

### 固定原则

- UDP beacon 是自动发现唯一正式链路。
- heartbeat 不参与发现，也不创建 discovered。
- Client 不自行生成 `clientId`。
- `clientId` 由 Node Server 分配，是中心身份。
- bind 成功后才允许写回 Client 本地注册文件。
- 解绑会删除中心绑定记录，并尝试清理 Client 本地注册文件。

## 2. GET /api/v1/edge-clients/discovered

### 业务语义

返回 Node Server 当前通过 UDP beacon 收到的临时 discovered 视图。

### 它负责什么

- 返回当前内存里的 discovered client 列表。
- 辅助管理员确认局域网里有哪些 Client 正在广播。

### 它不负责什么

- 不创建正式 `edge_clients`。
- 不分配 `clientId`。
- 不绑定账号。
- 不写回 Client 本地注册文件。

### 状态与前置条件

- Node Server 必须能监听 `43000/udp`。
- Client 必须启用 UDP beacon。
- beacon 必须通过平台字段校验。

### SSE 说明

- 不使用 SSE。
- 原因：这是普通临时视图查询，同步 HTTP 已足够表达结果。

## 3. POST /api/v1/edge-clients/heartbeat

### 业务语义

接收已知 Client 主动上报的活性心跳。

### 它负责什么

- 只更新已知节点的 heartbeat 摘要。
- 命中已知节点后，允许补刷 CPU、内存、Docker 版本等设备摘要。
- 返回本次 heartbeat 是否匹配到已知节点。

### 它不负责什么

- 不参与发现。
- 不创建 discovered。
- 不创建正式节点。
- 不生成 `clientId`。
- 不改变账号归属。

### 请求体要点

请求体沿用 Client beacon 的非敏感摘要字段，例如：

```json
{
  "magic": "PRIVATE_BROWSER_CLIENT_DISCOVERY",
  "service": "private-browser-client",
  "protocolVersion": 1,
  "group": "default",
  "clientIp": "192.168.111.119",
  "baseUrl": "http://192.168.111.119:3300",
  "lastHeartbeatAt": 1782800000
}
```

### 状态与前置条件

- 只有 `baseUrl` 或 `clientIp` 能匹配到已知正式节点时，才更新节点心跳。
- 未匹配到已知节点时，接口仍可返回 received，但不会落正式发现或绑定。

### SSE 说明

- 不使用 SSE。
- 原因：心跳是短链路活性回执，同步 HTTP 已足够表达结果。

## 4. POST /api/v1/edge-clients/bind

### 业务语义

把一个已探测通过的 Client 绑定到指定账号，并由 Node Server 分配中心 `clientId`。

### 它负责什么

- 校验 `accountId` 和 `clientIp`。
- 根据 `clientIp` 探测 Client `/health` 和 `/api/v1/edge/device-info`。
- 探测 Client `/api/v1/edge/node-registration`，确认本地没有 `node-registration.json` 绑定锁。
- 创建正式 `edge_clients` 记录。
- 分配 `clientId`，格式为 `accountId + 4位序号`。
- 自动调用 push-client-id，把中心身份写回 Client。

### 它不负责什么

- 不允许同一个 Client 静默覆盖到另一个账号。
- 不跳过 Client HTTP 探测。
- 不覆盖 Client 本地已有 `node-registration.json`。
- 不创建 browser-env。
- 不初始化平台额度。

### 请求示例

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/bind" \
  -H "Content-Type: application/json" \
  -d '{
    "accountId": "906090119",
    "clientIp": "192.168.111.119"
  }' | jq
```

### 成功响应要点

```json
{
  "code": 1000,
  "data": {
    "clientId": "9060901190003",
    "accountId": "906090119",
    "status": "bound",
    "clientIp": "192.168.111.119",
    "baseUrl": "http://192.168.111.119:3300",
    "bindStatus": "success",
    "pushStatus": "success"
  },
  "message": "success"
}
```

### 状态与前置条件

- `clientIp` 必须可达。
- Client `/health` 必须可达。
- Client `/api/v1/edge/device-info` 必须可达。
- Client `/api/v1/edge/node-registration` 必须可达。
- Client 本地不能已有 `node-registration.json`。
- 同一 `clientIp/baseUrl` 不能已经绑定到其它账号。

### 失败收口

- 已绑定到当前账号：返回无需重复绑定。
- 已绑定到其它账号：必须先解绑，不能静默覆盖。
- Client 本地已有 `node-registration.json`：普通 bind 必须拒绝。需要换 Node 时先旧 Node unbind；旧 Node 不可用时由管理员手动登录 Client 机器删除本地注册文件，再由新 Node 重新发起普通 bind。当前不提供 force bind 接口。
- 探测失败：不创建正式节点。
- push 写回失败：bind 不回滚，但 `pushStatus=failed`，管理员可重试 push。

### SSE 说明

- 不使用 SSE。
- 原因：bind 是同步治理动作，内部探测和写回可以在一次 HTTP 响应里表达清楚。

## 5. POST /api/v1/edge-clients/{clientId}/push-client-id

### 业务语义

把 Node Server 已分配的中心身份写回 Client 本地注册文件。

### 它负责什么

- 根据 `clientId` 找到当前正式节点。
- 调用 Client 注册写回接口。
- 写入 `clientId/accountId/nodeServerBaseUrl` 等中心控制信息。
- 更新 push 留痕。

### 它不负责什么

- 不分配新的 `clientId`。
- 不改变账号归属。
- 不重新探测设备身份。
- 不替代 bind。

### 请求示例

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/push-client-id" \
  -H "Content-Type: application/json" \
  -d '{
    "accountId": "906090119",
    "nodeServerBaseUrl": "http://192.168.111.120:3400",
    "source": "manual-repush"
  }' | jq
```

### SSE 说明

- 不使用 SSE。
- 原因：这是一次 Node -> Client 的短链路配置写回，同步 HTTP 已足够表达成功或失败。

## 6. POST /api/v1/edge-clients/{clientId}/unbind

### 业务语义

解除账号与 Client 的当前绑定关系，并尝试清理 Client 本地 `node-registration.json`。

### 它负责什么

- 删除中心当前有效绑定记录。
- 写入解绑审计日志。
- 调用 Client 清理本地注册文件。
- 返回本地清理是否成功。

### 它不负责什么

- 不删除 Client 本机 browser-env 资产。
- 不删除 Client 本机 slot 容器。
- 不清理 Server 历史任务审计。
- 不自动重新绑定。

### 请求示例

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/unbind" \
  -H "Content-Type: application/json" \
  -d '{"source":"manual-unbind"}' | jq
```

### 成功响应要点

```json
{
  "code": 1000,
  "data": {
    "clientId": "9060901190003",
    "accountId": "906090119",
    "status": "unbound",
    "clearRegistrationStatus": "success",
    "unboundAt": 1782800000
  },
  "message": "success"
}
```

### 失败收口

- 中心解绑成功但 Client 本地清理失败时，不回滚中心解绑。
- 返回 `clearRegistrationStatus=failed` 和错误摘要，管理员可后续处理。

### SSE 说明

- 不使用 SSE。
- 原因：解绑是短链路治理动作，同步 HTTP 已足够表达中心解绑和本地清理结果。

## 7. POST /api/v1/edge-clients/{clientId}/recheck

### 业务语义

会话校验。用于管理员手动确认当前登记的 Client 会话是否仍然有效。

### 它负责什么

- 读取当前正式节点。
- 重探当前 `baseUrl` 的 `/health` 和 `/api/v1/edge/device-info`。
- 更新 `healthStatus/discoveryStatus/discoveryReason/lastError`。
- 发现 `ip_mismatch` 或 `identity_changed` 时进入阻断态。

### 它不负责什么

- 不重新 bind。
- 不自动修改 IP。
- 不自动确认地址漂移。
- 不改变账号归属。

### 请求示例

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/recheck" \
  -H "Content-Type: application/json" \
  -d '{"source":"manual-session-check"}' | jq
```

### 状态收口

- 探测成功且身份一致：恢复或保持 `healthStatus=healthy`、`discoveryStatus=verified`。
- 探测失败：收口为 `healthStatus=offline`、`discoveryStatus=blocked`、`discoveryReason=probe_failed`。
- 设备事实冲突：收口为 `discoveryStatus=blocked`，原因写入 `identity_changed` 或 `ip_mismatch`。

### SSE 说明

- 不使用 SSE。
- 原因：会话校验是单次探测和同步状态回写，不需要多阶段事件流。

## 8. POST /api/v1/edge-clients/{clientId}/confirm-address-update

### 业务语义

管理员确认节点新 IP 或新 baseUrl 后，更新中心节点接入地址。

### 它负责什么

- 校验管理员提交的新 `clientIp/baseUrl`。
- 对新地址重新执行 `/health` 和 `/api/v1/edge/device-info` 探测。
- 确认设备身份未变化后更新中心地址。
- 更新健康摘要和地址治理审计。

### 它不负责什么

- 不自动发现新地址。
- 不替代 `recheck`。
- 不改变 `clientId`。
- 不改变账号归属。
- 不处理平台额度或 slot 额度。

### 请求示例

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/confirm-address-update" \
  -H "Content-Type: application/json" \
  -d '{
    "clientIp": "192.168.111.119",
    "baseUrl": "http://192.168.111.119:3300",
    "source": "manual-confirm-address"
  }' | jq
```

### 状态与前置条件

- 新地址必须可达。
- 新地址上的设备事实必须与原节点身份一致。
- `clientIp` 与 `baseUrl` 主机必须一致。

### SSE 说明

- 不使用 SSE。
- 原因：这是管理员确认后的同步治理动作，结果可以通过普通 HTTP 返回。

## 9. 联调验收标准

### 成功路径

- Node 能通过 UDP discovered 看到 Client。
- bind 成功后，`edge_clients` 出现正式 `clientId`。
- Client 本地 `/Business/data/node-registration.json` 被写入。
- Client heartbeat 能命中已知节点。
- recheck 成功后节点保持 `healthy + verified`。
- unbind 成功后中心绑定记录删除，Client 本地注册文件被清理。

### 关键失败路径

- heartbeat 未匹配已知节点时不能创建正式节点。
- 同一 Client 绑定到其它账号时必须拒绝。
- Client 本地已有 `node-registration.json` 时，普通 bind 必须拒绝。
- 旧 Node 不可用时，管理员手动删除 Client 本地 `/Business/data/node-registration.json` 后才能重新 bind。
- recheck 探测失败时必须落 `offline + blocked + probe_failed`。
- 地址变化必须先 recheck 发现，再由管理员 confirm-address-update。
- unbind 后本地清理失败不能回滚中心解绑，但必须返回清理失败原因。

### 阻塞上线条件

- OpenAPI 路径、Markdown 文档、Routes 和 handler 不一致。
- SSE 标注错误，把同步接口写成事件流。
- heartbeat 被重新当成发现链路。
- bind 前跳过 Client HTTP 探测。
- 普通 bind 覆盖 Client 本地 `node-registration.json`。
- push-client-id 被误当成 Client 自注册接口。
