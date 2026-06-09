package Rom

import "private_browser_server/Settings"

var initialized bool

// Init 是中心数据库初始化入口。
//
// 当前只建立 MySQL 配置契约和初始化边界，尚未接入 GORM/MySQL driver。
// 这样做是为了先搭好 Server 架构，下一步实现 Repository 时再统一引入数据库依赖；
// 后续不要让 Service 或 Routes 直接创建数据库连接。
func Init() error {
	_ = Settings.Conf.MySQLConfig
	initialized = true
	return nil
}

// Close 是中心数据库关闭入口。
//
// 当前阶段没有真实连接需要释放；后续接入 MySQL 后在这里统一关闭连接池。
func Close() {
	initialized = false
}

// IsInitialized 供健康检查确认基础设施是否完成初始化。
func IsInitialized() bool {
	return initialized
}
