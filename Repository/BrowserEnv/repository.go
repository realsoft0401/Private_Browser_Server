package BrowserEnv

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	BrowserEnvDAO "private_browser_server/Dao/BrowserEnv"
	BrowserEnvModel "private_browser_server/Models/BrowserEnv"
	CommonRepo "private_browser_server/Repository/Common"
)

type Repository struct{}

var ErrNotFound = errors.New("server browser env not found")

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) Upsert(ctx context.Context, row *BrowserEnvDAO.Row) error {
	_, err := CommonRepo.DB().ExecContext(ctx, `INSERT INTO server_browser_envs (
		env_id, main_account_id, client_id, user_id, rpa_type, name, status, container_status, runtime_status,
		current_slot_id, cdp_url, web_vnc_url, last_task_id, last_error, last_synced_at, created_at, updated_at, deleted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(env_id) DO UPDATE SET
		main_account_id = excluded.main_account_id,
		client_id = excluded.client_id,
		user_id = excluded.user_id,
		rpa_type = excluded.rpa_type,
		name = excluded.name,
		status = excluded.status,
		container_status = excluded.container_status,
		runtime_status = excluded.runtime_status,
		current_slot_id = excluded.current_slot_id,
		cdp_url = excluded.cdp_url,
		web_vnc_url = excluded.web_vnc_url,
		last_task_id = excluded.last_task_id,
		last_error = excluded.last_error,
		last_synced_at = excluded.last_synced_at,
		updated_at = excluded.updated_at,
		deleted_at = excluded.deleted_at`,
		row.EnvID, row.MainAccountID, row.ClientID, row.UserID, row.RPAType, row.Name, row.Status,
		row.ContainerStatus, row.RuntimeStatus, row.CurrentSlotID, row.CDPURL, row.WebVNCURL, row.LastTaskID,
		row.LastError, row.LastSyncedAt, row.CreatedAt, row.UpdatedAt, row.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert server_browser_envs failed: %w", err)
	}
	return nil
}

func (r *Repository) GetByEnvID(ctx context.Context, envID string) (*BrowserEnvModel.ServerBrowserEnv, error) {
	row := CommonRepo.DB().QueryRowContext(ctx, `SELECT
		env_id, main_account_id, client_id, user_id, rpa_type, name, status, container_status, runtime_status,
		current_slot_id, cdp_url, web_vnc_url, last_task_id, last_error, last_synced_at, created_at, updated_at, deleted_at
		FROM server_browser_envs WHERE env_id = ?`, envID)
	var item BrowserEnvModel.ServerBrowserEnv
	err := row.Scan(
		&item.EnvID, &item.MainAccountID, &item.ClientID, &item.UserID, &item.RPAType, &item.Name, &item.Status,
		&item.ContainerStatus, &item.RuntimeStatus, &item.CurrentSlotID, &item.CDPURL, &item.WebVNCURL,
		&item.LastTaskID, &item.LastError, &item.LastSyncedAt, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query server_browser_envs failed: %w", err)
	}
	return &item, nil
}

// DeleteByEnvID 从中心缓存中删除一条 browser-env 聚合记录。
//
// 设计来源：
// - package delete 成功后，当前节点上的环境资产已经被彻底销毁；
// - 这时继续把旧记录保留在 `server_browser_envs` 主视图里，只会误导后续列表和生命周期调用；
// - 因此这里直接删除中心缓存行，把“已删除”事实留给 `server_tasks` 审计链路承接。
func (r *Repository) DeleteByEnvID(ctx context.Context, envID string) error {
	result, err := CommonRepo.DB().ExecContext(ctx, `DELETE FROM server_browser_envs WHERE env_id = ?`, envID)
	if err != nil {
		return fmt.Errorf("delete server_browser_envs failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete rows affected failed: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// List 按中心缓存条件查询 browser-env 列表。
//
// 职责边界：
// - 只查询 `server_browser_envs` 当前缓存，不主动探测 Edge；
// - 只支持当前正式字段上的稳定过滤，不做模糊搜索；
// - 默认排除 `deleted_at > 0` 的历史记录，避免把软删除 env 再次暴露给业务主列表。
func (r *Repository) List(ctx context.Context, query BrowserEnvModel.ListQuery) ([]BrowserEnvModel.ServerBrowserEnv, error) {
	baseSQL := `SELECT
		env_id, main_account_id, client_id, user_id, rpa_type, name, status, container_status, runtime_status,
		current_slot_id, cdp_url, web_vnc_url, last_task_id, last_error, last_synced_at, created_at, updated_at, deleted_at
		FROM server_browser_envs WHERE deleted_at = 0`
	args := make([]any, 0)
	if value := strings.TrimSpace(query.AccountID); value != "" {
		baseSQL += ` AND main_account_id = ?`
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.ClientID); value != "" {
		baseSQL += ` AND client_id = ?`
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.UserID); value != "" {
		baseSQL += ` AND user_id = ?`
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.RPAType); value != "" {
		baseSQL += ` AND rpa_type = ?`
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.Status); value != "" {
		baseSQL += ` AND status = ?`
		args = append(args, value)
	}
	baseSQL += ` ORDER BY updated_at DESC, env_id DESC`

	rows, err := CommonRepo.DB().QueryContext(ctx, baseSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("query server_browser_envs list failed: %w", err)
	}
	defer rows.Close()

	items := make([]BrowserEnvModel.ServerBrowserEnv, 0)
	for rows.Next() {
		var item BrowserEnvModel.ServerBrowserEnv
		if err = rows.Scan(
			&item.EnvID, &item.MainAccountID, &item.ClientID, &item.UserID, &item.RPAType, &item.Name, &item.Status,
			&item.ContainerStatus, &item.RuntimeStatus, &item.CurrentSlotID, &item.CDPURL, &item.WebVNCURL,
			&item.LastTaskID, &item.LastError, &item.LastSyncedAt, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan server_browser_envs list failed: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate server_browser_envs list failed: %w", err)
	}
	return items, nil
}
