# Server Browser Env Query Regression

这份文档用于回归三条中心 browser-env 查询 / 刷新接口：

- `GET /api/v1/browser-envs`
- `GET /api/v1/browser-envs/{envId}`
- `POST /api/v1/browser-envs/{envId}/refresh`

## 1. 基础变量

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export ACCOUNT_ID="906090119"
export CLIENT_ID="9060901190003"
export ENV_ID="906090001_tk_324867594169356288"
```

## 2. 列表失败路径：缺少 accountId

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs" | jq
```

预期：

- `code=1002`
- `message=accountId 不能为空`

## 3. 列表查询

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs?accountId=$ACCOUNT_ID" | jq
```

预期：

- `code=1000`
- `data.items` 为数组
- `data.total` 与数组长度一致

如果当前中心库还没有任何 env，允许：

- `total=0`
- `items=[]`

## 4. 详情失败路径：env 不存在

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs/not_exist_env" | jq
```

预期：

- `code=1004`
- `message=server browser env not found`

## 5. 刷新失败路径：env 不存在

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/not_exist_env/refresh" | jq
```

预期：

- `code=1004`
- `message=server browser env not found`

## 6. Happy path：详情与 refresh

只有当前中心库里已经存在目标 `ENV_ID` 时，再执行下面两步。

### 6.1 先看中心旧缓存

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs/$ENV_ID" | jq
```

### 6.2 再执行 refresh

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/refresh" | jq
```

预期：

- `code=1000`
- 返回新的 `status/runtimeStatus/containerStatus`
- `lastSyncedAt` 更新

### 6.3 再次读取 detail

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs/$ENV_ID" | jq
```

预期：

- `detail.lastSyncedAt` 与 refresh 后一致
- 中心 detail 能看到 refresh 后的新摘要

## 7. 节点未就绪时的 refresh 阻断

如果目标节点当前不是 `healthy + verified`，预期：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/refresh" | jq
```

返回：

- `code=1005`
- `message=edge client is not healthy and verified`
