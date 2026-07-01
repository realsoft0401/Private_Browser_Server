# Node Server 部署与回归测试

这份文档用于 Node Server 镜像部署后的最小验收。

它覆盖：

- 镜像构建
- Docker 运行
- 文档入口
- UDP discovered
- 普通 bind 本地锁拒绝
- 旧 Node 不可用时的手动处理边界

## 1. 构建镜像

### ARM64

```bash
cd /Users/lining/Documents/Browser_virtualization/Private_Browser_Server
DEBIAN_MIRROR=deb.debian.org \
./scripts/build-server-image.sh \
  --platform linux/arm64 \
  --image crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server \
  --tag 0.1.1-arm64 \
  --push
```

### AMD64

```bash
cd /Users/lining/Documents/Browser_virtualization/Private_Browser_Server
DEBIAN_MIRROR=deb.debian.org \
./scripts/build-server-image.sh \
  --platform linux/amd64 \
  --image crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server \
  --tag 0.1.1-amd64 \
  --push
```

说明：

- `linux/arm64` 是 64 位 ARM。
- `linux/arm` 通常表示 32 位 ARM，除非明确目标机器是 32 位，否则不要用。
- `DEBIAN_MIRROR=deb.debian.org` 用于避开个别国内镜像 403 问题。

## 2. 运行容器

Node Server 需要：

- `3400/tcp` 提供 HTTP API。
- `43000/udp` 监听 Client UDP beacon。

建议使用 host 网络：

```bash
docker run -d \
  --name private-browser-node-server \
  --restart always \
  --network host \
  -v /Business/server-data:/app/data \
  crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_node_server:0.1.1-arm64
```

如果目标机器是 AMD64，把镜像 tag 改成 `0.1.1-amd64`。

## 3. 基础入口验收

```bash
export SERVER_BASE="http://127.0.0.1:3400"

curl -s "$SERVER_BASE/health" | jq
curl -s -o /dev/null -w '%{http_code}\n' "$SERVER_BASE/swagger"
curl -s -o /dev/null -w '%{http_code}\n' "$SERVER_BASE/scalar"
curl -s "$SERVER_BASE/openapi.yaml" | sed -n '1,8p'
```

通过标准：

- `/health` 返回 `code=1000`。
- `/swagger` 返回 `200`。
- `/scalar` 返回 `200`。
- `/openapi.yaml` 返回 `openapi: 3.0.3`。

## 4. UDP discovered 验收

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients/discovered" | jq
```

通过标准：

- 能看到局域网内正在广播的 Client。
- 未 bind 的 Client 可以出现 `clientId=""`，这是正常 discovered 视图。
- discovered 只是发现线索，不代表已经绑定。

## 5. 普通 bind 本地锁验收

先确认 Client 本地是否已有注册锁：

```bash
export CLIENT_BASE="http://192.168.111.119:3300"

curl -s "$CLIENT_BASE/api/v1/edge/node-registration" | jq
```

如果返回：

```json
{
  "data": {
    "cacheStatus": "cached",
    "cachedRegistration": {
      "clientId": "9060901190003",
      "nodeServerBaseUrl": "http://192.168.111.220:3400"
    }
  }
}
```

再发普通 bind：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/bind" \
  -H "Content-Type: application/json" \
  -d '{
    "accountId": "906090119",
    "clientIp": "192.168.111.119"
  }' | jq
```

通过标准：

- 返回 `code=1005`。
- message 明确包含 `node-registration.json`。
- 当前 Node 的 `edge_clients` 不应新增这台 Client。

## 6. 换 Node 规则

正式规则：

- 普通 bind 前，只要 Client 本地有 `node-registration.json`，必须拒绝。
- 正常换 Node：先旧 Node unbind，清掉 Client 本地注册文件，再新 Node bind。
- 旧 Node 不可用：管理员手动登录 Client 机器，删除本地注册文件，再新 Node 普通 bind。
- 当前不提供 force bind 接口。

手动清理示例：

```bash
rm -f /Business/data/node-registration.json
```

注意：

- 手动删除只用于旧 Node 不可用的运维场景。
- 删除前应确认这台 Client 确实要切换到新 Node。
- 不要在普通 bind 里自动覆盖旧注册文件。
