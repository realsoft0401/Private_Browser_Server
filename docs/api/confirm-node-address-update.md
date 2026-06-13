# Node Server 接口设计：`POST /api/v1/edge-clients/{clientId}/confirm-address-update`

## 1. 功能目标

`POST /api/v1/edge-clients/{clientId}/confirm-address-update` 用于处理“管理员已经确认这还是原节点，只是接入地址变了”的恢复动作。

它解决的是：

- 节点已进入 `blocked + ip_mismatch`
- 原 `clientId` 不能变
- 但 `baseUrl/clientIp/dockerApiUrl` 需要改到新的地址
- 改完后必须重新完整探测，不能直接人工放行

## 2. 设计来源

- 用户已经明确，`ip_mismatch` 不能靠 discovery 自动覆盖旧地址。
- 同时也不能让管理员通过手工改库恢复节点，否则文档、接口和审计都会失真。
- 因此需要一个受控接口，把“管理员确认同一台节点”与“重新完整 verify”收敛成一次正式动作。

## 3. 业务边界

### 3.1 负责什么

- 只处理 `blocked + ip_mismatch`
- 接收新的 `baseUrl/clientIp/dockerApiUrl`
- 保持原 `clientId` 不变
- 立即重新执行 heartbeat、`/health`、`device-info`、Docker 2375、架构校验
- 成功时恢复 `verified`
- 设备事实明显变化时继续保持 `blocked + device_fact_changed`

### 3.2 不负责什么

- 不处理普通 `blocked`
- 不处理 `device_fact_changed`
- 不允许人工直接传入 `verified`
- 不修改历史任务、环境包绑定和审计里的 `clientId`

前端按钮口径：

- 只有 `blocked + ip_mismatch` 才应显示这个按钮
- `verified`、`blocked + ""`、`blocked + device_fact_changed` 都不应显示

## 4. 请求与响应

```http
POST /api/v1/edge-clients/{clientId}/confirm-address-update
```

必须带 Platform Header。

请求体：

```json
{
  "baseUrl": "http://192.168.10.120:3300",
  "clientIp": "192.168.10.120",
  "dockerApiUrl": "http://192.168.10.120:2375"
}
```

成功返回重点：

- `data.client.clientId` 仍是原值
- `data.client.baseUrl/clientIp/dockerApiUrl` 已切到新地址
- `data.client.discoveryStatus`
- `data.client.discoveryReason`
- `data.checks`

## 5. 前置校验

必须同时满足：

- 节点存在且属于当前主账号
- `discoveryStatus=blocked`
- `discoveryReason=ip_mismatch`
- 新 `baseUrl` 合法
- 新 `dockerApiUrl` 合法

必须拒绝：

- `verified` 节点直接走这个接口
- `blocked + discoveryReason=""`
- `blocked + device_fact_changed`
- 新 `baseUrl` 已被其它活动节点占用

## 6. 状态流转

### 6.1 成功路径

1. 管理员确认原节点只是换了地址
2. Server 更新内存中的 `baseUrl/clientIp/dockerApiUrl`
3. 立即重跑完整探测
4. 全部通过
5. 持久化为：
   - `discoveryStatus=verified`
   - `discoveryReason=""`
   - `healthStatus=healthy`

### 6.2 失败路径

如果新地址可达但关键探测失败：

- `discoveryStatus=blocked`
- `discoveryReason=""`
- `healthStatus` 按失败阶段写 `stale/unhealthy`
- `lastError` 写明失败原因与修复建议

如果新地址虽然可达，但设备事实与原节点差异过大：

- `discoveryStatus=blocked`
- `discoveryReason=device_fact_changed`
- `lastError` 写明差异字段

## 7. 成功判定

- 原 `clientId` 保持不变
- 新地址写入成功
- 完整探测通过
- 节点恢复为 `verified`

## 8. 失败判定

- 前置状态不是 `blocked + ip_mismatch`
- 新地址非法
- 新地址探测失败
- 设备事实变化过大
- SQLite 回写失败

## 9. 错误与日志规范

失败时至少保留：

- API 响应里的 `checks`
- `edge_clients.last_error`
- 服务端结构化日志

建议记录：

- `clientId`
- `previousBaseUrl`
- `newBaseUrl`
- `previousClientIp`
- `newClientIp`
- `discoveryReason`
- `stage`
- `error`
- `nextAction`

## 10. 联调验收标准

- `blocked + ip_mismatch` 节点能通过该接口恢复
- 恢复后 `clientId` 不变
- 新地址健康时能回到 `verified`
- 新地址健康但设备事实变化过大时会进入 `blocked + device_fact_changed`
- 非 `ip_mismatch` 节点调用时被明确拒绝
