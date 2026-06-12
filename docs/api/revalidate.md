# Node Server 接口设计：`POST /api/v1/envs/{envId}/revalidate`

## 1. 功能目标

`POST /api/v1/envs/{envId}/revalidate` 规划用于让 `Private_Browser_Server` 在管理员完成修复后，重新校验一个异常环境是否允许重新进入正常生命周期。

它的目标不是运行环境，而是恢复“准入资格”。

## 2. 设计来源

- 用户已经明确：`status=error` 的环境不能被 `run/stop/backup/proxy update` 隐式恢复。
- 必须存在一个独立、受控、可审计的重新准入动作。

## 3. 业务边界

### 3.1 负责什么

- 创建中心 task
- 调用 Edge `revalidate`
- 根据 Edge 返回结果，把中心缓存从异常态恢复到允许继续生命周期的正常态

### 3.2 不负责什么

- 不启动容器
- 不拉镜像
- 不替代 `restore`
- 不替代 `run`
- 不修复登录态本身

## 4. 前置校验

规划建议：

1. env 必须存在
2. client 必须通过 `EnsureClientReadyForBusiness`
3. 当前 env 应处于 `error` 或等价异常态

## 5. 任务编排

建议采用中心 task：

- 创建中心 task
- 调 Edge `POST /api/v1/edge/browser-envs/{envId}/revalidate`
- 绑定 `edgeTaskId`

## 6. 成功判定

规划建议：

- Edge task 成功，或
- Edge task 丢失但再次读取 env detail 后，确认环境已回到允许动作的正常状态

## 7. 失败判定

- 节点不 ready
- env 当前并非异常态
- Edge revalidate 失败
- Edge task 丢失且无法确认准入恢复事实

## 8. 中心缓存收口

成功后至少更新：

- `status`
- `containerStatus`
- `monitorStatus`
- `lastTaskId`
- `lastError`

其中正常回归后的具体状态，应以 Edge 实际返回为准。

## 9. 当前实现状态

截至 `2026-06-12`：

- 尚未落地
- 已进入正式生命周期代理规划范围
