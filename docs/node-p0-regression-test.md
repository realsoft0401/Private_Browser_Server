# Node Server P0 回归测试

## 1. 文档目标

这份文档只测试当前 `Private_Browser_Server P0` 已经落地的节点主链路：

```text
UDP discovery
  -> bind
  -> Node 下发 node-registration.json
  -> Client 主动 heartbeat
  -> Node healthy/offline 收口
  -> recovery
  -> unbind
```

这份文档不覆盖：

- browser-env 中心调度
- 平台额度
- run/stop/backup/restore/delete
- server task 持久化

## 2. 测试范围

本次回归要验证 7 件事：

1. Node 能通过 UDP 正式发现 Client
2. discovered 阶段 `clientId` 为空是正确行为
3. bind 后 Node 能生成稳定 `clientId`
4. Node 能把中心身份写回 Client 本地 `node-registration.json`
5. Client 写本地 JSON 成功后，会主动向 Node 发 heartbeat
6. Node 只保留 `healthy / offline` 两态，并能按心跳超时收口
7. unbind 后能删除当前有效绑定结果、清空 Client 本地注册文件，且再次 bind 生成新的 `clientId`

## 3. 默认联调口径

本文件按本地开发默认地址编写：

```bash
export CLIENT_BASE="http://127.0.0.1:3300"
export NODE_BASE="http://127.0.0.1:3400"
export ACCOUNT_ID="906090119"
export CLIENT_IP="192.168.111.119"
```

如果你的 Client 不在本机，而是在局域网另一台机器：

- `NODE_BASE` 仍然是当前本机 `3400`
- `CLIENT_IP` 必须换成那台 Client 的真实内网地址

## 4. 测试前准备

### 4.1 启动前清理

确认没有旧进程占用端口：

```bash
lsof -nP -iTCP:3300
lsof -nP -iTCP:3400
lsof -nP -iUDP:43000
```

如果 `3400` 或 `43000` 被占用，先停掉旧进程。

### 4.2 启动服务

先启动 Node：

```bash
cd /Users/lining/Documents/Browser_virtualization/Private_Browser_Server
go run .
```

再启动 Client：

```bash
cd /Users/lining/Documents/Browser_virtualization/Private_Browser_Client
go run .
```

### 4.3 清理旧测试数据

Node SQLite：

```bash
sqlite3 /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
DELETE FROM edge_client_bind_logs;
DELETE FROM edge_clients;
"
```

Client 本地注册文件：

```bash
rm -f /Users/lining/Documents/Browser_virtualization/Private_Browser_Client/data/node-registration.json
```

说明：

- 如果 Client 跑在远程 Linux 宿主机，要改成对应机器上的 `data/node-registration.json`

## 5. 用例一：UDP discovery

### 5.1 目标

验证 Node 只通过 UDP 发现 Client，而不是通过 heartbeat 发现 Client。

### 5.2 操作

等待 Client 启动后至少一个 UDP beacon 周期，然后查看 discovered：

```bash
curl -s "$NODE_BASE/api/v1/edge-clients/discovered" | jq
```

### 5.3 预期

返回里应出现一条 discovered 项，并满足：

- `clientId` 为空字符串
- `accountId` 为空字符串
- `status=discovered`
- `clientIp` 是当前 Client 的真实内网 IP
- `baseUrl` 类似 `http://<client-ip>:3300`
- `healthStatus=healthy`

### 5.4 失败排查

如果 discovered 为空，优先检查：

1. Client 是否真的在发 UDP beacon
2. Node 的 `43000/udp` 是否已监听
3. Client 和 Node 是否在同一可广播局域网
4. 防火墙是否拦截了广播

## 6. 用例二：bind 并写回本地注册文件

### 6.1 目标

验证 Node 能完成：

- probe Client
- 生成 `clientId`
- 落中心库
- 写回 Client 本地 `node-registration.json`

### 6.2 操作

执行 bind：

```bash
curl -s -X POST "$NODE_BASE/api/v1/edge-clients/bind" \
  -H "Content-Type: application/json" \
  -d "{
    \"accountId\": \"$ACCOUNT_ID\",
    \"clientIp\": \"$CLIENT_IP\"
  }" | jq
```

### 6.3 预期

返回成功，且：

- `status=bound`
- `bindStatus=success`
- `pushStatus=success`
- `clientId` 格式为 `9060901190001` 这种 `主账号 + 4位序号`

记录：

```bash
export CLIENT_ID="$(curl -s "$NODE_BASE/api/v1/edge-clients?accountId=$ACCOUNT_ID" | jq -r '.data.items[0].clientId')"
echo "$CLIENT_ID"
```

### 6.4 中心库校验

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,main_account_id,client_sequence,client_ip,base_url,health_status,discovery_status,push_status
FROM edge_clients
WHERE client_id='$CLIENT_ID';
"
```

预期：

- `main_account_id = 906090119`
- `discovery_status = verified`
- `push_status = success`
- `health_status = healthy`

### 6.5 Client 本地文件校验

本地开发口径：

```bash
cat /Users/lining/Documents/Browser_virtualization/Private_Browser_Client/data/node-registration.json
```

预期：

- `clientId` 非空
- `mainAccountId = 906090119`
- `nodeServerBaseUrl = http://<node-lan-ip>:3400`
- 不能是 `http://127.0.0.1:3400`

## 7. 用例三：heartbeat 后中心状态与设备摘要

### 7.1 目标

验证 bind 完成后的 Client 会主动 heartbeat，Node 会刷新：

- `last_heartbeat_at`
- `last_heartbeat_reported_at`
- `last_heartbeat_source`
- `health_status`
- `cpu_cores`
- `memory_total_mb`
- `docker_version`

### 7.2 操作

等待一个 heartbeat 周期后查询中心库：

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,health_status,last_heartbeat_at,last_heartbeat_reported_at,last_heartbeat_source,
       cpu_cores,memory_total_mb,docker_version
FROM edge_clients
WHERE client_id='$CLIENT_ID';
"
```

### 7.3 预期

- `health_status = healthy`
- `last_heartbeat_source = heartbeat_http`
- `last_heartbeat_at > 0`
- `cpu_cores > 0`
- `memory_total_mb > 0` 或至少不是长期空值
- `docker_version` 在 Docker 可达时应为非空

说明：

- 如果 `docker_version` 为空，但 `dockerApiUrl` 配置不可达，也不应影响 bind 主链路
- 但在正式联调环境里，这三个字段不应长期缺失

## 8. 用例四：offline 收口

### 8.1 目标

验证 Node 只保留 `healthy / offline` 两态，并能按 heartbeat 超时自动转 `offline`。

### 8.2 操作

停止 Client 进程，然后等待超过 `offline_after_seconds`。

当前默认配置：

- `monitor_interval_seconds = 15`
- `offline_after_seconds = 90`

所以建议等待 `100-110` 秒后执行：

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,health_status,last_heartbeat_at,updated_at
FROM edge_clients
WHERE client_id='$CLIENT_ID';
"
```

### 8.3 预期

- `health_status = offline`
- 不应出现 `stale`
- 不应出现 `unhealthy`

## 9. 用例五：recovery

### 9.1 目标

验证 Client 恢复上线后，Node 能回到 `healthy`。

### 9.2 操作

重新启动 Client，然后等待一个 heartbeat 周期后执行：

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,health_status,last_heartbeat_at,last_heartbeat_source
FROM edge_clients
WHERE client_id='$CLIENT_ID';
"
```

### 9.3 预期

- `health_status = healthy`
- `last_heartbeat_source = heartbeat_http`

## 10. 用例六：unbind

### 10.1 目标

验证 unbind 后：

- 中心解绑成立
- 当前有效绑定结果被删除
- Client 本地 `node-registration.json` 被清空

### 10.2 操作

```bash
curl -s -X POST "$NODE_BASE/api/v1/edge-clients/$CLIENT_ID/unbind" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "manual-unbind"
  }' | jq
```

### 10.3 预期

- `status = unbound`
- `clearRegistrationStatus = success`

### 10.4 中心库校验

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,main_account_id,discovery_status,discovery_reason,push_status
FROM edge_clients
WHERE client_id='$CLIENT_ID';
"
```

预期：

- 原 `$CLIENT_ID` 不应再作为当前有效绑定结果存在
- 如果仍保留历史审计或历史日志，那属于审计留痕，不属于当前有效节点

### 10.5 Client 本地文件校验

```bash
ls -la /Users/lining/Documents/Browser_virtualization/Private_Browser_Client/data/node-registration.json
```

预期：

- 文件不存在，或内容已被清空到无中心身份

## 11. 用例七：rebind 生成新的 clientId

### 11.1 目标

验证同一台 Client 解绑后再次 bind，会重新生成新的 `clientId`。

### 11.2 操作

再次执行 bind：

```bash
curl -s -X POST "$NODE_BASE/api/v1/edge-clients/bind" \
  -H "Content-Type: application/json" \
  -d "{
    \"accountId\": \"$ACCOUNT_ID\",
    \"clientIp\": \"$CLIENT_IP\"
  }" | jq
```

### 11.3 预期

- 新返回的 `clientId` 不能仍然等于旧 `$CLIENT_ID`
- 必须生成新的设备序号

再查中心库确认：

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT client_id,main_account_id,client_sequence
FROM edge_clients
WHERE client_ip='$CLIENT_IP';
"
```

预期：

- 会出现新的 `client_id`
- 新旧绑定不应再共享同一个当前有效节点身份

## 12. 阻塞上线的问题口径

以下任一情况，都不应视为 P0 可验收：

1. 没有 UDP beacon，但 Node 仍然能“发现” Client
2. bind 成功后没有写回 Client 本地 `node-registration.json`
3. `nodeServerBaseUrl` 被错误写成 `127.0.0.1:3400`
4. heartbeat 收到后，Node 仍不能把状态收口到 `healthy`
5. heartbeat 超时后，Node 仍不能收口到 `offline`
6. 中心库里的 `cpu_cores / memory_total_mb / docker_version` 长期为空
7. unbind 后再次 bind 仍然复用旧 `clientId`
8. unbind 后 clear 失败却回滚了中心解绑

## 13. 测试完成后的建议收尾

如果只是本地回归，测试完成后建议清理：

```bash
sqlite3 /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
DELETE FROM edge_client_bind_logs;
DELETE FROM edge_clients;
"

rm -f /Users/lining/Documents/Browser_virtualization/Private_Browser_Client/data/node-registration.json
```

如果下一步马上要进入 `browser-env` 中心层开发，则建议保留：

- 当前一组稳定的已绑定节点数据
- 一份 heartbeat 正常的样本记录
- 一份 offline/recovery 的对照记录

这样后面做 `Server -> Client browser-env` 聚合时，可以直接复用这套稳定节点底座。
