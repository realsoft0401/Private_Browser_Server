# Private_Browser_Server V1 当前状态封板

## 1. 当前结论

截至当前阶段，Node Server 已经完成 V1 主体骨架：

- 节点发现与绑定主线
- Client 本地 `node-registration.json` 绑定锁
- 中心 browser-env 聚合视图
- browser-env 生命周期代理
- Server task 查询与 SSE
- Node Server Docker 构建与部署脚本
- Swagger / Scalar 文档入口
- Node Admin Demo 只读管理视图

当前不继续推进平台额度、slot 数量商业约束和 run admission 最终收口。

## 2. 当前已验证部署方式

正式构建入口：

```bash
cd /Users/lining/Documents/Browser_virtualization/Private_Browser_Server
DEBIAN_MIRROR=deb.debian.org \
./scripts/build-server-image.sh \
  --platform linux/arm64 \
  --image crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server \
  --tag 0.1.1-arm64 \
  --push
```

正式运行入口：

```bash
docker run -d \
  --name private-browser-node-server \
  --restart always \
  --network host \
  -v /Business/server-data:/app/data \
  crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server:0.1.1-arm64
```

说明：

- `--network host` 是正式推荐方式，因为 Node Server 要监听 `3400/tcp` 和 `43000/udp`。
- `/app/data` 保存 Node Server SQLite 数据库。
- `linux/arm64` 是 64 位 ARM；AMD64 机器应使用 `linux/amd64` 和对应 tag。

详细部署回归见：

- [server-deploy-regression.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-deploy-regression.md)

## 3. 文档入口

Node Server 当前内置：

- `GET /swagger`
- `GET /scalar`
- `GET /admin`
- `GET /openapi.yaml`

规则：

- `/scalar` 是唯一 Scalar 入口。
- 不提供 `/Scalar`。
- 不维护独立 Scalar 展示服务。
- 不新增 `Dockerfile.scalar`。
- Swagger 和 Scalar 都读取同一份 `docs/openapi.yaml`。
- `/admin` 是当前阶段只读 Demo，只调用现有查询接口观察节点、环境包和任务事实。
- `/admin` 不提供 bind、unbind、run、stop、backup、restore、delete、force bind 等动作按钮。
- 后续正式 Vue 管理台上线后，应删除 `/admin` 路由、`public/admin.html` 和对应测试引用，不继续扩展这套静态页。

## 4. 节点接入当前状态

当前已实现并已回归：

- `GET /api/v1/edge-clients/discovered`
- `POST /api/v1/edge-clients/heartbeat`
- `POST /api/v1/edge-clients/bind`
- `POST /api/v1/edge-clients/{clientId}/push-client-id`
- `POST /api/v1/edge-clients/{clientId}/unbind`
- `POST /api/v1/edge-clients/{clientId}/recheck`
- `POST /api/v1/edge-clients/{clientId}/confirm-address-update`
- `GET /api/v1/edge-clients`
- `GET /api/v1/edge-clients/{clientId}`

核心规则：

- UDP beacon 是自动发现唯一正式链路。
- heartbeat 只更新已知节点活性，不参与发现。
- discovered 只是发现视图，不是正式绑定事实。
- bind 前必须探测 Client `/health`、`/api/v1/edge/device-info`、`/api/v1/edge/node-registration`。
- 只要 Client 本地已有 `node-registration.json`，普通 bind 必须拒绝。
- 正常换 Node：先旧 Node unbind，再新 Node bind。
- 旧 Node 不可用：管理员手动删除 Client 本地 `/Business/data/node-registration.json` 后，再由新 Node 普通 bind。
- 当前不提供 force bind 接口。

详细说明见：

- [server-edge-client-access-governance.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/server-edge-client-access-governance.md)

## 5. Browser Env 当前状态

当前已实现并已回归：

- `GET /api/v1/browser-envs`
- `GET /api/v1/browser-envs/{envId}`
- `POST /api/v1/browser-envs/{envId}/refresh`
- `POST /api/v1/browser-envs`
- `POST /api/v1/browser-envs/import-package`
- `POST /api/v1/browser-envs/{envId}/run`
- `POST /api/v1/browser-envs/{envId}/stop`
- `POST /api/v1/browser-envs/{envId}/backup`
- `POST /api/v1/browser-envs/{envId}/restore`
- `PATCH /api/v1/browser-envs/{envId}/runtime-image`
- `POST /api/v1/browser-envs/{envId}/revalidate`
- `DELETE /api/v1/browser-envs/{envId}/del`
- `DELETE /api/v1/browser-envs/{envId}/package`

当前边界：

- Server 不直接读 Client SQLite。
- Server 不直接读 Client 环境包目录。
- Server 只通过 Client HTTP API 执行生命周期动作。
- `server_browser_envs` 是中心聚合缓存，不是 Edge 本地真实资产源。
- `run` 当前必须显式传 `slotId`，不自动选 slot。
- 平台额度和 run admission 最终商业准入当前暂停。

## 6. Server Task 当前状态

当前已实现并已回归：

- `GET /api/v1/server-tasks`
- `GET /api/v1/server-tasks/{taskId}`
- `GET /api/v1/server-tasks/{taskId}/events`

规则：

- Server task 是中心长期任务事实。
- Client task 只用于 Edge 本机短期执行观察。
- 真正需要多阶段观察的生命周期动作才走 `server_tasks + SSE`。
- 不滥用 SSE；同步短链路接口继续用普通 HTTP。

## 7. 当前暂停项

这些能力当前明确暂停，不作为下一阶段任务：

- 平台额度正式下发
- slot 数量商业约束最终收口
- run admission 最终商业准入
- Platform Server 商业闭环
- 中心自动选节点
- 自动跨节点迁移 browser-env
- force bind / 管理员接管接口
- 管理员诊断接口

说明：

- `GET /api/v1/edge-clients/{clientId}/run-quota` 和 `POST /run-quota/refresh` 已作为节点治理能力落地并回归，但平台正式 quota 下发链路暂停。
- 旧 Node 不可用时的绑定恢复，当前只走人工删除 Client 本地注册文件，不做 force bind API。

## 8. 当前不做项

V1 当前不做：

- Client 用户体系
- Client JWT / RBAC
- Client 多节点管理
- Node 直接读取 Client 文件系统
- Node 直接读取 Client SQLite
- Node 绕过 Client API 操作 Docker 或环境包
- 裸容器接口作为业务主入口
- 独立 Scalar 文档容器

## 9. 已验证多 Node 场景

已验证场景：

```text
Node 209 discovered Client 119
Client 119 本地已有 node-registration.json
node-registration 指向旧 Node 220
Node 209 普通 bind Client 119
=> 返回 code=1005
=> message 明确包含 node-registration.json
=> Node 209 edge_clients 不新增记录
```

结论：

- 多个 Node 可以同时 discovered 同一个 Client。
- 只有没有本地注册锁的 Client 才能被普通 bind。
- Client 本地 `node-registration.json` 是跨 Node SQLite 的本地互斥锁。

## 10. 当前下一阶段候选任务

如果继续开发，建议从下面选一个：

- 做一轮 Node Server 全接口真实回归，并把结果回填到 `docs/api/*-regression.md`。
- 开始 Platform Server 文档设计，不写代码。
- 做 Node Server 管理前端需要的 API 字段整理，不新增业务动作。
- 做 Server/Client 部署脚本和版本 tag 规范整理。
- 暂停新功能，先打镜像并做多机器部署验收。

当前不建议马上做：

- 平台额度最终收口。
- run admission 最终商业准入。
- 管理员诊断接口。
- force bind。
