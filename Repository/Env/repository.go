package Env

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	model "private_browser_server/Models/Env"
	"private_browser_server/Rom"
)

var ErrEnvNotFound = errors.New("server browser env not found")

// Repository 是 server_browser_envs 表的底层访问入口。
//
// 设计来源：
// - Node Server 需要保存中心环境包聚合缓存，但不能把 Edge 真实环境包实体复制进 SQLite；
// - 因此这一层只处理中心索引表的 CRUD，所有业务校验和 Edge API 调用都留在 Service；
// - 后续如果要切换查询条件、索引策略或数据库驱动，应优先在这里调整，而不是把 SQL 散到 HTTP 层。
//
// 职责边界：
// - 只负责 SQLite 访问、RowsAffected 和 sql.ErrNoRows 归一化；
// - 不做 Platform Header 解析，不做 imagePolicy 选择，不调用 Edge API；
// - 不返回面向前端的中文业务提示，业务语义由 Service 层包装。
type Repository struct{}

// Create 写入一条中心环境包索引记录。
//
// 写入时机是 Edge 创建环境包成功之后；如果 Edge 失败，这里不能先落半成品记录。
func (Repository) Create(ctx context.Context, env *model.ServerBrowserEnv) error {
	if env == nil {
		return fmt.Errorf("server browser env 不能为空")
	}
	_, err := Rom.DB().ExecContext(ctx, `INSERT INTO server_browser_envs (
		env_id, main_account_id, client_id, rpa_type, name, status, container_status, monitor_status,
		cdp_url, web_vnc_url, last_task_id, last_error, created_by_user_id, created_by_username,
		created_at, updated_at, deleted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		env.EnvID, env.MainAccountID, env.ClientID, env.RPAType, env.Name, env.Status, env.ContainerStatus, env.MonitorStatus,
		env.CDPURL, env.WebVNCURL, env.LastTaskID, env.LastError, env.CreatedByUserID, env.CreatedByUsername,
		env.CreatedAt, env.UpdatedAt, env.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert server_browser_envs failed: %w", err)
	}
	return nil
}

// GetByID 查询主账号下的单个中心环境包索引。
func (Repository) GetByID(ctx context.Context, mainAccountID string, envID string) (*model.ServerBrowserEnv, error) {
	row := Rom.DB().QueryRowContext(ctx, `SELECT
		env_id, main_account_id, client_id, rpa_type, name, status, container_status, monitor_status,
		cdp_url, web_vnc_url, last_task_id, last_error, created_by_user_id, created_by_username,
		created_at, updated_at, deleted_at
		FROM server_browser_envs
		WHERE main_account_id = ? AND env_id = ? AND deleted_at = 0`,
		mainAccountID, envID,
	)
	env, err := scanEnv(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEnvNotFound
	}
	if err != nil {
		return nil, err
	}
	return env, nil
}

// ListByMainAccount 查询主账号下的中心环境包列表。
//
// 这里保留最小过滤条件：clientId、rpaType、status；后续 Dashboard 统计和任务列表都可以复用这组索引。
func (Repository) ListByMainAccount(ctx context.Context, mainAccountID string, query model.ListEnvQuery) ([]model.ServerBrowserEnv, int, error) {
	whereSQL, args := buildListWhere(mainAccountID, query)

	countSQL := "SELECT COUNT(1) FROM server_browser_envs WHERE " + whereSQL
	var total int
	if err := Rom.DB().QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count server_browser_envs failed: %w", err)
	}

	limit := query.PageSize
	offset := (query.Page - 1) * query.PageSize
	listArgs := append(append([]any{}, args...), limit, offset)
	listSQL := `SELECT
		env_id, main_account_id, client_id, rpa_type, name, status, container_status, monitor_status,
		cdp_url, web_vnc_url, last_task_id, last_error, created_by_user_id, created_by_username,
		created_at, updated_at, deleted_at
		FROM server_browser_envs
		WHERE ` + whereSQL + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`
	rows, err := Rom.DB().QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query server_browser_envs failed: %w", err)
	}
	defer rows.Close()

	items := make([]model.ServerBrowserEnv, 0)
	for rows.Next() {
		env, scanErr := scanEnv(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *env)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate server_browser_envs failed: %w", err)
	}
	return items, total, nil
}

// UpdateTaskSummary 回写环境包最近一次中心任务摘要。
//
// 设计来源：
// - run/stop/create 都需要把最近任务痕迹挂回 server_browser_envs，方便列表页直接看到最后一次动作；
// - 这里只更新 lastTaskId/lastError/updatedAt，不在任务启动瞬间伪造运行状态；
// - 真正的 status/containerStatus/monitorStatus 更新应在查询到 Edge 事实后再写入。
func (Repository) UpdateTaskSummary(ctx context.Context, mainAccountID string, envID string, taskID string, lastError string, updatedAt int64) error {
	result, err := Rom.DB().ExecContext(ctx, `UPDATE server_browser_envs SET
		last_task_id = ?, last_error = ?, updated_at = ?
		WHERE main_account_id = ? AND env_id = ? AND deleted_at = 0`,
		taskID, lastError, updatedAt, mainAccountID, envID,
	)
	if err != nil {
		return fmt.Errorf("update server_browser_envs task summary failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		return ErrEnvNotFound
	}
	return nil
}

// UpdateSnapshot 回写环境包当前中心缓存摘要。
//
// 它用于根据 Edge detail 或 Edge task 结果刷新中心状态，不直接猜测运行事实。
func (Repository) UpdateSnapshot(ctx context.Context, env *model.ServerBrowserEnv) error {
	if env == nil {
		return fmt.Errorf("server browser env 不能为空")
	}
	result, err := Rom.DB().ExecContext(ctx, `UPDATE server_browser_envs SET
		client_id = ?, rpa_type = ?, name = ?, status = ?, container_status = ?, monitor_status = ?,
		cdp_url = ?, web_vnc_url = ?, last_task_id = ?, last_error = ?, updated_at = ?, deleted_at = ?
		WHERE main_account_id = ? AND env_id = ?`,
		env.ClientID, env.RPAType, env.Name, env.Status, env.ContainerStatus, env.MonitorStatus,
		env.CDPURL, env.WebVNCURL, env.LastTaskID, env.LastError, env.UpdatedAt, env.DeletedAt,
		env.MainAccountID, env.EnvID,
	)
	if err != nil {
		return fmt.Errorf("update server_browser_envs snapshot failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		return ErrEnvNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEnv(scanner rowScanner) (*model.ServerBrowserEnv, error) {
	env := new(model.ServerBrowserEnv)
	if err := scanner.Scan(
		&env.EnvID,
		&env.MainAccountID,
		&env.ClientID,
		&env.RPAType,
		&env.Name,
		&env.Status,
		&env.ContainerStatus,
		&env.MonitorStatus,
		&env.CDPURL,
		&env.WebVNCURL,
		&env.LastTaskID,
		&env.LastError,
		&env.CreatedByUserID,
		&env.CreatedByUsername,
		&env.CreatedAt,
		&env.UpdatedAt,
		&env.DeletedAt,
	); err != nil {
		return nil, err
	}
	return env, nil
}

func buildListWhere(mainAccountID string, query model.ListEnvQuery) (string, []any) {
	clauses := []string{"main_account_id = ?", "deleted_at = 0"}
	args := []any{mainAccountID}

	if value := strings.TrimSpace(query.ClientID); value != "" {
		clauses = append(clauses, "client_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.RPAType); value != "" {
		clauses = append(clauses, "rpa_type = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.Status); value != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, value)
	}
	return strings.Join(clauses, " AND "), args
}
