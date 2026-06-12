# Node Server 接口设计：`DELETE /api/v1/envs/{envId}/package`

## 1. 功能目标

`DELETE /api/v1/envs/{envId}/package` 用于让 `Private_Browser_Server` 发起一次“彻底销毁环境包资产”的中心任务。

该接口成功后，目标 Edge 上应删除：

- 环境目录
- `browser-data/profile`
- 已停止容器
- Edge 本地 SQLite 索引

中心层则保留一条 `status=deleted` 的聚合历史。

## 2. 设计来源

- 用户要求 `/package` 是真正的资产销毁动作，不得与 `/del` 混淆。
- Edge 物理删除后，Node Server 不可能再从 Edge 拉到 env detail，因此中心层必须有自己的 deleted 收口策略。

## 3. 业务边界

### 3.1 负责什么

- 校验 env 与 client
- 创建中心 task
- 调 Edge `/package`
- 绑定 `edgeTaskId`
- 在 Edge 详情已消失的前提下，仍把中心状态收口为 `deleted`

### 3.2 不负责什么

- 不保留可恢复资产
- 不自动 backup
- 不自动跨节点搬运环境

## 4. 请求与响应

```http
DELETE /api/v1/envs/{envId}/package
```

立即返回中心任务摘要：

- `taskId`
- `taskType=delete_env_package`
- `clientId`
- `envId`
- `edgeTaskId`
- `eventsUrl`

## 5. 前置校验

1. env 必须存在
2. client 必须通过 `EnsureClientReadyForBusiness`
3. 调用方必须清楚这是不可逆资产动作

## 6. 任务编排

```text
package delete request
  -> 创建中心 task(delete_env_package)
  -> 调 Edge /package
  -> 绑定 edgeTaskId
  -> task detail 轮询 Edge task
  -> finalize
```

## 7. 成功判定

下面两种情况之一成立即可：

1. Edge task 明确 `success`
2. Edge task 丢失，但再次确认时：
   - Edge env detail 不存在
   - 或能确认 env 已进入 `deleted`

## 8. 中心缓存收口

成功后必须显式收口：

- `status=deleted`
- `containerStatus=missing`
- `monitorStatus=unknown`
- `cdpUrl=""`
- `webVncUrl=""`
- `lastTaskId=当前中心 taskId`
- `lastError=""`

## 9. 失败判定

- 节点不 ready
- Edge `/package` 调用失败
- Edge task failed
- Edge task 丢失且无法确认环境已删除
- 中心 deleted 收口失败

## 10. 错误与日志规范

至少保留：

- `server_tasks.error_message`
- `env.last_error`
- task SSE
- 服务端结构化日志

建议记录：

- `taskId`
- `taskType=delete_env_package`
- `clientId`
- `envId`
- `edgeTaskId`
- `stage`
- `errorSource`
- `error`
- `suggestion`

## 11. 联调验收标准

- 正常删除成功
- 删除成功后中心仍保留 `status=deleted`
- Edge task 丢失但环境已不存在时仍能成功收口
- 删除失败时管理员可在任务详情、env 摘要、日志中看到一致错误
