package Task

// Repository 是 tasks 表的底层访问入口。
//
// 任务状态更新要在这里收敛数据库细节，Service 只表达任务语义。
type Repository struct{}
