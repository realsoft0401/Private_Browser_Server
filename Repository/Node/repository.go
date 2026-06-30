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
		target_slot_count, actual_slot_count, available_slot_count, running_slot_count, slot_exception_status,
		slot_exception_reason, last_slot_checked_at, api_key_hash, last_discovered_at, last_heartbeat_at,
		last_heartbeat_reported_at, last_heartbeat_source, last_checked_at, last_error, created_by_user_id,
		created_by_username, created_at, updated_at, deleted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ClientID, row.MainAccountID, row.ClientSequence, row.Name, row.ClientIP, row.BaseURL, row.DockerAPIURL,
		row.OS, row.Arch, row.CPUCores, row.MemoryTotalMB, row.DockerVersion, row.HealthStatus, row.DiscoveryStatus,
		row.DiscoveryReason, row.PushStatus, row.TargetSlotCount, row.ActualSlotCount, row.AvailableSlotCount,
		row.RunningSlotCount, row.SlotExceptionStatus, row.SlotExceptionReason, row.LastSlotCheckedAt,
		row.APIKeyHash, row.LastDiscoveredAt, row.LastHeartbeatAt, row.LastHeartbeatReportedAt, row.LastHeartbeatSource,
		row.LastCheckedAt, row.LastError, row.CreatedByUserID, row.CreatedByUsername, row.CreatedAt, row.UpdatedAt,
		row.DeletedAt,
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
		target_slot_count, actual_slot_count, available_slot_count, running_slot_count, slot_exception_status,
		slot_exception_reason, last_slot_checked_at, last_discovered_at, last_heartbeat_at, last_heartbeat_reported_at,
		last_heartbeat_source, last_checked_at, last_error, created_by_user_id, created_by_username, created_at,
		updated_at, deleted_at
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
		target_slot_count, actual_slot_count, available_slot_count, running_slot_count, slot_exception_status,
		slot_exception_reason, last_slot_checked_at, last_discovered_at, last_heartbeat_at, last_heartbeat_reported_at,
		last_heartbeat_source, last_checked_at, last_error, created_by_user_id, created_by_username, created_at,
		updated_at, deleted_at
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
		target_slot_count, actual_slot_count, available_slot_count, running_slot_count, slot_exception_status,
		slot_exception_reason, last_slot_checked_at, last_discovered_at, last_heartbeat_at, last_heartbeat_reported_at,
		last_heartbeat_source, last_checked_at, last_error, created_by_user_id, created_by_username, created_at,
		updated_at, deleted_at
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
		target_slot_count, actual_slot_count, available_slot_count, running_slot_count, slot_exception_status,
		slot_exception_reason, last_slot_checked_at, last_discovered_at, last_heartbeat_at, last_heartbeat_reported_at,
		last_heartbeat_source, last_checked_at, last_error, created_by_user_id, created_by_username, created_at,
		updated_at, deleted_at
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
// - 现在 unbind 已经继续收紧为“直接删除 edge_clients 当前记录”，因此不能再只看当前节点表；
// - 当前表 `edge_clients` 只保存“当前有效节点”，历史 bind/unbind 留痕在 `edge_client_bind_logs`；
// - 所以这里必须同时参考“当前有效节点表 + 历史绑定审计”，保证即使解绑后删记录，下一次也不会回到 `0001`。
func (r *Repository) AllocateNextSequence(ctx context.Context, mainAccountID string) (int64, error) {
	var sequence sql.NullInt64
	err := CommonRepo.DB().QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence), 0) FROM (
		SELECT client_sequence AS sequence
		FROM edge_clients
		WHERE main_account_id = ?
		UNION ALL
		SELECT CAST(SUBSTR(client_id, LENGTH(?) + 1) AS INTEGER) AS sequence
		FROM edge_client_bind_logs
		WHERE main_account_id = ?
		  AND action = 'bind'
		  AND result = 'success'
		  AND client_id LIKE ? || '%'
	)`, mainAccountID, mainAccountID, mainAccountID, mainAccountID).Scan(&sequence)
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

// Rebind 保留为历史兼容辅助方法，但当前正式 bind 主线已经不再复用旧身份。
//
// 职责边界：
// - 当前正式口径已经改成：unbind 删除当前有效绑定结果，后续 bind 必须生成新的 clientId；
// - 因此这条方法不再服务当前主链路，只保留给后续历史数据修复或一次性迁移场景；
// - 正常 bind / unbind 流程不要再调用这里，避免把“旧身份复用”偷偷带回正式实现。
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

// Unbind 删除当前有效绑定结果，但保留历史审计链路。
//
// 设计边界：
// - 当前需求已经明确：unbind 之后不能继续保留一个可直接复用的有效中心身份；
// - 因此这里直接物理删除 `edge_clients` 当前记录，让后续 bind 必须重新分配新的 clientId；
// - 历史审计不在这张表里回收，edge_client_bind_logs 仍保留完整治理留痕；
// - 业务层如需清理 Client 本地 JSON，必须在 repository 之外继续走 Edge API。
func (r *Repository) Unbind(ctx context.Context, clientID string, updatedAt int64) error {
	_ = updatedAt
	result, err := CommonRepo.DB().ExecContext(ctx, `DELETE FROM edge_clients
		WHERE client_id = ? AND deleted_at = 0`, clientID)
	if err != nil {
		return fmt.Errorf("delete edge_clients on unbind failed: %w", err)
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

// UpdateSlotSummary 负责把 Node 对某个 Client 的 slot 汇总摘要受控回写到主节点表。
//
// 设计来源：
// - 文档已经收口：slot 数量、占用数、异常标记要聚合展示在 edge_clients 主表；
// - 但 slot 明细仍在 edge_client_slots，因此这里单独负责“只改汇总字段”；
// - 后续 slot_reconcile、rebind 后初始化、管理员治理动作都应通过这里统一更新，避免多处手写不同口径。
func (r *Repository) UpdateSlotSummary(ctx context.Context, row *NodeDAO.Row) error {
	if row == nil {
		return fmt.Errorf("node row 不能为空")
	}
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET
		target_slot_count = CASE WHEN ? >= 0 THEN ? ELSE target_slot_count END,
		actual_slot_count = CASE WHEN ? >= 0 THEN ? ELSE actual_slot_count END,
		available_slot_count = CASE WHEN ? >= 0 THEN ? ELSE available_slot_count END,
		running_slot_count = CASE WHEN ? >= 0 THEN ? ELSE running_slot_count END,
		slot_exception_status = CASE WHEN ? <> '' THEN ? ELSE slot_exception_status END,
		slot_exception_reason = ?,
		last_slot_checked_at = CASE WHEN ? > 0 THEN ? ELSE last_slot_checked_at END,
		updated_at = CASE WHEN ? > 0 THEN ? ELSE updated_at END
		WHERE client_id = ? AND deleted_at = 0`,
		row.TargetSlotCount, row.TargetSlotCount,
		row.ActualSlotCount, row.ActualSlotCount,
		row.AvailableSlotCount, row.AvailableSlotCount,
		row.RunningSlotCount, row.RunningSlotCount,
		row.SlotExceptionStatus, row.SlotExceptionStatus,
		row.SlotExceptionReason,
		row.LastSlotCheckedAt, row.LastSlotCheckedAt,
		row.UpdatedAt, row.UpdatedAt,
		row.ClientID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients slot summary failed: %w", err)
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

// UpdateTargetSlotCount 只更新中心目标 slot 数和由此推导的异常摘要。
//
// 设计来源：
// - `target_slot_count` 属于中心治理事实，不是 Client 自报事实；
// - 在平台正式下发接口接入前，需要先允许管理员手工设置这个值；
// - 这里刻意不改 `actual_slot_count`，避免把中心目标误写成 Client 实际资源结果。
func (r *Repository) UpdateTargetSlotCount(
	ctx context.Context,
	clientID string,
	targetSlotCount int64,
	slotExceptionStatus string,
	slotExceptionReason string,
	updatedAt int64,
) error {
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET
		target_slot_count = ?,
		slot_exception_status = CASE WHEN ? <> '' THEN ? ELSE slot_exception_status END,
		slot_exception_reason = ?,
		last_slot_checked_at = CASE WHEN ? > 0 THEN ? ELSE last_slot_checked_at END,
		updated_at = CASE WHEN ? > 0 THEN ? ELSE updated_at END
		WHERE client_id = ? AND deleted_at = 0`,
		targetSlotCount,
		slotExceptionStatus, slotExceptionStatus,
		slotExceptionReason,
		updatedAt, updatedAt,
		updatedAt, updatedAt,
		clientID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients target slot count failed: %w", err)
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

// ReplaceSlots 用一次事务全量刷新当前 Client 的 slot 明细缓存。
//
// 设计来源：
// - 需求已经明确：slot_reconcile 应按 Client 当前返回全量刷新，而不是增量猜；
// - rebind 后 slot 也要按“空白重新初始化后的全量事实”重建；
// - 因此这里采用 delete + insert 的显式全量替换，避免保留旧 slot 脏数据。
func (r *Repository) ReplaceSlots(ctx context.Context, clientID string, slots []NodeDAO.SlotRow) error {
	if clientID == "" {
		return fmt.Errorf("clientID 不能为空")
	}
	tx, err := CommonRepo.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace edge_client_slots transaction failed: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM edge_client_slots WHERE client_id = ?`, clientID); err != nil {
		return fmt.Errorf("delete edge_client_slots failed: %w", err)
	}
	for _, slot := range slots {
		if _, err = tx.ExecContext(ctx, `INSERT INTO edge_client_slots (
			client_id, slot_id, status, current_env_id, current_run_id, container_id, container_name,
			cdp_port, vnc_port, last_error, last_synced_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			clientID,
			slot.SlotID,
			slot.Status,
			slot.CurrentEnvID,
			slot.CurrentRunID,
			slot.ContainerID,
			slot.ContainerName,
			slot.CDPPort,
			slot.VNCPort,
			slot.LastError,
			slot.LastSyncedAt,
			slot.CreatedAt,
			slot.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert edge_client_slot %s failed: %w", slot.SlotID, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit replace edge_client_slots transaction failed: %w", err)
	}
	return nil
}

// ListSlotsByClientID 返回某个已绑定节点当前缓存的 slot 明细。
func (r *Repository) ListSlotsByClientID(ctx context.Context, clientID string) ([]NodeModel.EdgeClientSlot, error) {
	rows, err := CommonRepo.DB().QueryContext(ctx, `SELECT
		id, client_id, slot_id, status, current_env_id, current_run_id, container_id, container_name,
		cdp_port, vnc_port, last_error, last_synced_at, created_at, updated_at
		FROM edge_client_slots
		WHERE client_id = ?
		ORDER BY slot_id ASC`, clientID)
	if err != nil {
		return nil, fmt.Errorf("query edge_client_slots failed: %w", err)
	}
	defer rows.Close()

	result := make([]NodeModel.EdgeClientSlot, 0)
	for rows.Next() {
		var item NodeModel.EdgeClientSlot
		if err = rows.Scan(
			&item.ID, &item.ClientID, &item.SlotID, &item.Status, &item.CurrentEnvID, &item.CurrentRunID,
			&item.ContainerID, &item.ContainerName, &item.CDPPort, &item.VNCPort, &item.LastError,
			&item.LastSyncedAt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan edge_client_slots failed: %w", err)
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate edge_client_slots failed: %w", err)
	}
	return result, nil
}

// CreateSlotLog 记录一次中心层 slot 治理动作留痕。
func (r *Repository) CreateSlotLog(ctx context.Context, row *NodeDAO.SlotLogRow) error {
	if row == nil {
		return fmt.Errorf("slot log row 不能为空")
	}
	_, err := CommonRepo.DB().ExecContext(ctx, `INSERT INTO edge_client_slot_logs (
		client_id, slot_id, action, result, env_id, run_id, message, operator_user_id, operator_username, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ClientID, row.SlotID, row.Action, row.Result, row.EnvID, row.RunID, row.Message,
		row.OperatorUserID, row.OperatorUsername, row.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert edge_client_slot_logs failed: %w", err)
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

// UpdateSessionCheckResult 负责把一次“会话校验 / recheck”的最终摘要回写到中心节点表。
//
// 设计来源：
// - 会话校验的核心不是重新 bind，而是刷新 Node 自己对这台节点“是否仍可信”的判断；
// - 这次判断既可能成功恢复为 `healthy + verified`，也可能收口为 `blocked + probe_failed`；
// - 因此这里把“设备摘要刷新”和“治理状态刷新”合并到同一次受控更新里，避免 Service 层散着改多张字段。
//
// 职责边界：
// - 允许更新设备摘要、健康摘要、discovery 摘要和错误字段；
// - 不修改 `client_id`、`main_account_id`、`client_sequence` 这些中心身份字段；
// - 不在这里自动改地址治理策略，地址确认更新应走独立接口。
func (r *Repository) UpdateSessionCheckResult(ctx context.Context, row *NodeDAO.Row) error {
	if row == nil {
		return fmt.Errorf("node row 不能为空")
	}
	result, err := CommonRepo.DB().ExecContext(ctx, `UPDATE edge_clients SET
		docker_api_url = CASE WHEN ? <> '' THEN ? ELSE docker_api_url END,
		os = CASE WHEN ? <> '' THEN ? ELSE os END,
		arch = CASE WHEN ? <> '' THEN ? ELSE arch END,
		cpu_cores = CASE WHEN ? > 0 THEN ? ELSE cpu_cores END,
		memory_total_mb = CASE WHEN ? > 0 THEN ? ELSE memory_total_mb END,
		docker_version = CASE WHEN ? <> '' THEN ? ELSE docker_version END,
		health_status = CASE WHEN ? <> '' THEN ? ELSE health_status END,
		discovery_status = CASE WHEN ? <> '' THEN ? ELSE discovery_status END,
		discovery_reason = ?,
		last_checked_at = CASE WHEN ? > 0 THEN ? ELSE last_checked_at END,
		last_error = ?,
		updated_at = CASE WHEN ? > 0 THEN ? ELSE updated_at END
		WHERE client_id = ? AND deleted_at = 0`,
		row.DockerAPIURL, row.DockerAPIURL,
		row.OS, row.OS,
		row.Arch, row.Arch,
		row.CPUCores, row.CPUCores,
		row.MemoryTotalMB, row.MemoryTotalMB,
		row.DockerVersion, row.DockerVersion,
		row.HealthStatus, row.HealthStatus,
		row.DiscoveryStatus, row.DiscoveryStatus,
		row.DiscoveryReason,
		row.LastCheckedAt, row.LastCheckedAt,
		row.LastError,
		row.UpdatedAt, row.UpdatedAt,
		row.ClientID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients session check result failed: %w", err)
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

// UpdateAddressAndSessionCheckResult 负责在管理员确认后更新中心节点地址，并同步回写一次节点治理摘要。
//
// 设计来源：
// - `confirm-address-update` 的职责不是重新 bind，而是把“确认仍是同一台机器”的新地址写回中心；
// - 地址写回之后，仍然要同时刷新健康状态、发现状态和设备摘要，避免出现“地址改了但摘要还是旧的”；
// - 因此这里单独提供一个“地址 + 治理摘要”的受控更新入口，避免 Service 层拼多条 SQL。
//
// 职责边界：
// - 允许更新 `client_ip / base_url` 以及治理摘要字段；
// - 不修改 `client_id / main_account_id / client_sequence`；
// - 不做地址探测与冲突判断，这些由 Service 层负责后再调用这里落库。
func (r *Repository) UpdateAddressAndSessionCheckResult(ctx context.Context, row *NodeDAO.Row) error {
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
		health_status = CASE WHEN ? <> '' THEN ? ELSE health_status END,
		discovery_status = CASE WHEN ? <> '' THEN ? ELSE discovery_status END,
		discovery_reason = ?,
		last_checked_at = CASE WHEN ? > 0 THEN ? ELSE last_checked_at END,
		last_error = ?,
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
		row.HealthStatus, row.HealthStatus,
		row.DiscoveryStatus, row.DiscoveryStatus,
		row.DiscoveryReason,
		row.LastCheckedAt, row.LastCheckedAt,
		row.LastError,
		row.UpdatedAt, row.UpdatedAt,
		row.ClientID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients address and session check result failed: %w", err)
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
		&node.TargetSlotCount, &node.ActualSlotCount, &node.AvailableSlotCount, &node.RunningSlotCount,
		&node.SlotExceptionStatus, &node.SlotExceptionReason, &node.LastSlotCheckedAt, &node.LastDiscoveredAt,
		&node.LastHeartbeatAt, &node.LastHeartbeatReportedAt, &node.LastHeartbeatSource, &node.LastCheckedAt,
		&node.LastError, &node.CreatedByUserID, &node.CreatedByUsername, &node.CreatedAt, &node.UpdatedAt,
		&node.DeletedAt,
	)
	return node, err
}
