# Node Server 接口设计：`GET /health`

## 1. 功能目标

`GET /health` 用于返回 `Private_Browser_Server` 自身的基础健康摘要，帮助开发、运维和部署脚本快速确认服务是否已启动、配置是否已加载、SQLite 初始化是否完成。

它的目标是“确认 Node Server 自己是否活着”，不是确认整个平台所有节点都健康。

## 2. 设计来源

- 当前 `Private_Browser_Server` 需要独立部署在控制面节点上，运维首先需要一个不依赖业务 Header 的存活检查入口。
- 用户已经把节点健康、环境包状态、任务状态区分为多层事实，因此 `/health` 不能越权扮演 Dashboard 或业务调度状态入口。

## 3. 业务边界

### 3.1 负责什么

- 返回服务名、版本、模式
- 返回配置文件路径
- 返回 SQLite 基础配置
- 返回 `romInitialized`

### 3.2 不负责什么

- 不扫描所有 Edge 节点
- 不聚合环境包和任务统计
- 不给出 `healthy/unhealthy/offline` 这类平台业务状态结论

## 4. 请求与响应

```http
GET /health
```

无请求体，无 Platform Header 要求。

返回重点字段：

- `ok`
- `service`
- `mode`
- `version`
- `configFile`
- `sqlite.path`
- `sqlite.maxOpenConns`
- `sqlite.maxIdleConns`
- `romInitialized`

## 5. 成功判定

只要 Node Server 进程能够正常返回该结构，即视为服务接口可用。

其中：

- `romInitialized=true` 表示 SQLite 初始化完成
- `romInitialized=false` 表示服务虽然活着，但控制面基础设施尚未完成

## 6. 联调验收标准

- 服务启动后 `/health` 能返回 200
- 配置文件路径与当前运行配置一致
- SQLite 初始化成功时 `romInitialized=true`

## 7. 相关接口

- [dashboard.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/api/dashboard.md)
