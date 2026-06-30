# Private_Browser_Server API Docs

这份索引页只做一件事：

- 把当前 `Private_Browser_Server` 已经落地或已经收口的 API 文档按主题挂起来

当前目录下的文档分 5 组：

## 1. 状态总览

| 分组 | 接口 / 文档 | 当前状态 |
| --- | --- | --- |
| 节点治理 | `GET /api/v1/edge-clients/{clientId}/slots` | 已实现并已回归 |
| 节点治理 | `POST /api/v1/edge-clients/{clientId}/target-slot-count` | 已实现并已回归 |
| 节点治理 | `POST /api/v1/edge-clients/{clientId}/slot-reconcile` | 已实现并已回归 |
| 节点治理 | `GET /api/v1/edge-clients/{clientId}/run-quota` | 已实现并已回归 |
| 节点治理 | `POST /api/v1/edge-clients/{clientId}/run-quota/refresh` | 已实现并已回归 |
| Browser Env 查询 | `GET /api/v1/browser-envs` | 已实现并已回归 |
| Browser Env 查询 | `GET /api/v1/browser-envs/{envId}` | 已实现并已回归 |
| Browser Env 查询 | `POST /api/v1/browser-envs/{envId}/refresh` | 已实现并已回归 |
| Browser Env 生命周期 | `POST /api/v1/browser-envs/{envId}/run` | 已实现并已回归 |
| Browser Env 生命周期 | `POST /api/v1/browser-envs/{envId}/stop` | 已实现并已回归 |
| Browser Env 生命周期 | `POST /api/v1/browser-envs/{envId}/backup` | 已实现并已回归 |
| Browser Env 生命周期 | `POST /api/v1/browser-envs/{envId}/restore` | 已实现并已回归 |
| Browser Env 生命周期 | `DELETE /api/v1/browser-envs/{envId}/package` | 已实现并已回归 |
| Browser Env 生命周期 | `DELETE /api/v1/browser-envs/{envId}/del` | 已实现并已回归 |
| 中心 Task | `GET /api/v1/server-tasks/{taskId}` | 已实现并已用于现有任务链 |
| 中心 Task | `GET /api/v1/server-tasks/{taskId}/events` | 已实现并已回归 |

状态口径固定为：

- `已实现并已回归`
  - 代码已落地，且至少完成过一轮真实接口回归

## 2. 节点治理

- [server-slot-governance-apis.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-slot-governance-apis.md)
  - `GET /api/v1/edge-clients/{clientId}/slots`
  - `POST /api/v1/edge-clients/{clientId}/target-slot-count`
- [server-slot-reconcile-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-slot-reconcile-regression.md)
  - `POST /api/v1/edge-clients/{clientId}/slot-reconcile`
  - `GET /api/v1/server-tasks/{taskId}`
  - `GET /api/v1/server-tasks/{taskId}/events`
- [server-run-quota-apis.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-run-quota-apis.md)
  - `GET /api/v1/edge-clients/{clientId}/run-quota`
  - `POST /api/v1/edge-clients/{clientId}/run-quota/refresh`
- [server-slot-governance-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-slot-governance-regression.md)
  - slot 目标值 / 异常摘要回归
- [server-run-quota-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-run-quota-regression.md)
  - quota / admission 回归

## 3. Browser Env 查询

- [server-browser-env-query-apis.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-query-apis.md)
  - `GET /api/v1/browser-envs`
  - `GET /api/v1/browser-envs/{envId}`
  - `POST /api/v1/browser-envs/{envId}/refresh`
- [server-browser-env-query-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-query-regression.md)
  - browser-env 查询 / refresh 回归

## 4. Browser Env 生命周期

- [server-browser-env-run.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-run.md)
  - `POST /api/v1/browser-envs/{envId}/run`
- [server-browser-env-stop.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-stop.md)
  - `POST /api/v1/browser-envs/{envId}/stop`
- [server-browser-env-backup.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-backup.md)
  - `POST /api/v1/browser-envs/{envId}/backup`
- [server-browser-env-restore.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-restore.md)
  - `POST /api/v1/browser-envs/{envId}/restore`
- [server-browser-env-delete-package.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-delete-package.md)
  - `DELETE /api/v1/browser-envs/{envId}/package`
- [server-browser-env-delete-image.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-delete-image.md)
  - `DELETE /api/v1/browser-envs/{envId}/del`
- [server-browser-env-run-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-run-regression.md)
  - 中心 run 回归
- [server-browser-env-stop-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-stop-regression.md)
  - 中心 stop 回归
- [server-browser-env-backup-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-backup-regression.md)
  - 中心 backup 回归
- [server-browser-env-restore-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-restore-regression.md)
  - 中心 restore 回归
- [server-browser-env-delete-package-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-delete-package-regression.md)
  - 中心 package delete 回归
- [server-browser-env-delete-image-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-browser-env-delete-image-regression.md)
  - 中心 del 回归

## 5. 中心 Task

- [server-task-detail.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-task-detail.md)
  - `GET /api/v1/server-tasks/{taskId}`
- [server-task-events.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-task-events.md)
  - `GET /api/v1/server-tasks/{taskId}/events`

## 6. 上层总文档

如果要看更大的结构，不要只停在单接口文档，还应一起看：

- [openapi.yaml](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/openapi.yaml)
  - Swagger / OpenAPI 正式协议入口
- [server-v1-api-plan.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-api-plan.md)
  - V1 API 分阶段实现路线
- [server-v1-central-node-technical-design.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-central-node-technical-design.md)
  - 中心节点技术设计
- [server-v1-database-design.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-database-design.md)
  - 中心数据库与缓存设计

## 7. 当前阅读顺序建议

如果是第一次接手这个项目，建议按下面顺序看：

1. `server-v1-api-plan.md`
2. `server-v1-central-node-technical-design.md`
3. `server-v1-database-design.md`
4. `docs/openapi.yaml`
5. 具体接口文档
6. 对应回归文档
