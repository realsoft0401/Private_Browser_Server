# Node / Client 绑定流正式定案

## 1. 文档定位

本文件是当前第一阶段 `Node Server <-> Private_Browser_Client` 绑定流的正式定案。

> 文档适用范围说明：
> 这份文档只约束当前正式绑定流：发现 -> 绑定 -> 写回 Client 本地留痕。
> 最新总口径下，中心 `clientId` 身份真相、节点归属和业务放行判断始终属于 Server。

后续如果：

- 重建新的 `Private_Browser_Server`
- 调整新的 `Private_Browser_Client`
- 编写 OpenAPI
- 落数据库
- 做前后端联调

都以这份文档为准。

如果其它草稿、时序图、问题清单与本文件冲突，以本文件为准。

## 2. 第一阶段主线

当前正式主线固定为：

```text
Node 找到 Client
  -> Node 发起绑定
  -> Node 生成 clientId
  -> Node 下发 clientId 给 Client
  -> Client 写本地 node-registration.json
```

这里的关键原则是：

- `clientId` 由 Node Server 生成
- Client 不生成 `clientId`
- Client 在当前阶段只负责接收并本地留痕
- Node 先发现 Client，再 bind，再写回 Client 本地留痕；不能反过来写

## 3. 正式定案

### 3.1 bind 输入口径

第一阶段 `bind` 正式输入固定为：

```text
accountId + clientIp
```

当前不使用：

```text
discoveryId
```

原因：

- 你已经明确不希望第一阶段把 discovered 做成正式持久化实体
- 先走最直接的绑定链路

## 3.2 discovered 不是正式表实体

第一阶段 `discovered` 只表示：

- Node 当前发现到了这台 Client
- 当前可用于绑定前确认

但它不是一张必须正式落库的中心表。

正式中心表只保留已绑定节点，例如：

```text
edge_clients
```

也就是说：

- discovered 是过程
- bound 后的节点才是正式对象

## 3.3 discovered 阶段 `clientId` 为空是正确结果

在发现阶段，如果接口返回：

```text
clientId = ""
```

这是正确结果，不是错误。

它的语义是：

- 当前这台 Client 还没有完成绑定
- Node 还没有为它生成中心身份

## 3.4 bind 才是 `clientId` 生成点

`clientId` 只能在 Node Server 执行 bind 时生成。

不允许退回成这些错误口径：

- Client 启动时自己生成
- discovered 时自动生成
- 本地 JSON 反向决定中心 `clientId`

## 3.5 bind 成功后自动立即 push

第一阶段正式规则：

- bind 成功后，Node 自动立即触发一次 `push-client-id`
- 同时保留单独补推接口

这样收口的意义是：

- 正常链路顺
- 异常时能单独补推

## 3.6 push 的具体内容

这里的 `push` 不是泛指状态同步。

这里的 `push` 固定指 Node 向 Client 下发：

```text
clientId
accountId
source
assignedAt
```

目的只有一个：

- 让 Client 写入本地 `data/node-registration.json`

## 3.7 Client 本地只写 JSON，不写 SQLite

Client 对 `clientId` 的本地留存方式固定为：

```text
data/node-registration.json
```

不写入 Client 本地 SQLite。

## 3.8 Client 本地 JSON 不是中心真相源

`data/node-registration.json` 只是：

- 本地留痕
- 重启后缓存
- 联调排障依据

这也意味着：

- 它不是后续正式业务接口的必需输入
- 它不是节点已验证的最终依据
- 未来如果绑定流完全收口到 Server 单边中心真相，这层本地缓存可以继续降级甚至移除

它不是：

- 中心绑定真相
- 最终业务放行依据

当前有效身份始终以 Node 为准。

## 3.9 Client assign 接口沿用 old 的校验方式

第一阶段 `POST /api/v1/edge/node-registration/assign` 的校验方式暂定为：

```text
X-Edge-API-Key
```

也就是：

- Node 调 Client `assign` 时带 `X-Edge-API-Key`
- Client 按 old 的 Edge API Key 方式校验
- 不另外发明新的 assign 专用 token

这条规则同时和现在的 [client.go](/Users/lining/Documents/Browser_virtualization/Private_Browser_Server/EdgeClient/client.go) 一致。

这里要特别注明：

- 它只用于 Node -> Client 的受控写回
- 它不代表 Client 自注册

## 3.10 解绑后清空 Client 本地 JSON

第一阶段正式规则：

- 解绑后，Client 本地 `node-registration.json` 要清空

原因：

- 解绑后不应继续保留一个看起来仍有效的本地中心身份
- 前后端认知更干净

## 3.11 重绑后 `clientId` 不变

同一台物理 Client：

- 解绑
- 再绑定到新账号

后，继续沿用原 `clientId`。

不重新生成新的 `clientId`。

原因：

- 历史任务稳定
- 审计稳定
- 避免同一物理 Client 产生多个中心身份

## 3.12 Client 允许覆盖旧 JSON

如果 Node 重新下发新的绑定结果：

- Client 允许覆盖旧 JSON

但必须记覆盖日志，至少包括：

```text
oldClientId
newClientId
oldAccountId
newAccountId
source
updatedAt
```

## 3.13 绑定后必须主动 heartbeat

bind 成功且 Client 本地 `node-registration.json` 写入成功后，Client 必须主动向 Node Server 发送 heartbeat。

这条 heartbeat 的作用只包括：

- 证明这台已绑定 Client 仍在线
- 让 Node Server 持续刷新 `edge_clients` 的活性摘要

Node 收到 heartbeat 后，至少要更新：

- `last_heartbeat_at`
- `last_heartbeat_reported_at`
- `last_heartbeat_source`
- `client_ip`
- `base_url`
- `health_status=healthy`

heartbeat 不参与 discovery，也不重新生成 `clientId`。

如果后续超过配置的 `offline_after_seconds` 仍未继续收到 heartbeat，Node Server 必须把这台 Client 直接收口为 `offline`，不再经过 `stale`。

## 3.14 push 成功的第一阶段判断口径

第一阶段 push 成功判断先定为：

- Client `assign` 接口返回成功

当前不额外要求：

- push 后再回读一次 Client 本地缓存接口

## 3.15 bind 成功但 push 失败，不回滚 bind

如果出现：

- bind 成功
- push 失败

正式收口是：

- 中心绑定关系保留
- `pushStatus=failed`
- 后续允许补推

不能因为一次 push 失败就把中心 bind 回滚掉。

## 3.16 discovered 视图不展示本地缓存 clientId

第一阶段 discovered 视图只看中心发现事实。

不展示：

- Client 本地缓存里的旧 `clientId`

避免误导前端把“历史本地缓存”看成“当前已绑定中心身份”。

## 3.17 clientId 格式正式锁定

第一阶段 `clientId` 格式正式锁定为：

```text
mainAccountId + 4位设备序号
```

示例：

```text
9060901190001
```

## 3.18 Node 第一阶段直接接收 accountId

第一阶段 bind 接口直接接收：

```text
accountId
```

当前先不强制要求 PlatformServer 统一转发后才能调用。

后续如果要收回到 PlatformServer，也是在这套正式规则之上做上游封装，不影响 Node / Client 当前主线。

## 4. Node 侧正式职责

Node Server 第一阶段负责：

- 找到当前在线 Client
- 判断是否已被绑定
- 根据 `accountId + clientIp` 发起绑定
- 生成 `clientId`
- 写入中心正式节点记录
- 自动 push `clientId` 给 Client
- 记录 push 成功或失败

## 5. Client 侧正式职责

Client 第一阶段负责：

- 提供本机发现事实
- 接收 Node 下发的 `clientId`
- 校验 `X-Edge-API-Key`
- 把结果写入 `data/node-registration.json`
- 提供本地缓存查询接口

Client 不负责：

- 生成 `clientId`
- 决定账号绑定关系
- 用本地文件反向覆盖 Node 真相

## 6. 第一阶段最小接口口径

Node 侧建议至少有：

- `GET /api/v1/edge-clients/discovered`
- `POST /api/v1/edge-clients/bind`
- `POST /api/v1/edge-clients/{clientId}/push-client-id`
- `GET /api/v1/edge-clients`
- `GET /api/v1/edge-clients/{clientId}`

Client 侧建议至少有：

- `GET /api/v1/edge/node-registration`
- `POST /api/v1/edge/node-registration/assign`

## 7. 第一阶段成功判定

这条链路第一阶段完成的验收标准是：

1. Node 能通过 `clientIp` 找到当前在线 Client
2. bind 成功后生成正式 `clientId`
3. Node 自动调用 Client `assign`
4. Client 本地生成 `data/node-registration.json`
5. 使用错误或缺失 `X-Edge-API-Key` 时会被拒绝
6. bind 成功但 push 失败时，中心节点记录仍存在

## 8. 不能退回的原则

后续开发时，不能退回下面这些旧混乱口径：

1. 不能让 Client 自己生成 `clientId`
2. 不能把本地 JSON 当中心真相源
3. 不能在 discovered 阶段强行给 `clientId`
4. 不能因为 push 失败就回滚 bind
5. 不能为 assign 重新发明一套和 old 完全无关的鉴权头
6. 不能让解绑后继续保留一个看起来仍有效的本地 JSON

## 9. 结论

当前第一阶段绑定流正式收口为一句话：

> Node 使用 `accountId + clientIp` 找到并绑定 Client，生成 `clientId` 后自动通过 `X-Edge-API-Key` 保护的 assign 接口下发给 Client；Client 只把结果写入本地 `node-registration.json`，解绑时清空，本地 JSON 不作为中心真相源。
