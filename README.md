# Private_Browser_Server

新的 `Private_Browser_Server` 当前处于 V1 重建早期阶段。

当前已完成的只是节点接入主线中的第一段：

```text
Node 找到 Client
  -> bind
  -> 生成 clientId
  -> 过渡期兼容下发中心身份留痕
```

这条 bind 主线只是 V1 的入口子能力，不等于 `Private_Browser_Server` 的完整正式定位。

`Private_Browser_Server` 的正式定位仍然是：

- 中心节点治理
- browser-env 聚合视图
- 平台级 task 持久化
- run admission / quota 收口
- 前端与平台统一入口

当前刻意不把 old 的 Env、Task、RPA、Dashboard 一起机械搬回来，是为了避免新旧逻辑再次混杂；不是说这些能力不再属于 Server，而是要按新的中心服务边界重建。

当前只保留四份核心文档：

- [server-v1-central-node-technical-design.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-central-node-technical-design.md)
- [server-v1-database-design.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-database-design.md)
- [server-v1-api-plan.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-api-plan.md)
- [server-v1-current-status.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-current-status.md)
- [openapi.yaml](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/openapi.yaml)

## API 文档入口

Node Server 当前内置 API 文档页面和一个临时只读管理 Demo，全部挂在 3400 主服务内：

- `/swagger`
- `/scalar`
- `/admin`
- `/openapi.yaml`

访问示例：

```text
http://127.0.0.1:3400/swagger
http://127.0.0.1:3400/scalar
http://127.0.0.1:3400/admin
http://127.0.0.1:3400/openapi.yaml
```

其中 `/scalar` 是唯一 Scalar 正式入口，不维护独立 Scalar 展示服务，也不新增单独 Dockerfile。

`/admin` 是当前阶段的只读 Node Admin Demo，只用于观察 discovered clients、bound clients、browser envs 和 server tasks，不提供 bind、unbind、run、delete 等高危动作。后续正式 Vue 管理台上线后，应删除 `/admin` 路由、`public/admin.html` 和对应测试引用，不把它扩展成第二套前端工程。

## 镜像构建

当前统一构建入口是：

```bash
cd /Users/lining/Documents/Browser_virtualization/Private_Browser_Server
DEBIAN_MIRROR=deb.debian.org \
./scripts/build-server-image.sh \
  --platform linux/arm64 \
  --image crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server \
  --tag 0.1.1-arm64 \
  --push
```

如果目标机器是 `amd64`，把平台和 tag 改成：

```bash
--platform linux/amd64
--tag 0.1.1-amd64
```

## Docker 运行

Node Server 需要监听 `3400/tcp` 和 `43000/udp`，正式部署建议使用 host 网络：

```bash
docker run -d \
  --name private-browser-node-server \
  --restart always \
  --network host \
  -v /Business/server-data:/app/data \
  crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server:0.1.1-arm64
```

验证入口：

```bash
curl -s http://127.0.0.1:3400/health | jq
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3400/swagger
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3400/scalar
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:3400/admin
```
