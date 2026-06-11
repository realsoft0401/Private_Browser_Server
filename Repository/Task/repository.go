package Task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	model "private_browser_server/Models/Task"
	"private_browser_server/Rom"
)

var ErrTaskNotFound = errors.New("server task not found")

// Repository 是 server_tasks 表的底层访问入口。
//
// 设计来源：
// - Node Server 需要把中心 taskId、主账号归属和 Edge task 绑定持久化到 SQLite；
// - 任务更新会在 run/stop/查询刷新等多个入口发生，数据库细节必须收口，避免 HTTP 层散写 SQL；
// - 当前先服务环境包生命周期任务，后续 pull-image / RPA 继续复用同一仓储。
//
// 职责边界：
// - 只负责 SQLite 访问、查无记录归一化和基本列表过滤；
// - 不做 Platform Header 解析、不调用 Edge API、不决定任务成功失败语义；
// - 不返回面向前端的中文提示，业务语义由 Service 层补充。
type Repository struct{}

// Create 写入一条中心任务记录。
func (Repository) Create(ctx context.Context, task *model.ServerTask) error {
	if task == nil {
		return fmt.Errorf("server task 不能为空")
	}
	_, err := Rom.DB().ExecContext(ctx, `INSERT INTO server_tasks (
		id, main_account_id, operator_user_id, operator_username, client_id, env_id,
		type, status, edge_task_id, events_url, error_message, created_at, updated_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.TaskID, task.MainAccountID, task.OperatorUserID, task.OperatorUsername, task.ClientID, task.EnvID,
		task.Type, task.Status, task.EdgeTaskID, task.EventsURL, task.ErrorMessage, task.CreatedAt, task.UpdatedAt, task.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("insert server_tasks failed: %w", err)
	}
	return nil
}

// GetByID 查询主账号下的单个中心任务。
func (Repository) GetByID(ctx context.Context, mainAccountID string, taskID string) (*model.ServerTask, error) {
	row := Rom.DB().QueryRowContext(ctx, `SELECT
		id, main_account_id, operator_user_id, operator_username, client_id, env_id,
		type, status, edge_task_id, events_url, error_message, created_at, updated_at, finished_at
		FROM server_tasks
		WHERE main_account_id = ? AND id = ?`,
		mainAccountID, taskID,
	)
	task, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

// ListByMainAccount 查询主账号下的中心任务列表。
func (Repository) ListByMainAccount(ctx context.Context, mainAccountID string, query model.ListTaskQuery) ([]model.ServerTask, int, error) {
	whereSQL, args := buildListWhere(mainAccountID, query)

	countSQL := "SELECT COUNT(1) FROM server_tasks WHERE " + whereSQL
	var total int
	if err := Rom.DB().QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count server_tasks failed: %w", err)
	}

	limit := query.PageSize
	offset := (query.Page - 1) * query.PageSize
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := Rom.DB().QueryContext(ctx, `SELECT
		id, main_account_id, operator_user_id, operator_username, client_id, env_id,
		type, status, edge_task_id, events_url, error_message, created_at, updated_at, finished_at
		FROM server_tasks
		WHERE `+whereSQL+`
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query server_tasks failed: %w", err)
	}
	defer rows.Close()

	items := make([]model.ServerTask, 0)
	for rows.Next() {
		task, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, *task)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate server_tasks failed: %w", err)
	}
	return items, total, nil
}

// Update 回写一条中心任务当前状态。
//
// 当前版本直接按 taskId + mainAccountId 覆盖当前摘要，避免为 pending/running/success/failed
// 拆出多个数据库细粒度方法，让 Service 层更容易围绕“任务事实对象”推演。
func (Repository) Update(ctx context.Context, task *model.ServerTask) error {
	if task == nil {
		return fmt.Errorf("server task 不能为空")
	}
	result, err := Rom.DB().ExecContext(ctx, `UPDATE server_tasks SET
		operator_user_id = ?, operator_username = ?, client_id = ?, env_id = ?, type = ?, status = ?,
		edge_task_id = ?, events_url = ?, error_message = ?, updated_at = ?, finished_at = ?
		WHERE main_account_id = ? AND id = ?`,
		task.OperatorUserID, task.OperatorUsername, task.ClientID, task.EnvID, task.Type, task.Status,
		task.EdgeTaskID, task.EventsURL, task.ErrorMessage, task.UpdatedAt, task.FinishedAt,
		task.MainAccountID, task.TaskID,
	)
	if err != nil {
		return fmt.Errorf("update server_tasks failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil && rowsAffected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner rowScanner) (*model.ServerTask, error) {
	task := new(model.ServerTask)
	if err := scanner.Scan(
		&task.TaskID,
		&task.MainAccountID,
		&task.OperatorUserID,
		&task.OperatorUsername,
		&task.ClientID,
		&task.EnvID,
		&task.Type,
		&task.Status,
		&task.EdgeTaskID,
		&task.EventsURL,
		&task.ErrorMessage,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.FinishedAt,
	); err != nil {
		return nil, err
	}
	return task, nil
}

func buildListWhere(mainAccountID string, query model.ListTaskQuery) (string, []any) {
	clauses := []string{"main_account_id = ?"}
	args := []any{mainAccountID}

	if value := strings.TrimSpace(query.ClientID); value != "" {
		clauses = append(clauses, "client_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.EnvID); value != "" {
		clauses = append(clauses, "env_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.Type); value != "" {
		clauses = append(clauses, "type = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(query.Status); value != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, value)
	}
	return strings.Join(clauses, " AND "), args
}
