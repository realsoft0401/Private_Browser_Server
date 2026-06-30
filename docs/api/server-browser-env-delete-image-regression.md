# Server Browser Env Delete Image Regression

这份文档用于回归：

- `DELETE /api/v1/browser-envs/{envId}/del`

## 1. 测试目标

确认下面 4 件事：

1. 中心 `/del` 是同步 HTTP，不是 task + SSE
2. 中心会把 `/del` 委派到目标 Edge 正式 `/del`
3. 成功后中心 env 主状态不变
4. 失败路径能返回正式业务错误，而不是 404

## 2. 基础变量

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export ENV_ID="906090001_tk_330198837593378816"
```

## 3. 失败路径：中心 env 不存在

```bash
curl -i -s -X DELETE \
  "$SERVER_BASE/api/v1/browser-envs/not_exist_env/del"
```

预期：

- `code=1004`
- `message=server browser env not found`

## 4. Happy path：中心 `/del`

前提：

- 目标 env 不是 `running`
- Edge 侧该 env 的 `runtime.image` 存在

```bash
curl -s -X DELETE \
  "$SERVER_BASE/api/v1/browser-envs/$ENV_ID/del" | jq
```

预期：

- `code=1000`
- `data.envId=$ENV_ID`
- `data.image` 非空
- `data.imageRemoved` 为 `true` 或 `false`
- `data.warningMessage` 可为空

## 5. 结果解释

### 如果 `imageRemoved=true`

- 表示本次 Docker 确实删除了镜像标签或镜像层

### 如果 `imageRemoved=false` 且 `warningMessage=image already missing`

- 表示本机已经没有这张镜像
- 当前业务仍按成功收口

## 6. 核对中心 detail

```bash
curl -s "$SERVER_BASE/api/v1/browser-envs/$ENV_ID" | jq
```

成功后预期：

- `status` 不变
- `runtimeStatus` 不变
- `lastError=''`

## 7. 重要说明

这条回归不能放在 package delete 之后。

原因：

- package delete 成功后，中心和 Edge 都已经没有这条 env
- 那时只能再测 not-found 路径，不能测 `/del` happy path

## 8. 本轮已完成回归结论

已确认下面结果：

- 中心 `/del` happy path 可真实调用成功
- 返回同步 JSON，不会生成 `server_task`
- Edge 镜像删除结果会原样返回中心调用方
- 中心 env 主状态保持不变，不会因为 `/del` 被当成 package delete
