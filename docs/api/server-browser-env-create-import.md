# Server Browser Env Create / Import

这份文档说明两个中心环境包入口：

- `POST /api/v1/browser-envs`
- `POST /api/v1/browser-envs/import-package`

当前状态：已实现并已回归。

---

## 1. 业务语义

这两个接口解决同一个核心问题：让 browser-env 正式进入 Node Server 的中心主视图 `server_browser_envs`。

`POST /api/v1/browser-envs` 用于普通配置创建流程。调用方传 `clientId/userId/rpaType/runtime/environment/proxy`，Node 按 `clientId` 找到目标 Client，调用 Edge 创建环境包，成功后立即写入中心缓存。

`POST /api/v1/browser-envs/import-package` 用于已有标准 tgz 包导入流程。Node 接收上传包和 `clientId`，把包转发到目标 Client，由 Edge 解压、校验、重分配端口并写入本机 SQLite。Edge import task 成功后，Node 再回读 Edge detail 写入中心缓存。

---

## 2. 职责边界

### Node 负责

- 校验目标 `clientId` 已绑定、`healthy + verified`
- 选择目标 Client 的 `baseUrl`
- 调用 Edge 正式 API
- 写入或刷新 `server_browser_envs`
- 为 import-package 创建中心 task 和 SSE 事件

### Node 不负责

- 不自动选择 Client
- 不解析 tgz 包内 profile/proxy/fingerprint
- 不保存 profile、proxy 明文、fingerprint raw 或 browser-data
- 不自动 run
- 不自动 pull image
- 不自动 reinit slot

### Edge 负责

- 创建本机环境包目录
- 导入 tgz 包
- 校验原子材料
- 写入 Client 本机 SQLite
- 分配本机 envSequence、CDP/VNC 端口

---

## 3. 状态机与前置条件

### 允许调用

- 目标 Client 已经绑定中心 `clientId`
- `edge_clients.health_status=healthy`
- `edge_clients.discovery_status=verified`
- 目标 Client 3300 HTTP 可达

### 必须拒绝

- `clientId` 为空
- Client 不存在
- Client 是 `discovered/unhealthy/offline/stale/identity_changed`
- Edge 创建或导入返回失败
- import-package 上传文件缺失或为空

### 成功后状态

- create 成功后：中心 `server_browser_envs.status=created`
- import-package 成功后：中心从 Edge detail 同步状态，通常为 `created`
- 二者都不会自动进入 `running`

---

## 4. OpenAPI 协议

### 4.1 POST /api/v1/browser-envs

请求：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs" \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "9060901190003",
    "userId": "906090001",
    "rpaType": "tk",
    "name": "tk-main-account",
    "runtime": {
      "image": "crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_edge:1.2-amd64",
      "startupUrl": "https://www.tiktok.com/",
      "shmSize": "1g"
    },
    "environment": {
      "timezone": "America/New_York",
      "language": "us-en",
      "screen": {
        "width": 1280,
        "height": 720,
        "depth": 24
      }
    },
    "proxy": {
      "enabled": false,
      "type": "",
      "configBase64": ""
    }
  }' | jq
```

成功响应：

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "envId": "906090001_tk_324867594169356288",
    "clientId": "9060901190003",
    "accountId": "906090119",
    "userId": "906090001",
    "rpaType": "tk",
    "name": "tk-main-account",
    "status": "created"
  }
}
```

### SSE 说明

- 不使用 SSE
- 原因：创建配置文件是短链路同步动作
- 成功只表示环境包创建并写入中心缓存，不表示已启动浏览器

---

### 4.2 POST /api/v1/browser-envs/import-package

请求：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/import-package" \
  -F "clientId=9060901190003" \
  -F "file=@/Users/lining/Documents/Browser_virtualization/318275706305908736_tk_319725200528642048-backup.tar.gz" | jq
```

接单响应：

```json
{
  "code": 1000,
  "message": "success",
  "data": {
    "taskId": "server-task-1782800001001",
    "taskType": "browser_env_import_package",
    "resourceType": "browser_env",
    "resourceId": "",
    "clientId": "9060901190003",
    "eventsUrl": "/api/v1/server-tasks/server-task-1782800001001/events"
  }
}
```

订阅 SSE：

```bash
export SERVER_TASK_ID="server-task-1782800001001"
curl -N "$SERVER_BASE/api/v1/server-tasks/$SERVER_TASK_ID/events"
```

成功事件：

```text
event: task.completed
data: {"event":"task.completed","taskId":"server-task-1782800001001","taskType":"browser_env_import_package","resourceType":"browser_env","resourceId":"318275706305908736_tk_319725200528642048","clientId":"9060901190003","envId":"318275706305908736_tk_319725200528642048","stage":"finalize_success","status":"success","message":"browser env import completed"}
```

### SSE 说明

- 使用 SSE
- 发起接口只表示中心接单成功
- 最终结果必须看 `/api/v1/server-tasks/{taskId}/events`
- SSE 中断后，用 `GET /api/v1/server-tasks/{taskId}` 查终态
- `task.completed` 后，中心 `server_browser_envs` 必须已经写入对应 env

---

## 5. 验收标准

- create 成功后，`GET /api/v1/browser-envs?accountId=906090119` 能看到新 env
- create 成功后，`GET /api/v1/browser-envs/{envId}` 返回 `status=created`
- import-package 接单返回 `taskId/eventsUrl`
- import-package SSE 最终出现 `task.completed`
- import-package 成功后，`server_tasks.resource_id/env_id` 回填真实 envId
- import-package 成功后，`server_browser_envs` 有对应记录

---

## 6. 相关接口

- `GET /api/v1/browser-envs`
- `GET /api/v1/browser-envs/{envId}`
- `POST /api/v1/browser-envs/{envId}/run`
- `POST /api/v1/server-tasks/{taskId}/events`
- Edge `POST /api/v1/edge/browser-envs`
- Edge `POST /api/v1/edge/browser-envs/import-package`
