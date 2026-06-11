# Private_Browser_Server 开发代理规范

## 当前职责定位

`Private_Browser_Server` 当前定位为节点管理与自动化调度 Server，而不是最终客户登录系统。

它负责管理多个 `Private_Browser_Client` 节点、维护节点健康和设备能力、聚合环境包状态、编排生命周期任务，并作为后续 RPA / CDP 操作数据的中心来源、下发入口和审计入口。它不直接操作本机 Docker，也不保存浏览器真实登录态数据。

2026-06-09 demo 口径：

- Node Server 可以部署在 RK3528 4G+64G 控制设备上。
- RK3528 只跑 Node Server、前端静态资源和 SQLite。
- 所有 `Private_Browser_Client` 都部署在 x86 服务器上，负责 Docker、Chromium、VNC/CDP、代理和 RPA 实际执行。
- Node Server 不运行浏览器容器，不把本机 Docker 当浏览器运行节点。
- PlatformServer 的 slot、机位、Redis 商业授权闭环放入 V2；Node Server V1 不实现这些商业授权逻辑。

本目录下的 `project.md` 是 Server 子项目的阶段计划；根目录 `/Users/lining/Documents/Browser_virtualization/project.md` 仍是整个项目的最高源头。如两者冲突，以根目录 `project.md` 和根目录 `AGENTS.md` 为准。

## 与 Private_Browser_Client 的边界

- `Private_Browser_Client` 只作为边缘服务，负责本机 Docker、本机浏览器容器、本机环境包文件和 `/api/v1/edge/*`。
- `Private_Browser_Server` 负责节点注册、节点设备编号、节点健康状态、环境包中心聚合、Server 任务、镜像策略、RPA/CDP 操作任务和审计留痕。
- Server 调用 Client 时必须通过 Edge API 和受控节点凭证，不得绕过 Edge 直接读写 Edge 的 SQLite、profile、browser-data 或宿主机目录。
- 最终客户登录、套餐、计费、客户权限和业务订单属于更上层业务平台，不是当前 Node Server 的第一阶段职责。
- 不要把节点管理、中心任务、RPA/CDP 调度能力加回 `Private_Browser_Client`。
- V1 demo 中，前端先登录 PlatformServer，再把 `X-Main-Account-Id`、`X-Platform-User-Id`、`X-Platform-Username`、`X-Platform-Role` 传给 Node Server。Node Server 只记录这些 Header 做归属和审计，不把它们扩展成自己的客户登录体系。

## 设计原则

- 第一版目标是证明“一个Node Server 可以安全管理多个 Edge Client，并统一下发生命周期与自动化任务”，不是一次性完成客户登录系统或完整 SaaS 能力。
- Node Server 自己的节点、环境包聚合、任务、RPA/CDP 操作和审计状态使用本地 SQLite 管理；上层平台管理端才使用 MySQL 保存客户、套餐、业务订单和跨节点业务视图。
- 所有可查询状态必须来源 Node Server SQLite、Edge 心跳或 Edge API 响应，不要把关键状态只放在 Server 内存中。
- Server 只保存环境包索引、状态摘要、节点能力、生命周期任务、RPA/CDP 操作任务、结果摘要和审计信息；不得保存 proxy 明文、fingerprint raw、browser-data、Cookies、Local Storage、IndexedDB 等敏感实体。
- RPA/CDP 操作数据可以由 Server 保存和下发，但必须是受控任务、步骤、参数摘要和结果摘要，不能把浏览器登录态、代理明文或指纹 raw 写入 Server。
- 节点 CPU 架构必须在 Service 层归一化为 `amd64`、`arm64`、`unknown`，业务逻辑不得散写 `x86_64`、`aarch64` 等原始值。
- 镜像选择由后端 `ImagePolicy` 决定，普通前端或用户不能随意传镜像字符串；当前允许 Platform 受控下发 `imagePolicy` 值，过渡期该值暂时直接等于已登记镜像字符串，Node Server 仍必须校验它是否属于受控策略。
- `unknown` 架构禁止自动创建环境包，必须先完成 Docker 能力探测。

## 推荐分层

后续实现优先采用以下调用链路：

```text
Interfaces / Routes
  -> Service
  -> Dao
  -> Repository
  -> SQLite / Edge HTTP Client
```

- `Routes`：只做路由注册、参数绑定、调用 Service、返回统一响应。
- `Service`：做节点状态判断、任务编排、Edge 调用编排、RPA/CDP 操作准入、错误语义。
- `Dao`：保留业务动作入口，整理 Service 参数并协调 Repository。
- `Repository`：只处理 SQLite 访问、RowsAffected、查无记录归一化。
- `EdgeClient`：封装 Server 到 Edge 的 HTTP 请求、超时、API Key Header 和错误映射；不能做底层自动重试，避免资产类动作被静默重放。

## 注释要求

新增关键类型、常量、函数、状态流转和接口实现时，必须写清中文注释：

- 这段实现为什么出现，来自哪一阶段设计。
- 当前负责什么，不负责什么。
- 为什么这样设计，避免了哪些旧方案问题。
- 后续维护时哪些原则不能破坏。

涉及节点注册、Docker 能力探测、心跳、任务状态、环境包索引、镜像选择、RPA/CDP 操作下发和审计日志的代码，必须有明确中文注释和可执行错误提示。

如果后续增加管理员登录或 API Key，它只服务Node Server 自身的运维保护，不能把当前项目重新改成最终客户账号密码数据库。

## 文件切割规则

- 任何单个 `.go` 文件一旦超过 `900` 行，必须优先切割重构，不能继续在原文件里无边界堆叠功能。
- 切割时优先按职责边界拆分，例如 `http` 入口、请求校验、任务编排、外部调用、错误映射、类型定义分开。
- 新增功能如果会把文件推到 `900` 行以上，必须先拆文件再继续写，不允许“先写完再说”。

## V1 暂缓能力

- 自动跨节点迁移：放到 V1.1。
- 最终客户登录、套餐、计费和业务订单：属于上层业务平台，当前 Node Server V1 暂缓。
- Server 集群化：放到 V2.0。
- Marketplace / Webhook：放到 V3.0。
- 原始 CDP 命令网关：风险高，先只做受控 RPA/CDP 原子动作。
