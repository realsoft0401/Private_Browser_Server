# Node Server 接口设计：`POST /api/v1/edge-clients/{clientId}/verify`

## 1. 功能目标

`POST /api/v1/edge-clients/{clientId}/verify` 用于把一个“已注册但尚未业务放行”的中心节点，推进到 `verified`。

verify 成功后，节点才允许参与：

- create env
- run
- stop
- backup
- restore
- delete
- import-package

## 2. 设计来源

- 用户明确要求必须区分：发现、注册、验证、业务放行。
- 不能因为节点已经被登记，就默认它可用于商业环境包动作。
- verify 必须按固定顺序检查 UDP 心跳、Client 健康、Client 设备信息、Docker 2375 和架构一致性。

## 3. 业务边界

### 3.1 负责什么

- 读取中心节点记录
- 动态计算 `heartbeatStatus`
- 调 Client `/health`
- 调 Client `/api/v1/edge/device-info`
- 调 Docker 2375
- 比对和归一化架构
- 失败时写 `lastError`
- 成功时写 `discoveryStatus=verified`

### 3.2 不负责什么

- 不自动注册节点
- 不自动更新 `baseUrl/clientIp`
- 不把 discovery 命中直接当 verify 成功
- 不允许带病放行业务动作

## 4. 固定检查顺序

verify 的顺序不能打乱：

1. `heartbeatStatus` 必须 `online`
2. Client `/health` 必须 `healthy`
3. Client `/api/v1/edge/device-info` 必须可读
4. Docker `2375` 必须可达
5. Client 架构和 Docker 架构必须一致且可归一化

## 5. 成功判定

全部通过后，中心节点更新为：

- `healthStatus=healthy`
- `discoveryStatus=verified`
- `lastError=""`
- `arch` 为 `amd64` 或 `arm64`

## 6. 失败判定

任意一步失败都必须中止，并且：

- 不允许把 `discoveryStatus` 改成 `verified`
- 更新 `healthStatus`
- 更新 `lastCheckedAt`
- 更新 `lastError`

## 7. 错误与日志规范

verify 失败必须至少留下：

- API 响应中的 `checks`
- `edge_clients.last_error`
- 服务端结构化日志

建议记录：

- `clientId`
- `baseUrl`
- `dockerApiUrl`
- `stage`
- `errorSource`
- `error`
- `nextAction`

## 8. 联调验收标准

至少覆盖：

- 正常 verify 成功
- heartbeat 不是 `online`
- Client `/health` 失败
- device-info 不可读
- Docker 2375 不可达
- Client 架构与 Docker 架构不一致

## 9. 当前实现状态

截至 `2026-06-12`：

- 已实现
- 是所有正式业务动作的统一节点放行门槛之一
