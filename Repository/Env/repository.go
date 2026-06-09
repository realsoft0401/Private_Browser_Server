package Env

// Repository 是 server_browser_envs 表的底层访问入口。
//
// 中心侧只保存环境包索引和状态摘要，不能把 Edge 的真实 profile 或登录态实体写入这里。
type Repository struct{}
