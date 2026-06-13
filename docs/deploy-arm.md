# Private_Browser_Server ARM 发布说明

## 1. 文档目标

这份文档用于沉淀 `Private_Browser_Server` 在 ARM 控制设备上的正式发布口径，覆盖：

- 本地构建 ARM64 镜像
- 推送到阿里云容器仓库
- 远端 ARM 服务器拉取并运行
- SQLite 挂载与端口映射
- 发布后验证与常见排障

它服务的是“发版与部署”这个动作，不负责讲节点治理、接口状态机和业务流程设计；这些内容仍以 `README.md`、`project.md` 和 `docs/api/*.md` 为准。

## 2. 当前正式镜像信息

当前已验证可用的正式镜像地址：

```text
crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server:1.0-arm
```

当前已验证可用的镜像摘要：

```text
sha256:b48b6464bc61d28597c08efd852ac9c781eae5fdcfa6fbff986ddc4f5d3a9e78
```

## 3. 构建口径

### 3.1 Dockerfile 设计口径

当前 `Private_Browser_Server/Dockerfile` 采用：

- `FROM` 走国内可访问镜像前缀 `docker.m.daocloud.io`
- `GOPROXY` 走 `https://goproxy.cn,direct`
- 运行时使用 `scratch`
- SQLite 数据目录通过挂载卷提供，不内置在镜像中

这样设计的原因是：

- 项目要求正式构建链路不能只改 `apt` 或 `go mod`，基础镜像入口也必须收敛；
- 2026-06-13 实测清华 TUNA 不承担 Docker Hub 基础镜像入口；
- Server 当前使用静态编译二进制，不依赖 CGO，`scratch` 运行时更轻，也避免了额外 Debian 层下载卡顿。

### 3.2 本地构建命令

在项目目录执行：

```bash
cd /Users/lining/Documents/Browser_virtualization/Private_Browser_Server

docker buildx build \
  --progress=plain \
  --platform linux/arm64 \
  -t private-browser-server:test-arm \
  --load \
  .
```

这一步的目标是先确认镜像能稳定构建成功，再进行正式 tag 和 push，避免把“构建失败”和“仓库推送失败”混在一起排障。

### 3.3 正式打 tag 并 push

```bash
docker tag \
  private-browser-server:test-arm \
  crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server:1.0-arm

docker push \
  crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server:1.0-arm
```

如果后续版本升级，建议改 tag，不要反复覆盖同一个业务版本号。

## 4. 远端部署口径

### 4.1 宿主机约定

ARM 控制设备当前正式约定：

- 宿主机数据目录：`/Business/data`
- 容器内数据目录：`/app/data`
- SQLite 文件：`/app/data/private_browser_server.db`
- HTTP 端口：`3400/tcp`
- UDP discovery 端口：`43000/udp`

这里必须明确区分：

- `/Business/data` 只负责 Node Server 自身的 SQLite 与本机持久化数据；
- 它不是 Edge Client 的环境包目录，也不承接浏览器 profile、容器资产或节点业务数据。

### 4.2 远端 pull

```bash
docker pull \
  crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server:1.0-arm
```

### 4.3 推荐启动命令

```bash
mkdir -p /Business/data

docker rm -f private_browser_node_server >/dev/null 2>&1 || true

docker run -d \
  --name private_browser_node_server \
  --restart unless-stopped \
  -p 3400:3400 \
  -p 43000:43000/udp \
  -v /Business/data:/app/data \
  crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server:1.0-arm
```

当前这条命令是正式部署口径，原因如下：

- 显式 `-p` 能让 `docker ps` 直接展示端口映射，实施和运维更容易复核；
- `43000/udp` 对应节点自动发现能力，不能漏掉；
- `/Business/data:/app/data` 让 SQLite 不随容器销毁而丢失；
- 当前 Server 只需要少量明确端口，不需要为了省事切到 `--network host`。

## 5. 发布后验证

### 5.1 查看容器状态

```bash
docker ps --filter name=private_browser_node_server
```

正常情况下应看到类似：

```text
0.0.0.0:3400->3400/tcp
0.0.0.0:43000->43000/udp
```

### 5.2 健康检查

```bash
curl http://127.0.0.1:3400/health
```

至少应确认这些字段：

- `ok=true`
- `mode=docker`
- `romInitialized=true`
- `sqlite.path=/app/data/private_browser_server.db`

### 5.3 查看日志

```bash
docker logs --tail 50 private_browser_node_server
```

至少应看到：

- 服务监听 `0.0.0.0:3400`
- UDP discovery 监听 `0.0.0.0:43000`

## 6. 常见问题

### 6.1 为什么 `docker ps` 的 `PORTS` 是空的

如果使用了：

```bash
docker run ... --network host ...
```

那么 `docker ps` 的 `PORTS` 列会显示为空。这不是端口没开，而是因为容器直接复用了宿主机网络栈，Docker 不再展示 `-p` 端口映射。

当前正式口径不推荐这么部署，除非后续有非常明确的网络侧要求。

### 6.2 为什么不把基础镜像直接改成清华 Docker 镜像

2026-06-13 实测发现，清华 TUNA 当前不承担 Docker Hub 基础镜像入口，因此 `FROM golang/debian` 不能直接依赖它完成 Docker 元数据拉取。

所以当前正式口径是：

- `FROM` 继续走可访问的国内 Docker Hub 镜像前缀
- Go 依赖走 `goproxy.cn`

这和“容器内 Debian 包源是否使用清华”是两回事，不能混为一谈。

### 6.3 为什么运行时用 `scratch`

因为当前 Server：

- 不依赖 CGO
- 不需要容器内 shell 参与业务运行
- 只需要静态二进制、证书、时区库和静态资源

这样镜像更轻，也绕开了额外运行时基础层下载问题。

如果后续引入 CGO、系统命令依赖或共享库依赖，必须重新评估，不能继续机械沿用 `scratch`。

## 7. 2026-06-13 实测闭环结果

本轮 ARM 发布已在下面环境完成实测：

- Node Server 宿主机：`192.168.10.209`
- Node Server 容器名：`private_browser_node_server`
- Edge Client 宿主机：`192.168.10.119`
- Edge Client HTTP 入口：`http://192.168.10.119:3300`
- Edge Client Docker 入口：`http://192.168.10.119:2375`

### 7.1 Server 部署结果

已确认：

- ARM 镜像可正常 `pull`
- `docker run` 后 `3400/tcp` 与 `43000/udp` 显式映射正常
- `/health` 返回 `ok=true`
- `/Business/data/private_browser_server.db` 成功落盘
- 日志中能看到：
  - `UDP discovery listening on 0.0.0.0:43000`
  - `Private_Browser_Server RESTful service listening on http://0.0.0.0:3400`

### 7.2 发现态验证结果

Node Server 已成功收到 `192.168.10.119` 的 UDP beacon。

发现列表关键结果：

```text
clientId=""
clientIp="192.168.10.119"
baseUrl="http://192.168.10.119:3300"
service="private-browser-client"
version="0.1.9"
```

这说明自动发现阶段符合当前口径：

- discovered 阶段允许 `clientId` 为空
- Node Server 先看到入口线索，再由中心分配正式 `clientId`

### 7.3 注册与 verify 结果

对 `192.168.10.119` 执行注册后，中心成功生成：

```text
clientId = demo-main-account0001
```

注册刚完成时节点状态符合设计预期：

- `healthStatus=stale`
- `discoveryStatus=blocked`
- `heartbeatStatus=offline`

随后重新拉取 `/api/v1/edge-clients/discovered` 触发心跳回写，再执行 `POST /api/v1/edge-clients/{clientId}/verify` 后，节点成功进入：

```text
healthStatus=healthy
discoveryStatus=verified
heartbeatStatus=online
arch=amd64
```

本次 verify 已通过的固定检查项：

- `heartbeat`
- `clientHealth`
- `clientDeviceInfo`
- `docker2375`
- `arch`

这说明当前部署后的最小业务闭环已经成立：

```text
ARM 镜像部署成功
-> Server 3400/43000 正常
-> SQLite 落盘到 /Business/data
-> 收到 Client discovery
-> discovered 阶段 clientId 为空
-> 注册生成中心 clientId
-> verify 成功进入 healthy + verified + online
```

如果后续需要继续做环境包生命周期联调，应在这个状态基础上继续，不要跳过 verify。

## 8. 推荐发布顺序

每次正式发版建议按这个顺序执行：

1. 本地 `buildx --load` 验证 ARM 镜像能否构建成功。
2. 给镜像打正式 tag 并 push 到仓库。
3. 远端 `docker pull` 新镜像。
4. 重建 `private_browser_node_server` 容器。
5. 执行 `docker ps`、`/health`、`docker logs` 三步验证。
6. 再进入节点发现、接口联调和业务验收阶段。

不要跳过本地构建验证，也不要只看到容器 `Up` 就算部署完成。`/health`、SQLite 路径、UDP 监听和端口映射必须一起确认。
