package Task

import (
	"context"
	"fmt"

	TaskDAO "private_browser_server/Dao/Task"
	TaskModel "private_browser_server/Models/Task"
	CommonRepo "private_browser_server/Repository/Common"
)

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

func (r *Repository) Create(ctx context.Context, row *TaskDAO.Row) error {
	_, err := CommonRepo.DB().ExecContext(ctx, `INSERT INTO server_tasks (
		id, main_account_id, operator_user_id, operator_username, client_id, env_id, task_type, resource_type,
		resource_id, status, edge_task_id, events_url, error_message, suggestion, created_at, updated_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.MainAccountID, row.OperatorUserID, row.OperatorUsername, row.ClientID, row.EnvID,
		row.TaskType, row.ResourceType, row.ResourceID, row.Status, row.EdgeTaskID, row.EventsURL,
		row.ErrorMessage, row.Suggestion, row.CreatedAt, row.UpdatedAt, row.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("insert server_tasks failed: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*TaskModel.ServerTask, error) {
	row := CommonRepo.DB().QueryRowContext(ctx, `SELECT
		id, main_account_id, operator_user_id, operator_username, client_id, env_id, task_type, resource_type,
		resource_id, status, edge_task_id, events_url, error_message, suggestion, created_at, updated_at, finished_at
		FROM server_tasks WHERE id = ?`, id)
	var item TaskModel.ServerTask
	err := row.Scan(
		&item.ID, &item.MainAccountID, &item.OperatorUserID, &item.OperatorUsername, &item.ClientID, &item.EnvID,
		&item.TaskType, &item.ResourceType, &item.ResourceID, &item.Status, &item.EdgeTaskID, &item.EventsURL,
		&item.ErrorMessage, &item.Suggestion, &item.CreatedAt, &item.UpdatedAt, &item.FinishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("query server_tasks failed: %w", err)
	}
	return &item, nil
}
