# Private_Browser 四层架构与中心节点层技术设计

## 1. 文档目标

本文定义 `Private_Browser` 四层架构及 `Node Server P0` 中心节点层技术设计。

范围包括：

1. 四层应用关系
2. 四层数据关系
3. 四层状态关系
4. 四层调用关系
5. 四层责任边界
6. `Node Server P0` 的技术定位

目标：

- 明确 `Virtual Browser`、`Edge Client`、`Node Server`、`Platform Server` 的层级关系、职责边界、数据归属、状态归属和准入关系。

## 2. 设计依据

设计依据：

- [AGENTS.md](/Users/lining/Documents/Browser_virtualization/AGENTS.md)
- [project.md](/Users/lining/Documents/Browser_virtualization/project.md)
- [server-v1-central-node-layer-breakdown.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-central-node-layer-breakdown.md)
- [server-v1-database-design.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-database-design.md)
- [server-v1-open-questions.md](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/docs/server-v1-open-questions.md)

不参考：

- old 历史库存

## 3. 四层架构

四层结构：

```text
Platform Server
  -> Node Server
    -> Edge Client
      -> Virtual Browser
```

分层原则：

- 按职责层级分层，不按代码量分层。
- 同一份核心事实在系统中只允许有一个真相源层。
- 上层不直接绕过中间层访问下层真相源。

## 4. 四层总体定位

## 4.1 Virtual Browser

`Virtual Browser` 是浏览器运行层。

层职责：

1. 提供浏览器实际运行单元
2. 承载 Chromium 进程
3. 承载 CDP/VNC/WebVNC 运行能力
4. 承载容器内启动参数、运行时环境和镜像 contract 执行结果

直接管理对象：

1. 浏览器容器
2. 浏览器进程
3. CDP 端口
4. VNC 端口
5. 容器内运行时环境

不承担：

1. 商业 `clientId`
2. 主账号归属
3. 平台额度
4. 中心节点治理状态
5. 平台任务语义

## 4.2 Edge Client

`Edge Client` 是边缘执行层。

层职责：

1. 管理本机 Docker 与本机浏览器运行资源
2. 管理 slot 与 browser-env 本机资产
3. 管理 Virtual Browser 生命周期
4. 暴露边缘受控 API
5. 对上提供本机健康、设备、运行摘要

直接管理对象：

1. 本机 Docker
2. 本机 slot
3. 本机 browser-env
4. 本机 profile / binding / proxy / fingerprint
5. 本机 `/health`
6. 本机 `/api/v1/edge/device-info`
7. Virtual Browser 运行单元

不承担：

1. 多节点治理
2. 商业归属最终真相
3. 平台额度最终真相
4. 平台级任务真相

## 4.3 Node Server

`Node Server` 是中心节点治理层。

层职责：

1. 节点发现
2. 节点 probe
3. 节点 bind / unbind
4. 节点 heartbeat / health 收口
5. 节点中心身份 `clientId`
6. 节点审计
7. 节点层 `server_tasks`
8. 平台额度快照

直接管理对象：

1. Edge Client 节点集合
2. 节点中心身份
3. 节点归属关系
4. 节点治理状态
5. 节点中心任务事实

不直接持有：

1. Edge 本机资产正文
2. Virtual Browser 运行细节正文
3. Platform 商业最终真相

## 4.4 Platform Server

`Platform Server` 是商业与平台规则层。

层职责：

1. 主账号体系
2. 商业设备归属
3. 平台额度真相
4. 平台级审计
5. 平台级产品规则
6. 平台放行结论

直接管理对象：

1. 账号
2. 商业机位
3. 额度
4. 平台规则

不直接持有：

1. Edge 本机文件
2. Edge 本机 Docker 细节
3. Virtual Browser 运行细节

## 5. 四层应用关系

## 5.1 Virtual Browser 与 Edge Client

关系定义：

- `Virtual Browser` 提供浏览器运行事实
- `Edge Client` 管理并封装浏览器运行单元

### Virtual Browser 向 Edge Client 暴露的运行事实

1. 容器是否存在
2. 容器是否 running
3. 浏览器进程是否成功启动
4. CDP 是否可连
5. VNC / WebVNC 是否可连
6. 当前 env 是否已真正加载

### Edge Client 对 Virtual Browser 的控制动作

1. 创建空白 waiting 容器
2. 将 browser-env 加载到指定 slot
3. 启动浏览器
4. 停止浏览器
5. 释放运行态
6. 恢复空白 waiting 容器

约束：

- Virtual Browser 只提供运行事实，不承担平台语义。

## 5.2 Edge Client 与 Node Server

关系定义：

- `Edge Client` 提供本机能力
- `Node Server` 编排和治理本机能力

### Edge Client 向 Node Server 暴露的应用能力

1. `/health`
2. `/api/v1/edge/device-info`
3. discovery / heartbeat
4. Docker 状态与动作
5. browser-env 生命周期 API
6. WebVNC / CDP 地址摘要

### Edge Client 向 Node Server 暴露的镜像版本事实

1. slot 默认基础镜像配置
2. 每个 slot 当前实际 `runtimeImage`
3. 每个 browser-env 当前正式 `runtime.image`

约束：

- Node Server 不能把“默认基础镜像”误当成“所有 slot 当前实际镜像”
- Node Server 不能因为默认值变更就自动认定老 slot 已升级
- Node Server 如果后续编排镜像升级，必须显式区分：
  1. 改默认值
  2. 升级某个已有 slot 基础镜像
  3. 升级某个 browser-env 正式运行镜像

### Node Server 对 Edge Client 的治理要求

1. 可发现
2. 可探测
3. 可治理
4. 可留痕
5. 可阻断

镜像治理补充：

1. Node 可以决定后续新建 slot 使用哪个默认基础镜像
2. Node 可以显式触发某个 waiting slot 的基础镜像重初始化升级
3. Node 可以显式触发某个 browser-env 的正式运行镜像变更
4. Node 不能因为默认镜像从 `1.1` 变成 `1.2`，就自动覆盖老 slot 或老 env 的当前镜像事实
5. Node 修改 browser-env 正式运行镜像时，中心 env 状态必须是 `created` 或 `stopped`；`loading/running/ending/backed_up/deleted/error` 一律拒绝
6. browser-env 镜像修改后不自动 run、不自动 pull image、不自动 reinit slot，下一次 run 才读取新的 `runtime.image`
7. `created` 表示首次运行前配置态；`stopped` 表示运行后已与 slot/container 彻底隔离的干净态；二者都允许修改 runtime.image

约束：

- Edge Client 不承担平台业务语义，只提供稳定的边缘事实与受控执行能力。

## 5.3 Node Server 与 Platform Server

关系定义：

- `Node Server` 负责节点中心治理
- `Platform Server` 负责商业与平台决策

### Node Server 向 Platform Server 上报或同步的内容

1. `clientId`
2. 节点归属结果
3. 节点健康摘要
4. 节点发现状态摘要
5. 节点最近错误摘要
6. 平台对账所需的最小运行摘要

### Platform Server 向 Node Server 下发的内容

1. 商业归属结果
2. 运行额度
3. 放行 / 阻断规则
4. 上层业务动作

约束：

- Platform Server 不直接调用 Edge Client，不直接理解容器和 profile 细节。

## 5.4 Platform Server 与 Edge Client

该关系不构成正式主链路。

正式链路：

```text
Platform Server -> Node Server -> Edge Client
```

约束依据：

1. 平台不应绕过中心节点治理层
2. 平台不应直接面对边缘实现细节
3. 多节点审计和归属必须由 Node 收口

## 5.5 Platform Server 与 Virtual Browser

该关系不建立正式主链路。

Platform Server 不应：

1. 直接启动浏览器容器
2. 直接探测 CDP / VNC
3. 直接决定容器内运行细节

正式影响链路：

```text
Platform Server -> Node Server -> Edge Client -> Virtual Browser
```

## 5.6 Node Server 与 Virtual Browser

该关系不直接耦合。

Node Server 感知的 Virtual Browser 只能表现为：

- 由 Edge Client 汇报或执行后的浏览器运行结果摘要

约束：

- Node Server 的治理对象是节点，不是浏览器进程。

## 6. 四层数据关系

核心约束：

- 同一份数据不得在四层同时作为真相源。

## 6.1 Virtual Browser 的数据职责

Virtual Browser 只保存运行期事实。

### 正式数据对象

1. 容器运行时进程
2. 浏览器进程
3. 容器内临时运行环境
4. CDP 可连事实
5. VNC 可连事实
6. 当前 env 运行加载态

### 不承担的真相

1. 平台归属
2. 中心任务
3. 节点身份
4. 平台额度
5. 账号商业关系

约束：

- Virtual Browser 是运行单元，不是业务主资产主表。

## 6.2 Edge Client 的数据职责

Edge Client 只保存本机资产真相。

### 正式数据对象

1. 本机设备事实
2. 本机 Docker 事实
3. 本机 slot 真相
4. 本机 browser-env 资产
5. 本机容器运行摘要
6. 本机 profile / binding / proxy / fingerprint / browser-data

### 资产属性说明

例如：

1. `browser-data/profile`
2. `profile.json`
3. `binding.json`
4. 本机容器 ID
5. 本机 CDP/VNC 端口
6. 本机备份包路径

以上内容均属于 Edge 层本机资产真相。

## 6.3 Node Server 的数据职责

Node Server 只保存中心索引与治理摘要。

### 正式数据对象

1. `edge_clients`
2. `edge_client_bind_logs`
3. `server_tasks`
4. `client_run_quotas`
5. `server_browser_envs`

### 数据属性说明

Node Server 保存治理事实，不保存本机资产正文。

例如：

1. 节点是谁
2. 节点归谁
3. 节点是否 `healthy`
4. 节点是否 `verified`
5. 节点最近为什么被阻断
6. 某个 env 当前归属哪个 `clientId`
7. 平台最近给该 `clientId` 多少额度

### 不保存

1. Edge 的 profile 全文
2. Edge 的 binding 全文
3. Edge 的代理明文
4. Edge 的指纹 raw
5. Edge 的浏览器登录态
6. Edge 的目录全文清单
7. Virtual Browser 的容器内部运行正文

## 6.4 Platform Server 的数据职责

### 数据属性说明

- Platform Server 保存平台商业真相。

### 正式数据对象

1. 主账号
2. 商业设备归属
3. 运行额度真相
4. 平台级审计
5. 产品规则
6. 商业放行结论

### 不保存

1. Edge 本机 profile 资产
2. Edge 本机 slot 真相
3. Virtual Browser 容器运行细节
4. Edge Docker 细节

## 6.5 四层数据对照表

| 数据对象 | Virtual Browser | Edge Client | Node Server | Platform Server |
| --- | --- | --- | --- | --- |
| 浏览器进程运行事实 | 真相源 | 管理并摘要 | 不直接持有 | 不持有 |
| CDP/VNC 可用性 | 真相源 | 摘要与管理 | 摘要引用 | 不持有 |
| 本机设备事实 | 不持有 | 真相源 | 摘要缓存 | 不持有或仅极简摘要 |
| 本机 Docker 事实 | 局部运行事实 | 真相源 | 摘要缓存 | 不持有 |
| slot 状态 | 运行承载事实 | 真相源 | 不建主表，只保存引用摘要 | 不持有 |
| browser-env 目录与登录态 | 运行加载态 | 真相源 | 不保存正文，只保存聚合摘要 | 不持有 |
| 节点中心身份 `clientId` | 不生成 | 不主导 | 真相源 | 接收并引用 |
| 节点绑定关系 | 不主导 | 不主导 | 真相源 | 商业确认与引用 |
| 平台额度 | 不生成 | 不生成 | 快照缓存 | 真相源 |
| 平台级账号归属 | 不主导 | 不主导 | 中心执行层摘要 | 真相源 |
| 中心 task | 不保存 | 不保存 | 真相源 | 可引用摘要 |

## 7. 四层状态关系

状态必须按层拆分。

## 7.1 Virtual Browser 的状态

Virtual Browser 只负责运行态。

### 状态项

1. 容器是否存在
2. 容器是否 running
3. 浏览器是否成功启动
4. CDP 是否可用
5. VNC 是否可用
6. 当前 env 是否已真正加载

状态类别：

- 浏览器运行状态

## 7.2 Edge Client 的状态

Edge Client 只负责本机状态。

### 状态项

1. `/health` 本机健康
2. 本机 Docker 可达性
3. 本机容器状态
4. 本机 slot 状态
5. 本机 browser-env 生命周期状态

状态类别：

- 边缘本机状态

### slot 正式状态机

这里必须严格贴合当前 Client 的运行模型，不再使用 `occupied / releasing` 这类另一套命名。

slot 正式状态固定只有 4 个：

1. `waiting`
   - slot 已存在
   - 当前空闲
   - 可被 Node 分配给新的 browser-env
2. `loading`
   - Client 正在把指定 browser-env 加载到这个 slot
   - 该 slot 已被占住，不能再次分配
3. `running`
   - 该 slot 当前正在承载一个运行中的 browser-env
4. `ending`
   - 该 slot 上的 browser-env 正在结束
   - Client 正在释放运行态并恢复空白容器

正式状态流转固定为：

`waiting -> loading -> running -> ending -> waiting`

约束：

1. slot 状态不沿用节点状态命名，也不直接复用包的 `pending`。
2. 包主状态仍然看 `pending -> loading -> running -> ending -> pending`。
3. 资源调度看 slot；用户主视角看包。
4. `ending` 完成后，slot 必须回到空白 `waiting`。

### slot 数量异常边界

slot 数量问题不按常规治理分支设计，而按异常事件设计。

原因：

1. 平台先给 Node Server 配置某个 Client 应持有多少个 slot。
2. Node Server 只按这个目标值驱动 Client 创建对应数量的 slot。
3. 用户后续运行时，只能在这批已经创建出来的 slot 内分配使用。

因此只要创建链路正常：

1. 不应凭空多出 slot。
2. 不应正常少出 slot。
3. 不应出现越权新增 slot 的常规业务路径。

正式口径：

1. 正常链路下，不设计“slot 数量漂移”作为常态治理状态。
2. 如果 Node Server 发现目标 slot 数与 Client 实际 slot 数不一致，应直接视为 `slot 异常`。
3. `slot 异常` 发生后，必须阻断新的 `run`。
4. `slot 异常` 不要求继续自动放行业务动作，应提示“该服务器 slot 异常，请联系管理员处理”。
5. 这类问题优先按非法篡改、脏数据、执行异常或程序错误排查，而不是按正常资源波动处理。

## 7.3 Node Server 的状态

Node Server 负责中心治理状态。

### 节点主状态

1. `health_status`
   - `healthy`
   - `unhealthy`
   - `stale`
   - `offline`
2. `discovery_status`
   - `verified`
   - `blocked`
3. `discovery_reason`
   - `not_bound`
   - `ip_mismatch`
   - `identity_changed`
   - `probe_failed`
   - `stale`

状态属性：

- 中心治理判断

## 7.4 Platform Server 的状态

Platform Server 负责商业放行状态。

### 状态项

1. 节点是否归属某账号
2. 商业机位是否有效
3. 当前额度是否足够
4. 平台规则是否允许执行

状态属性：

- 商业状态

## 7.5 需要明确区分的状态关系

### 1. Client `/health` 与 Node `health_status`

- Client `/health` = 本机事实
- Node `health_status` = 中心判断后的节点可用性

### 2. Node `verified` 与 Platform 放行

- `verified` = 节点身份与能力验证通过
- 平台放行 = 商业规则也通过

### 3. Virtual Browser 容器 running 与平台业务可用

- 容器 running 只代表运行事实
- 平台业务可用还要看中心状态和商业规则

### 4. Virtual Browser 可连接 与 Edge Client 健康

- Virtual Browser 可连接 = 某个浏览器运行单元当前可连
- Edge Client 健康 = 本机边缘服务整体可被管理

### 5. Edge Client 健康 与 Node 节点可放行

- Edge Client 健康只是本机事实正常
- Node 节点可放行还要看中心 verified、绑定关系和治理冲突

## 8. 四层调用关系

## 8.1 正向调用链

正式主链：

```text
Platform Server
  -> Node Server
    -> Edge Client
      -> Virtual Browser
```

## 8.2 反向上报链

可接受的反向事实链：

```text
Edge Client
  -> discovery / heartbeat
    -> Node Server
      -> 节点摘要 / 审计 / 额度对接
        -> Platform Server
```

约束：

- Virtual Browser 的运行事实由 Edge Client 吸收后再向上汇总，不直接越层上报 Platform。

## 8.3 禁止调用链

明确禁止：

1. Platform Server 直接调 Edge Client 做正式业务动作
2. Platform Server 直接读 Edge 数据
3. Platform Server 直接调 Virtual Browser
4. Node Server 直接读 Edge 文件
5. Node Server 直接管理 Virtual Browser 细节
6. Client 直接主导平台额度
7. Client 直接主导商业 `clientId`

## 9. 四层责任边界

## 9.1 Virtual Browser 的责任边界

负责：

1. 实际浏览器运行
2. 提供 CDP/VNC 运行结果
3. 提供容器运行事实

不负责：

1. 账号归属
2. 节点归属
3. 平台额度
4. 中心审计

## 9.2 Edge Client 的责任边界

负责：

1. 本机事实真实
2. 本机执行受控
3. 本机健康可探测
4. 本机 API 稳定可调
5. 管理 Virtual Browser 生命周期

不负责：

1. 多节点治理
2. 商业归属
3. 商业额度
4. 平台审计主事实

## 9.3 Node Server 的责任边界

负责：

1. 节点中心身份
2. 节点归属治理
3. 节点发现与验证
4. 节点健康收口
5. 业务准入前置判断
6. 中心任务与审计

不负责：

1. 本机资产正文
2. Virtual Browser 运行细节真相
3. 平台额度最终规则
4. 平台账号体系最终真相

## 9.4 Platform Server 的责任边界

负责：

1. 商业主账号
2. 商业设备归属
3. 平台额度真相
4. 平台级审计与产品规则

不负责：

1. 边缘本机执行细节
2. Virtual Browser 运行细节
3. Edge 本机资产操作

## 10. Node Server P0 在四层中的位置

`Node Server P0` 在四层中属于中间治理枢纽层。

核心作用：

1. 将下游 Edge 的本机事实和 Virtual Browser 运行摘要转换为中心可治理的节点事实
2. 将上游 Platform 的商业规则转换为节点可执行的准入约束

分层属性：

1. 转译层
2. 治理层
3. 缓冲层
4. 收口层

## 11. Node Server P0 的核心职责

`P0` 只做节点层，不展开 env 生命周期。

核心职责：

1. 自动发现
2. 手动接入
3. probe
4. bind
5. unbind
6. heartbeat / health 收口
7. 中心节点审计
8. 平台额度快照容器

## 12. Node Server P0 的应用流程

## 12.1 自动发现流程

```text
Edge Client 发 UDP beacon
  -> Node Server 收 beacon
  -> 校验平台字段
  -> probe /health
  -> probe /device-info
  -> 进入 discovered 内存视图
  -> 若命中已登记节点则回写摘要
  -> Node 后续发起 bind
  -> bind 成功后再把 clientId/accountId 写回 Client 本地 JSON
```

收口：

- UDP beacon 是自动发现唯一正式发现链路
- heartbeat 不参与发现，不是主链路，也不是辅链路
- 即使 heartbeat 正常，也不能替代 UDP 自动发现成立这一事实
- 发现先于绑定，绑定先于本地 JSON 留痕；不要把 Node 地址当成 Client 启动前置条件

## 12.2 bind 流程

```text
管理员/上游发起 bind
  -> Node Server probe 目标 Edge
  -> 查重
  -> 校验是否已被其它账号占用
  -> 生成 clientId
  -> 写 edge_clients
  -> 写 bind 日志
  -> 兼容 push clientId 到 Edge
  -> Edge 写入本地 node-registration.json
  -> 返回 bind 结果
```

## 12.3 unbind 流程

```text
管理员/上游发起 unbind
  -> Node Server 删除当前有效绑定结果
  -> Node Server 删除当前 node-slot 关系
  -> 写 unbind 日志
  -> 调 Edge 清空本地 node-registration.json
  -> 失败留痕但不回滚解绑
```

## 12.3.1 rebind 后的 slot 重初始化规则

重新 binding 后，slot 不恢复旧关系，而走一套完整的空白重初始化流程。

正式流程：

```text
旧绑定已解绑
  -> 管理员/上游重新发起 bind
  -> Node Server 建立新的 clientId 关系
  -> Node Server 驱动 Edge 清空当前全部 slot
  -> Node Server 按当前目标 slot 数重新初始化空白 slot
  -> slot 编号从 slot001 开始连续生成
  -> 初始化完成后自动触发 slot 对账任务
  -> Node Server 重建 edge_client_slots 与 edge_clients slot 摘要
```

约束：

1. rebind 后不恢复旧 slot 关系。
2. rebind 后不读取旧 slot 配置。
3. rebind 后不恢复旧 env 运行关系。
4. rebind 后不自动 run。
5. 重新初始化的 slot 只代表新的空白运行资源池。

## 12.4 heartbeat 流程

```text
Edge Client 发 heartbeat
  -> Node Server 更新心跳摘要
  -> 更新已知节点活性摘要
  -> 推进节点 health_status
```

收口：

- heartbeat 不参与发现
- 它如果保留，只回答“这台已知 Client 最近是否仍在主动上报”
- 它不替代 UDP beacon，也不单独证明自动发现已经成立
- 它不能创建新的 discovered 事实，只能回写已知节点的活性摘要

## 12.5 后续 run 准入流程中的位置

正式 run 准入链：

```text
Platform Server 决定是否允许
  -> Node Server 判断节点是否 healthy + verified
  -> Node Server 判断是否已有 clientId 与可用额度
  -> Node Server 调用 Edge Client run
    -> Edge Client 将 env 加载到 Virtual Browser
```

结论：

- `P0` 虽不展开 run 生命周期，但必须先完成 run 准入前置链路。

## 13. Node Server P0 的数据模型设计

## 13.1 `edge_clients`

定位：

- Node Server 的正式节点真相表

表达：

1. 节点身份
2. 节点归属
3. 节点治理状态
4. 最近发现/心跳/探测结果

不表达：

1. slot 运行真相
2. browser-env 目录真相
3. 平台额度真相
4. Virtual Browser 运行正文

## 13.2 `edge_client_bind_logs`

定位：

- 归属治理审计表

表达：

1. bind 行为
2. unbind 行为
3. 当时目标地址
4. push / clear 本地登记结果

## 13.3 `server_tasks`

定位：

- Node Server 中心任务真相表

`P0` 至少表达：

1. bind
2. unbind
3. recheck
4. slot_reconcile

## 13.4 `client_run_quotas`

定位：

- Platform -> Node 的额度快照容器

约束：

- `client_run_quotas` 只作为缓存，不作为最终额度事实。

## 13.5 `recheck` 节点治理动作边界

定位：

- 管理员手动触发的节点重探测动作
- 中文业务语义名统一叫“会话校验”

负责：

1. 重新调用 Client `/health`
2. 重新调用 Client `/api/v1/edge/device-info`
3. 重新校验 `clientIp / baseUrl / hostname / os / arch / docker` 摘要
4. 刷新 `health_status / discovery_status / discovery_reason / last_error / last_checked_at`

不负责：

1. 不重新 bind
2. 不生成新的 `clientId`
3. 不自动确认 IP 漂移
4. 不自动覆盖 `identity_changed / ip_mismatch`
5. 不直接放行业务 run

## 13.6 `slot_reconcile` 节点 slot 对账动作边界

定位：

- Node Server 的中心 slot 收口任务
- 正式按 task + SSE 设计
- 用于重建 node-slot 关系缓存和节点 slot 摘要

负责：

1. 读取 `edge_clients` 当前 slot 摘要
2. 调用 Client 正式 slot 查询接口
3. 全量刷新 `edge_client_slots`
4. 重算 `edge_clients` 上的：
   - `actual_slot_count`
   - `available_slot_count`
   - `running_slot_count`
   - `slot_exception_status`
   - `slot_exception_reason`
   - `last_slot_checked_at`
5. 记录 `edge_client_slot_logs`
6. 输出 task + SSE 进度与最终结果

不负责：

1. 不直接创建 Client 本机 slot
2. 不直接删除 Client 本机 slot
3. 不直接 reinit Client 本机 slot
4. 不修改平台目标 slot 数
5. 不自动 run
6. 不把完整 SSE 历史事件流写入 SQLite

约束：

1. `slot_reconcile` 的成功只表示对账任务动作完成，不等于 slot 资源层一定正常。
2. 如果发现目标 slot 数与 Client 实际 slot 数不一致，应直接收口为 `slot 异常`。
3. `slot_reconcile` 的最终 slot 状态口径固定沿用 `waiting / loading / running / ending`。
4. 这里的 `running` 只表示“已有 browser-env 配置包挂载并正在该 slot 上运行”的包运行态，不表示底层 slot 基础容器仅仅处于存活状态；Client 的 `occupied` 在中心统一归一化成 `running`。
5. rebind 后 slot 不恢复旧关系，而应通过重新初始化后的 `slot_reconcile` 重建。

## 14. Node Server P0 的状态设计

## 14.1 `health_status`

只允许：

- `healthy`
- `unhealthy`
- `stale`
- `offline`

## 14.2 `discovery_status`

只允许：

- `verified`
- `blocked`

## 14.3 `discovery_reason`

统一承接阻断原因，例如：

- `not_bound`
- `probe_failed`
- `ip_mismatch`
- `identity_changed`
- `stale`

## 14.4 设计约束

1. 主状态少而稳
2. 细原因落 `reason`
3. `identity_changed`、`ip_mismatch`、`discovered` 不进入正式主状态枚举

## 15. Virtual Browser 与 Edge Client 的数据交界面

该交界面需要单独定义。

## 15.1 Virtual Browser -> Edge 可上收的数据

1. 容器存在性
2. 容器运行状态
3. CDP 可连结果
4. VNC 可连结果
5. 浏览器启动成败
6. 运行时错误摘要

## 15.2 Virtual Browser -> Edge 不上收为长期资产的数据

1. 临时进程细节全文
2. 无业务意义的临时日志原文
3. 平台归属语义

## 15.3 交界面约束依据

1. Node 不应理解容器内部细节
2. Edge 封装层不能失效
3. 运行策略必须由 Edge 吸收后再上收

## 16. Edge Client 与 Node Server 的数据交界面

该交界面需要单独定义。

## 16.1 Edge -> Node 可上收的数据

1. `/health` 摘要
2. `/device-info` 摘要
3. beacon 摘要
4. heartbeat 摘要
5. env 状态摘要
6. Virtual Browser 运行结果摘要

## 16.2 Edge -> Node 不允许上收的数据

1. profile 正文
2. proxy 明文
3. fingerprint raw
4. browser-data 登录态正文
5. 本机文件系统扫描结果全文
6. Virtual Browser 容器内部正文

## 16.3 交界面约束依据

1. Node 不是边缘资产仓库
2. Node 不是浏览器运行时仓库
3. 安全边界、真相源和恢复链路必须保持清晰

## 17. Node Server 与 Platform Server 的数据交界面

## 17.1 Node -> Platform 上报内容

1. `clientId`
2. `mainAccountId`
3. 节点健康摘要
4. 节点发现状态摘要
5. 节点最近错误摘要
6. 平台需要的最小运行状态摘要

## 17.2 Platform -> Node 下发内容

1. 节点商业归属确认
2. 运行额度
3. 放行 / 阻断结果
4. 上层业务动作

## 17.3 交界面约束依据

1. Platform 不直接触达 Edge 或 Virtual Browser
2. 节点治理层不能被绕开
3. 商业规则与运行事实不能直接耦合

## 18. 四层视角下的真相源表

| 主题 | 真相源层 | 其它层角色 |
| --- | --- | --- |
| 浏览器进程运行事实 | Virtual Browser | Edge 管理并摘要，Node/Platform 不直接持有 |
| CDP/VNC 可用性 | Virtual Browser | Edge 摘要，Node 引用，Platform 不直接持有 |
| 本机设备与 Docker 事实 | Edge Client | Node 保存摘要，Platform 通常不持有 |
| 节点中心身份 `clientId` | Node Server | Edge 不生成，Platform 引用 |
| 节点绑定关系 | Node Server / Platform确认 | Edge 与 Virtual Browser 都不主导 |
| 平台额度 | Platform Server | Node 快照缓存，Edge 与 Virtual Browser 不解释 |
| slot 真相 | Edge Client | Node 只引用，Platform 不持有 |
| browser-env 资产正文 | Edge Client | Virtual Browser 只运行加载，Node 聚合摘要，Platform 不持有 |
| 中心任务事实 | Node Server | Edge 只执行，Virtual Browser 只运行，Platform 可引用摘要 |
| 平台商业任务或产品规则 | Platform Server | Node 执行约束，Edge 与 Virtual Browser 不主导 |

## 19. 四层接口归属表

## 19.1 Virtual Browser 层接口归属

说明：

- Virtual Browser 不对外暴露平台级正式业务 API。
- 它的能力通过 Edge Client 封装后再向上提供。

| 能力 | 实际归属层 | 对上暴露方式 |
| --- | --- | --- |
| 浏览器进程启动 | Virtual Browser | 由 Edge Client run 封装 |
| 浏览器进程停止 | Virtual Browser | 由 Edge Client stop 封装 |
| CDP 可连 | Virtual Browser | 由 Edge Client 提供摘要或测试结果 |
| VNC / WebVNC 可连 | Virtual Browser | 由 Edge Client 提供地址与可用性摘要 |
| 容器运行事实 | Virtual Browser | 由 Edge Client 汇总后上报 |

## 19.2 Edge Client 层接口归属

| 接口或能力 | 归属层 | 说明 |
| --- | --- | --- |
| `/health` | Edge Client | 本机健康事实 |
| `/api/v1/edge/device-info` | Edge Client | 本机设备与 Docker 摘要 |
| discovery / heartbeat | Edge Client | 向 Node 上报发现与活性事实 |
| `/api/v1/edge/docker/*` | Edge Client | 本机 Docker 管理能力 |
| `/api/v1/edge/slots/*` | Edge Client | 本机 slot 管理真相 |
| `/api/v1/edge/browser-envs/*` | Edge Client | 本机 browser-env 生命周期主入口 |
| `/web-vnc.html` 等运行查看入口 | Edge Client | 对 Virtual Browser 的查看封装 |

## 19.3 Node Server 层接口归属

| 接口或能力 | 归属层 | 说明 |
| --- | --- | --- |
| `/api/v1/edge-clients/discovered` | Node Server | 发现态视图 |
| `/api/v1/edge-clients/bind` | Node Server | 节点正式绑定 |
| `/api/v1/edge-clients/{clientId}/unbind` | Node Server | 节点正式解绑 |
| `/api/v1/edge-clients/{clientId}/recheck` | Node Server | 节点治理重探测 |
| `/api/v1/edge-clients` | Node Server | 中心节点列表 |
| `/api/v1/edge-clients/{clientId}` | Node Server | 中心节点详情 |
| `/api/v1/server-tasks/{taskId}` | Node Server | 中心任务事实查询 |
| `/api/v1/edge-clients/{clientId}/run-quota` | Node Server | 平台额度快照查询 + 当前 run admission 判断 |

## 19.4 Platform Server 层接口归属

| 接口或能力 | 归属层 | 说明 |
| --- | --- | --- |
| 主账号管理接口 | Platform Server | 平台账号真相 |
| 商业设备归属接口 | Platform Server | 归属规则真相 |
| 额度下发接口 | Platform Server | 运行额度真相 |
| 平台放行接口 | Platform Server | 上层产品规则与商业准入 |
| 平台级审计接口 | Platform Server | 商业与产品审计 |

## 19.5 接口归属约束

1. Platform Server 不直接暴露 Edge 级接口。
2. Node Server 不直接暴露 Virtual Browser 原始运行接口。
3. Edge Client 不直接暴露平台商业接口。
4. Virtual Browser 不直接对上暴露正式业务 API。

## 20. 四层数据库归属表

## 20.1 Virtual Browser 层数据落点

说明：

- Virtual Browser 主要承载运行时数据，不承担长期中心数据库角色。

| 数据类别 | 归属层 | 存储形态 |
| --- | --- | --- |
| 浏览器进程运行态 | Virtual Browser | 容器运行时 / 进程内存态 |
| CDP/VNC 即时可用性 | Virtual Browser | 运行时事实 |
| 容器临时运行环境 | Virtual Browser | 容器内临时文件和环境变量 |

## 20.2 Edge Client 层数据落点

| 数据类别 | 归属层 | 存储形态 |
| --- | --- | --- |
| 设备信息摘要 | Edge Client | 本机配置 / API 计算结果 |
| Docker 事实 | Edge Client | Docker API 实时事实 |
| slot 真相 | Edge Client | 本机 SQLite / 本机索引 |
| browser-env 资产 | Edge Client | 本机目录 + 本机 SQLite |
| profile / binding / proxy / fingerprint | Edge Client | 本机文件 |
| browser-data/profile | Edge Client | 本机文件 |

## 20.3 Node Server 层数据落点

| 数据类别 | 归属层 | 表或存储 |
| --- | --- | --- |
| 节点中心身份 | Node Server | `edge_clients` |
| 节点 bind/unbind 审计 | Node Server | `edge_client_bind_logs` |
| 中心任务事实 | Node Server | `server_tasks` |
| 平台额度快照 | Node Server | `client_run_quotas` |
| env 聚合摘要 | Node Server | `server_browser_envs` |
| discovered 视图 | Node Server | 内存态 |

## 20.4 Platform Server 层数据落点

| 数据类别 | 归属层 | 表或存储 |
| --- | --- | --- |
| 主账号真相 | Platform Server | 平台账号表 |
| 商业设备归属真相 | Platform Server | 平台设备归属表 |
| 平台额度真相 | Platform Server | 平台额度表 |
| 平台级审计 | Platform Server | 平台审计表 |
| 产品规则与放行结论 | Platform Server | 平台规则表 / 缓存 |

## 20.5 数据库归属约束

1. Edge Client 的本机资产不迁移到 Node Server 作为正文存储。
2. Node Server 的节点治理表不下沉到 Edge Client。
3. Platform Server 的商业真相不由 Node Server 重建。
4. Virtual Browser 运行态不作为长期中心数据库事实保存。

## 21. 关键时序

## 21.1 发现与登记时序

```text
Virtual Browser          Edge Client             Node Server             Platform Server
     |                        |                       |                         |
     |                        |-- UDP beacon ------->|                         |
     |                        |                       |-- probe /health ------>|
     |                        |<-- health request ----|                         |
     |                        |-- health response --->|                         |
     |                        |<-- device-info req ---|                         |
     |                        |-- device-info resp -->|                         |
     |                        |                       |-- discovered view       |
```

## 21.2 bind 时序

```text
Platform Server          Node Server              Edge Client             Virtual Browser
     |                        |                        |                         |
     |-- bind request ------->|                        |                         |
     |                        |-- probe /health ----->|                         |
     |                        |-- probe device-info -->|                         |
     |                        |-- allocate clientId    |                         |
     |                        |-- write edge_clients   |                         |
     |                        |-- push clientId ----->|                         |
     |                        |<-- push result --------|                         |
     |<-- bind response ------|                        |                         |
```

## 21.3 后续 run 准入时序

```text
Platform Server          Node Server              Edge Client             Virtual Browser
     |                        |                        |                         |
     |-- allow run ---------->|                        |                         |
     |                        |-- check healthy        |                         |
     |                        |-- check verified       |                         |
     |                        |-- check clientId       |                         |
     |                        |-- check quota snapshot |                         |
     |                        |-- run env ----------->|                         |
     |                        |                        |-- load env ----------->|
     |                        |                        |-- start browser ------>|
```

## 21.4 stop / ending / backup 收口时序

```text
Platform/Operator        Node Server              Edge Client             Virtual Browser
     |                        |                        |                         |
     |-- stop/backup -------->|                        |                         |
     |                        |-- call edge action -->|                         |
     |                        |                        |-- stop browser ------->|
     |                        |                        |-- release runtime ---->|
     |                        |                        |-- restore waiting ---->|
     |                        |<-- final summary ------|                         |
```

约束：

1. `ending`、`stop`、`backup` 成功后，slot 必须回到空白 waiting 容器。
2. Virtual Browser 的旧运行态不得遗留到下一次包加载。
3. 这里的 slot 状态口径固定沿用 `waiting / loading / running / ending`，不得再扩成另一套运行资源命名。

## 22. P0 技术实现落地建议

## 22.1 Node Server 代码域建议

建议按以下域拆分：

1. `Discovery`
2. `Node`
3. `Bind`
4. `EdgeClient`
5. `Task`
6. `Platform`

## 22.2 各域职责

### `Discovery`

- UDP listener
- beacon 校验
- discovered 内存视图
- 过期清理

### `Node`

- 节点详情
- 节点列表
- 心跳摘要
- 节点治理状态更新

### `Bind`

- bind
- unbind
- bind log
- push / clear 本地登记

### `EdgeClient`

- 对 Edge 的 HTTP 调用
- `/health`
- `/device-info`
- 错误映射

### `Task`

- 节点层中心任务事实

### `Platform`

- 平台额度快照
- 后续平台侧协同接口

## 23. 逐层验收口径

## 23.1 Virtual Browser 层验收

1. 浏览器成功启动
2. CDP 可连
3. VNC / WebVNC 可连
4. stop / ending 后可释放运行态
5. backup / ending 成功后可回到空白 waiting 容器

## 23.2 Edge Client 层验收

1. discovery 可用
2. `/health` 可用
3. `/device-info` 可用
4. 边缘受控 API 可用
5. Virtual Browser 管理链路可用

## 23.3 Node Server 层验收

1. 节点发现可用
2. bind 可用
3. unbind 可用
4. 节点治理可用
5. 状态收口可用
6. 中心审计可用

## 23.4 Platform Server 层验收

1. 商业归属可用
2. 额度真相可用
3. 平台放行规则可用

## 24. 结论

系统层级结论：

1. `Virtual Browser` 负责真实浏览器运行
2. `Edge Client` 负责本机边缘管理与浏览器运行封装
3. `Node Server` 负责中心节点治理与边缘编排
4. `Platform Server` 负责商业规则与平台最终真相

中心节点层结论：

- `Node Server P0` 的核心任务是建立稳定的中间治理层，承接节点身份、节点归属、节点健康、节点治理、节点审计和后续业务准入前提。
