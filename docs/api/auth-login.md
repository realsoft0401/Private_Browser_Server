# Node Server 接口设计：`POST /api/v1/auth/login`

## 1. 当前状态

- 尚未实现
- 当前返回 `CodeNotImplemented`

## 2. 规划定位

- 预留 Node Server 自身登录入口
- 当前 V1 不作为业务主线

## 3. 业务边界

- 当前不校验密码
- 不签发 JWT
- 不承接最终客户登录、套餐、计费和订单体系

## 4. 当前响应语义

```http
POST /api/v1/auth/login
```

当前固定返回统一包装，业务 `code=1099`，表示路由已挂载但登录能力尚未开放。

## 5. 后续约束

- JWT 只能由 Node Server 或更上层中心服务签发
- 不能把用户认证逻辑下沉到 Edge Client
- 即使后续实现，也只服务中心控制面的管理保护
