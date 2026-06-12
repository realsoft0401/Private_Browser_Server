# 全能力真实 VPN 测试方案

## 1. 文档目标

这份文档用于在真实 Linux 边缘节点 `192.168.10.119` 上，按完整业务链路回归：

```text
提交配置(create)
-> run 配置(first run)
-> 打包配置(backup)
-> load 配置(import-package)
-> run 配置(second run)
-> 删除配置(delete package)
```

本次方案明确要求：

- 代理配置使用真实 `Clash` VPN，而不是空代理或 mock 配置
- 浏览器运行与 VNC 验证都以 `119 Linux` 为准，不以本机 macOS 浏览器链路为准
- 每一步都同时检查 API 返回、中心数据库、Edge 详情、磁盘目录和 VNC 可用性
- 失败时必须保留可复现证据，不能只说“感觉不对”

## 2. 本次测试基线

- `Node Server`
  - `http://127.0.0.1:3400`
- `Edge Client`
  - `http://192.168.10.119:3300`
- `Docker API`
  - `tcp://192.168.10.119:2375`
- `MainAccountId`
  - `906090001`
- `PlatformUserId`
  - `user_1780995561009325000_000001`
- `PlatformUsername`
  - `user_906090001`
- `PlatformRole`
  - `owner`
- `clientId`
  - `9060900010001`
- `amd64 imagePolicy`
  - `crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_edge:1.1-amd64`
- 真实 Clash 配置文件
  - [ClashVerge.yaml](/Users/lining/Documents/analysis_ins/proxy/ClashVerge.yaml)

## 3. 术语映射

- `提交配置`
  - `POST /api/v1/envs`
- `run 配置`
  - `POST /api/v1/envs/{envId}/run`
- `打包配置`
  - `POST /api/v1/envs/{envId}/backup`
- `load 配置`
  - `POST /api/v1/envs/import-package`
- `删除配置`
  - `DELETE /api/v1/envs/{envId}/package`

## 4. 测试前准备

### 4.1 导出统一变量

```bash
export NODE_BASE="http://127.0.0.1:3400"
export EDGE_BASE="http://192.168.10.119:3300"
export DOCKER_HOST_EDGE="tcp://192.168.10.119:2375"

export MAIN_ACCOUNT_ID="906090001"
export PLATFORM_USER_ID="user_1780995561009325000_000001"
export PLATFORM_USERNAME="user_906090001"
export PLATFORM_ROLE="owner"
export CLIENT_ID="9060900010001"

export IMAGE_POLICY="crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_edge:1.1-amd64"
export CLASH_FILE="/Users/lining/Documents/analysis_ins/proxy/ClashVerge.yaml"

export TEST_NAME="E2E-Real-VPN-Regression"
export TEST_RPA_TYPE="tk"
```

### 4.2 运行时生成代理 Base64

不要把超长 base64 手工写进文档或代码仓库，统一在测试前现场生成：

```bash
export CLASH_BASE64="$(base64 < "$CLASH_FILE" | tr -d '\n')"
printf '%s' "$CLASH_BASE64" | wc -c
```

通过标准：

- `CLASH_BASE64` 非空
- 长度明显大于 1000

### 4.3 检查 Node / Edge / Docker 基线

```bash
curl -s "$NODE_BASE/health" | jq
curl -s "$EDGE_BASE/health" | jq
docker -H "$DOCKER_HOST_EDGE" ps --format '{{.Names}}|{{.Status}}'
docker -H "$DOCKER_HOST_EDGE" inspect private-browser-edge-server --format '{{json .HostConfig.CapAdd}} {{json .HostConfig.Devices}}'
```

通过标准：

- Node `ok=true`
- Edge `ok=true`
- Edge `version` 符合当前待测版本
- `private-browser-edge-server` 正在运行
- 容器启动参数里能看到 `NET_ADMIN`
- 容器启动参数里能看到 `/dev/net/tun`

### 4.4 检查中心节点登记状态

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT id,base_url,arch,health_status,discovery_status,last_heartbeat_at,last_error
FROM edge_clients;
"
```

通过标准：

- `id=9060900010001`
- `base_url=http://192.168.10.119:3300`
- `arch=amd64`
- `health_status=healthy`
- `discovery_status=verified`

### 4.5 清理上一次测试残留

如果上次同名环境还存在，先记录再删，不要直接忽略：

```bash
curl -s "$NODE_BASE/api/v1/envs?page=1&pageSize=50" \
  -H "accept: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" | jq
```

建议：

- 使用一个新的测试名字，例如 `E2E-Real-VPN-Regression-20260612-01`
- 如果要复用旧 `envId`，必须记录删除前后状态

## 5. 观察命令清单

下面这些命令在全流程中会反复使用，建议单独开一个终端窗口保留。

### 5.1 中心环境表

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT env_id,client_id,status,container_status,monitor_status,last_task_id,last_error,updated_at
FROM server_browser_envs
ORDER BY updated_at DESC;
"
```

### 5.2 中心任务表

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT id,type,status,client_id,env_id,edge_task_id,error_message,created_at,updated_at,finished_at
FROM server_tasks
ORDER BY created_at DESC
LIMIT 20;
"
```

### 5.3 Edge 环境详情

```bash
curl -s "$EDGE_BASE/api/v1/edge/browser-envs/$ENV_ID" | jq
```

### 5.4 Node 任务详情

```bash
curl -s "$NODE_BASE/api/v1/server/tasks/$TASK_ID" \
  -H "accept: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" | jq
```

### 5.5 Edge 磁盘目录

```bash
docker -H "$DOCKER_HOST_EDGE" exec private-browser-edge-server sh -lc '
find /app/data/browser-envs/users/'"$MAIN_ACCOUNT_ID"'/'"$TEST_RPA_TYPE"' -maxdepth 2 | sort
'
```

### 5.6 Edge 浏览器容器

```bash
docker -H "$DOCKER_HOST_EDGE" ps -a --format '{{.Names}}|{{.Image}}|{{.Status}}' | grep 'private-browser-edge-' || true
```

## 6. Step 1: 提交配置 Create

### 6.1 发起创建

```bash
CREATE_RESP="$(curl -s -X POST "$NODE_BASE/api/v1/envs" \
  -H "accept: application/json" \
  -H "Content-Type: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" \
  -d "{
    \"clientId\": \"$CLIENT_ID\",
    \"name\": \"$TEST_NAME\",
    \"rpaType\": \"$TEST_RPA_TYPE\",
    \"runtime\": {
      \"imagePolicy\": \"$IMAGE_POLICY\",
      \"startupUrl\": \"https://www.tiktok.com/\",
      \"shmSize\": \"1gb\"
    },
    \"environment\": {
      \"timezone\": \"Asia/Shanghai\",
      \"language\": \"zh-CN\",
      \"screen\": {
        \"width\": 1440,
        \"height\": 900,
        \"depth\": 24
      }
    },
    \"proxy\": {
      \"enabled\": true,
      \"type\": \"clash-verge\",
      \"mode\": \"rule\",
      \"configBase64\": \"$CLASH_BASE64\"
    },
    \"metadata\": {
      \"source\": \"manual-e2e\",
      \"description\": \"real VPN full capability regression\"
    }
  }")"

printf '%s\n' "$CREATE_RESP" | jq
export ENV_ID="$(printf '%s' "$CREATE_RESP" | jq -r '.data.envId')"
echo "$ENV_ID"
```

### 6.2 创建后的观察点

检查：

- 返回 `code=1000`
- `envId` 非空
- `status=created`
- `clientId=9060900010001`
- `webVncUrl` 已按 `192.168.10.119:3300` 生成，不是 `127.0.0.1`

数据库观察：

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT env_id,status,container_status,monitor_status,last_task_id,last_error
FROM server_browser_envs
WHERE env_id = '$ENV_ID';
"
```

通过标准：

- 中心表出现一条新记录
- `status=created`
- `container_status=unknown`
- `monitor_status=unknown`
- `last_error=''`

Edge 观察：

```bash
curl -s "$EDGE_BASE/api/v1/edge/browser-envs/$ENV_ID" | jq '.data.index'
```

通过标准：

- `status=created`
- `proxyConfigExists=true`
- `browserDataExists=true`

## 7. Step 2: 第一次 Run

### 7.1 发起运行

```bash
RUN1_RESP="$(curl -s -X POST "$NODE_BASE/api/v1/envs/$ENV_ID/run" \
  -H "accept: application/json" \
  -H "Content-Type: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" \
  -d '{"forceRecreate":false}')"

printf '%s\n' "$RUN1_RESP" | jq
export RUN1_TASK_ID="$(printf '%s' "$RUN1_RESP" | jq -r '.data.taskId')"
echo "$RUN1_TASK_ID"
```

### 7.2 轮询任务直到完成

```bash
curl -s "$NODE_BASE/api/v1/server/tasks/$RUN1_TASK_ID" \
  -H "accept: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" | jq
```

需要观察的阶段：

- `server_precheck`
- `image_check`
- `pulling_image`
- `edge_run`
- `finalize`

### 7.3 Run 成功后的通过标准

- Node task `status=success`
- 中心环境 `status=running`
- Edge 环境详情 `status=running`
- 浏览器容器已创建并运行
- `cdpUrl` 非空
- `webVncUrl` 非空
- 没有 `last_error`

建议检查：

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT env_id,status,container_status,monitor_status,last_task_id,last_error
FROM server_browser_envs
WHERE env_id = '$ENV_ID';
"

curl -s "$EDGE_BASE/api/v1/edge/browser-envs/$ENV_ID" | jq '.data.index'

docker -H "$DOCKER_HOST_EDGE" ps -a --format '{{.Names}}|{{.Image}}|{{.Status}}' | grep "$ENV_ID"
```

## 8. Step 3: 第一次 VNC 检查

### 8.1 读取 VNC 信息

```bash
curl -s "$EDGE_BASE/api/v1/edge/browser-envs/$ENV_ID/vnc-info" | jq
```

通过标准：

- `code=1000`
- `vncUrl` 不是 `127.0.0.1`
- `wsUrl` 不是 `127.0.0.1`
- `webVncUrl` 不是 `127.0.0.1`
- `webVncUrl` 应类似 `http://192.168.10.119:3300/web-vnc.html?envId=...`

补充说明：

- `run` 刚发起、中心 task 仍处于 `running` 时，Edge 环境详情可能还停留在 `created`
- 这个短窗口里调用 `vnc-info` 可能返回 `环境包未运行，不能返回 VNC 连接信息`
- 只要稍后 task 收口为 `success`、Edge 状态切到 `running`，再次读取 `vnc-info` 应恢复正常

### 8.2 手工打开 Web VNC

在浏览器中打开：

```text
http://192.168.10.119:3300/web-vnc.html?envId=$ENV_ID
```

需要手工确认：

- 页面能打开，不是空白页
- 能看到浏览器桌面画面
- 鼠标和键盘能操作
- 浏览器实际打开 TikTok 或启动页
- 没有反复断线

### 8.3 真实 VPN 验证点

这一轮必须用真实代理验证，而不是只看容器 running：

- TikTok 页面能打开
- 无明显 DNS 污染或秒跳国内错误页
- VNC 里访问 `https://www.tiktok.com/` 时页面可正常加载
- 如果页面长期转圈、证书异常、无网络、直连国内结果异常，都算失败

说明：

- 当前 macOS 本地代理环境会干扰浏览器链路，因此真正的业务可用性以 `119` 的 VNC 观察结果为准

## 9. Step 4: 打包配置 Backup

### 9.1 先停环境

如果环境还在 `running`，先停：

```bash
STOP_RESP="$(curl -s -X POST "$NODE_BASE/api/v1/envs/$ENV_ID/stop" \
  -H "accept: application/json" \
  -H "Content-Type: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" \
  -d '{}')"

printf '%s\n' "$STOP_RESP" | jq
export STOP_TASK_ID="$(printf '%s' "$STOP_RESP" | jq -r '.data.taskId')"
```

停成功标准：

- Node task `status=success`
- 中心环境 `status=stopped`
- Edge 环境 `status=stopped` 或 `created`
- 浏览器容器不再 `running`

### 9.2 发起备份

```bash
BACKUP_RESP="$(curl -s -X POST "$NODE_BASE/api/v1/envs/$ENV_ID/backup" \
  -H "accept: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE")"

printf '%s\n' "$BACKUP_RESP" | jq
export BACKUP_TASK_ID="$(printf '%s' "$BACKUP_RESP" | jq -r '.data.taskId')"
```

### 9.3 Backup 后观察点

```bash
curl -s "$EDGE_BASE/api/v1/edge/browser-envs/$ENV_ID" | jq '.data.index | {envId,status,backupPath,backupChecksum,backupAt}'

docker -H "$DOCKER_HOST_EDGE" exec private-browser-edge-server sh -lc 'ls -lh /app/data/browser-envs/users/'"$MAIN_ACCOUNT_ID"'/'"$TEST_RPA_TYPE"'/*backup.tar.gz'
```

通过标准：

- 中心 task `status=success`
- 中心环境 `status=backed_up`
- Edge 环境 `status=backed_up`
- `backupPath` 非空
- `backupChecksum` 非空
- 备份包文件存在
- 原环境目录已释放

磁盘观察：

- `.../$ENV_ID-backup.tar.gz` 应存在
- `.../$ENV_ID/` 运行目录应不存在或已清空

## 10. Step 5: 复制标准包到本地

这一步是为了后续 `load 配置(import-package)` 使用外部标准包，而不是复用 Edge 本机 `backupPath`。

```bash
mkdir -p /Users/lining/Documents/Browser_virtualization/tmp/import-test

docker -H "$DOCKER_HOST_EDGE" cp \
  private-browser-edge-server:/app/data/browser-envs/users/$MAIN_ACCOUNT_ID/$TEST_RPA_TYPE/${ENV_ID}-backup.tar.gz \
  /Users/lining/Documents/Browser_virtualization/tmp/import-test/${ENV_ID}-backup.tar.gz

ls -lh /Users/lining/Documents/Browser_virtualization/tmp/import-test/${ENV_ID}-backup.tar.gz
shasum -a 256 /Users/lining/Documents/Browser_virtualization/tmp/import-test/${ENV_ID}-backup.tar.gz
```

通过标准：

- 本地 `.tar.gz` 文件存在
- `shasum` 与 Edge `backupChecksum` 一致

## 11. Step 6: 删除配置 Delete Package

### 11.1 发起删除

```bash
DELETE_RESP="$(curl -s -X DELETE "$NODE_BASE/api/v1/envs/$ENV_ID/package" \
  -H "accept: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE")"

printf '%s\n' "$DELETE_RESP" | jq
export DELETE_TASK_ID="$(printf '%s' "$DELETE_RESP" | jq -r '.data.taskId')"
```

### 11.2 推进删除任务到最终态

注意：

- 当前 `delete package` 的最终收口仍依赖读取 Node task detail 或 SSE
- 如果不读 task detail，中心层可能暂时停在 `pending`

执行：

```bash
curl -s "$NODE_BASE/api/v1/server/tasks/$DELETE_TASK_ID" \
  -H "accept: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" | jq
```

### 11.3 删除后通过标准

- Node task `status=success`
- 中心环境 `status=deleted`
- Edge 环境详情返回不存在
- `cdpUrl` 清空
- `webVncUrl` 清空
- Edge 运行目录不存在

检查：

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT env_id,status,container_status,monitor_status,last_task_id,last_error
FROM server_browser_envs
WHERE env_id = '$ENV_ID';
"

curl -i -s "$EDGE_BASE/api/v1/edge/browser-envs/$ENV_ID"
```

## 12. Step 7: Load 配置 Import Package

### 12.1 发起导入

```bash
IMPORT_RESP="$(curl -s -X POST "$NODE_BASE/api/v1/envs/import-package" \
  -H "accept: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" \
  -F "clientId=$CLIENT_ID" \
  -F "file=@/Users/lining/Documents/Browser_virtualization/tmp/import-test/${ENV_ID}-backup.tar.gz")"

printf '%s\n' "$IMPORT_RESP" | jq
export IMPORT_TASK_ID="$(printf '%s' "$IMPORT_RESP" | jq -r '.data.taskId')"
```

### 12.2 Import 后通过标准

- Node task `status=success`
- 中心环境从 `deleted` 复活成 `created`
- Edge 环境 `status=created`
- `clientId` 仍是 `9060900010001`
- `envId` 不变
- `envSequence/CDP/VNC` 为当前节点重新分配结果

检查：

```bash
sqlite3 -header -column /Users/lining/Documents/Browser_virtualization/Private_Browser_Server/data/private_browser_server.db "
SELECT env_id,status,container_status,monitor_status,client_id,last_task_id,last_error,created_at,updated_at
FROM server_browser_envs
WHERE env_id = '$ENV_ID';
"

curl -s "$EDGE_BASE/api/v1/edge/browser-envs/$ENV_ID" | jq '.data.index | {envId,status,envSequence,cdpPort,vncPort,updatedAt}'
```

特别说明：

- 这一步是本轮回归的关键检查点
- 如果这里还出现“中心已存在相同 envId，禁止重复导入”，说明 tombstone 复活逻辑回归了

## 13. Step 8: 第二次 Run

### 13.1 发起第二次运行

```bash
RUN2_RESP="$(curl -s -X POST "$NODE_BASE/api/v1/envs/$ENV_ID/run" \
  -H "accept: application/json" \
  -H "Content-Type: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" \
  -d '{"forceRecreate":false}')"

printf '%s\n' "$RUN2_RESP" | jq
export RUN2_TASK_ID="$(printf '%s' "$RUN2_RESP" | jq -r '.data.taskId')"
```

### 13.2 第二次运行后的通过标准

- Node task `status=success`
- 中心环境 `status=running`
- Edge 环境 `status=running`
- 第二次 VNC 信息仍然正确
- TikTok 页面仍可通过真实代理打开

检查：

```bash
curl -s "$EDGE_BASE/api/v1/edge/browser-envs/$ENV_ID/vnc-info" | jq

curl -s "$NODE_BASE/api/v1/server/tasks/$RUN2_TASK_ID" \
  -H "accept: application/json" \
  -H "X-Main-Account-Id: $MAIN_ACCOUNT_ID" \
  -H "X-Platform-User-Id: $PLATFORM_USER_ID" \
  -H "X-Platform-Username: $PLATFORM_USERNAME" \
  -H "X-Platform-Role: $PLATFORM_ROLE" | jq
```

## 14. 最终删除收尾

如果本轮测试结束后不需要保留环境，建议按下面顺序收尾：

1. `POST /api/v1/envs/{envId}/stop`
2. `DELETE /api/v1/envs/{envId}/package`
3. 读取 `task detail` 让删除任务收口
4. 确认中心为 `deleted`
5. 确认 Edge 环境不存在

## 15. 必查数据库变化

### 15.1 创建后

- `server_browser_envs.status=created`

### 15.2 第一次 run 后

- `server_tasks.type=run_env`
- `server_tasks.status=success`
- `server_browser_envs.status=running`

### 15.3 backup 后

- `server_tasks.type=backup_env`
- `server_browser_envs.status=backed_up`

### 15.4 delete package 后

- `server_tasks.type=delete_env_package`
- `server_browser_envs.status=deleted`

### 15.5 import-package 后

- `server_tasks.type=import_env_package`
- `server_tasks.status=success`
- `server_browser_envs.status=created`

### 15.6 第二次 run 后

- 新增一条 `run_env` 成功任务
- `server_browser_envs.status=running`

## 16. 必查磁盘变化

### 16.1 创建后

- Edge 目录 `/app/data/browser-envs/users/$MAIN_ACCOUNT_ID/$TEST_RPA_TYPE/$ENV_ID/` 存在

### 16.2 backup 后

- `${ENV_ID}-backup.tar.gz` 存在
- 原运行目录已释放

### 16.3 delete package 后

- Edge 上 `$ENV_ID/` 目录不存在
- Edge 本地 SQLite 索引已移除

### 16.4 import-package 后

- `$ENV_ID/` 目录重新出现
- `profile.json`
- `binding.json`
- `proxy/clash.yaml`
- `browser-data/profile`

## 17. 失败时必须收集的证据

一旦失败，至少保留下面四类证据：

- 失败接口完整返回 JSON
- `server_tasks` 最近 20 条
- `server_browser_envs` 当前记录
- `curl "$EDGE_BASE/api/v1/edge/browser-envs/$ENV_ID"` 返回

建议额外保留：

```bash
docker -H "$DOCKER_HOST_EDGE" ps -a --format '{{.Names}}|{{.Image}}|{{.Status}}'
docker -H "$DOCKER_HOST_EDGE" logs --tail 200 private-browser-edge-server
```

## 18. 本轮通过标准

必须同时满足下面所有条件，才能算“全能力真实 VPN 测试通过”：

- 真实 Clash 配置成功提交到 create 请求
- 第一次 run 成功
- 第一次 VNC 可正常打开并看到 TikTok 页面
- backup 成功并生成标准包
- delete package 成功并让中心收口到 `deleted`
- import-package 成功把中心从 `deleted` 恢复为 `created`
- 第二次 run 成功
- 第二次 VNC 仍可正常打开
- 全流程中 `webVncUrl/wsUrl/vncUrl` 不出现错误的 `127.0.0.1`
- 中心数据库、Edge 详情、磁盘文件三方事实一致

## 19. 当前已知注意事项

- `delete package` 当前最终收口仍依赖读取 `task detail` 或 SSE，不读可能暂时停在 `pending`
- macOS 本地代理会干扰浏览器链路判断，因此浏览器业务可用性必须以 `119 Linux + VNC` 为准
- `import-package` 当前已经支持“同 clientId 的 deleted tombstone 复活”；如果后续再次出现重复导入冲突，要优先回归这一点
- `proxy.type` 当前正式值必须使用 `clash-verge`；如果传 `clash`，Edge 会直接返回 `proxy.type 第一版仅支持 clash-verge`
- 第二次 `run` 的启动窗口内，`vnc-info` 短暂返回未运行属于正常时序，不应误判为回归失败
