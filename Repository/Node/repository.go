package Node

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	NodeDAO "private_browser_server/Dao/Node"
	NodeModel "private_browser_server/Models/Node"
	CommonRepo "private_browser_server/Repository/Common"
)

var ErrNotFound = errors.New("edge client not found")

type Repository struct{}

func NewRepository() *Repository {
	return &Repository{}
}

// Create 写入中心正式节点。
//
// 职责边界：
// - 这里只写中心身份与节点摘要；
// - 不负责探测 Client，不负责下发 clientId，也不负责 browser-env 聚合；
// - bind/discovery/heartbeat 只能通过明确字段更新这里，避免 repository 里偷偷夹带业务流程。
func (r *Repository) Create(ctx context.Context, row *NodeDAO.Row) error {
	_, err := CommonRepo.DB().ExecContext(ctx, `INSERT INTO edge_clients (
		client_id, main_account_id, client_sequence, name, client_ip, base_url, docker_api_url, os, arch,
		cpu_cores, memory_total_mb, docker_version, health_status, discovery_status, discovery_reason, push_status,
		api_key_hash, last_discovered_at, last_heartbeat_at, last_heartbeat_reported_at, last_heartbeat_source,
		last_checked_at, last_error, created_by_user_id, created_by_username, created_at, updated_at, deleted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ClientID, row.MainAccountID, row.ClientSequence, row.Name, row.ClientIP, row.BaseURL, row.DockerAPIURL,
		row.OS, row.Arch, row.CPUCores, row.MemoryTotalMB, row.DockerVersion, row.HealthStatus, row.DiscoveryStatus,
		row.DiscoveryReason, row.PushStatus, row.APIKeyHash, row.LastDiscoveredAt, row.LastHeartbeatAt,
		row.LastHeartbeatReportedAt, row.LastHeartbeatSource, row.LastCheckedAt, row.LastError,
		row.CreatedByUserID, row.CreatedByUsername, row.CreatedAt, row.UpdatedAt, row.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert edge_clients failed: %w", err)
	}
	return nil
}

func (r *Repository) ListByMainAccountID(ctx context.Context, mainAccountID string) ([]NodeModel.EdgeClient, error) {
	rows, err := CommonRepo.DB().QueryContext(ctx, `SELECT
		client_id, main_account_id, client_sequence, name, client_ip, base_url, docker_api_url, os, arch,
		cpu_cores, memory_total_mb, docker_version, health_status, discovery_status, discovery_reason, push_status,
		last_discovered_at, last_heartbeat_at, last_heartbeat_reported_at, last_heartbeat_source, last_checked_at,
		last_error, created_by_user_id, created_by_username, created_at, updated_at, deleted_at
		FROM edge_clients WHERE main_account_id = ? AND deleted_at = 0 ORDER BY created_at DESC`, mainAccountID)
	if err != nil {
		return nil, fmt.Errorf("query edge_clients failed: %w", err)
	}
	defer rows.Close()

	result := make([]NodeModel.EdgeClient, 0)
	for rows.Next() {
		node, scanErr := scanNode(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, node)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate edge_clients failed: %w", err)
	}
	return result, nil
}

// ListByAccountID 保留给当前第一阶段 HTTP 入口兼容使用。
//
// 这里不再把 `account_id` 当正式数据库字段，而是明确映射到新的 `main_account_id`。
func (r *Repository) ListByAccountID(ctx context.Context, accountID string) ([]NodeModel.EdgeClient, error) {
	return r.ListByMainAccountID(ctx, accountID)
}

func (r *Repository) GetByClientID(ctx context.Context, clientID string) (*NodeModel.EdgeClient, error) {
	row := CommonRepo.DB().QueryRowContext(ctx, `SELECT
		client_id, main_account_id, client_sequence, name, client_ip, base_url, docker_api_url, os, arch,
		cpu_cores, memory_total_mb, docker_version, health_status, discovery_status, discovery_reason, push_status,
		last_discovered_at, last_heartbeat_at, last_heartbeat_reported_at, last_heartbeat_source, last_checked_at,
		last_error, created_by_user_id, created_by_username, created_at, updated_at, deleted_at
		FROM edge_clients WHERE client_id = ? AND deleted_at = 0`, clientID)
	node, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *Repository) GetByClientIP(ctx context.Context, clientIP string) (*NodeModel.EdgeClient, error) {
	row := CommonRepo.DB().QueryRowContext(ctx, `SELECT
		client_id, main_account_id, client_sequence, name, client_ip, base_url, docker_api_url, os, arch,
		cpu_cores, memory_total_mb, docker_version, health_status, discovery_status, discovery_reason, push_status,
		last_discovered_at, last_heartbeat_at, last_heartbeat_reported_at, last_heartbeat_source, last_checked_at,
		last_error, created_by_user_id, created_by_username, created_at, updated_at, deleted_at
		FROM edge_clients WHERE client_ip = ? AND deleted_at = 0`, clientIP)
	node, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (r *Repository) GetByBaseURL(ctx context.Context, baseURL string) (*NodeModel.EdgeClient, error) {
	row := CommonRepo.DB().QueryRowContext(ctx, `SELECT
		client_id, main_account_id, client_sequence, name, client_ip, base_url, docker_api_url, os, arch,
		cpu_cores, memory_total_mb, docker_version, health_status, discovery_status, discovery_reason, push_status,
		last_discovered_at, last_heartbeat_at, last_heartbeat_reported_at, last_heartbeat_source, last_checked_at,
		last_error, created_by_user_id, created_by_username, created_at, updated_at, deleted_at
		FROM edge_clients WHERE base_url = ? AND deleted_at = 0`, baseURL)
	node, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// AllocateNextSequence 负责按主账号分配不可回收的设备序号。
//
// 设计来源：
// - 旧的 `len(nodes)+1` 在解绑、软删除、并发 bind 场景下会重复；
// - 现在口径已经收紧为 `MAX(client_sequence)+1`，且即使解绑也不回收；
// - 因此序号分配必须下沉到 repository，避免业务层各自猜。
func (r *Repository) AllocateNextSequence(ctx context.Context, mainAccountID string) (int64, error) {
	var sequence sql.NullInt64
	err := CommonRepo.DB().QueryRowContext(ctx, `SELECT COALESCE(MAX(client_sequence), 0)
		FROM edge_clients WHERE main_account_id = ?`, mainAccountID).Scan(&sequence)
	if err != nil {
		return 0, fmt.Errorf("query next client sequence failed: %w", err)
	}
	return sequence.Int64 + 1, nil
}

func (r *Repository) UpdatePushStatus(ctx context.Context, clientID, pushStatus string, updatedAt int64) error {
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET push_status = ?, updated_at = ? WHERE client_id = ?`,
		pushStatus, updatedAt, clientID)
	if err != nil {
		return fmt.Errorf("update edge_clients push status failed: %w", err)
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

// Rebind 更新已存在但当前未绑定节点的中心归属与最新探测摘要。
//
// 设计来源：
// - 已解绑节点后续重新绑定时，必须沿用原 clientId 和 clientSequence；
// - 因此这里不能再走 insert，而要在保留中心身份的前提下更新归属与节点摘要；
// - 这个方法只更新“允许随着重绑刷新”的字段，不触碰历史 created_at 和中心身份主键。
func (r *Repository) Rebind(ctx context.Context, row *NodeDAO.Row) error {
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET
		main_account_id = ?, name = ?, client_ip = ?, base_url = ?, docker_api_url = ?, os = ?, arch = ?,
		cpu_cores = ?, memory_total_mb = ?, docker_version = ?, health_status = ?, discovery_status = ?,
		discovery_reason = ?, push_status = ?, api_key_hash = ?, last_discovered_at = ?, last_heartbeat_at = ?,
		last_heartbeat_reported_at = ?, last_heartbeat_source = ?, last_checked_at = ?, last_error = ?, updated_at = ?,
		deleted_at = 0
		WHERE client_id = ?`,
		row.MainAccountID, row.Name, row.ClientIP, row.BaseURL, row.DockerAPIURL, row.OS, row.Arch,
		row.CPUCores, row.MemoryTotalMB, row.DockerVersion, row.HealthStatus, row.DiscoveryStatus,
		row.DiscoveryReason, row.PushStatus, row.APIKeyHash, row.LastDiscoveredAt, row.LastHeartbeatAt,
		row.LastHeartbeatReportedAt, row.LastHeartbeatSource, row.LastCheckedAt, row.LastError, row.UpdatedAt,
		row.ClientID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients rebind failed: %w", err)
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

// Unbind 解除当前节点的中心归属，但保留原 clientId 和历史审计链路。
//
// 设计边界：
// - unbind 不是删节点，而是把节点收回到“未绑定但身份保留”的中心状态；
// - 这里不删除记录，不回收 clientSequence，不改 clientId；
// - 业务层如需清理 Client 本地 JSON，必须在 repository 之外继续走 Edge API。
func (r *Repository) Unbind(ctx context.Context, clientID string, updatedAt int64) error {
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET
		main_account_id = '', discovery_status = 'blocked', discovery_reason = 'not_bound',
		push_status = 'pending', last_error = '', updated_at = ?
		WHERE client_id = ? AND deleted_at = 0`,
		updatedAt, clientID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients unbind failed: %w", err)
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

func (r *Repository) UpdateLastError(ctx context.Context, clientID, lastError string, updatedAt int64) error {
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET last_error = ?, updated_at = ? WHERE client_id = ? AND deleted_at = 0`,
		lastError, updatedAt, clientID)
	if err != nil {
		return fmt.Errorf("update edge_clients last error failed: %w", err)
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

// MarkHeartbeatHealthy 在收到已知节点 heartbeat 后刷新在线事实。
//
// 设计来源：
// - 最新口径已经拍板：UDP 才是 discovery，heartbeat 只证明服务存在；
// - 同时 Node 在线状态已经收口成 `healthy / offline`，因此 heartbeat 成功后必须直接回到 `healthy`；
// - 这里仍然只更新已知正式节点，不建新节点、不改绑定归属。
func (r *Repository) MarkHeartbeatHealthy(ctx context.Context, clientID string, receivedAt, reportedAt int64, source, clientIP, baseURL string) error {
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET
		client_ip = CASE WHEN ? <> '' THEN ? ELSE client_ip END,
		base_url = CASE WHEN ? <> '' THEN ? ELSE base_url END,
		last_heartbeat_at = ?,
		last_heartbeat_reported_at = ?,
		last_heartbeat_source = ?,
		last_checked_at = ?,
		health_status = 'healthy',
		last_error = '',
		updated_at = ?
		WHERE client_id = ? AND deleted_at = 0`,
		clientIP, clientIP, baseURL, baseURL, receivedAt, reportedAt, source, receivedAt, receivedAt, clientID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients heartbeat facts failed: %w", err)
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

// MarkOfflineByHeartbeatTimeout 负责把超过 heartbeat 阈值的已知节点统一收口成 offline。
//
// 设计来源：
// - 用户已经明确要求只保留 `healthy / offline`，不再经过 `stale`；
// - 因此后台巡检不再做中间态缓存，而是只根据“最后一次有效 heartbeat / 最近一次绑定探测事实”是否超时来判离线；
// - 这里不删除节点、不解绑账号，只改在线状态，保留原 clientId 和历史链路。
func (r *Repository) MarkOfflineByHeartbeatTimeout(ctx context.Context, now, offlineAfterSeconds int64) (int64, error) {
	if offlineAfterSeconds <= 0 {
		return 0, nil
	}
	cutoff := now - offlineAfterSeconds
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET
		health_status = 'offline',
		updated_at = ?
		WHERE deleted_at = 0
		  AND health_status <> 'offline'
		  AND (
			(last_heartbeat_at > 0 AND last_heartbeat_at <= ?)
			OR (last_heartbeat_at = 0 AND last_checked_at > 0 AND last_checked_at <= ?)
		  )`,
		now, cutoff, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("mark edge_clients offline by heartbeat timeout failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read rows affected failed: %w", err)
	}
	return affected, nil
}

// UpdateDeviceFacts 刷新已知节点的宿主机设备摘要。
//
// 设计来源：
// - bind 会落一次设备事实，但已存在节点后续不能指望“重新绑定”才能更新硬件摘要；
// - 因此 heartbeat 命中已知节点后，也允许顺手刷新 `/device-info` 探测到的宿主机摘要；
// - 这里只更新设备事实字段，不触碰账号归属、discovery 状态和 heartbeat 主时间戳。
func (r *Repository) UpdateDeviceFacts(ctx context.Context, row *NodeDAO.Row) error {
	if row == nil {
		return fmt.Errorf("node row 不能为空")
	}
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET
		client_ip = CASE WHEN ? <> '' THEN ? ELSE client_ip END,
		base_url = CASE WHEN ? <> '' THEN ? ELSE base_url END,
		docker_api_url = CASE WHEN ? <> '' THEN ? ELSE docker_api_url END,
		os = CASE WHEN ? <> '' THEN ? ELSE os END,
		arch = CASE WHEN ? <> '' THEN ? ELSE arch END,
		cpu_cores = CASE WHEN ? > 0 THEN ? ELSE cpu_cores END,
		memory_total_mb = CASE WHEN ? > 0 THEN ? ELSE memory_total_mb END,
		docker_version = CASE WHEN ? <> '' THEN ? ELSE docker_version END,
		last_checked_at = CASE WHEN ? > 0 THEN ? ELSE last_checked_at END,
		updated_at = CASE WHEN ? > 0 THEN ? ELSE updated_at END
		WHERE client_id = ? AND deleted_at = 0`,
		row.ClientIP, row.ClientIP,
		row.BaseURL, row.BaseURL,
		row.DockerAPIURL, row.DockerAPIURL,
		row.OS, row.OS,
		row.Arch, row.Arch,
		row.CPUCores, row.CPUCores,
		row.MemoryTotalMB, row.MemoryTotalMB,
		row.DockerVersion, row.DockerVersion,
		row.LastCheckedAt, row.LastCheckedAt,
		row.UpdatedAt, row.UpdatedAt,
		row.ClientID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients device facts failed: %w", err)
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

type scanner interface {
	Scan(dest ...any) error
}

func scanNode(row scanner) (NodeModel.EdgeClient, error) {
	var node NodeModel.EdgeClient
	err := row.Scan(
		&node.ClientID, &node.MainAccountID, &node.ClientSequence, &node.Name, &node.ClientIP, &node.BaseURL,
		&node.DockerAPIURL, &node.OS, &node.Arch, &node.CPUCores, &node.MemoryTotalMB, &node.DockerVersion,
		&node.HealthStatus, &node.DiscoveryStatus, &node.DiscoveryReason, &node.PushStatus,
		&node.LastDiscoveredAt, &node.LastHeartbeatAt, &node.LastHeartbeatReportedAt, &node.LastHeartbeatSource,
		&node.LastCheckedAt, &node.LastError, &node.CreatedByUserID, &node.CreatedByUsername,
		&node.CreatedAt, &node.UpdatedAt, &node.DeletedAt,
	)
	return node, err
}
