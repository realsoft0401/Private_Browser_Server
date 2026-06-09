package Node

// Repository 是 control_nodes 表的底层访问入口。
//
// 它只处理数据库访问，不做 Docker 探测、API Key 生成或中文业务提示。
type Repository struct{}
