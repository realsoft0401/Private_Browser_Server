# Node Server 逐接口文档索引

本目录用于存放 `Private_Browser_Server` 的逐接口 Markdown 文档，采用“一接口一文件”方式，方便开发、联调、实施、管理员和后续企业级 API 对接直接阅读。

## 当前正式接口

### System

- [health.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/health.md)
- [dashboard.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/dashboard.md)

### Auth

- [auth-register.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/auth-register.md)
- [auth-login.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/auth-login.md)
- [auth-me.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/auth-me.md)

### Edge Heartbeat

- [heartbeat.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/heartbeat.md)

### Edge Clients

- [probe-docker.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/probe-docker.md)
- [register-node.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/register-node.md)
- [list-nodes.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/list-nodes.md)
- [list-discovered-clients.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/list-discovered-clients.md)
- [node-detail.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/node-detail.md)
- [refresh-node-device-info.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/refresh-node-device-info.md)
- [verify-node.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/verify-node.md)

### Browser Envs

- [create-env.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/create-env.md)
- [list-envs.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/list-envs.md)
- [env-detail.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/env-detail.md)
- [run-env.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/run-env.md)
- [stop-env.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/stop-env.md)
- [backup.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/backup.md)
- [restore.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/restore.md)
- [revalidate.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/revalidate.md)
- [import-package.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/import-package.md)
- [delete-env-image.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/delete-env-image.md)
- [delete-env-package.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/delete-env-package.md)

### Server Tasks

- [list-tasks.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/list-tasks.md)
- [task-detail.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/task-detail.md)
- [task-events.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/task-events.md)

## 使用原则

- `docs/openapi.yaml` 继续作为协议事实源。
- `docs/api/*.md` 负责补足业务语义、状态机、任务编排、失败收口、错误日志和联调标准。
- 如果接口行为发生变化，必须同步更新 `openapi.yaml` 与对应 md，不能只改其中一份。
