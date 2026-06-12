# Node Server 接口设计：`POST /api/v1/envs/import-package`

## 1. 功能目标

`POST /api/v1/envs/import-package` 规划用于让 `Private_Browser_Server` 代理目标 Edge 导入外部标准环境包，并把导入结果纳入中心资产视图。

成功后的业务结论应是：

- Edge 已导入标准包
- 原 `envId/userId/rpaType` 被保留
- 本机 `envSequence/CDP/VNC/containerName/containerId/containerStatus/monitorStatus` 已重新分配
- 环境进入可后续 `run` 的创建态

## 2. 设计来源

- 用户要求导入必须保留环境身份，但运行摘要必须按当前服务器重新分配。
- 导入不是 restore，因为 restore 只认本机已登记备份包；import-package 处理的是外部标准包。

## 3. 业务边界

### 3.1 负责什么

- 校验目标 `clientId`
- 创建中心 task
- 上传或转发标准包给 Edge
- 导入成功后写入或刷新中心环境聚合记录

### 3.2 不负责什么

- 不自动 run
- 不自动 pull-image
- 不自动创建运行容器
- 不跨节点随意迁移已存在环境

## 4. 前置校验

规划建议：

1. 指定明确 `clientId`
2. `clientId` 必须通过 `EnsureClientReadyForBusiness`
3. 导入包必须符合 Edge 已接受的标准格式

## 5. 任务编排

建议采用中心 task：

- `taskType=import_package_env` 或未来补充常量
- 创建中心 task
- 调 Edge `POST /api/v1/edge/browser-envs/import-package`
- 绑定 `edgeTaskId`

## 6. 成功判定

规划建议：

- Edge task 成功，或
- Edge task 丢失但能重新读取到导入后的 env detail，并确认其进入可运行前的创建态

## 7. 中心缓存收口

导入成功后，中心层至少应写入：

- `envId`
- `clientId`
- `status=created`
- `rpaType`
- `name`
- `cdpUrl/webVncUrl` 按导入后实际值或创建态规则
- `lastTaskId`
- `lastError=""`

## 8. 失败判定

- 节点不 ready
- 导入包不合法
- Edge import-package 失败
- Edge task 丢失且无法确认导入事实

## 9. 当前实现状态

截至 `2026-06-12`：

- 尚未落地
- 已进入正式生命周期代理规划范围
