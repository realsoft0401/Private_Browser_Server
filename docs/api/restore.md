# Node Server 接口设计：`POST /api/v1/envs/{envId}/restore`

## 1. 功能目标

`POST /api/v1/envs/{envId}/restore` 规划用于让 `Private_Browser_Server` 代理目标 Edge，从本机已登记备份包恢复环境目录。

成功后的业务结论应是：

- Edge 已恢复环境目录
- env 主状态回到 `created`
- 备份包按 Edge 规则被删除或清理
- 调用方下一步可显式执行 `run`

## 2. 设计来源

- `backup` 已被重新定义为“归档并释放运行目录”，所以必须有与之配对的 `restore`。
- 用户明确要求 restore 不应重新上传同一份文件，而应复用 Edge 本机 `backupPath`。

## 3. 业务边界

### 3.1 负责什么

- 校验 env 与 client
- 校验 env 当前应处于 `backed_up/archived`
- 创建中心 task
- 调用 Edge restore
- 把中心缓存收口回 `created`

### 3.2 不负责什么

- 不自动 run
- 不跨节点恢复
- 不从 Node Server 上传备份包到 Edge

## 4. 前置校验

规划中建议：

1. env 必须存在
2. client 必须通过 `EnsureClientReadyForBusiness`
3. 读取 Edge env detail
4. 仅允许 `status=backed_up` 或等价备份态进入 restore

## 5. 任务编排

建议采用中心 task：

- `taskType=restore_env` 或未来补充对应常量
- 创建中心 task
- 调 Edge `POST /api/v1/edge/browser-envs/{envId}/restore`
- 绑定 `edgeTaskId`

## 6. 成功判定

规划建议：

- Edge task 成功，或
- Edge task 丢失但再次读取 env detail 后，确认 `status=created`

## 7. 失败判定

- 节点不 ready
- env 不是备份态
- Edge restore 失败
- Edge task 丢失且无法确认 `created`

## 8. 中心缓存收口

成功后应更新：

- `status=created`
- `containerStatus=unknown`
- `monitorStatus=unknown`
- 清空运行期连接入口或按 Edge detail 刷新
- `lastTaskId`
- `lastError=""`

## 9. 错误与日志规范

应沿用 backup 的管理员留痕规则：

- `server_tasks.error_message`
- `env.last_error`
- task SSE
- 结构化日志

## 10. 当前实现状态

截至 `2026-06-12`：

- 尚未落地
- 已进入正式生命周期代理规划范围
- 后续应作为 [backup.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/backup.md) 的配对接口一起实现
