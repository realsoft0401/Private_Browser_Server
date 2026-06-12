# Node Server 接口设计：`POST /api/v1/edge-clients/probe-docker`

## 1. 功能目标

`POST /api/v1/edge-clients/probe-docker` 用于对一个 Docker Engine HTTP API `2375` 地址做只读探测，并返回节点能力摘要。

它适合用在：

- 注册前手工探测
- 联调 Docker 2375
- 管理员确认目标机器架构

## 2. 设计来源

- 用户要求节点设备能力的第一来源是 Docker `2375`，不是自定义远端接口。
- 用户要求架构必须统一归一化为 `amd64/arm64/unknown`，避免业务代码散落解析 `x86_64/aarch64`。

## 3. 业务边界

### 3.1 负责什么

- 请求 `/_ping`
- 请求 `/info`
- 请求 `/version`
- 返回 OS、架构、CPU、内存、Docker 版本、容器数、镜像数

### 3.2 不负责什么

- 不创建节点
- 不更新中心库
- 不验证 Client `/health`
- 不判断节点是否可直接业务放行

## 4. 请求与响应

```http
POST /api/v1/edge-clients/probe-docker
```

请求体：

```json
{
  "dockerApiUrl": "http://192.168.10.119:2375"
}
```

返回重点：

- `dockerApiUrl`
- `os`
- `arch`
- `rawArch`
- `cpuCores`
- `memoryTotalMB`
- `dockerVersion`
- `dockerApiVersion`
- `containers`
- `images`

## 5. 成功判定

- 三个 Docker 接口都可访问
- 架构可正常归一化

## 6. 失败判定

- URL 非法
- `2375` 不可达
- Docker HTTP 返回异常
- JSON 解析失败

## 7. 错误与日志规范

错误必须带修复建议，尤其说明：

- 当前 Node Server 无法确认 Docker 能力
- 需要检查 Docker daemon `2375`、防火墙、内网可达性
