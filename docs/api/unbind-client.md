# POST /api/v1/edge-clients/{clientId}/unbind

## 功能目标

删除某个正式已绑定节点的当前有效绑定结果，并尝试清理 Client 本地 `node-registration.json` 留痕。

> 当前文档定位：这是 `Private_Browser_Server` 的正式中心治理接口。
> 它负责“中心解绑收口”，不是发现接口，也不是环境包生命周期接口。

## 业务边界

- 负责根据 `clientId` 删除当前有效绑定结果
- 负责记录 unbind / clear-registration 审计
- 负责调用 Client `/api/v1/edge/node-registration/clear`
- 不负责删除节点历史任务
- 不负责删除历史审计
- 不负责删除任何 browser-env、slot 或本机业务资产

## 前置校验

- `clientId` 必填
- 目标节点必须存在
- 目标节点当前必须处于已绑定状态
- 请求体可为空；如传 body，必须是合法 JSON

## 状态流转

成功后：

- 当前有效绑定结果被删除
- 尝试清理 Client 本地 `node-registration.json`
- 后续同一台 Client 如果再次 bind，必须重新生成新的 `clientId`

失败后：

- 如果中心解绑失败，归属关系保持不变
- 如果中心解绑成功但 Client 本地 clear 失败，中心解绑仍成立，只把 clear 结果记为 failed

## 请求与响应

### 请求

```http
POST /api/v1/edge-clients/9060901190001/unbind
Content-Type: application/json
```

```json
{
  "source": "manual-unbind"
}
```

也允许空 body：

```http
POST /api/v1/edge-clients/9060901190001/unbind
```

### 成功响应

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "clientId": "9060901190001",
    "accountId": "906090119",
    "status": "unbound",
    "clearRegistrationStatus": "success",
    "unboundAt": 1718500001
  }
}
```

Client 本地清理失败但中心解绑成功：

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "clientId": "9060901190001",
    "accountId": "906090119",
    "status": "unbound",
    "clearRegistrationStatus": "failed",
    "clearRegistrationMessage": "request failed: Post \"http://127.0.0.1:3300/api/v1/edge/node-registration/clear\": context deadline exceeded",
    "unboundAt": 1718500001
  }
}
```

### 失败响应

目标当前未绑定：

```json
{
  "code": 1005,
  "message": "该 Client 当前未绑定，无需解绑"
}
```

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
  "message": "unbind request body 非法"
}
```

## SSE 说明

- 本接口不使用 SSE
- 原因：unbind 是一次同步中心治理动作，结果和本地 clear 收口都可以在当前响应里表达清楚

## 任务编排

当前接口不创建 `server_tasks`。

当前第一阶段按同步 HTTP 收口：

1. 读取中心节点
2. 中心删除当前有效绑定结果
3. 写 unbind 审计
4. 调用 Client clear 接口
5. 写 clear-registration 审计
6. 返回最终结果

## 成功判定

- 中心解绑成功
- 返回 `status=unbound`
- 如果本地 clear 也成功，则 `clearRegistrationStatus=success`

## 失败判定

- `clientId` 为空
- 目标节点不存在
- 目标节点当前未绑定
- 中心库更新失败

补充说明：

- `clearRegistrationStatus=failed` 不算接口失败，它只表示“中心解绑成功，但 Client 本地缓存清理失败”
- 这里的“中心解绑成功”应理解为“当前有效绑定结果删除成功”，不是“旧身份继续保留待复用”

## 日志字段

- `action=unbind`
- `action=clear_registration`
- `clientId`
- `accountId`
- `clientIp`
- `result`
- `message`

## 联调验收标准

- bind 成功后的节点，可以成功 unbind
- unbind 后当前有效绑定结果必须被删除
- unbind 后 Client 本地 `node-registration.json` 应被删除
- clear 失败时，不允许回滚中心 unbind
- 后续再次 bind 同一节点时，必须重新生成新的 `clientId`
