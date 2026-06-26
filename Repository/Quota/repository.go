package Quota

import (
	"context"
	"fmt"

	QuotaDAO "private_browser_server/Dao/Quota"
	QuotaModel "private_browser_server/Models/Quota"
	CommonRepo "private_browser_server/Repository/Common"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) Upsert(ctx context.Context, row *QuotaDAO.Row) error {
	_, err := CommonRepo.DB().ExecContext(ctx, `INSERT INTO client_run_quotas (
		client_id, quota_limit, quota_used_snapshot, quota_available_snapshot, fetched_at, expires_at, status, last_error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(client_id) DO UPDATE SET
		quota_limit = excluded.quota_limit,
		quota_used_snapshot = excluded.quota_used_snapshot,
		quota_available_snapshot = excluded.quota_available_snapshot,
		fetched_at = excluded.fetched_at,
		expires_at = excluded.expires_at,
		status = excluded.status,
		last_error = excluded.last_error`,
		row.ClientID, row.QuotaLimit, row.QuotaUsedSnapshot, row.QuotaAvailableSnapshot,
		row.FetchedAt, row.ExpiresAt, row.Status, row.LastError,
	)
	if err != nil {
		return fmt.Errorf("upsert client_run_quotas failed: %w", err)
	}
	return nil
}

func (r *Repository) GetByClientID(ctx context.Context, clientID string) (*QuotaModel.ClientRunQuota, error) {
	row := CommonRepo.DB().QueryRowContext(ctx, `SELECT
		client_id, quota_limit, quota_used_snapshot, quota_available_snapshot, fetched_at, expires_at, status, last_error
		FROM client_run_quotas WHERE client_id = ?`, clientID)
	var item QuotaModel.ClientRunQuota
	err := row.Scan(
		&item.ClientID, &item.QuotaLimit, &item.QuotaUsedSnapshot, &item.QuotaAvailableSnapshot,
		&item.FetchedAt, &item.ExpiresAt, &item.Status, &item.LastError,
	)
	if err != nil {
		return nil, fmt.Errorf("query client_run_quotas failed: %w", err)
	}
	return &item, nil
}
