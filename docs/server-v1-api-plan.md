# Private_Browser_Server V1 正式 API 清单与分阶段实现顺序

## 1. 文档目标

这份文档只回答两件事：

1. `Private_Browser_Server` V1 正式应该有哪些 API。
2. 这些 API 应按什么顺序实现，才不会和当前骨架、数据库设计、run 准入规则打架。

它服务的是：

- `Routes`
- `Service/*_http.go`
- `docs/openapi.yaml`
- 后续逐接口 Markdown 文档

> 口径说明：
> 这份 API 规划描述的是 Server V1 正式分阶段路线。
> 其中如果出现 `push-client-id` 或清理 `node-registration.json`，都应理解为“Node 发现 Client 后完成绑定并把中心身份写回 Client”的配套动作；顺序必须是 discovery -> bind -> 写回，而不是先写回再发现。

## 2. 先说结论

当前新 Server 不能再按“想到哪个接口就先补哪个”的方式推进了。

正确顺序应该是：

1. 先稳定节点治理主线 API
2. 再补环境聚合查询 API
3. 再补正式生命周期代理 API
4. 再补中心 task 查询与 SSE 观察
5. 平台额度、slot 数量和 run admission 最终收口先暂停，不作为当前下一阶段任务

原因很简单：

- 节点还没收口，env 调度就没有稳定目标
- `server_browser_envs` 还没立住，生命周期接口就没有中心事实源
- `server_tasks` 没立住，长链路生命周期动作就没有中心审计事实
- 平台额度属于商业约束，必须等需求再次确认后单独进入，不和当前非平台任务混做

## 3. 正式命名空间

这次已经拍板，后续统一按下面三组走：

- `/api/v1/edge-clients/*`
- `/api/v1/browser-envs/*`
- `/api/v1/server-tasks/*`

不再新增：

- `/api/v1/server/*`

历史旧路径：

- `/api/v1/server/edge-clients/heartbeat`

当前已收口为：

- `/api/v1/edge-clients/heartbeat`

## 4. Phase A：节点治理主线

这组 API 是 V1 最优先级。

没有它们，后续所有 env 和任务编排都不稳。

### 4.1 基础入口

- `GET /health`
- `GET /swagger`
- `GET /scalar`
- `GET /openapi.yaml`

### 4.2 discovery / heartbeat

- `GET /api/v1/edge-clients/discovered`
- `POST /api/v1/edge-clients/heartbeat`

说明：

- `discovered` 是过程视图，不是正式节点表
- `heartbeat` 只更新已知节点心跳摘要，不参与发现，也不创建 discovered

### 4.3 bind / push / unbind

- `POST /api/v1/edge-clients/bind`
- `POST /api/v1/edge-clients/{clientId}/push-client-id`
- `POST /api/v1/edge-clients/{clientId}/unbind`

说明：

- bind 成功且探测通过后，节点进入正式绑定态，随后再把 `clientId/accountId` 写回 Client 本地 JSON
- `push-client-id` 是 Node -> Client 的写回配套接口，不是 Client 自注册接口
- unbind 后应清空 Client 本地 `node-registration.json` 留痕
- 普通 bind 前必须探测 Client `/api/v1/edge/node-registration`；只要 Client 本地已有 `node-registration.json`，普通 bind 一律拒绝。换 Node 必须先旧 Node unbind；旧 Node 不可用时，由管理员手动登录 Client 机器删除本地注册文件后，再由新 Node 重新发起普通 bind。当前不提供 force bind 接口。

### 4.4 节点查询

- `GET /api/v1/edge-clients`
- `GET /api/v1/edge-clients/{clientId}`

### 4.5 节点治理辅助动作

这组不是第一天就必须写，但属于节点治理域，应保留在 Phase A 末尾：

- `POST /api/v1/edge-clients/{clientId}/recheck`
- `POST /api/v1/edge-clients/{clientId}/confirm-address-update`
- `POST /api/v1/edge-clients/{clientId}/slot-reconcile`
- `POST /api/v1/edge-clients/{clientId}/target-slot-count`
- `GET /api/v1/edge-clients/{clientId}/slots`

说明：

- `recheck` 用于强制重探 `/health + /device-info`
- `recheck` 的中文业务语义名统一叫“会话校验”
- `confirm-address-update` 用于管理员确认 IP 漂移后的地址更新
- 顺序必须固定为：先 `recheck` 发现 `ip_mismatch`，再由管理员发起 `confirm-address-update`
- `slot-reconcile` 用于中心重建 node-slot 关系缓存和节点 slot 摘要
- `slot-reconcile` 正式按 task + SSE 设计
- `target-slot-count` 已实现为节点治理辅助入口；平台正式下发链路当前暂停，不作为下一阶段任务
- `GET /slots` 返回中心缓存的当前 node-slot 明细和 slot 摘要，不直接穿透到 Client

## 5. Phase B：环境聚合查询 API

这组 API 是让 Server 真正成为中心视图服务的开始。

### 5.1 环境列表与详情

- `GET /api/v1/browser-envs`
- `GET /api/v1/browser-envs/{envId}`

说明：

- 这两条先以 `server_browser_envs` 为主视图
- 当前已按该口径落地
- 需要时再显式触发 `refresh`

### 5.2 环境聚合刷新

- `POST /api/v1/browser-envs/{envId}/refresh`

说明：

- 这条是中心主动同步某个 env 当前 Edge 事实
- 它不是业务生命周期动作
- 当前已按同步 HTTP 落地，不使用 SSE

## 6. Phase C：正式生命周期代理 API

这组是业务动作主线，必须在：

- `edge_clients`
- `server_browser_envs`
- `server_tasks`

三层基础都立住之后再做。

### 6.1 create / run / stop

- `POST /api/v1/browser-envs`
- `POST /api/v1/browser-envs/{envId}/run`
- `POST /api/v1/browser-envs/{envId}/stop`

说明：

- 当前 `create` 已经落地并已回归：
  - 同步调用目标 Edge 创建环境包
  - 成功后写入 `server_browser_envs`
  - 不使用 SSE
- 当前 `run` 已经先落最小正式骨架：
  - 必须显式传 `slotId`
  - 再调用目标 Edge run
  - 最终通过 `server_tasks + SSE` 收口
- 当前 `stop` 也已经落地：
  - 采用同步 HTTP 最终结果
  - 内部仍创建 `server_tasks` 审计事实
  - stop 成功后必须再次同步 Edge detail 回写中心 env 摘要
- 当前 `backup / restore` 也已经落地最小正式骨架：
  - 采用 `server_tasks + SSE`
  - 通过 Edge 正式 task 接口执行
  - Edge success 后必须再次同步中心 env 摘要
- 当前 `package delete` 也已经落地：
  - 采用 `server_tasks + SSE`
  - 通过 Edge 正式 delete task 执行
  - Edge success 后中心直接删除 `server_browser_envs` 缓存
- 当前 `/del` 也已经落地：
  - 采用同步 HTTP
  - 通过 Edge 正式 `/del` 执行
  - 成功后只回写最近同步时间和错误摘要，不删除中心 env 缓存
- 当前 `revalidate` 已经落地并已回归：
  - 采用 `server_tasks + SSE`
  - 只用于异常环境包受控重新校验
  - Edge success 后重新同步中心 env 缓存
- 当前 `import-package` 已经落地并已回归：
  - 采用 `server_tasks + SSE`
  - 由 Node 转发 tgz 到目标 Edge
  - Edge success 后回填真实 envId 并写入中心缓存
- `run` 当前不自动选 slot
- 平台额度和 run admission 最终收口当前暂停，不继续展开

### 6.2 backup / restore / revalidate / import-package

- `POST /api/v1/browser-envs/{envId}/backup`
- `POST /api/v1/browser-envs/{envId}/restore`
- `PATCH /api/v1/browser-envs/{envId}/runtime-image`
- `POST /api/v1/browser-envs/{envId}/revalidate`
- `POST /api/v1/browser-envs/import-package`

### 6.3 删除类动作

- `DELETE /api/v1/browser-envs/{envId}/del`
- `DELETE /api/v1/browser-envs/{envId}/package`

说明：

- `/del` 只删运行镜像
- `/package` 才是彻底销毁环境资产

## 7. Phase D：任务 API

任务 API 不只是附属查询，而是正式平台事实入口。

### 7.1 任务查询

- `GET /api/v1/server-tasks`：已实现并已回归
- `GET /api/v1/server-tasks/{taskId}`：已实现并已回归

### 7.2 任务事件

- `GET /api/v1/server-tasks/{taskId}/events`：已实现并已回归

说明：

- 这条是中心 SSE
- 只有当 Server 侧真正需要多阶段过程可见时才做
- 当前 `slot-reconcile` 已经明确需要走这条 SSE

## 8. Phase E：run admission / platform quota 相关 API

这组 API 当前明确暂停，不作为下一阶段任务。

暂停原因：

- 你已经明确要求“平台额度、slot 数量、run admission 的最终收口先不做”
- 这部分属于平台商业约束，不应和当前 Node 普通服务能力继续混做
- 后续恢复时，需要重新从平台、Node、Client 三层关系开始确认

### 8.1 额度查询

- `GET /api/v1/edge-clients/{clientId}/run-quota`

说明：

- 返回最近一次可信平台额度快照
- 同时返回当前中心 run admission 判断
- 这是管理员和排障接口
- 不是平台真相源
- 不使用 SSE

### 8.2 额度刷新

- `POST /api/v1/edge-clients/{clientId}/run-quota/refresh`

说明：

- 当前先允许管理员手工写入额度快照
- 平台正式 quota API 接入后，再改成 Node 主动向平台刷新额度快照
- 不使用 SSE

### 8.3 run admission 内部使用

run admission 本身不一定要单独暴露成公开 API。

更合理的是：

- 作为 `POST /api/v1/browser-envs/{envId}/run` 的前置内部流程

## 9. 明确不进入 V1 正式主线的 API

V1 明确不进主线：

- 中心 `slots` CRUD
- PlatformServer 的机位 / Redis 商业闭环 API
- 任何直接透传 `baseUrl` 去调 Edge 的业务动作接口
- 任何读取 Edge SQLite 或 Edge 文件系统的接口

## 10. 当前 API 状态

### 当前已经有

- `GET /health`
- `GET /swagger`
- `GET /scalar`
- `GET /openapi.yaml`
- `GET /api/v1/edge-clients/discovered`
- `POST /api/v1/edge-clients/heartbeat`
- `POST /api/v1/edge-clients/bind`
- `POST /api/v1/edge-clients/{clientId}/push-client-id`
- `POST /api/v1/edge-clients/{clientId}/unbind`
- `POST /api/v1/edge-clients/{clientId}/recheck`
- `POST /api/v1/edge-clients/{clientId}/confirm-address-update`
- `POST /api/v1/edge-clients/{clientId}/slot-reconcile`
- `POST /api/v1/edge-clients/{clientId}/target-slot-count`
- `GET /api/v1/edge-clients/{clientId}/slots`
- `GET /api/v1/edge-clients/{clientId}/run-quota`
- `POST /api/v1/edge-clients/{clientId}/run-quota/refresh`
- `GET /api/v1/edge-clients`
- `GET /api/v1/edge-clients/{clientId}`
- `GET /api/v1/browser-envs`
- `GET /api/v1/browser-envs/{envId}`
- `POST /api/v1/browser-envs`
- `POST /api/v1/browser-envs/{envId}/run`
- `POST /api/v1/browser-envs/{envId}/stop`
- `POST /api/v1/browser-envs/{envId}/backup`
- `POST /api/v1/browser-envs/{envId}/restore`
- `PATCH /api/v1/browser-envs/{envId}/runtime-image`
- `POST /api/v1/browser-envs/{envId}/revalidate`
- `POST /api/v1/browser-envs/import-package`
- `DELETE /api/v1/browser-envs/{envId}/del`
- `DELETE /api/v1/browser-envs/{envId}/package`
- `GET /api/v1/server-tasks`
- `GET /api/v1/server-tasks/{taskId}`
- `GET /api/v1/server-tasks/{taskId}/events`

### 当前暂停项

- 平台额度正式下发
- slot 数量商业约束最终收口
- run admission 最终商业准入

### 当前下一阶段建议

- 不做平台约束
- 优先做 Node 文档、OpenAPI、回归脚本和管理视图一致性收口
- 其次补普通运维/诊断类 API 的文档与回归，不改变业务状态机
- 当前已补充节点接入与会话治理 API 文档，确保 discovery、heartbeat、bind、push、unbind、recheck、confirm-address-update 不再只停留在代码层

## 11. 推荐实现顺序

建议严格按下面顺序：

### 第一步

- 收口 `edge-clients` 路由命名空间
- 完成 `heartbeat` 路径修正
- 补 `unbind`
- 收口 Node discovery -> bind -> Client 本地 JSON 写回的顺序说明

### 第二步

- 先补 `server_browser_envs` 查询 API
- 只做列表 / 详情 / recheck

### 第三步

- 补 `server_tasks` 查询 API
- 详情、列表、事件流均已落地并已回归

### 第四步

- 接 `browser-envs` 生命周期代理
- 顺序建议：
  - `create`
  - `run`
  - `stop`
  - `backup`
  - `restore`
  - `revalidate`
  - `import-package`
  - `del`
  - `package`

### 第五步：暂停

- 平台额度快照接口
- slot 数量最终约束
- run admission 最终商业准入

这一步当前暂停，恢复前必须重新确认需求。

## 12. 一句话收口

Server V1 的正式 API，不是“先把所有 Edge 接口在中心侧复制一遍”。

正确做法是：

- 先完成节点治理主线
- 再完成中心环境与任务事实
- 再完成生命周期代理
- 平台额度和 run admission 最终收口当前暂停

这样 Server 才不会再次退回“只有接口，没有中心事实源”的旧状态。
