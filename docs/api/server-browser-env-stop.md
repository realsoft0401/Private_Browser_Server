# Server Browser Env Stop

这份文档只说明中心正式接口：

- `POST /api/v1/browser-envs/{envId}/stop`

---

## 1. 业务语义

由 Node Server 发起一次中心 browser-env stop。

它不是只把请求转发给 Edge 就结束，而是要：

- 读取中心 env 聚合记录
- 校验目标节点仍然允许被中心安全访问
- 同步调用目标 Edge stop
- 把停止后的 env 摘要重新同步回中心缓存
- 同时落一条 `server_task` 审计事实

---

## 2. 它负责什么

- 读取 `server_browser_envs` 中心聚合记录
- 校验目标 env 当前绑定到哪个 `clientId`
- 校验目标节点当前 `healthy + verified`
- 透传正式 `timeoutSeconds` 给目标 Edge stop
- 创建 `server_tasks`
- 在成功或失败后回写 `server_browser_envs.lastTaskId / lastError / lastSyncedAt`

---

## 3. 它不负责什么

- 不自动重试 stop
- 不做强制 kill 扩展参数
- 不跨 Client stop
- 不直接修改 slot 目标值
- 不通过 stop 删除 env 资产

---

## 4. 当前请求体口径

```json
{
  "timeoutSeconds": 10
}
```

当前正式只收这一个字段：

- `timeoutSeconds`
  - 选填
  - 默认按 Client stop 正式协议收口
  - 表示 Docker 优雅停止等待秒数

明确不允许：

- `slotId`
- `force`
- `clientId`
- 任意 Docker 参数透传

---

## 5. 前置条件

必须同时满足：

1. `server_browser_envs` 能查到这条 env
2. env 已绑定到某个正式 `clientId`
3. 目标节点当前 `healthStatus=healthy`
4. 目标节点当前 `discoveryStatus=verified`

这里刻意不复用 run admission 的 slot/quota 约束。

原因：

- stop 是收口动作，不是新的运行准入动作
- 不能因为 quota 已过期或 slot 摘要短时漂移，反而连 stop 都做不了

---

## 6. 状态机与收口

### 中心任务

- 发起 stop 时，会同步创建一条 `server_tasks`
- 任务终态只允许：
  - `success`
  - `failed`

### 中心 env 缓存

成功后必须把 `server_browser_envs` 至少收口到：

- `status=stopped`
- `containerStatus=missing`
- `runtimeStatus=stopped`
- `currentSlotId=''`
- `cdpUrl=''`
- `webVncUrl=''`

### 成功判定

要同时满足：

1. Edge stop 同步返回成功
2. Node 能再次读取 Edge `browser-env detail`
3. Node 能把新的停止后摘要同步回 `server_browser_envs`

### 失败判定

任一环节失败都必须 failed，包括：

- 中心 env 不存在
- 目标节点不可达
- 节点不是 `healthy + verified`
- Edge stop 返回失败
- Edge stop 成功但中心无法再次确认 env 事实

---

## 7. SSE 说明

- 本接口不使用 SSE
- 原因：当前 stop 是短链路、同步收口动作
- 发起后直接等待最终结果返回即可

补充说明：

- 虽然本接口不要求调用方订阅 SSE
- 但中心内部仍会创建并持久化一条 `server_task` 审计记录
- 这样 run / stop / backup / restore 最终都能统一落到中心任务事实源

---

## 8. 与相近接口的边界

它不会替代：

- `POST /api/v1/browser-envs/{envId}/run`
  - run 是新的运行准入与执行动作
- `POST /api/v1/edge/browser-envs/{envId}/stop`
  - 这是 Edge 本机正式执行接口，不是中心接口
- `POST /api/v1/browser-envs/{envId}/refresh`
  - refresh 只同步摘要，不发 stop 动作
