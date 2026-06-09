package Auth

// Repository 是 users 表的底层访问入口。
//
// 当前文件只占位分层边界；后续接入 MySQL 后，查无记录、RowsAffected 等数据库细节应在这里归一化。
type Repository struct{}
