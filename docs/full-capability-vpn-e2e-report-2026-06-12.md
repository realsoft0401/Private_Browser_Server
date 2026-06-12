# 全能力真实 VPN 回归测试记录

## 1. 测试目标

验证下面这条完整链路在真实 Linux 边缘节点和真实 Clash 代理下可用：

```text
create
-> first run
-> VNC / TikTok / Clash 实测
-> stop
-> backup
-> delete package
-> import-package
-> second run
-> second VNC
```

测试日期：

- `2026-06-12`

## 2. 测试环境

- Node Server
  - `http://127.0.0.1:3400`
- Edge Client
  - `http://192.168.10.119:3300`
- Docker API
  - `tcp://192.168.10.119:2375`
- `clientId`
  - `9060900010001`
- `mainAccountId`
  - `906090001`
- 真实代理配置
  - [ClashVerge.yaml](/Users/lining/Documents/analysis_ins/proxy/ClashVerge.yaml)
- `proxy.type`
  - `clash-verge`
- `imagePolicy`
  - `crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_edge:1.1-amd64`

## 3. 前置检查结果

- Node `/health`
  - `ok=true`
- Edge `/health`
  - `ok=true`
- Edge 版本
  - `0.1.9`
- Edge Docker 检查
  - 正常
- `NET_ADMIN`
  - 已挂载
- `/dev/net/tun`
  - 已挂载
- 中心节点状态
  - `health_status=healthy`
  - `discovery_status=verified`
  - `arch=amd64`

## 4. 测试对象

- `envId`
  - `906090001_tk_323785648626470912`
- `name`
  - `E2E-Real-VPN-Regression-20260612-01`
- 首次创建返回端口
  - `cdp=8109`
  - `vnc=9109`

## 5. 执行结果

### 5.1 Create

- 结果
  - 成功
- 状态
  - `created`
- 关键返回
  - `cdpUrl=http://192.168.10.119:8109`
  - `webVncUrl=http://192.168.10.119:3300/web-vnc.html?envId=906090001_tk_323785648626470912`

### 5.2 First Run

- 中心 task
  - `task_1781263758724504000`
- Edge task
  - `task_1781263759226016689_16746`
- 结果
  - 成功
- 最终状态
  - 中心 `running`
  - Edge `running`

### 5.3 First VNC / VPN

- `vnc-info`
  - 成功
- `vncUrl`
  - `vnc://192.168.10.119:9109`
- `wsUrl`
  - `ws://192.168.10.119:3300/api/v1/edge/browser-envs/906090001_tk_323785648626470912/vnc/ws`
- `webVncUrl`
  - `http://192.168.10.119:3300/web-vnc.html?envId=906090001_tk_323785648626470912`
- 手工验证
  - VNC 正常打开
  - TikTok 正常打开
  - Clash 代理已连接，出口链路正常

### 5.4 Stop

- 中心 task
  - `task_1781263965412709000`
- Edge task
  - `task_1781263965873824167_24330`
- 结果
  - 成功
- 最终状态
  - Edge `stopped`
  - `containerStatus=exited`

### 5.5 Backup

- 中心 task
  - `task_1781264034081213000`
- 结果
  - 成功
- 最终状态
  - 中心 `backed_up`
  - Edge `backed_up`
- `backupPath`
  - `data/browser-envs/users/906090001/tk/906090001_tk_323785648626470912-backup.tar.gz`
- `backupChecksum`
  - `sha256:2db30e8e1c5393ee58141cd077b2deec10340900d9e0828966de2575eabe13df`

### 5.6 Delete Package

- 中心 task
  - `task_1781264107582750000`
- Edge task
  - `task_1781264108043462600_62761`
- 结果
  - 成功
- 最终状态
  - 中心 `deleted`
  - Edge `deleted`

### 5.7 Import Package

- 中心 task
  - `task_1781264148416709000`
- 结果
  - 成功
- 最终状态
  - 中心从 `deleted` 恢复到 `created`
  - Edge 从不存在恢复到 `created`
- 关键事实
  - `import-package` 的 deleted tombstone 复活逻辑生效

### 5.8 Second Run

- 中心 task
  - `task_1781264208027964000`
- Edge task
  - `task_1781264208532039689_39758`
- 结果
  - 成功
- 最终状态
  - 中心 `running`
  - Edge `running`

### 5.9 Second VNC

- `vnc-info`
  - 成功
- `vncUrl`
  - `vnc://192.168.10.119:9109`
- `wsUrl`
  - `ws://192.168.10.119:3300/api/v1/edge/browser-envs/906090001_tk_323785648626470912/vnc/ws`
- `webVncUrl`
  - `http://192.168.10.119:3300/web-vnc.html?envId=906090001_tk_323785648626470912`
- 时序说明
  - 第二次 `run` 刚发起时，task 处于 `running`，Edge 仍短暂是 `created`
  - 该短窗口里 `vnc-info` 返回 `环境包未运行，不能返回 VNC 连接信息`
  - 数秒后 task 收口为 `success`，Edge 进入 `running`，`vnc-info` 恢复正常

## 6. 本轮确认通过的点

- 真实 Clash 配置可通过中心 create 成功下发
- 真实 TikTok 页面可通过 `119 + VNC` 正常打开
- `vncUrl/wsUrl/webVncUrl` 地址生成正确，不再错误返回 `127.0.0.1`
- backup 后 `container_status=missing` 属于正常事实
- `delete package` 后中心可稳定收口到 `deleted`
- `import-package` 后同 `clientId` 的 deleted tombstone 能正确复活到 `created`
- import 后第二次 `run` 和第二次 VNC 都正常

## 7. 本轮发现并确认的规则

- `proxy.type` 当前正式值必须使用 `clash-verge`
- 传 `clash` 会收到：
  - `调用 Edge 创建环境包失败: proxy.type 第一版仅支持 clash-verge`
- `delete package` 当前最终收口仍依赖读取 task detail 或 SSE
- `run` 启动窗口内 `vnc-info` 暂时返回未运行，属于正常时序，不是回归失败

## 8. 结论

本轮 `2026-06-12` 全能力真实 VPN 回归测试通过。

通过口径：

- create 通过
- first run 通过
- first VNC 通过
- 真实 TikTok + Clash 代理通过
- stop 通过
- backup 通过
- delete package 通过
- import-package 通过
- second run 通过
- second VNC 通过
