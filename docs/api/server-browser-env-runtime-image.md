# Server Browser Env Runtime Image

这份文档说明中心正式接口：

- `PATCH /api/v1/browser-envs/{envId}/runtime-image`

当前状态：已实现并已回归。

---

## 1. 业务语义

由 Node Server 发起一次 browser-env 正式运行镜像地址修改。

它不是 Docker 镜像拉取接口，也不是 slot 基础镜像升级接口。

它只表达：

- 修改某个 browser-env 后续 run 使用的 `runtime.image`
- 仅当中心 env 状态为 `created` 或 `stopped` 时允许修改
- 修改成功后不自动 run、不自动 pull image、不自动 reinit slot

---

## 2. 它负责什么

- 读取 `server_browser_envs` 中心聚合记录
- 确认 env 绑定的 `clientId`
- 确认中心缓存中 env 当前处于 `waiting`
- 调用目标 Edge：
  - `PATCH /api/v1/edge/browser-envs/{envId}/runtime-image`
- Edge 修改成功后刷新中心 env 摘要
- 记录中心任务或操作日志

---

## 3. 它不负责什么

- 不自动 run
- 不自动 pull image
- 不自动 reinit slot
- 不修改 slot 默认基础镜像
- 不修改 slot 当前实际 `runtimeImage`
- 不删除旧镜像
- 不接受 proxy / fingerprint / Docker 参数

---

## 4. 请求体

```json
{
  "image": "crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_edge:1.2-amd64"
}
```

字段：

- `image`
  - 必填
  - 完整 Docker image 引用
  - Node 不拆 tag，不拼接默认值

明确不允许：

- `clientId`
- `slotId`
- `force`
- `proxy`
- `fingerprint`
- 任意 Docker HostConfig 字段

---

## 5. 前置条件

必须同时满足：

1. `server_browser_envs` 能查到 env
2. env 已绑定到正式 `clientId`
3. 目标节点 `healthStatus=healthy`
4. 目标节点 `discoveryStatus=verified`
5. 中心 env 当前状态为 `created` 或 `stopped`

当前实现补充：

- `created` 表示首次运行前配置态，还没有挂载 slot 或运行容器
- `stopped` 表示运行后已经释放 slot/container 关系，是配置与容器隔离后的干净态

必须拒绝：

- `loading`
- `running`
- `ending`
- `backed_up`
- `deleted`
- `error`

关键原则：

- 只有 `created` 或 `stopped` 可以修改运行镜像。
- 修改后不代表镜像已经拉取或容器已经切换。
- 下一次 `run` 才会使用新的 `runtime.image`。

---

## 6. SSE 说明

不使用 SSE。

原因：

- 这是短链路配置修改动作。
- 不执行 run、不拉镜像、不重建容器。
- 同步 HTTP 足够表达成功或失败。

---

## 7. 与相近接口的边界

- `POST /api/v1/edge-clients/{clientId}/run-quota/refresh`
  - 管额度，不管镜像
- `POST /api/v1/edge-clients/{clientId}/target-slot-count`
  - 管目标 slot 数，不管 browser-env 镜像
- `POST /api/v1/edge-clients/{clientId}/slot-reconcile`
  - 刷新 slot 事实，不改镜像
- `DELETE /api/v1/browser-envs/{envId}/del`
  - 清理当前 env 关联镜像，不改 `runtime.image`
- `POST /api/v1/browser-envs/{envId}/run`
  - 使用当前 `runtime.image` 运行，不允许请求体临时覆盖 image

---

## 8. 回归记录

- 2026-07-01：使用远端 Client `192.168.111.119:3300` 和本地 Node `127.0.0.1:3400` 完成回归
- 测试镜像：`crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_edge:1.2-amd64`
- Node 中心接口可成功转发到 Edge runtime-image 修改接口
- `created/stopped` 修改准入已按最终口径收口
- `running` 状态修改会被拒绝
- 修改后不自动 run、不自动 pull image、不自动 reinit slot
- 下一次 Client run 已确认读取新 `runtime.image`
- stop + slot reinit 后，env 保持 `stopped` 且 slot 回到空白 `waiting`
