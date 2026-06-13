# Node Server 接口设计：`GET /api/v1/server/dashboard`

## 1. 功能目标

`GET /api/v1/server/dashboard` 规划用于返回 Node Server 的最小中心统计视图，给管理员和管理端快速看到当前节点规模、环境规模与任务失败风险。

## 2. 当前状态

- 尚未实现
- 当前返回 `CodeNotImplemented`

## 3. 规划来源

- 随着 Node Server 开始承载节点、环境和任务聚合，单看 `/health` 已不足以支撑管理员判断平台运行情况。
- 用户已经明确把 Dashboard 放在生命周期代理之后作为下一阶段工作。

## 4. 计划统计项

- 节点总数
- `healthy + verified + online` 节点数
- 环境包总数
- `running` 环境包数
- 失败任务数

## 5. 数据来源

必须来自：

- `edge_clients`
- `server_browser_envs`
- `server_tasks`

不能依赖：

- Server 内存临时状态
- 未落库的瞬时统计

## 6. 业务边界

- 只做中心层汇总
- 不逐个实时扫 Edge 节点
- 不替代任务列表、节点列表、环境包列表明细接口

## 7. 落地要求

实现时必须保证：

- 查询成本可控
- 统计定义稳定
- 和 `list-*` 接口的数字口径一致
