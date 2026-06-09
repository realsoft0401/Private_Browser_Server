# Private_Browser_Server 开发代理规范

## 当前职责定位

`Private_Browser_Server` 是私有浏览器云的中心调度层。它负责商业用户入口、多节点管理、任务编排、环境包聚合状态和审计留痕，不直接操作本机 Docker，也不保存浏览器真实登录态数据。

本目录下的 `project.md` 是 Server 子项目的阶段计划；根目录 `/Users/lining/Documents/Browser_virtualization/project.md` 仍是整个项目的最高源头。如两者冲突，以根目录 `project.md` 和根目录 `AGENTS.md` 为准。

## 与 Private_Browser_Client 的边界

- `Private_Browser_Client` 只作为边缘服务，负责本机 Docker、本机浏览器容器、本机环境包文件和 `/api/v1/edge/*`。
- `Private_Browser_Server` 才能实现用户、JWT、角色、节点注册、设备编号、多节点调度、全局任务和 Dashboard。
- Server 调用 Client 时必须通过 Edge API 和 API Key，不得绕过 Edge 直接读写 Edge 的 SQLite、profile、browser-data 或宿主机目录。
- 不要把 `/api/v1/auth/*`、`/api/v1/nodes/*`、用户表、节点表、雪花 ID、中心任务表重新加回 `Private_Browser_Client`。

## 设计原则

- 第一版目标是证明“一个中心入口可以安全管理多个 Edge 节点”，不是一次性完成所有商业化能力。
- 所有可查询状态必须来源 MySQL、Edge 心跳或 Edge API 响应，不要把关键状态只放在 Server 内存中。
- Server 只保存环境包索引、状态摘要、节点能力、任务和审计信息；不得保存 proxy 明文、fingerprint raw、browser-data、Cookies、Local Storage、IndexedDB 等敏感实体。
- 节点 CPU 架构必须在 Service 层归一化为 `amd64`、`arm64`、`unknown`，业务逻辑不得散写 `x86_64`、`aarch64` 等原始值。
- 镜像选择由后端 `ImagePolicy` 决定，前端或调用方不能随意传镜像字符串。
- `unknown` 架构禁止自动创建环境包，必须先完成 Docker 能力探测。

## 推荐分层

后续实现优先采用以下调用链路：

```text
Interfaces / Routes
  -> Service
  -> Dao
  -> Repository
  -> MySQL / Edge HTTP Client
```

- `Routes`：只做路由注册、参数绑定、调用 Service、返回统一响应。
- `Service`：做业务校验、JWT、RBAC、状态判断、任务编排、错误语义。
- `Dao`：保留业务动作入口，整理 Service 参数并协调 Repository。
- `Repository`：只处理 MySQL 访问、RowsAffected、查无记录归一化。
- `EdgeClient`：封装 Server 到 Edge 的 HTTP 请求、超时、API Key Header 和错误映射；不能做底层自动重试，避免资产类动作被静默重放。

## 注释要求

新增关键类型、常量、函数、状态流转和接口实现时，必须写清中文注释：

- 这段实现为什么出现，来自哪一阶段设计。
- 当前负责什么，不负责什么。
- 为什么这样设计，避免了哪些旧方案问题。
- 后续维护时哪些原则不能破坏。

涉及认证、JWT、密码、节点注册、Docker 能力探测、心跳、任务状态、环境包索引、镜像选择和审计日志的代码，必须有明确中文注释和可执行错误提示。

## V1 暂缓能力

- 自动跨节点迁移：放到 V1.1。
- 计费系统：放到 V1.2。
- Server 集群化：放到 V2.0。
- Marketplace / Webhook：放到 V3.0。
- 原始 CDP 命令网关：风险高，先只做 Level 1 安全原子动作。
