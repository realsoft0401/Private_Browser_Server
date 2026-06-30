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
- [openapi.yaml](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/openapi.yaml)
