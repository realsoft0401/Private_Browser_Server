package Auth

// Repository 是 users 表的底层访问入口。
//
// 当前文件只占位分层边界；如果后续 Node Server 需要本地运维账号，
// 查无记录、RowsAffected 等 SQLite 数据库细节应在这里归一化。
type Repository struct{}
