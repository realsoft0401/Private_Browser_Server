# Scalar API Reference 方案

## 1. 当前结论

`Private_Browser_Server` 的 Scalar 页面收口为 Node Server 主服务内置能力。

它和 `/swagger` 一样，直接由 3400 服务提供，不维护单独 Scalar 展示服务，也不增加独立 Dockerfile。

## 2. 访问入口

正式入口只有一个：

```text
http://127.0.0.1:3400/scalar
```

其中：

- `/scalar` 是唯一正式路径。
- 页面读取同源 `/openapi.yaml`。
- 页面和真实 Server API 在同一个 Node Server 服务内。
- 不再出现文档端口和 API 端口分裂。

## 3. 事实源

当前只保留一份协议事实源：

- `docs/openapi.yaml`

当前只保留一份 Scalar 展示页面：

- `public/scalar.html`

维护原则：

- 不新增第二份 OpenAPI。
- 不新增独立 Scalar 展示服务。
- 不维护单独 Scalar 构建链路。
- Node Server 正式部署必须携带 `docs` 和 `public`，确保 `/scalar`、`/swagger`、`/openapi.yaml` 可访问。

## 4. 与 Swagger 的分工

- `/swagger`
  - 继续作为研发调试页和快速联调入口。
- `/scalar`
  - 作为更正式的 API Reference 展示入口。
- `docs/api/*.md`
  - 继续承担业务语义、状态机、SSE、失败收口和排障说明。

## 5. 验收标准

- `GET /swagger` 返回 200。
- `GET /scalar` 返回 200。
- `GET /openapi.yaml` 返回 200。
- 页面里的协议地址仍然是 `/openapi.yaml`。
- 仓库内不新增独立 Scalar 展示服务构建文件。
