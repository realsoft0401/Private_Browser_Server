# Private_Browser_Server V1 中心节点层开发内容拆解

## 1. 文档目标

这份文档只做一件事：

- 把 `Private_Browser_Server` 第一阶段 `P0：中心节点层` 具体要开发什么，拆成可执行、可验收、可排障的文档清单。

这份文档只谈：

- 节点中心身份
- 节点发现与验证
- 节点绑定与解绑
- 节点健康与心跳收口
- 节点治理审计
- 节点层最小任务事实
- 节点层本地额度快照

这份文档不谈：

- `browser-env` 生命周期编排实现
- `run/stop/backup/restore/import-package/delete` 主业务链路
- 平台正式额度接口联调细节
- 前端页面样式
- SDK、门户、企业级展示层

一句话收口：

- `P0` 的目标不是先把业务动作做花，而是先把 `Server` 作为中心节点事实源立起来。

## 2. P0 定位

`Private_Browser_Server` 在 `P0` 的正式定位必须固定为：

```text
中心节点治理层
  -> 发现 Client
  -> 识别 Client
  -> 绑定 Client
  -> 解绑 Client
  -> 验证 Client
  -> 维护 Client 健康摘要
  -> 维护 Client 中心身份
  -> 为后续 browser-env 生命周期提供准入前提
```

这一层负责回答下面这些中心问题：

- 这台 Client 是不是我们平台里的正式节点。
- 这台 Client 当前归属哪个主账号。
- 这台 Client 当前是否 `healthy + verified`。
- 这台 Client 的 IP、入口地址、设备事实是否仍然可信。
- 这台 Client 最近一次发现、心跳、探测、解绑、重绑是什么结果。
- 这台 Client 当前本地记住的额度快照是什么。

这一层不负责回答：

- 某个 `browser-env` 现在能不能 run。
- 某个 `slot` 现在是不是被某个包占用。
- 某个 backup / restore 任务是否成功。

这些属于后续 `P1/P2`。

## 3. P0 范围清单

`P0` 明确纳入范围的开发内容只有下面 8 类：

1. `edge_clients` 正式节点表
2. UDP discovery 监听与内存 discovered 视图
3. HTTP probe 校验链路
4. bind
5. unbind
6. heartbeat / health 收口
7. `server_tasks` 的节点层最小用法
8. `client_run_quotas` 本地额度快照

## 3.1 当前实现状态说明

为了避免后续把“已经落地”和“还在设计”混在一起，这里统一加一层状态口径：

- `已完成，待测试`
  - 代码和接口骨架已经存在，下一步重点是联调、回归和错误口径确认。
- `部分完成，待补齐`
  - 已有一部分表、路由、服务或文档，但还没达到正式可验收状态。
- <span style="color:red">`未完成`</span>
  - 当前还只是方案、占位，或者实现还没落到可验收状态。

按当前仓库代码状态，`P0` 先做一次初步标记：

1. `edge_clients` 正式节点表：`已完成，待测试`
2. UDP discovery 监听与内存 discovered 视图：`部分完成，待补齐`
3. HTTP probe 校验链路：`已完成，待测试`
4. bind：`已完成，待测试`
5. unbind：`已完成，待测试`
6. heartbeat / health 收口：`部分完成，待补齐`
7. `server_tasks` 的节点层最小用法：<span style="color:red">`未完成`</span>
8. `client_run_quotas` 本地额度快照：`部分完成，待补齐`

## 3.2 P0 总览跟踪表

| 模块 | 当前状态 | 说明 |
| --- | --- | --- |
| `edge_clients` | `已完成，待测试` | 建表、基础字段、列表/详情查询已经有代码骨架 |
| discovery 视图 | `部分完成，待补齐` | 当前明确只有 UDP beacon 才参与发现；memory discovered 查询已落地，UDP 联调与完整回写待补 |
| probe 校验链路 | `已完成，待测试` | `/health`、`/device-info` 探测已进入主链路 |
| bind | `已完成，待测试` | bind 主链路、查重、序号生成、push 兼容链路已落地 |
| unbind | `已完成，待测试` | 中心解绑、Client 本地登记清理、审计回写已经落地，待联调回归 |
| heartbeat / health | `部分完成，待补齐` | 心跳接收已有，完整状态推进与恢复机制未收口 |
| `server_tasks` | <span style="color:red">`未完成`</span> | 节点层最小任务事实还未立住 |
| `client_run_quotas` | `部分完成，待补齐` | 表结构已在，查询与写入接口未落地 |

## 4. P0 不在范围

下面这些能力本期不要混进来：

- `browser-env` 创建、运行、停止、备份、恢复、导入、删除
- 中心 `slot` 正式主表
- 自动调度到不同 Client
- 平台正式额度实时同步协议
- 批量节点操作
- 节点公网鉴权方案
- 页面级大盘和运营后台
- SDK 自动生成

## 5. 分域开发内容

## 5.1 Discovery 域

### 目标

- 让 Server 能在独立内网里看到哪些 Client 正在广播自己。

### 当前状态

- `部分完成，待补齐`

当前已经具备：

- discovered 内存视图
- `GET /api/v1/edge-clients/discovered`
- `POST /api/v1/edge-clients/heartbeat`
- discovery probe 结果归一化到 discovered 视图

当前还没完全收口：

- UDP beacon 监听真实联调回归
- discovered 过期清理口径验证
- 与已绑定节点的发现事实回写完整验证

### 负责内容

- 监听 UDP beacon。
- 校验 `discoveryMagic / service / discoveryGroup / protocolVersion`。
- 过滤掉不是本平台或不是当前发现域的广播。
- 生成内存态 discovered 列表。
- 对已登记节点回写最近发现时间。

### 不负责内容

- 不生成 `clientId`。
- 不自动绑定账号。
- 不自动放行业务动作。
- 不把 discovered 当成正式持久实体。

### 要开发的具体项

1. UDP 监听器：`部分完成，待补齐`
2. Beacon payload 校验器：`已完成，待测试`
3. Discovered 内存存储：`已完成，待测试`
4. 已登记节点发现时间回写器：`部分完成，待补齐`
5. Discovered 查询接口：`已完成，待测试`
6. Discovered 过期清理策略：`部分完成，待补齐`

### 关键规则

- `discovered` 只是过程视图，不是正式节点表状态。
- discovered 默认放内存，不单独落正式表。
- 正式持久化只回写到 `edge_clients.last_discovered_at` 等摘要字段。
- UDP 载荷只能包含非敏感服务摘要，不能带登录态、代理明文、profile 细节、指纹 raw。

## 5.2 Probe / Verify 域

### 目标

- Server 不能只看到 UDP 就相信节点，必须通过 Client HTTP API 再验证一次节点事实。

### 当前状态

- `已完成，待测试`

当前代码里已经有：

- Edge HTTP client
- `/health` 探测
- `/api/v1/edge/device-info` 探测
- bind / heartbeat 里的 probe 调用链

但还需要继续测试：

- 失败超时的错误映射是否稳定
- 设备事实变化时的阻断口径是否完全符合文档
- `identity_changed / ip_mismatch` 的正式收口是否已经完全落盘

### 负责内容

- 调用 Client `/health`
- 调用 Client `/api/v1/edge/device-info` 或等价接口
- 核验 `baseUrl/clientIp`
- 核验设备事实摘要
- 核验 Docker 能力摘要
- 给节点打出 `verified / blocked` 结论

### 不负责内容

- 不读取 Client SQLite
- 不读取 Client 环境包目录
- 不通过 SSH 绕过 Edge API

### 要开发的具体项

1. Edge HTTP client：`已完成，待测试`
2. `/health` 探测器：`已完成，待测试`
3. `/device-info` 探测器：`已完成，待测试`
4. 设备事实归一化器：`已完成，待测试`
5. 探测失败错误映射：`已完成，待测试`
6. 探测结果收口器：`部分完成，待补齐`

### 关键规则

- 只有探测成功，节点才能进入 `verified`。
- 如果设备事实变化明显，不能静默覆盖，必须打成 `identity_changed`。
- 如果 `clientIp/baseUrl` 和历史记录冲突，不能自动改写，必须打成 `ip_mismatch`。
- `verify` 不作为常规主链路步骤；正常情况下 bind 成功且探测通过就直接 `verified`。

## 5.3 Bind 域

### 目标

- 把一个正式 Client 绑定到一个主账号名下，并生成稳定的中心节点身份。

### 当前状态

- `已完成，待测试`

当前已经落地：

- `POST /api/v1/edge-clients/bind`
- bind 请求校验
- probe 后绑定
- `clientId` 生成
- `edge_clients` 写库
- `edge_client_bind_logs` 写入
- 过渡兼容 `push-client-id`

当前还要重点测试：

- 重复绑定
- 跨账号重复绑定
- bind 成功但 push 失败不回滚
- 并发 bind 下序号分配稳定性

### 负责内容

- 接收 bind 请求
- 通过 `accountId + clientIp` 或显式 `baseUrl` 定位 Client
- 先探测，后绑定
- 生成 `clientId`
- 回写 `edge_clients`
- 记录绑定审计

### 不负责内容

- 不创建 `browser-env`
- 不下发 run 额度
- 不自动执行后续业务动作

### 要开发的具体项

1. Bind 请求校验：`已完成，待测试`
2. 节点查重逻辑：`已完成，待测试`
3. 节点归属互斥校验：`已完成，待测试`
4. `client_sequence` 分配器：`已完成，待测试`
5. `clientId = mainAccountId + 4位设备序号` 生成器：`已完成，待测试`
6. Bind 审计日志写入：`已完成，待测试`
7. 过渡兼容的 Client 本地登记下发结果记录：`已完成，待测试`

### 关键规则

- 同一个 Client 不能跨账号重复绑定。
- 若已绑定其它账号，必须明确返回“先解绑再绑定”。
- `client_sequence` 按 `MAX(client_sequence)+1` 分配，不回收旧序号。
- bind 成功且 probe 成功后，节点直接进入 `verified`。
- 绑定成功后如果兼容链路下发 Client 本地登记失败，中心绑定仍然成立，但必须留痕。

## 5.4 Unbind 域

### 目标

- 显式解除账号和节点的归属关系，但不能破坏节点中心身份与历史审计。

### 当前状态

- <span style="color:red">`未完成`</span>

### 负责内容

- 接收 unbind 请求
- 校验当前节点是否已绑定
- 解除归属关系
- 尝试清空 Client 本地 `node-registration.json`
- 记录解绑审计

### 不负责内容

- 不分配新 `clientId`
- 不删除历史任务
- 不删除历史环境聚合记录

### 要开发的具体项

1. Unbind 请求校验：<span style="color:red">`未完成`</span>
2. 中心解绑写库：<span style="color:red">`未完成`</span>
3. Client 本地登记清理调用：<span style="color:red">`未完成`</span>
4. 清理失败留痕：<span style="color:red">`未完成`</span>
5. Unbind 审计日志写入：<span style="color:red">`未完成`</span>

### 关键规则

- unbind 后原 `clientId` 不变。
- 重新绑定时继续沿用原节点身份与历史审计口径。
- 清空 Client 本地 JSON 失败时，不回滚中心解绑。
- 解绑是治理动作，不是删除节点历史。

## 5.5 Heartbeat / Health 域

### 目标

- Server 要持续知道这台节点是不是还在线、是不是还能正常提供服务。

### 当前状态

- `部分完成，待补齐`

当前已经落地：

- `GET /health`
- `POST /api/v1/edge-clients/heartbeat`
- `edge_clients` 里心跳与最近检查摘要字段

当前还没完全收口：

- `healthy / unhealthy / stale / offline` 全状态推进
- 超时阈值任务化收敛
- 节点恢复后的自动重探测

### 负责内容

- 收心跳
- 维护 `healthy / unhealthy / stale / offline`
- 维护 `last_heartbeat_at / last_checked_at`
- 处理探测失败阈值
- 对恢复上线节点重新做探测

### 不负责内容

- 不直接放行业务动作
- 不替代 `browser-env` 状态同步

### 要开发的具体项

1. 心跳接收接口或心跳收口服务：`已完成，待测试`
2. 心跳阈值配置：`部分完成，待补齐`
3. 超时状态推进器：<span style="color:red">`未完成`</span>
4. 节点恢复重探测器：<span style="color:red">`未完成`</span>
5. 健康摘要查询接口：`部分完成，待补齐`

### 关键规则

- `health_status` 只允许：
  - `healthy`
  - `unhealthy`
  - `stale`
  - `offline`
- `discovery_status` 只允许：
  - `blocked`
  - `verified`
- 业务放行必须同时满足 `health_status=healthy` 和 `discovery_status=verified`
- 建议默认阈值：
  - `heartbeat_interval_seconds=15`
  - `stale_after_seconds=30`
  - `offline_after_seconds=90`
  - `failure_threshold=3`

## 5.6 Edge Client Identity 域

### 目标

- 让中心知道“这是谁”，并能稳定追踪它，而不是只认一段会变化的 IP。

### 当前状态

- `部分完成，待补齐`

当前已经落地：

- `edge_clients` 基础表
- 已绑定节点列表与详情接口
- 节点基础身份字段

当前还没完全收口：

- `identity_changed / ip_mismatch` 人工治理闭环
- 节点人工重验入口
- 更完整的事实差异比对

### 负责内容

- 维护 `edge_clients`
- 维护 `clientId`
- 维护设备事实摘要
- 维护 `identity_changed / ip_mismatch` 等治理原因

### 不负责内容

- 不保存 Edge 全量业务资产
- 不保存 slot 真相
- 不保存 profile / proxy / fingerprint 原文

### 要开发的具体项

1. `edge_clients` 建表与迁移：`已完成，待测试`
2. 节点详情查询：`已完成，待测试`
3. 节点列表查询：`已完成，待测试`
4. 节点事实差异比对：`部分完成，待补齐`
5. 节点人工重验入口：<span style="color:red">`未完成`</span>

### 关键规则

- 中心正式身份真相源只能是 `edge_clients`
- `clientIp/baseUrl` 只是接入事实，不是长期稳定身份
- 节点事实变化不能静默覆盖，必须先阻断、再确认

## 5.7 Server Task 域

### 目标

- 即使在 `P0` 还没做 browser-env 生命周期，节点层关键治理动作也要有最小任务事实和审计留痕。

### 当前状态

- <span style="color:red">`未完成`</span>

### 负责内容

- 为 bind / unbind / verify / repair 类动作预留 `server_tasks` 最小事实
- 保存最终 `success / failed`
- 保存错误摘要与建议

### 不负责内容

- 不做 Edge 侧长期任务镜像
- 不和 Client task 形成双真相源

### 要开发的具体项

1. `server_tasks` 最小字段定义：<span style="color:red">`未完成`</span>
2. 节点层动作任务创建器：<span style="color:red">`未完成`</span>
3. 动作完成收口器：<span style="color:red">`未完成`</span>
4. 任务查询接口：<span style="color:red">`未完成`</span>

### 关键规则

- Server task 才是平台任务真相源。
- Client 重启、SSE 丢失、瞬时异常，都不能让 Server 默认成功。
- 节点层任务终态只有 `success / failed`。

## 5.8 Quota Snapshot 域

### 目标

- 为后续 `P2` 的 run 准入提前准备一个中心本地额度快照容器，但本期不展开平台联调。

### 当前状态

- `部分完成，待补齐`

当前已经落地：

- `client_run_quotas` 表结构

当前还没有落地：

- 快照写入接口
- 快照查询接口
- 过期策略和状态口径

### 负责内容

- 保存平台下发到该 Client 的本地额度快照
- 记录快照时间、来源、版本或备注

### 不负责内容

- 不在 `P0` 正式拦 run
- 不替代平台端的真实额度真相源

### 要开发的具体项

1. `client_run_quotas` 建表与迁移：`已完成，待测试`
2. 快照写入接口或内部服务：<span style="color:red">`未完成`</span>
3. 快照查询接口：<span style="color:red">`未完成`</span>
4. 快照过期字段设计：`部分完成，待补齐`

### 关键规则

- 平台额度真相源始终是 PlatformServer。
- Server 本地只保留快照，不自作主张推导商业额度。
- `P0` 先把存储位置和查询口径立住，正式准入到 `P2` 再收。

## 6. 状态机与收口规则

## 6.1 Discovery 侧

```text
UDP seen
  -> discovered(内存态)
  -> probe success -> bind success -> verified
  -> probe failed -> blocked + discovery_reason
```

关键收口：

- `discovered` 不进入 `edge_clients.discovery_status`
- 正式节点只看 `blocked / verified`
- 原因统一放 `discovery_reason`

## 6.2 Health 侧

```text
healthy
  -> unhealthy   (Client 可达但本机 checks 异常)
  -> stale       (缓存不可信/短时无法确认)
  -> offline     (节点失联或超阈值未恢复)
```

恢复规则：

```text
stale/offline
  -> 重新 /health + /device-info + Docker 摘要探测
  -> 探测成功且 checks 正常 -> healthy
  -> 探测成功但 checks 异常 -> unhealthy
  -> 事实变化明显 -> blocked + discovery_reason=identity_changed/ip_mismatch
```

## 6.3 Bind / Unbind 侧

```text
未登记线索
  -> bind success -> edge_clients 正式节点
  -> verified

verified 节点
  -> unbind success -> 中心解除归属
  -> 保留原 clientId 与历史审计
```

## 7. 存储与审计要求

## 7.1 `edge_clients`

至少要承载下面几类事实：

- 中心身份：`clientId/main_account_id/client_sequence`
- 接入地址：`base_url/client_ip/docker_api_url`
- 设备摘要：`os/arch/cpu/memory/docker_version`
- 健康摘要：`health_status/last_checked_at/last_error`
- 发现摘要：`discovery_status/discovery_reason/last_discovered_at/last_heartbeat_at`
- 审计字段：`created_at/updated_at/deleted_at`

## 7.2 `edge_client_bind_logs`

必须记录：

- 谁发起 bind / unbind
- 目标账号
- 目标 Client
- 当时的 `baseUrl/clientIp`
- 操作结果
- 失败原因
- 是否完成 Client 本地登记下发或清理

## 7.3 `server_tasks`

节点层最小任务建议至少记录：

- `taskId`
- `taskType`
- `resourceType=edge_client`
- `resourceId=clientId`
- `status`
- `error_message`
- `suggestion`
- `created_by`
- `started_at`
- `finished_at`

## 7.4 `client_run_quotas`

建议至少记录：

- `clientId`
- `mainAccountId`
- `quota_total`
- `quota_used`
- `quota_available`
- `snapshot_status`
- `snapshot_version`
- `snapshot_at`
- `last_error`

## 7.5 明确不能存的内容

Server SQLite 明确不要保存：

- `profile.json` 全文
- `binding.json` 全文
- 代理明文
- 指纹 raw
- 浏览器登录态内容
- Cookies / Local Storage / IndexedDB / Session Storage / Login Data
- Edge SQLite 镜像

## 8. 接口开发内容

`P0` 建议至少形成下面这些正式接口文档与实现规划。

## 8.0 接口状态总览表

| 接口 | 当前状态 | 备注 |
| --- | --- | --- |
| `GET /api/v1/edge-clients/discovered` | `已完成，待测试` | discovered 临时视图查询已存在 |
| `GET /api/v1/edge-clients` | `已完成，待测试` | 已绑定节点列表已存在 |
| `GET /api/v1/edge-clients/{clientId}` | `已完成，待测试` | 已绑定节点详情已存在 |
| `POST /api/v1/edge-clients/bind` | `已完成，待测试` | bind 主链路已存在 |
| `POST /api/v1/edge-clients/{clientId}/push-client-id` | `已完成，待测试` | 过渡兼容补推链路已存在 |
| `POST /api/v1/edge-clients/{clientId}/unbind` | `已完成，待测试` | 中心解绑与 Client 本地登记清理已落地 |
| `POST /api/v1/edge-clients/{clientId}/recheck` | <span style="color:red">`未完成`</span> | 管理员重探测接口未落地 |
| `POST /api/v1/edge-clients/heartbeat` | `已完成，待测试` | 当前仓库实际实现的是无 path 参数版本 |
| `GET /api/v1/edge-clients/{clientId}/quota` | <span style="color:red">`未完成`</span> | quota 查询未落地 |
| `GET /api/v1/server-tasks/{taskId}` | <span style="color:red">`未完成`</span> | 节点层 task 查询未落地 |

## 8.1 Discovery / Node 查询

1. `GET /api/v1/edge-clients/discovered`
2. `GET /api/v1/edge-clients`
3. `GET /api/v1/edge-clients/{clientId}`

当前状态：

- `GET /api/v1/edge-clients/discovered`：`已完成，待测试`
- `GET /api/v1/edge-clients`：`已完成，待测试`
- `GET /api/v1/edge-clients/{clientId}`：`已完成，待测试`

### 这组接口负责

- 给管理员看当前看到哪些节点
- 看哪些已经正式登记
- 看某个节点当前治理状态

### 这组接口不负责

- 不触发生命周期动作
- 不修改节点归属

## 8.2 Bind / Unbind

1. `POST /api/v1/edge-clients/bind`
2. `POST /api/v1/edge-clients/{clientId}/unbind`

当前状态：

- `POST /api/v1/edge-clients/bind`：`已完成，待测试`
- `POST /api/v1/edge-clients/{clientId}/unbind`：`已完成，待测试`
- `POST /api/v1/edge-clients/{clientId}/push-client-id`：`已完成，待测试`

### 这组接口负责

- 正式建立或解除节点归属

### 这组接口不负责

- 不直接创建环境包
- 不直接启动浏览器容器

## 8.3 Probe / Repair

1. `POST /api/v1/edge-clients/{clientId}/recheck`

当前状态：

- `POST /api/v1/edge-clients/{clientId}/recheck`：<span style="color:red">`未完成`</span>

### 这组接口负责

- 管理员手动重探测
- 修复 `stale / ip_mismatch / identity_changed` 后的重新确认

### 这组接口不负责

- 不作为正常 bind 主线必须步骤

## 8.4 Heartbeat / Quota / Task

1. `POST /api/v1/edge-clients/{clientId}/heartbeat`
2. `GET /api/v1/edge-clients/{clientId}/quota`
3. `GET /api/v1/server-tasks/{taskId}`

当前状态：

- 当前仓库实际已实现的是 `POST /api/v1/edge-clients/heartbeat`：`已完成，待测试`
- `GET /api/v1/edge-clients/{clientId}/quota`：<span style="color:red">`未完成`</span>
- `GET /api/v1/server-tasks/{taskId}`：<span style="color:red">`未完成`</span>

### 这组接口负责

- 维护节点活性摘要
- 暴露本地额度快照
- 查询节点层任务事实

## 8.5 SSE 口径

`P0` 的节点层接口原则上优先普通 HTTP，不滥用 SSE。

建议口径：

- `discovered/list/detail/quota`：普通 HTTP
- `bind/unbind/recheck`：如果第一版动作较快，可先普通 HTTP；只有明确拆成多阶段、耗时明显、管理员需要持续观察时，再升级成 task + SSE
- `heartbeat`：普通 HTTP

当前建议：

- `P0` 先不要为了“看起来高级”把节点层全部做成 SSE。
- 等节点重探测、批量修复、平台协同明显变长时，再为特定动作引入 task + SSE。

## 9. 日志与错误规范

节点层至少要做到下面 4 层留痕：

1. API 响应错误
2. `server_tasks.error_message`
3. `edge_clients.last_error`
4. 结构化服务日志

日志字段至少建议包含：

- `taskId`
- `clientId`
- `mainAccountId`
- `clientIp`
- `baseUrl`
- `action`
- `healthStatus`
- `discoveryStatus`
- `discoveryReason`
- `errorSource`
- `error`
- `suggestion`

错误信息必须说清：

- 为什么失败
- 卡在哪一层
- 当前影响什么
- 管理员下一步应该怎么修

## 10. P0 完成标准

当下面这些点全部满足时，才算 `中心节点层 P0` 完成：

1. Server 能稳定接收 UDP beacon，并看到 discovered 列表
2. Server 能通过 `/health + /device-info` 探测节点事实
3. Server 能把一个节点正式 bind 到主账号，并生成稳定 `clientId`
4. 同一个 Client 被其它账号重复绑定时，Server 会明确拒绝
5. Server 能执行 unbind，并保留原 `clientId` 与历史审计
6. Server 能识别 `healthy / unhealthy / stale / offline`
7. Server 能识别 `verified / blocked`，并用 `discovery_reason` 表达阻断原因
8. 节点 IP 或设备事实变化时，Server 不会静默覆盖，而会进入治理状态
9. `edge_clients / edge_client_bind_logs / server_tasks / client_run_quotas` 四类核心中心数据已经立住
10. 节点层错误能做到 API、库表、日志三方可追

补充说明：

- 其中已经标成 `已完成，待测试` 的接口，不代表可以直接封板。
- 只有回归通过、错误口径确认、OpenAPI 示例与真实实现一致后，才算真正可验收。

## 11. P0 之后自然进入的下一步

`P0` 完成后，后续应该顺着下面顺序推进：

1. `P1`：Server 调 Client 的 `browser-env` 生命周期主链路
2. `P2`：平台额度与 run 准入
3. `P3`：解绑 / 重绑治理补全
4. `P4`：SDK 与企业级 API 门户

## 12. 仍保留但不阻塞 P0 的问题

下面这些问题仍然值得记录，但不阻塞 `P0` 开工：

1. 平台额度正式回调接口最终长什么样
2. bind / recheck 是否在第二版升级成任务化 + SSE
3. 未来公网或共享内网时的节点鉴权方案
4. `push clientId / node-registration.json` 兼容链路未来怎么退场
5. SQLite 到 MySQL 的正式迁移时间点

一句话收口：

- `P0` 先把“谁是正式节点、谁归属谁、谁可用、谁不可用、为什么不可用”这套中心真相源立住，再进入真正的业务生命周期。
