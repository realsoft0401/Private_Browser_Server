package Rom

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"private_browser_server/Settings"
)

var db *sql.DB

// Init 是 Node Server 本地 SQLite 的唯一初始化入口。
//
// 设计来源：用户确认平台管理端用 MySQL，而 Node Server 只承担节点控制面，
// 因此本服务使用 SQLite 保存节点索引、环境聚合缓存和中心任务事实，降低 RK3528 等小设备部署成本。
// 这里负责创建连接、开启基础 pragma、执行最小迁移；Service、Dao、Repository 都不能绕过这里自行打开数据库。
func Init() error {
	if db != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(Settings.Conf.SQLiteConfig.Path), 0755); err != nil {
		return fmt.Errorf("创建 SQLite 数据目录失败: %w", err)
	}

	conn, err := sql.Open("sqlite", Settings.Conf.SQLiteConfig.Path)
	if err != nil {
		return fmt.Errorf("打开 SQLite 数据库失败: %w", err)
	}
	conn.SetMaxOpenConns(Settings.Conf.SQLiteConfig.MaxOpenConns)
	conn.SetMaxIdleConns(Settings.Conf.SQLiteConfig.MaxIdleConns)

	if _, err := conn.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`); err != nil {
		_ = conn.Close()
		return fmt.Errorf("初始化 SQLite pragma 失败: %w", err)
	}
	if err := migrate(conn); err != nil {
		_ = conn.Close()
		return err
	}
	db = conn
	return nil
}

// DB 返回已初始化的 SQLite 连接池。
//
// Repository 层通过这个函数访问数据库，业务层不要缓存连接或拼接 DSN；
// 这样后续如果迁移连接参数、加事务封装或替换驱动，只需要收敛在 Rom/Repository。
func DB() *sql.DB {
	return db
}

// Close 统一释放 SQLite 连接。
func Close() {
	if db != nil {
		_ = db.Close()
		db = nil
	}
}

// IsInitialized 供健康检查确认基础设施是否完成初始化。
func IsInitialized() bool {
	return db != nil
}

// migrate 创建 Node Server V1 的最小中心表。
//
// 这些表只保存控制面索引和状态摘要：节点连接事实、环境包聚合缓存、Server 任务事实。
// 真实 profile、代理明文、fingerprint raw、browser-data 登录态仍在 Edge Client 环境包内，
// Node Server 不通过 SQLite 复制这些敏感资产，避免形成第二套账号环境事实源。
func migrate(conn *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS edge_clients (
			id TEXT PRIMARY KEY,
			main_account_id TEXT NOT NULL DEFAULT '',
			client_sequence INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL DEFAULT '',
			base_url TEXT NOT NULL,
			client_ip TEXT NOT NULL DEFAULT '',
			docker_api_url TEXT NOT NULL DEFAULT '',
			os TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT 'unknown',
			cpu_cores INTEGER NOT NULL DEFAULT 0,
			memory_total_mb INTEGER NOT NULL DEFAULT 0,
			docker_version TEXT NOT NULL DEFAULT '',
			health_status TEXT NOT NULL DEFAULT 'stale',
			discovery_status TEXT NOT NULL DEFAULT 'manual',
			last_heartbeat_at INTEGER NOT NULL DEFAULT 0,
			last_heartbeat_reported_at INTEGER NOT NULL DEFAULT 0,
			last_heartbeat_source TEXT NOT NULL DEFAULT '',
			last_checked_at INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			created_by_user_id TEXT NOT NULL DEFAULT '',
			created_by_username TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			deleted_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_edge_clients_base_url_active
			ON edge_clients(base_url) WHERE deleted_at = 0;`,
		`CREATE INDEX IF NOT EXISTS idx_edge_clients_main_account
			ON edge_clients(main_account_id, deleted_at);`,
		`CREATE INDEX IF NOT EXISTS idx_edge_clients_health
			ON edge_clients(health_status, discovery_status, deleted_at);`,

		`CREATE TABLE IF NOT EXISTS server_browser_envs (
			env_id TEXT PRIMARY KEY,
			main_account_id TEXT NOT NULL,
			client_id TEXT NOT NULL,
			rpa_type TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'created',
			container_status TEXT NOT NULL DEFAULT '',
			monitor_status TEXT NOT NULL DEFAULT '',
			cdp_url TEXT NOT NULL DEFAULT '',
			web_vnc_url TEXT NOT NULL DEFAULT '',
			last_task_id TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			created_by_user_id TEXT NOT NULL DEFAULT '',
			created_by_username TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			deleted_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_server_browser_envs_account
			ON server_browser_envs(main_account_id, deleted_at);`,
		`CREATE INDEX IF NOT EXISTS idx_server_browser_envs_client
			ON server_browser_envs(client_id, status, deleted_at);`,
		`CREATE TABLE IF NOT EXISTS server_tasks (
			id TEXT PRIMARY KEY,
			main_account_id TEXT NOT NULL,
			operator_user_id TEXT NOT NULL DEFAULT '',
			operator_username TEXT NOT NULL DEFAULT '',
			client_id TEXT NOT NULL DEFAULT '',
			env_id TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			edge_task_id TEXT NOT NULL DEFAULT '',
			events_url TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			finished_at INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE INDEX IF NOT EXISTS idx_server_tasks_account
			ON server_tasks(main_account_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_server_tasks_env
			ON server_tasks(env_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_server_tasks_status
			ON server_tasks(status, updated_at);`,
	}
	for _, statement := range statements {
		if _, err := conn.Exec(statement); err != nil {
			return fmt.Errorf("执行 SQLite 迁移失败: %w", err)
		}
	}
	return nil
}
