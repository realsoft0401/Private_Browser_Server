# Node Server 接口设计：`GET /api/v1/server/tasks/{taskId}`

## 1. 功能目标

`GET /api/v1/server/tasks/{taskId}` 用于返回单个中心任务详情，并在需要时同步 Edge 当前任务状态。

这个接口的意义不是简单查库，而是：

- 以中心 task 作为平台级持久事实
- 在 Edge task 仍存在时，读取 Edge 当前状态
- 在 Edge task 丢失时，通过环境状态再次确认动作是否完成

## 2. 设计来源

- 用户明确要求：Client task 只是短期观察事实，Server task 才是平台持久事实。
- 用户明确要求：Edge task 丢失后不能默认成功，必须再校验 env 当前状态。

## 3. 业务边界

### 3.1 负责什么

- 查询 `server_tasks`
- 如任务尚未终态且存在 `edgeTaskId`，尝试读取 Edge task
- 必要时根据 env 当前状态确认任务终态
- 在成功或失败后刷新中心环境缓存

### 3.2 不负责什么

- 不替代 SSE
- 不做自动重试
- 不因为 Edge task 查询失败就自动重放资产动作

## 4. 返回结构

- `task`：中心持久事实
- `edge`：当前可读到的 Edge task 快照，可为空

## 5. 同步规则

### 5.1 不需要同步的情况

- `task.status` 已经是中心终态
- `edgeTaskId` 为空

### 5.2 需要同步的情况

- `task.status` 仍是 `pending/running`
- `edgeTaskId` 存在

此时会：

1. 查 Edge task
2. 把 Edge task 映射到中心状态
3. 如进入终态，再刷新 env 缓存

## 6. 当前成功确认矩阵

- `run_env`
  - Edge task 成功
  - 或 env `status=running`
- `stop_env`
  - Edge task 成功
  - 或 env 已具备停止事实
- `delete_env_package`
  - Edge task 成功
  - 或 env 已不存在 / 已 deleted
- `backup_env`
  - 设计上应由 env `status=backed_up` 确认

## 7. 失败判定

下面这些都必须统一记为中心失败：

- Edge task 查询失败，且无法确认 env 事实
- Edge task 不存在，且无法确认 env 事实
- Edge task 状态不可识别
- 中心 task 或中心 env 缓存回写失败

## 8. 中心价值

管理员应优先看这个接口，而不是只看 Edge 内存任务，因为：

- 这里保留主账号归属
- 保留操作者
- 保留最终 success/failed
- 能看到 Edge task 丢失后的中心收口结论

## 9. 联调验收标准

至少覆盖：

- Edge task 正常 success
- Edge task 正常 failed
- Edge task 丢失但 env 可确认成功
- Edge task 丢失且 env 不可确认成功
- 中心 env 收口失败
