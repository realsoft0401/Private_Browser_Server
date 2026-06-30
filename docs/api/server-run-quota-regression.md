# Server Run Quota Regression

这份文档用于回归两条 Node Server run quota 接口：

- `GET /api/v1/edge-clients/{clientId}/run-quota`
- `POST /api/v1/edge-clients/{clientId}/run-quota/refresh`

## 1. 测试目标

确认下面 4 件事：

1. Node 能返回当前中心缓存的 quota 快照
2. Node 能返回当前中心 run admission 判断
3. 手工刷新 quota 后，准入结果会立即更新
4. slot 异常时，即使 quota 正常，也必须被 admission 阻断

## 2. 测试前置

```bash
export SERVER_BASE="http://127.0.0.1:3400"
export ACCOUNT_ID="906090119"
export CLIENT_ID="9060901190003"
```

建议先确认节点存在：

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients?accountId=$ACCOUNT_ID" | jq
```

## 3. 先看当前 quota 状态

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/run-quota" | jq
```

重点看：

- `data.status`
- `data.quotaLimit`
- `data.quotaAvailableSnapshot`
- `data.admission.allowed`
- `data.admission.reasons`

如果当前没有配过额度，预期通常是：

- `status=untrusted`
- `admission.allowed=false`
- `reasons` 里包含 `missing_run_quota`

## 4. 刷新一份可用 quota

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/run-quota/refresh" \
  -H "Content-Type: application/json" \
  -d '{
    "quotaLimit": 1,
    "quotaUsedSnapshot": 0,
    "quotaAvailableSnapshot": 1,
    "expiresAt": 1893456000,
    "status": "valid",
    "lastError": "",
    "source": "manual-regression-quota-valid"
  }' | jq
```

预期：

- `data.status=valid`
- `data.quotaAvailableSnapshot=1`
- 如果节点当前同时满足 healthy + verified + slot 正常 + 有可用 slot：
  - `data.admission.allowed=true`
  - `data.admission.status=allowed`

## 5. 再次读取 quota

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/run-quota" | jq
```

预期结果应和上一步返回一致。

## 6. 回归 quota 用尽阻断

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/run-quota/refresh" \
  -H "Content-Type: application/json" \
  -d '{
    "quotaLimit": 1,
    "quotaUsedSnapshot": 1,
    "quotaAvailableSnapshot": 0,
    "expiresAt": 1893456000,
    "status": "valid",
    "lastError": "",
    "source": "manual-regression-quota-exhausted"
  }' | jq
```

预期：

- `data.admission.allowed=false`
- `data.admission.reasons` 包含 `quota_exhausted`
- `data.admission.suggestion=refresh platform run quota first`

## 7. 回归 slot 异常优先阻断

先把目标 slot 数改成一个和实际不一致的值：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/target-slot-count" \
  -H "Content-Type: application/json" \
  -d '{
    "targetSlotCount": 2,
    "source": "manual-regression-slot-exception"
  }' | jq
```

然后重新写回一份可用 quota：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/run-quota/refresh" \
  -H "Content-Type: application/json" \
  -d '{
    "quotaLimit": 1,
    "quotaUsedSnapshot": 0,
    "quotaAvailableSnapshot": 1,
    "expiresAt": 1893456000,
    "status": "valid",
    "lastError": "",
    "source": "manual-regression-slot-exception-with-valid-quota"
  }' | jq
```

预期：

- `data.admission.allowed=false`
- `data.admission.reasons` 至少包含 `slot_exception`
- 说明 quota 正常并不能覆盖 slot 异常

## 8. 恢复正常状态

如果当前实际 slot 数是 `1`，恢复命令：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/target-slot-count" \
  -H "Content-Type: application/json" \
  -d '{
    "targetSlotCount": 1,
    "source": "manual-regression-restore-target-slot-count"
  }' | jq
```

再写回一份可用 quota：

```bash
curl -s -X POST "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/run-quota/refresh" \
  -H "Content-Type: application/json" \
  -d '{
    "quotaLimit": 1,
    "quotaUsedSnapshot": 0,
    "quotaAvailableSnapshot": 1,
    "expiresAt": 1893456000,
    "status": "valid",
    "lastError": "",
    "source": "manual-regression-restore-quota"
  }' | jq
```

最后确认：

```bash
curl -s "$SERVER_BASE/api/v1/edge-clients/$CLIENT_ID/run-quota" | jq
```

理想结果：

- `admission.allowed=true`
- `slotExceptionStatus=normal`
- `quotaAvailableSnapshot=1`

## 9. 结果解释

如果这里没有出现 `allowed=true`，优先检查：

1. `healthStatus` 是否还是 `healthy`
2. `discoveryStatus` 是否还是 `verified`
3. `availableSlotCount` 是否大于 `0`
4. `slotExceptionStatus` 是否已经恢复为 `normal`
5. `quota` 是否还是 `valid` 且未过期
