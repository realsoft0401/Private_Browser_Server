# Node Server 接口设计：`POST /api/v1/edge-clients/{clientId}/device-info/refresh`

## 1. 功能目标

`POST /api/v1/edge-clients/{clientId}/device-info/refresh` 用于重新探测目标节点的 Docker 2375 能力，并把最新设备事实回写中心库。

## 2. 设计来源

- 用户要求设备能力事实不能靠手工填写长期维持，必须能再次探测。
- 但 refresh 只确认 Docker 事实，不等价于 verify。

## 3. 业务边界

### 3.1 负责什么

- 校验 `dockerApiUrl` 已存在
- 重新探测 `/_ping/info/version`
- 回写 OS、架构、CPU、内存、Docker 版本、检查时间

### 3.2 不负责什么

- 不自动验证 `/health`
- 不自动置为 `verified`
- 不修改 `baseUrl/clientIp`

## 4. 请求与响应

```http
POST /api/v1/edge-clients/{clientId}/device-info/refresh
```

必须带 Platform Header，无请求体。

成功返回：

- `data.node`
- `data.probe`

其中：

- `data.node` 是回写后的中心节点摘要
- `data.probe` 是本次 Docker 2375 探测结果

## 5. 成功判定

- Docker 探测成功
- 中心记录更新成功

## 6. 失败判定

- `dockerApiUrl` 缺失
- Docker 2375 不可达
- 回写中心库失败

## 7. 联调验收标准

- 成功后 `os/arch/cpuCores/memoryTotalMb/dockerVersion` 会更新
- 探测失败时 `healthStatus` 应收口到 `unhealthy`
- 即使 refresh 成功，`discoveryStatus` 也不会自动变成 `verified`
