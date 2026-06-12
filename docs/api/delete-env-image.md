# Node Server 接口设计：`DELETE /api/v1/envs/{envId}/del`

## 1. 功能目标

`DELETE /api/v1/envs/{envId}/del` 用于让 `Private_Browser_Server` 代理目标 Edge 删除当前环境包关联的运行镜像。

这个接口只处理镜像清理，不处理环境资产销毁。

## 2. 设计来源

- 用户已经明确要求 `/del` 与 `/package` 必须拆开。
- `/del` 只删运行镜像，不能误删环境目录、登录态目录或 SQLite 索引。
- 由于 Edge 当前 `/del` 是同步接口，Node Server 不应人为包成假异步任务。

## 3. 业务边界

### 3.1 负责什么

- 校验 env 和 client
- 调 Edge `/api/v1/edge/browser-envs/{envId}/del`
- 返回镜像删除结果
- 必要时把 warning 摘要回写中心 `lastError`

### 3.2 不负责什么

- 不创建中心 task
- 不修改 env 主状态
- 不删除环境目录
- 不删除 Edge SQLite 索引

## 4. 请求与响应

```http
DELETE /api/v1/envs/{envId}/del
```

同步返回：

- `envId`
- `clientId`
- `image`
- `imageRemoved`
- `results`
- `warningMessage`
- `deletedAt`

## 5. 前置校验

1. env 必须存在
2. client 必须通过 `EnsureClientReadyForBusiness`

## 6. 成功判定

- Edge 返回同步成功结果
- 即使 `imageRemoved=false`，也要结合 warning/message 区分是失败还是“镜像本来删不掉”

## 7. 失败与 warning

### 7.1 失败

- 节点不 ready
- Edge HTTP 调用失败
- Docker 删除动作明确失败

### 7.2 warning

- 同一镜像仍被其它环境引用
- Docker 返回不可删除摘要

当有 warning 时：

- 不改 env 主状态
- 但应回写 `lastError`，方便管理员排障

## 8. 联调验收标准

- 镜像可删时返回 `imageRemoved=true`
- 镜像被引用时返回 warning
- env 目录和中心状态不受影响

## 9. 当前实现状态

截至 `2026-06-12`：

- 已实现
- 保持同步调用，不创建中心 task
