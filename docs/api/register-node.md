# Node Server 接口设计：`POST /api/v1/edge-clients`

## 1. 功能目标

`POST /api/v1/edge-clients` 用于把一个新的 `Private_Browser_Client` HTTP 入口和 Docker API 入口登记为中心节点。

成功后的关键结果是：

- Node Server 分配中心 `clientId`
- `edge_clients` 里有一条新的节点索引
- 后续 env、task、审计都能围绕该 `clientId` 跟踪

## 2. 设计来源

- 用户已确认 Client 不生成 `clientId`，中心身份必须由 Node Server 分配。
- 注册只是“把入口登记进中心”，不是“完成业务放行”。

## 3. 业务边界

### 3.1 负责什么

- 校验 `baseUrl` / `dockerApiUrl`
- 生成 `clientId`
- 写入 `edge_clients`
- 初始化基础状态

### 3.2 不负责什么

- 不自动探测 Docker
- 不自动探测 `/health`
- 不自动 verify
- 不自动标记 `healthy`

## 4. 请求与响应

最小请求体：

```json
{
  "name": "node-001",
  "baseUrl": "http://192.168.10.119:3300",
  "dockerApiUrl": "http://192.168.10.119:2375"
}
```

返回重点：

- `clientId`
- `mainAccountId`
- `clientSequence`
- `name`
- `baseUrl`
- `dockerApiUrl`
- `healthStatus=stale`
- `discoveryStatus=blocked`
- `arch=unknown`

## 5. 前置校验

- 需要 Platform Header
- `baseUrl` 必须是合法 HTTP 地址
- `dockerApiUrl` 如果存在也必须合法
- `baseUrl` 不能和已有活动节点重复

## 6. 成功判定

- 中心记录写入成功
- `clientId` 唯一

## 7. 失败判定

- URL 非法
- `baseUrl` 冲突
- SQLite 写入失败

## 8. 后续动作

注册成功后，管理员仍应继续执行：

- [refresh-node-device-info.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/refresh-node-device-info.md)
- [verify-node.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/verify-node.md)
