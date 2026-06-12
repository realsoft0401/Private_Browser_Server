# Node Server 接口设计：`POST /api/v1/auth/register`

## 1. 当前状态

- 尚未实现
- 当前返回 `CodeNotImplemented`

## 2. 规划定位

- 预留给 Node Server 自身未来的账号/运维保护入口
- 不属于当前企业客户主业务身份体系

## 3. 业务边界

- 当前不创建用户
- 不写密码
- 不签发 token
- 不替代 Platform Header

## 4. 当前响应语义

```http
POST /api/v1/auth/register
```

当前固定返回统一包装，业务 `code=1099`，表示接口已规划但尚未开放。

## 5. 后续约束

- 即使未来落地，也只能服务 Node Server 自身管理保护
- 不能把当前项目重新做成最终客户账号密码数据库
- 必须只保存密码哈希，不能保存明文密码
