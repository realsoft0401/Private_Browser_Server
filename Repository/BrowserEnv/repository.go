package BrowserEnv

import (
	"context"
	"fmt"

	BrowserEnvDAO "private_browser_server/Dao/BrowserEnv"
	BrowserEnvModel "private_browser_server/Models/BrowserEnv"
	CommonRepo "private_browser_server/Repository/Common"
)

type Repository struct{}

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
		return nil, fmt.Errorf("query server_browser_envs failed: %w", err)
	}
	return &item, nil
}
