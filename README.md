# Private_Browser_Server

`Private_Browser_Server` 当前定位为 **节点管理与自动化调度 Server**。

它负责把多个 `Private_Browser_Client` 边缘节点统一纳入管理，维护节点健康、设备能力、环境包聚合状态、Server 任务、镜像策略，并作为后续 RPA / CDP 操作数据的中心来源、下发入口和审计入口。

它不是最终客户登录系统，不负责客户账号密码、套餐、计费或业务订单。这些能力属于更上层业务平台。

2026-06-09 demo 口径：Node Server 可以跑在 RK3528 4G+64G 这类轻量控制设备上，只部署 Node Server、前端静态资源和 SQLite；浏览器容器、VNC/CDP、代理和 RPA 实际执行全部放在 x86 `Private_Browser_Client` 服务器。

## 当前阶段

当前阶段为 **Node Server V1.0 最小闭环开发**。

2026-06-09 起，V1 demo 需求拆解以 [task0609-node-server.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/task0609-node-server.md) 为当前执行清单。

V1.0 的目标不是一次性完成完整 SaaS 平台，也不是先做用户登录数据库，而是先证明：

```text
业务平台 / 管理端 / Apifox
  -> Private_Browser_Server(Node Server)
  -> 指定并校验 Edge Client
  -> 调用 Private_Browser_Client /api/v1/edge/*
  -> 聚合节点、环境包、任务、RPA/CDP 操作状态
```

V1 完成后，上层业务平台或管理端不再直接调用 Edge 创建环境包或下发自动化动作，而是通过 Node Server 完成节点校验、生命周期代理、RPA/CDP 操作下发和任务追踪。

## V1.0 必须能力

| 子系统 | 能力 | 验收口径 |
|--------|------|----------|
| Settings / SQLite / Repository | 配置读取、本地 SQLite、基础表 | 服务启动后可检查 SQLite 连接和基础表 |
| Node | 节点注册、Docker 探测、心跳接收、verified 状态机 | 节点只有 `healthy + verified` 才能承接业务动作 |
| EdgeClient | 调用 Edge API | 统一超时、错误映射、API Key Header；不做底层自动重试 |
| Env | 代理环境包创建、启动、停止、详情、列表 | 管理端只调用 Server API 即可完成单节点生命周期 |
| Task | Server 任务表、Edge taskId 绑定、状态刷新 | 耗时动作能在 Server 侧看到最终结果 |
| RPA / CDP Task | 保存受控操作数据、下发到指定节点、记录结果摘要 | 操作数据来自 Server，执行发生在 Edge，结果可审计 |
| ImagePolicy | 按架构选择镜像 | `unknown` 架构禁止自动创建环境包 |
| Dashboard | 最小统计 | 可查看节点数、环境包数、运行数、失败任务数 |

Auth / JWT 如果后续保留，只作为 Node Server 自身的管理员保护或上层平台调用保护，不作为最终客户账号密码数据库。

V1 demo 的用户上下文来自 PlatformServer 登录结果。前端调用 Node Server 时携带：

```text
X-Main-Account-Id
X-Platform-User-Id
X-Platform-Username
X-Platform-Role
```

Node Server V1 先记录这些 Header 做任务归属和审计，不实时回调 PlatformServer 校验 token。后续 V1.1/V2 再接 `verify-token`。

## 2026-06-09 阶段 4 当前落地

- `EdgeClient` 已实现一次性 HTTP JSON 调用、API Key Header、统一响应解析和 Edge 错误映射。
- `EdgeClient` 明确不做底层自动重试；即使配置里保留 `edge.retry_times`，资产动作失败也必须由用户或管理员重新发起新的 Server task。
- `Service/Task` 已增加 Edge task 到 Server task 的终态归一化：`success/done` 才能成功，`failed/error` 或 Edge task 丢失统一失败，不能默认成功。
- 完整 Env/Node/Task 落库调度仍待 SQLite Repository 接入后实现；当前不临时开放 `baseUrl` 透传入口，避免绕过 `clientId + healthy + verified` 规则。
- 文档已明确：当前 Server 是节点管理与自动化调度层，不是最终客户登录系统。
- 文档已明确：Node Server 使用本地 SQLite 管理节点侧调度数据；上层平台管理端才使用 MySQL。

## 建议目录

```text
Private_Browser_Server/
  agent.md
  README.md
  project.md
  Settings/
  Models/
  Interfaces/
  Service/
  Dao/
  Repository/
  EdgeClient/
  Rom/
  data/
  docs/
```

## 与 Edge 的接口关系

Server 只通过 Edge API 调用本机能力：

```text
POST /api/v1/edge/browser-envs
GET  /api/v1/edge/browser-envs
GET  /api/v1/edge/browser-envs/:envId
POST /api/v1/edge/browser-envs/:envId/run
POST /api/v1/edge/browser-envs/:envId/stop
POST /api/v1/edge/browser-envs/:envId/backup
POST /api/v1/edge/browser-envs/:envId/restore
POST /api/v1/edge/browser-envs/:envId/revalidate
POST /api/v1/edge/browser-envs/import-package
DELETE /api/v1/edge/browser-envs/:envId
GET  /api/v1/edge/tasks/:taskId
GET  /api/v1/edge/tasks/:taskId/events
```

Server 不直接读写 Edge 的 SQLite、Docker socket、环境包目录或 browser-data。

RPA / CDP 操作同样遵守这个边界：Server 可以保存和下发受控操作步骤、参数和任务元数据，但不能直接读取或保存 Cookies、Local Storage、IndexedDB、Session Storage、Login Data、proxy 明文、fingerprint raw 或 `browser-data/profile`。

## 第一验收目标

完成下面的闭环即可视为 V1 第一阶段可演示：

1. 注册或手动加入一个 Edge Client并完成 Docker 能力探测。
2. 节点进入 `healthy + verified`。
3. Edge 按节点凭证上报心跳。
4. 通过 Server 在指定节点创建一个环境包。
5. 通过 Server 启动环境包，并看到 Server task 绑定 Edge task 后完成。
6. 通过 Server 查询环境包详情，拿到 CDP / WebVNC 地址摘要。
7. 通过 Server 下发一个受控 RPA/CDP 测试动作，并记录执行结果摘要。
8. 停止环境包，Dashboard 状态同步变化。
