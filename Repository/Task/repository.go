package Task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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

// List 按稳定字段查询中心任务主记录。
//
// 职责边界：
// - 这里只读 `server_tasks` 主表，不读取进程内 SSE 缓存，也不访问 Edge；
// - 查询字段全部使用参数化条件，避免任务类型、状态等外部输入拼进 SQL；
// - 排序固定为最近更新优先，便于管理员排查最新失败任务。
func (r *Repository) List(ctx context.Context, query TaskModel.ListQuery) ([]TaskModel.ServerTask, int, error) {
	whereSQL, args := buildListWhere(query)
	countSQL := `SELECT COUNT(1) FROM server_tasks` + whereSQL
	var total int
	if err := CommonRepo.DB().QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count server_tasks failed: %w", err)
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := CommonRepo.DB().QueryContext(ctx, `SELECT
		id, main_account_id, operator_user_id, operator_username, client_id, env_id, task_type, resource_type,
		resource_id, status, edge_task_id, events_url, error_message, suggestion, created_at, updated_at, finished_at
		FROM server_tasks`+whereSQL+` ORDER BY updated_at DESC, created_at DESC, id DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query server_tasks list failed: %w", err)
	}
	defer rows.Close()

	items := make([]TaskModel.ServerTask, 0)
	for rows.Next() {
		var item TaskModel.ServerTask
		if err = rows.Scan(
			&item.ID, &item.MainAccountID, &item.OperatorUserID, &item.OperatorUsername, &item.ClientID, &item.EnvID,
			&item.TaskType, &item.ResourceType, &item.ResourceID, &item.Status, &item.EdgeTaskID, &item.EventsURL,
			&item.ErrorMessage, &item.Suggestion, &item.CreatedAt, &item.UpdatedAt, &item.FinishedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan server_tasks list failed: %w", err)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate server_tasks list failed: %w", err)
	}
	return items, total, nil
}

// buildListWhere 统一构造任务列表的 WHERE 条件。
//
// 这里单独抽函数，是为了把“允许过滤哪些字段”固定下来；后续不要随手开放任意排序字段或模糊 SQL，
// 否则中心任务审计表会从可控查询入口变成半个搜索引擎。
func buildListWhere(query TaskModel.ListQuery) (string, []any) {
	clauses := make([]string, 0)
	args := make([]any, 0)
	if value := strings.TrimSpace(query.ClientID); value != "" {
		clauses = append(clauses, "client_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.EnvID); value != "" {
		clauses = append(clauses, "env_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.ResourceID); value != "" {
		clauses = append(clauses, "resource_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.TaskType); value != "" {
		clauses = append(clauses, "task_type = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.Status); value != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, value)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
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
