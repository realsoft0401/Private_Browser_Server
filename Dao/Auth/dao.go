package Auth

// ModelHandler 保留项目既有 Dao 业务动作入口风格。
//
// Dao 负责把 Service 参数整理为 Repository 操作，不直接做密码哈希或 JWT 签发。
type ModelHandler struct{}
