package Task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	TaskDAO "private_browser_server/Dao/Task"
	TaskModel "private_browser_server/Models/Task"
	CommonRepo "private_browser_server/Repository/Common"
)

type Repository struct{}

var ErrNotFound = errors.New("server task not found")

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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query server_tasks failed: %w", err)
	}
	return &item, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, row *TaskDAO.Row) error {
	if row == nil {
		return fmt.Errorf("task row 不能为空")
	}
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE server_tasks SET
		env_id = CASE WHEN ? <> '' THEN ? ELSE env_id END,
		resource_id = CASE WHEN ? <> '' THEN ? ELSE resource_id END,
		status = CASE WHEN ? <> '' THEN ? ELSE status END,
		edge_task_id = CASE WHEN ? <> '' THEN ? ELSE edge_task_id END,
		events_url = CASE WHEN ? <> '' THEN ? ELSE events_url END,
		error_message = ?,
		suggestion = ?,
		updated_at = CASE WHEN ? > 0 THEN ? ELSE updated_at END,
		finished_at = CASE WHEN ? > 0 THEN ? ELSE finished_at END
		WHERE id = ?`,
		row.EnvID, row.EnvID,
		row.ResourceID, row.ResourceID,
		row.Status, row.Status,
		row.EdgeTaskID, row.EdgeTaskID,
		row.EventsURL, row.EventsURL,
		row.ErrorMessage,
		row.Suggestion,
		row.UpdatedAt, row.UpdatedAt,
		row.FinishedAt, row.FinishedAt,
		row.ID,
	)
	if err != nil {
		return fmt.Errorf("update server_tasks status failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read rows affected failed: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
