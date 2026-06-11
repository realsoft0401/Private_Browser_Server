# API 文档工具选型

## 设计背景

Private_Browser_Server 是状态机驱动型服务。Swagger/OpenAPI 适合表达接口路径、参数和响应结构，但不能精准表达：

- 接口调用前必须满足的状态。
- 调用后状态如何流转。
- 哪些接口必须按顺序调用。
- 失败后应该调用哪个修复接口。
- 哪些动作不能绕过 PlatformServer 或 verified 校验。
- 哪些资产动作失败后不能自动重试。

因此后续 API 文档分为两类：

- OpenAPI 类工具负责接口契约和在线调试。
- Markdown/产品文档负责流程、状态机和商业规则。

## 工具选型

```text
开发测试阶段:
  Swagger UI + Scalar

内部/客户演示阶段:
  Scalar

正式商业文档:
  Mintlify

嵌入自己后台:
  Scalar API Reference
```

## 落地原则

- `/openapi.yaml` 继续作为接口契约事实来源。
- Swagger UI 保留给开发阶段快速测试。
- Scalar 用于更现代的 API Reference 和客户演示。
- Mintlify 用于后续正式商业文档站。
- 后台管理系统如果需要内嵌 API 文档，优先嵌入 Scalar API Reference。
- 状态机、调用顺序、verified 放行、环境包生命周期、失败恢复路径不要只写在 Swagger 里，必须同步写入流程文档和状态文档。

## 后续文档规划

建议后续新增：

```text
docs/flow.md
docs/state.md
```

其中：

- `flow.md` 记录 discovered -> register -> refresh -> verify -> create env -> run -> stop 等调用顺序。
- `state.md` 记录 heartbeatStatus、healthStatus、discoveryStatus、env status、task status 的含义、允许动作和禁止动作。
