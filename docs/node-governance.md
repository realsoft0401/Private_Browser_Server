# Private_Browser_Server 节点治理设计

## 1. 文档定位

这份文档只描述当前 `Private_Browser_Server` 已落地和近期必须保持一致的节点治理规则。

它解决 4 件事：

1. 节点现在处于什么阶段。
2. 节点当前允许做什么、不允许做什么。
3. 管理员在 `blocked` 时应该点哪个接口恢复。
4. 业务动作为什么必须同时看 `healthStatus`、`discoveryStatus`、`heartbeatStatus`。

这份文档不讨论未来 AI、Agent、自动化平台扩展，也不替代逐接口 OpenAPI。

## 2. 设计背景

这一轮收口前，节点状态在文档和实现里曾经同时混用“健康异常”“身份变化”“发现态”“离线态”几种概念，导致两个问题：

1. 前端或管理员很难判断“当前应该点 verify，还是应该人工确认地址变化”。
2. 后端状态枚举越拆越多，但很多状态只是同一个阻断结果的不同原因，最终会把系统拖重。

因此当前正式口径收口为：

- `discoveryStatus` 只保留 `blocked / verified`
- “为什么 blocked” 下沉到 `discoveryReason`
- “节点是否在线” 交给 `heartbeatStatus`
- “节点本机能力是否健康” 交给 `healthStatus`

换句话说：

- `discoveryStatus` 只回答“中心当前是否确认这还是原来那台节点”
- `healthStatus` 只回答“这台节点本机能力是否健康”
- `heartbeatStatus` 只回答“这台节点最近是否在线”

三者不能互相替代。

## 3. 三组核心状态

### 3.1 `discoveryStatus`

```text
blocked
verified
```

含义：

- `blocked`
  中心当前不允许这台节点通过身份准入，但不直接说明原因。
- `verified`
  中心当前确认这就是登记过的那台节点，并且最近一次完整探测通过。

### 3.2 `discoveryReason`

```text
""
ip_mismatch
device_fact_changed
```

含义：

- `""`
  当前没有额外身份异常原因，通常表示“还没 verify 完成”。
- `ip_mismatch`
  最近发现到的 `clientIp/baseUrl` 与中心登记地址不一致。
- `device_fact_changed`
  最近探测到的 `hostname/os/arch/docker` 等设备事实变化过大，中心不能确认这还是原节点。

### 3.3 `healthStatus`

```text
healthy
unhealthy
stale
offline
```

含义：

- `healthy`
  `Client /health`、`device-info`、`Docker 2375` 都通过。
- `unhealthy`
  Client 可达，但关键检查失败。
- `stale`
  中心缓存不可信、心跳过期或动作前校验失败。
- `offline`
  Client 已确认不可达。

### 3.4 `heartbeatStatus`

```text
online
stale
offline
```

默认阈值：

```text
stale_after_seconds = 30
offline_after_seconds = 90
```

判定规则：

```text
online:
  now - last_heartbeat_at <= stale_after_seconds

stale:
  stale_after_seconds < now - last_heartbeat_at <= offline_after_seconds

offline:
  last_heartbeat_at = 0
  或 now - last_heartbeat_at > offline_after_seconds
```

## 4. 设备当前处于什么阶段

这是给接口、前端按钮和管理员排障统一复用的解释口径。

### 4.1 `blocked + discoveryReason=""`

设备已登记，但尚未完成中心准入验证。

可以理解为：

```text
档案已建，但还没有验明正身
```

此时重点不是业务放行，而是继续完成：

- `device-info/refresh`
- `verify`

### 4.2 `verified + discoveryReason=""`

设备身份连续性已确认，最近一次完整验证通过。

可以理解为：

```text
这台机器就是中心认可的那台节点
```

但这不等于一定允许业务动作。真正放行仍要同时满足：

- `healthStatus=healthy`
- `heartbeatStatus=online`

### 4.3 `blocked + ip_mismatch`

设备记录还在，节点也可能仍在线，但中心观察到的地址与登记地址不一致。

可以理解为：

```text
像原来的节点，但住址变了，不能自动认领
```

此时不能继续 `verify`，必须走管理员确认地址更新。

### 4.4 `blocked + device_fact_changed`

节点入口可能还在，但设备事实变化过大。

可以理解为：

```text
看起来已经不是原来那台机器，或者至少中心无法自动证明它还是
```

此时不能自动恢复，也不能走地址确认接口，只能人工排查或重新登记。

## 5. 节点治理动作矩阵

| `discoveryStatus` | `discoveryReason` | 当前阶段 | 允许动作 | 禁止动作 |
| --- | --- | --- | --- | --- |
| `blocked` | `""` | 已登记但未完成准入验证 | `verify`、`refresh device-info`、查看详情 | create env、run、stop、backup、restore、delete、import-package |
| `blocked` | `ip_mismatch` | 管理员待确认是否只是地址变化 | `confirm-address-update`、查看详情 | `verify`、create env、run、stop、backup、restore、delete、import-package |
| `blocked` | `device_fact_changed` | 新探测结果看起来已不是原节点 | 查看详情、人工排查、必要时重新登记 | `verify`、`confirm-address-update`、create env、run、stop、backup、restore、delete、import-package |
| `verified` | `""` | 身份连续性已确认 | create env、run、stop、backup、restore、delete、import-package | 无额外治理动作要求 |

补充说明：

- `refresh device-info` 只刷新设备事实，不会把 `blocked` 自动放回 `verified`。
- `confirm-address-update` 只处理 `blocked + ip_mismatch`。
- `device_fact_changed` 当前没有“强制恢复”接口，避免把另一台机器误绑回旧 `clientId`。

## 6. 节点治理标准流程

### 6.1 新节点接入

```text
1. Client 通过 UDP discovery 广播本机入口
2. Server 写入 discovered 观察项
3. 管理员注册节点
4. 刷新 device-info
5. 调用 verify
6. 进入 verified
7. 后续业务动作统一走 EnsureClientReadyForBusiness
```

### 6.2 已登记节点地址变化

```text
1. heartbeat/discovery 观察到地址与登记记录不一致
2. Server 把节点标成 blocked + ip_mismatch
3. 列表/详情不再允许继续 verify
4. 管理员确认“这还是原节点，只是地址变了”
5. 调用 POST /api/v1/edge-clients/{clientId}/confirm-address-update
6. Server 更新 baseUrl/clientIp/dockerApiUrl
7. 立即重跑 /health、device-info、Docker 2375、架构确认
8. 全部通过后恢复 verified
9. 如果设备事实变化过大，则转成 blocked + device_fact_changed
```

### 6.3 已登记节点设备事实变化

```text
1. refresh / verify / confirm-address-update 过程中发现 os/arch/docker/hostname 差异过大
2. Server 保持或转成 blocked + device_fact_changed
3. 管理员查看详情并人工排查
4. 必要时新建节点，而不是强行复用旧 clientId
```

## 7. 业务动作统一放行条件

所有环境包业务动作必须同时满足：

```text
healthStatus == healthy
discoveryStatus == verified
heartbeatStatus == online
arch in [amd64, arm64]
baseUrl 非空
dockerApiUrl 非空
lastError 为空
```

也就是说，只有：

```text
healthy + verified + online
```

才允许：

- create env
- run
- stop
- backup
- restore
- delete
- import-package

任何一个条件不满足，都必须拒绝，并返回明确修复方向。

## 8. 推荐接口顺序

```text
GET  /api/v1/edge-clients/discovered
POST /api/v1/edge-clients/probe-docker
POST /api/v1/edge-clients
POST /api/v1/edge-clients/{clientId}/device-info/refresh
POST /api/v1/edge-clients/{clientId}/verify
POST /api/v1/edge-clients/{clientId}/confirm-address-update
POST /api/v1/envs
POST /api/v1/envs/{envId}/run
POST /api/v1/envs/{envId}/stop
```

## 9. 明确禁止的事情

- 不能因为 UDP 在线就把 `healthStatus` 改成 `healthy`
- 不能因为 Docker 2375 可达就把 `discoveryStatus` 改成 `verified`
- 不能因为曾经 `verified` 就忽略 `heartbeatStatus=offline`
- 不能把 `blocked + discoveryReason!= ""` 混成 `unhealthy` 或 `stale`
- `blocked` 节点即使 `healthStatus=healthy` 且 `heartbeatStatus=online`，也必须阻断业务动作
- `blocked + ip_mismatch` 不能继续点 `verify`
- `blocked + device_fact_changed` 不能继续点 `confirm-address-update`

## 10. 当前正式接口

节点治理正式接口当前至少包括：

- `GET /api/v1/edge-clients/discovered`
- `GET /api/v1/edge-clients`
- `GET /api/v1/edge-clients/{clientId}`
- `POST /api/v1/edge-clients`
- `POST /api/v1/edge-clients/{clientId}/device-info/refresh`
- `POST /api/v1/edge-clients/{clientId}/verify`
- `POST /api/v1/edge-clients/{clientId}/confirm-address-update`
- `POST /api/v1/server/edge-clients/heartbeat`

## 11. 与现有文档的关系

- [state.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/state.md)
  保留为状态事实速查表。
- [flow.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/flow.md)
  保留为接入与恢复流程速查表。
- 本文档
  作为当前节点治理的正式设计收口文档，后续状态口径、按钮口径、接口前置条件都应以本文为准。
