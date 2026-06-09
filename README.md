# Private_Browser_Server

`Private_Browser_Server` 是私有浏览器云的中心调度服务，负责把多个 `Private_Browser_Client` 边缘节点统一成一个可认证、可调度、可审计的商业化入口。

## 当前阶段

当前阶段为 **Server V1.0 最小闭环开发**。

V1.0 的目标不是一次性完成完整 SaaS 平台，而是先证明：

```text
用户 / 前端 / Apifox
  -> Private_Browser_Server
  -> 选择合适 Edge 节点
  -> 调用 Private_Browser_Client /api/v1/edge/*
  -> 聚合环境包、任务、节点健康状态
```

V1 完成后，客户不再直接调用 Edge 创建环境包，而是通过 Server 入口完成创建、启动、停止、查询和任务追踪。

## V1.0 必须能力

| 子系统 | 能力 | 验收口径 |
|--------|------|----------|
| Auth | 注册、登录、JWT、角色字段 | 管理员可创建用户；普通用户只能查看自己的资源 |
| Node | 节点注册、Docker 探测、心跳接收 | 节点离线 90 秒内转为 `unhealthy`，恢复后回到 `healthy` |
| EdgeClient | 调用 Edge API | 统一超时、错误映射、API Key Header；不做底层自动重试 |
| Env | 代理环境包创建、启动、停止、详情、列表 | 前端只调用 Server API 即可完成单节点生命周期 |
| Task | Server 任务表、Edge taskId 绑定、状态刷新 | 耗时动作能在 Server 侧看到最终结果 |
| ImagePolicy | 按架构选择镜像 | `unknown` 架构禁止自动创建环境包 |
| Dashboard | 最小统计 | 可查看节点数、环境包数、运行数、失败任务数 |

## 2026-06-09 阶段 4 当前落地

- `EdgeClient` 已实现一次性 HTTP JSON 调用、API Key Header、统一响应解析和 Edge 错误映射。
- `EdgeClient` 明确不做底层自动重试；即使配置里保留 `edge.retry_times`，资产动作失败也必须由用户或管理员重新发起新的 Server task。
- `Service/Task` 已增加 Edge task 到 Server task 的终态归一化：`success/done` 才能成功，`failed/error` 或 Edge task 丢失统一失败，不能默认成功。
- 完整 Env/Node/Task 落库调度仍待 MySQL Repository 接入后实现；当前不临时开放 `baseUrl` 透传入口，避免绕过 `nodeId + healthy + verified` 规则。

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

## 第一验收目标

完成下面的闭环即可视为 V1 第一阶段可演示：

1. 管理员登录 Server。
2. 注册一个 Edge 节点并完成 Docker 能力探测。
3. Edge 按 API Key 上报心跳。
4. 通过 Server 创建一个环境包。
5. 通过 Server 启动环境包，并看到任务完成。
6. 通过 Server 查询环境包详情，拿到 CDP / WebVNC 地址。
7. 停止环境包，Dashboard 状态同步变化。
