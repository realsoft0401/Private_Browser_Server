# Node Server 接口设计：`GET /api/v1/auth/me`

## 1. 当前状态

- 尚未实现
- 当前返回 `CodeNotImplemented`

## 2. 规划定位

- 未来用于返回 Node Server 自身登录态摘要
- 当前不承担平台业务身份事实

## 3. 业务边界

- 不替代 Platform Header
- 不直接承接企业客户最终身份体系

## 4. 当前响应语义

```http
GET /api/v1/auth/me
```

当前固定返回统一包装，业务 `code=1099`，表示 JWT 解析和中心用户身份体系尚未接入。

## 5. 后续约束

- 后续只返回中心用户摘要，不暴露密码哈希等敏感字段
- 不能把 `auth/me` 误当成企业客户的最终业务身份接口
