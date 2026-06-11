# Private_Browser_Server 状态文档

## Edge Client 状态

Node Server 的 Client 状态由三组字段共同决定，不允许混用。

| 字段 | 来源 | 含义 |
| --- | --- | --- |
| `healthStatus` | `/health`、device-info、Docker 2375 探测 | Client 和宿主能力健康 |
| `discoveryStatus` | Node Server 注册/verify 流程 | Client 是否完成中心验证 |
| `heartbeatStatus` | `last_heartbeat_at` 动态计算 | Node Server 最近一次确认收到该 Client 心跳的时间是否新鲜 |

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
manual:
  已手动注册，但还没有完成 verify。

verified:
  已通过 UDP 心跳、Client /health、Client device-info、Docker 2375、架构一致性检查。
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
| healthy | manual | online | 否 |
| unhealthy | verified | online | 否 |
| stale | verified | online | 否 |

## 禁止规则

- 不能因为 UDP 在线就把 `healthStatus` 改成 healthy。
- 不能因为 Docker 2375 可达就把 `discoveryStatus` 改成 verified。
- 不能因为曾经 verified 就忽略 heartbeat offline。
- 未 verified 的 Client 禁止创建环境包、run、stop、backup、restore、delete、import-package、RPA/CDP。
