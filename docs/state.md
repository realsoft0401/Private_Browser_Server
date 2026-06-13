# Private_Browser_Server 状态文档

正式节点治理口径以 [node-governance.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/node-governance.md) 为准。
本文保留为状态字段和动作矩阵速查表。

## Edge Client 状态

Node Server 的 Client 状态由三组字段共同决定，不允许混用。

| 字段 | 来源 | 含义 |
| --- | --- | --- |
| `healthStatus` | `/health`、device-info、Docker 2375 探测 | Client 和宿主能力健康 |
| `discoveryStatus` | Node Server 注册/verify 流程 | Client 是否完成中心验证 |
| `heartbeatStatus` | `last_heartbeat_at` 动态计算 | Node Server 最近一次确认收到该 Client 心跳的时间是否新鲜 |

当前项目里，最容易混淆的是：

- `discoveryStatus` 回答“这是不是中心当前还能确认身份连续性的那台节点”
- `healthStatus` 回答“这台节点本机能力是不是健康”
- `heartbeatStatus` 回答“这台节点最近是不是在线”

三者必须分开看，不能拿其中一个替代另外两个。

## heartbeat 字段语义

```text
last_heartbeat_at:
  Node Server 实际收到心跳的服务端时间。
  heartbeatStatus 只能根据这个字段动态计算。

last_heartbeat_reported_at:
  Client 在 UDP beacon 或 HTTP heartbeat 里自报的时间。
  只用于排障和时钟偏差观察，不能直接拿来放行业务动作。
```

## heartbeatStatus

```text
online:
  now - last_heartbeat_at <= stale_after_seconds

stale:
  stale_after_seconds < now - last_heartbeat_at <= offline_after_seconds

offline:
  last_heartbeat_at = 0
  或 now - last_heartbeat_at > offline_after_seconds
```

默认阈值：

```text
stale_after_seconds = 30
offline_after_seconds = 90
```

## discoveryStatus

```text
blocked:
  当前不允许业务放行。
  它只表示“中心现在还没有给这台节点通过身份准入”，
  但不直接说明是“还没 verify”还是“身份发生变化待人工确认”，
  这部分要继续看 discoveryReason。

verified:
  已通过 heartbeat、Client /health、Client device-info、Docker 2375、架构一致性检查。
  它表示“中心当前确认这就是登记过的那台节点”，但仍需结合 healthStatus=healthy
  和 heartbeatStatus=online 才允许业务动作。
```

### discoveryStatus 对应的设备阶段

```text
blocked + discoveryReason="":
  设备已登记，但尚未完成准入验证。
  可以理解为“档案已建，尚未验明正身”。

verified:
  设备身份连续性已确认。
  可以理解为“这台机器就是中心认可的那台节点”。

blocked + discoveryReason=ip_mismatch/device_fact_changed:
  设备记录仍在，但身份连续性中断。
  可以理解为“像原来的节点，但住址或身份证明变了，不能自动放行”。
```

## discoveryReason

`discoveryReason` 不单独决定放行，它只解释 discoveryStatus 为什么处于当前结果。

```text
空字符串:
  没有额外身份异常原因。

ip_mismatch:
  新收到的 clientIp/baseUrl 与中心登记地址不一致。
  这不代表节点一定坏了，但中心不能自动覆盖原地址。
  当前实现里，这个判断依赖已登记节点至少有一个可比较的地址事实，
  例如 baseUrl 或 client_ip；如果历史记录里 client_ip 为空，
  单靠“新的 clientIp 变化”并不能总是稳定推导出 mismatch。

device_fact_changed:
  通过 /health、device-info、Docker 探测看到的设备事实变化过大，
  例如 arch、os、hostname、docker 环境与原记录明显不一致。
```

## lastDiscoveredAt

```text
lastDiscoveredAt:
  Node Server 最近一次通过 UDP discovery 或等价发现流程看到该节点入口的时间。

它只表示“最近一次被发现”的时刻，不表示：
  - verify 时间
  - /health 探测时间
  - 业务放行时间
```

## healthStatus

```text
healthy:
  Client /health healthy，device-info 可用，Docker 2375 可用。

unhealthy:
  Client 可达但关键检查失败。

stale:
  中心缓存不可信、心跳过期或动作前校验失败。

offline:
  Client 确认不可达。
```

## 业务动作状态表

| healthStatus | discoveryStatus | heartbeatStatus | 是否允许业务动作 |
| --- | --- | --- | --- |
| healthy | verified | online | 是 |
| healthy | verified | stale | 否 |
| healthy | verified | offline | 否 |
| healthy | blocked | online | 否 |
| unhealthy | verified | online | 否 |
| stale | verified | online | 否 |

## 节点治理动作矩阵

这一段专门回答“节点现在只能做什么”，供前端按钮和管理员操作口径直接复用。

| discoveryStatus | discoveryReason | 当前设备阶段 | 允许动作 | 禁止动作 |
| --- | --- | --- | --- | --- |
| `blocked` | `""` | 已登记但尚未完成准入验证 | `verify`、`refresh device-info`、查看 discovered/详情 | create env、run、stop、backup、restore、delete、import-package |
| `blocked` | `ip_mismatch` | 管理员待确认是否只是地址变化 | `confirm-address-update`、查看 discovered/详情 | `verify`、create env、run、stop、backup、restore、delete、import-package |
| `blocked` | `device_fact_changed` | 新地址或新探测结果看起来已不是原节点 | 查看详情、人工排查、必要时重新登记新节点 | `verify`、`confirm-address-update`、create env、run、stop、backup、restore、delete、import-package |
| `verified` | `""` | 身份连续性已确认 | create env、run、stop、backup、restore、delete、import-package | 无额外治理动作要求 |

补充说明：

- `refresh device-info` 只刷新 Docker 设备事实，不会把 `blocked` 自动恢复成 `verified`。
- `confirm-address-update` 只处理 `blocked + ip_mismatch`，成功后会立刻重跑完整探测，不是人工强制放行。
- `device_fact_changed` 当前没有单独的“强制恢复”接口，避免把另一台机器误绑回旧 `clientId`。

## 禁止规则

- 不能因为 UDP 在线就把 `healthStatus` 改成 healthy。
- 不能因为 Docker 2375 可达就把 `discoveryStatus` 改成 verified。
- 不能因为曾经 verified 就忽略 heartbeat offline。
- 不能把 `blocked + discoveryReason!= ""` 混成 `unhealthy` 或 `stale`；它表达的是身份连续性中断，不是本机能力异常。
- `blocked` 节点即使 `healthStatus=healthy` 且 `heartbeatStatus=online`，也必须阻止业务动作。
- 未 verified 的 Client 禁止创建环境包、run、stop、backup、restore、delete、import-package、RPA/CDP。
- `blocked + ip_mismatch` 不能继续点 `verify`，必须先走 `confirm-address-update`。
- `blocked + device_fact_changed` 不能继续点 `confirm-address-update`，必须先人工确认是不是已换机或设备重置。

## 2026-06-13 联调确认

- 删除已登记节点记录后，`GET /api/v1/edge-clients/discovered` 会返回 `clientId=""`，说明这是未绑定发现项。
- 恢复登记记录后，同一 discovery 项会重新补回原 `clientId`。
- 当已登记节点具备明确 `client_ip/baseUrl`，且收到不一致的 heartbeat/discovery 地址时，当前实现会进入：
  - `discoveryStatus=blocked`
  - `discoveryReason=ip_mismatch`
- `blocked + ip_mismatch` 状态下再次调用 verify，会被显式阻断，不会自动恢复为 verified。
