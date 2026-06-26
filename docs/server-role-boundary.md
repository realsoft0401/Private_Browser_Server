# Private_Browser_Server 职责边界

## 1. 文档目的

这份文档只回答一个核心问题：

```text
Private_Browser_Server 在整套系统里到底负责什么，不负责什么
```

后续讨论：

- Client down 机 / 掉线
- heartbeat
- 节点绑定 / 解绑
- clientId
- 平台额度
- run / stop / backup / restore 放行

都应先回到这份职责边界来判断，避免把本该属于 Client、Node、Platform 的事情混在一起。

## 2. 三层角色定位

当前系统三层角色固定为：

```text
Platform Server
  -> 商业规则、额度、上层账号体系

Private_Browser_Server
  -> 中心节点治理、节点身份、节点健康、任务调度入口

Private_Browser_Client
  -> 单机边缘执行器、本机 Docker、本机浏览器运行能力
```

## 3. Server 的核心定位

`Private_Browser_Server` 不是浏览器运行器，也不是登录态存储器。

它的正式定位是：

```text
中心控制层 / 中心治理层 / 中心事实源
```

也就是说：

- Client 负责“本机能不能跑”
- Server 负责“中心是否允许你用这台 Client”

## 4. Server 负责什么

## 4.1 负责发现 Client

Server 负责：

- 监听 UDP beacon
- 判断这是不是当前平台的合法 Client discovery 报文
- 根据 beacon 发现局域网中的 Client
- 再通过 HTTP probe 验证 Client 是否真的在线

当前正式口径：

- UDP beacon 是唯一正式 discovery 链路
- heartbeat 不参与 discovery

## 4.2 负责维护中心节点身份

Server 负责：

- 生成中心 `clientId`
- 维护 `main_account_id`
- 维护 Client 与账号的绑定关系
- 维护解绑后的身份延续

也就是说：

- `clientId` 属于 Server
- 不是 Client 自己生成
- 不是 Platform 直接写给 Client

## 4.3 负责节点健康状态判断

Server 负责判断：

- `healthy`
- `stale`
- `offline`
- `unhealthy`

这些状态是中心状态，不是 Client 自己声明的状态。

例如：

- Client 掉线后，Client 自己无法说“我 offline 了”
- 只能由 Server 根据 heartbeat 和 probe 超时推导

## 4.4 负责管理心跳事实

Server 负责：

- 接收 Client heartbeat
- 根据 heartbeat 匹配已知节点
- 把 heartbeat 结果写回中心节点表

至少包括：

- `last_heartbeat_at`
- `last_heartbeat_reported_at`
- `last_heartbeat_source`
- `client_ip`
- `base_url`

## 4.5 负责中心放行

Server 负责判断：

- 某个 Client 当前能不能被使用
- 某个 run 动作是否允许发给某个 Client
- 某个节点是否满足健康条件
- 某个节点是否已经有正式 `clientId`
- 某个节点是否有平台额度

所以：

- `run` 不是 Client 想跑就跑
- 必须经过 Server 中心放行

## 4.6 负责统一调度入口

前端、平台、管理端不应直接调用 Client 执行业务动作。

统一口径应是：

```text
前端 / 管理端 / Platform
  -> Server
    -> Client
```

也就是说：

- Server 是统一业务入口
- Client 是被调度对象

## 4.7 负责中心审计与记录

Server 负责记录：

- bind / unbind
- push clientId
- heartbeat 收到事实
- 节点健康变化
- 后续任务结果

中心审计要围绕：

- `clientId`
- `main_account_id`
- `client_ip`
- `base_url`
- `action/result/message`

## 5. Server 不负责什么

## 5.1 不负责运行浏览器

Server 不直接：

- 启动浏览器容器
- 管理本机 Chrome/Chromium
- 管理 CDP / VNC 端口

这些都属于 Client。

## 5.2 不负责保存登录态

Server 不保存：

- Cookies
- Local Storage
- IndexedDB
- Session Storage
- Login Data
- browser-data/profile

这些都属于 Client 本地资产。

## 5.3 不负责直接读 Client 本地文件

Server 不允许直接：

- 读 Client SQLite
- 读环境包目录
- 读 `browser-data/profile`
- 读本地代理明文
- 读指纹 raw

Server 只能通过 Client HTTP API 获取受控结果。

## 5.4 不负责代替 Platform 管商业规则

Server 不生成平台商业额度。

Server 只负责：

- 接收 Platform 下发的额度事实
- 在 run 前做准入判断

Platform 才负责：

- 给这个 Client 分几个可运行槽位
- 商业授权是否成立

## 6. 关于 Client down 机 / 掉线

这类问题属于 Server 负责判断，不属于 Client 自己判断。

原因：

- Client 掉线后，本机已经无法上报“我掉线了”
- 只有 Server 能通过 heartbeat 是否中断来收口

当前这题已经正式定案：

- Node 侧节点在线状态只保留 `healthy / offline` 两种
- 不再保留 `stale`
- 也不再保留其它中间态

正式规则是一刀切：

```text
有 heartbeat = healthy
没 heartbeat = offline
```

后续实现时只需要再补一个离线超时阈值：

- 超过阈值仍未收到 heartbeat
- 就直接进入 `offline`

当前默认实现配置：

- `monitor_interval_seconds=15`
- `offline_after_seconds=90`

一旦 `offline`：

- 禁止 run
- 禁止 stop
- 禁止 backup
- 禁止 restore
- 禁止 delete
- 禁止 import-package

## 7. 关于 node-registration.json

Client 本地 `node-registration.json` 不是中心真相。

它只是：

- Node bind 成功后的本地留痕
- Client 后续 heartbeat 的目标地址来源

真正的中心事实仍然在 Server：

- `clientId`
- `main_account_id`
- 节点健康状态
- heartbeat 事实
- 审计记录

## 8. 一句话总结

可以把 `Private_Browser_Server` 理解成：

```text
它不负责跑浏览器，
它负责决定“哪台 Client 是谁、是否在线、是否可用、是否允许被调度”。
```
