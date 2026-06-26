package SQLite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"private_browser_server/Settings"
)

var db *sql.DB

// Init 初始化新的第一阶段 SQLite。
//
// 设计来源：
// - 新 Server 第一阶段只保存正式已绑定节点和 bind/push 留痕；
// - discovered 当前只是过程，不落正式表；
// - 因此这里刻意只建最小表，避免重起第一天就把 old 全量中心表再搬回来。
func Init() error {
	dataDir := filepath.Dir(Settings.Conf.SQLiteConfig.Path)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir failed: %w", err)
	}
	conn, err := sql.Open("sqlite3", Settings.Conf.SQLiteConfig.Path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("open sqlite failed: %w", err)
	}
	conn.SetMaxOpenConns(Settings.Conf.SQLiteConfig.MaxOpenConns)
	conn.SetMaxIdleConns(Settings.Conf.SQLiteConfig.MaxIdleConns)
	if err = conn.Ping(); err != nil {
		_ = conn.Close()
		return fmt.Errorf("ping sqlite failed: %w", err)
	}
	db = conn
	if err = migrate(); err != nil {
		_ = conn.Close()
		db = nil
		return err
	}
	return nil
}

func DB() *sql.DB {
	return db
}

func Close() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}

func migrate() error {
	// 这里故意不用“只建新表”的黑盒方式。
	//
	// 设计来源：
	// - 新 Server 在第一阶段已经落过一版极简 SQLite；
	// - 现在进入正式中心层后，表字段明显增多，但我们不能要求每次都删库重来；
	// - 因此 migrate 既负责首建，也负责把旧表平滑补齐到当前正式字段。
	if err := createTables(); err != nil {
		return err
	}
	if err := ensureColumns(); err != nil {
		return err
	}
	if err := normalizeNodeHealthStatuses(); err != nil {
		return err
	}
	if err := createIndexes(); err != nil {
		return err
	}
	return nil
}

func createTables() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS edge_clients (
			client_id TEXT PRIMARY KEY,
			main_account_id TEXT NOT NULL DEFAULT '',
			client_sequence INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL DEFAULT '',
			client_ip TEXT NOT NULL DEFAULT '',
			base_url TEXT NOT NULL DEFAULT '',
			docker_api_url TEXT NOT NULL DEFAULT '',
			os TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT '',
			cpu_cores INTEGER NOT NULL DEFAULT 0,
			memory_total_mb INTEGER NOT NULL DEFAULT 0,
			docker_version TEXT NOT NULL DEFAULT '',
			health_status TEXT NOT NULL DEFAULT 'offline',
			discovery_status TEXT NOT NULL DEFAULT 'blocked',
			discovery_reason TEXT NOT NULL DEFAULT 'not_bound',
			push_status TEXT NOT NULL DEFAULT 'pending',
			api_key_hash TEXT NOT NULL DEFAULT '',
			last_discovered_at INTEGER NOT NULL DEFAULT 0,
			last_heartbeat_at INTEGER NOT NULL DEFAULT 0,
			last_heartbeat_reported_at INTEGER NOT NULL DEFAULT 0,
			last_heartbeat_source TEXT NOT NULL DEFAULT '',
			last_checked_at INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			created_by_user_id TEXT NOT NULL DEFAULT '',
			created_by_username TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0,
			deleted_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS edge_client_bind_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			client_id TEXT NOT NULL,
			main_account_id TEXT NOT NULL DEFAULT '',
			client_ip TEXT NOT NULL DEFAULT '',
			operator_user_id TEXT NOT NULL DEFAULT '',
			operator_username TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL DEFAULT '',
			result TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS server_browser_envs (
			env_id TEXT PRIMARY KEY,
			main_account_id TEXT NOT NULL DEFAULT '',
			client_id TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL DEFAULT '',
			rpa_type TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			container_status TEXT NOT NULL DEFAULT '',
			runtime_status TEXT NOT NULL DEFAULT '',
			current_slot_id TEXT NOT NULL DEFAULT '',
			cdp_url TEXT NOT NULL DEFAULT '',
			web_vnc_url TEXT NOT NULL DEFAULT '',
			last_task_id TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			last_synced_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0,
			deleted_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS server_tasks (
			id TEXT PRIMARY KEY,
			main_account_id TEXT NOT NULL DEFAULT '',
			operator_user_id TEXT NOT NULL DEFAULT '',
			operator_username TEXT NOT NULL DEFAULT '',
			client_id TEXT NOT NULL DEFAULT '',
			env_id TEXT NOT NULL DEFAULT '',
			task_type TEXT NOT NULL DEFAULT '',
			resource_type TEXT NOT NULL DEFAULT '',
			resource_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			edge_task_id TEXT NOT NULL DEFAULT '',
			events_url TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			suggestion TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0,
			finished_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS client_run_quotas (
			client_id TEXT PRIMARY KEY,
			quota_limit INTEGER NOT NULL DEFAULT 0,
			quota_used_snapshot INTEGER NOT NULL DEFAULT 0,
			quota_available_snapshot INTEGER NOT NULL DEFAULT 0,
			fetched_at INTEGER NOT NULL DEFAULT 0,
			expires_at INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT ''
		)`,
	}
	return execStatements(statements)
}

func ensureColumns() error {
	// 这里把“正式字段为什么存在”直接写在迁移层，避免后续维护只看 SQL 不知道设计边界。
	// Server SQLite 只保存中心索引和摘要，不保存 Edge 资产正文，因此补列时只允许增加身份、
	// 状态、审计和聚合缓存字段，不能顺手把 profile/proxy/fingerprint 等敏感资产塞进来。
	columnMap := map[string][]string{
		"edge_clients": {
			"main_account_id TEXT NOT NULL DEFAULT ''",
			"client_sequence INTEGER NOT NULL DEFAULT 0",
			"name TEXT NOT NULL DEFAULT ''",
			"client_ip TEXT NOT NULL DEFAULT ''",
			"base_url TEXT NOT NULL DEFAULT ''",
			"docker_api_url TEXT NOT NULL DEFAULT ''",
			"os TEXT NOT NULL DEFAULT ''",
			"arch TEXT NOT NULL DEFAULT ''",
			"cpu_cores INTEGER NOT NULL DEFAULT 0",
			"memory_total_mb INTEGER NOT NULL DEFAULT 0",
			"docker_version TEXT NOT NULL DEFAULT ''",
			"health_status TEXT NOT NULL DEFAULT 'offline'",
			"discovery_status TEXT NOT NULL DEFAULT 'blocked'",
			"discovery_reason TEXT NOT NULL DEFAULT 'not_bound'",
			"push_status TEXT NOT NULL DEFAULT 'pending'",
			"api_key_hash TEXT NOT NULL DEFAULT ''",
			"last_discovered_at INTEGER NOT NULL DEFAULT 0",
			"last_heartbeat_at INTEGER NOT NULL DEFAULT 0",
			"last_heartbeat_reported_at INTEGER NOT NULL DEFAULT 0",
			"last_heartbeat_source TEXT NOT NULL DEFAULT ''",
			"last_checked_at INTEGER NOT NULL DEFAULT 0",
			"last_error TEXT NOT NULL DEFAULT ''",
			"created_by_user_id TEXT NOT NULL DEFAULT ''",
			"created_by_username TEXT NOT NULL DEFAULT ''",
			"created_at INTEGER NOT NULL DEFAULT 0",
			"updated_at INTEGER NOT NULL DEFAULT 0",
			"deleted_at INTEGER NOT NULL DEFAULT 0",
		},
		"edge_client_bind_logs": {
			"main_account_id TEXT NOT NULL DEFAULT ''",
			"client_ip TEXT NOT NULL DEFAULT ''",
			"operator_user_id TEXT NOT NULL DEFAULT ''",
			"operator_username TEXT NOT NULL DEFAULT ''",
			"action TEXT NOT NULL DEFAULT ''",
			"result TEXT NOT NULL DEFAULT ''",
			"message TEXT NOT NULL DEFAULT ''",
			"created_at INTEGER NOT NULL DEFAULT 0",
		},
		"server_browser_envs": {
			"main_account_id TEXT NOT NULL DEFAULT ''",
			"client_id TEXT NOT NULL DEFAULT ''",
			"user_id TEXT NOT NULL DEFAULT ''",
			"rpa_type TEXT NOT NULL DEFAULT ''",
			"name TEXT NOT NULL DEFAULT ''",
			"status TEXT NOT NULL DEFAULT ''",
			"container_status TEXT NOT NULL DEFAULT ''",
			"runtime_status TEXT NOT NULL DEFAULT ''",
			"current_slot_id TEXT NOT NULL DEFAULT ''",
			"cdp_url TEXT NOT NULL DEFAULT ''",
			"web_vnc_url TEXT NOT NULL DEFAULT ''",
			"last_task_id TEXT NOT NULL DEFAULT ''",
			"last_error TEXT NOT NULL DEFAULT ''",
			"last_synced_at INTEGER NOT NULL DEFAULT 0",
			"created_at INTEGER NOT NULL DEFAULT 0",
			"updated_at INTEGER NOT NULL DEFAULT 0",
			"deleted_at INTEGER NOT NULL DEFAULT 0",
		},
		"server_tasks": {
			"main_account_id TEXT NOT NULL DEFAULT ''",
			"operator_user_id TEXT NOT NULL DEFAULT ''",
			"operator_username TEXT NOT NULL DEFAULT ''",
			"client_id TEXT NOT NULL DEFAULT ''",
			"env_id TEXT NOT NULL DEFAULT ''",
			"task_type TEXT NOT NULL DEFAULT ''",
			"resource_type TEXT NOT NULL DEFAULT ''",
			"resource_id TEXT NOT NULL DEFAULT ''",
			"status TEXT NOT NULL DEFAULT ''",
			"edge_task_id TEXT NOT NULL DEFAULT ''",
			"events_url TEXT NOT NULL DEFAULT ''",
			"error_message TEXT NOT NULL DEFAULT ''",
			"suggestion TEXT NOT NULL DEFAULT ''",
			"created_at INTEGER NOT NULL DEFAULT 0",
			"updated_at INTEGER NOT NULL DEFAULT 0",
			"finished_at INTEGER NOT NULL DEFAULT 0",
		},
		"client_run_quotas": {
			"quota_limit INTEGER NOT NULL DEFAULT 0",
			"quota_used_snapshot INTEGER NOT NULL DEFAULT 0",
			"quota_available_snapshot INTEGER NOT NULL DEFAULT 0",
			"fetched_at INTEGER NOT NULL DEFAULT 0",
			"expires_at INTEGER NOT NULL DEFAULT 0",
			"status TEXT NOT NULL DEFAULT ''",
			"last_error TEXT NOT NULL DEFAULT ''",
		},
	}
	for tableName, columns := range columnMap {
		if err := ensureTableColumns(tableName, columns); err != nil {
			return err
		}
	}
	return nil
}

func createIndexes() error {
	statements := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_edge_clients_base_url_active ON edge_clients(base_url, deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_edge_clients_main_account_id ON edge_clients(main_account_id, deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_edge_clients_health_discovery ON edge_clients(health_status, discovery_status, deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_edge_clients_client_ip ON edge_clients(client_ip, deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_bind_logs_client_created_at ON edge_client_bind_logs(client_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_server_browser_envs_main_account_id ON server_browser_envs(main_account_id, deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_server_browser_envs_client_id ON server_browser_envs(client_id, deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_server_browser_envs_status ON server_browser_envs(status, deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_server_tasks_main_account_id ON server_tasks(main_account_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_server_tasks_client_id ON server_tasks(client_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_server_tasks_env_id ON server_tasks(env_id, created_at)`,
	}
	return execStatements(statements)
}

// normalizeNodeHealthStatuses 把旧的中心在线状态值收敛到当前正式两态。
//
// 设计来源：
// - 早期联调里出现过 `stale`、`unhealthy` 等临时值；
// - 现在已经明确 Node 在线状态只保留 `healthy / offline`；
// - 因此迁移时直接把非正式值回收到 `offline`，等待后续 heartbeat 再把在线节点刷新回 `healthy`。
func normalizeNodeHealthStatuses() error {
	if _, err := db.Exec(`UPDATE edge_clients
		SET health_status = 'offline'
		WHERE health_status NOT IN ('healthy', 'offline')`); err != nil {
		return fmt.Errorf("normalize edge_clients health_status failed: %w", err)
	}
	return nil
}

func execStatements(statements []string) error {
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("migrate sqlite failed: %w", err)
		}
	}
	return nil
}

func ensureTableColumns(tableName string, definitions []string) error {
	existingColumns, err := listColumns(tableName)
	if err != nil {
		return err
	}
	for _, definition := range definitions {
		columnName := parseColumnName(definition)
		if columnName == "" || existingColumns[columnName] {
			continue
		}
		statement := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", tableName, definition)
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("alter table %s add column %s failed: %w", tableName, columnName, err)
		}
	}
	return nil
}

func listColumns(tableName string) (map[string]bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, fmt.Errorf("query sqlite table info for %s failed: %w", tableName, err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("scan sqlite table info for %s failed: %w", tableName, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite table info for %s failed: %w", tableName, err)
	}
	return columns, nil
}

func parseColumnName(definition string) string {
	fields := strings.Fields(strings.TrimSpace(definition))
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimSpace(fields[0])
}
