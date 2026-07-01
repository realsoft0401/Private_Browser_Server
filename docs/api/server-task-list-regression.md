# GET /api/v1/server-tasks 回归测试

## 测试目标

验证中心任务列表接口能用于“没有 taskId 时回看历史任务”，并确认它只是普通 HTTP 查询，不会触发 Edge、不会创建新任务、不会返回 SSE。

## 前置环境

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export CLIENT_ID="9060901190003"
export ENV_ID="318275706305908736_tk_319725200528642048"
```

## 1. 查询最近任务

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks?page=1&pageSize=20" | jq
```

通过标准：

- `code=1000`
- `data.items` 是数组
- `data.total` 是数字
- `data.page=1`
- `data.pageSize=20`
- 返回内容是 JSON，不是 `text/event-stream`

## 2. 按 Client 过滤

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks?clientId=$CLIENT_ID&page=1&pageSize=20" | jq
```

通过标准：

- `code=1000`
- 如果有任务，所有任务都属于同一个 `clientId` 对应的动作链
- 不要求 Client 当前在线，因为历史任务只读中心 SQLite

## 3. 按 Env 过滤

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks?envId=$ENV_ID&page=1&pageSize=20" | jq
```

通过标准：

- `code=1000`
- 返回当前 env 相关任务
- 如果没有历史任务，`items=[]` 且 `total=0` 也算通过

## 4. 按失败任务过滤

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks?status=failed&page=1&pageSize=20" | jq
```

通过标准：

- `code=1000`
- 如果有任务，`status=failed`
- 错误原因应在 `error/suggestion` 中体现

## 5. pageSize 上限校验

```bash
curl -s "$SERVER_BASE/api/v1/server-tasks?page=1&pageSize=500" | jq '.data.pageSize'
```

通过标准：

- 返回 `100`
- 说明服务端已把普通列表入口限制在安全分页范围内

## 结论

这条接口通过后，管理员可以不依赖手里已有的 `taskId`，直接从中心任务表回看 run、backup、restore、import-package、revalidate、slot-reconcile 等任务历史。
