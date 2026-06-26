package Bind

import (
	"context"
	"fmt"

	BindDAO "private_browser_server/Dao/Bind"
	CommonRepo "private_browser_server/Repository/Common"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) CreateLog(ctx context.Context, row *BindDAO.LogRow) error {
	_, err := CommonRepo.DB().ExecContext(ctx, `INSERT INTO edge_client_bind_logs (
		client_id, main_account_id, client_ip, operator_user_id, operator_username, action, result, message, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ClientID, row.MainAccountID, row.ClientIP, row.OperatorUserID, row.OperatorUsername,
		row.Action, row.Result, row.Message, row.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert edge_client_bind_logs failed: %w", err)
	}
	return nil
}
