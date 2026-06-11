package Node

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Rom"
)

var ErrNotFound = errors.New("edge client not found")

// Repository 是 edge_clients 表的底层访问入口。
//
// 它只处理 SQLite 读写、RowsAffected 和 sql.ErrNoRows 归一化；
// 不做 Docker 探测、不解析 Platform Header、不返回中文业务文案，避免数据库层膨胀成业务层。
type Repository struct{}

// Create 写入一个中心侧节点索引。
//
// 节点注册只保存接入信息和平台归属；设备能力来自后续 Docker 2375/Client health 探测，
// 因此这里不会伪造 os/arch/dockerVersion，也不会把节点直接标成 verified。
func (Repository) Create(ctx context.Context, node *NodeModel.EdgeClient) error {
	_, err := Rom.DB().ExecContext(ctx, `INSERT INTO edge_clients (
		id, main_account_id, client_sequence, name, base_url, client_ip, docker_api_url,
		os, arch, cpu_cores, memory_total_mb, docker_version, health_status, discovery_status,
		last_heartbeat_at, last_heartbeat_reported_at, last_heartbeat_source, last_checked_at, last_error, created_by_user_id, created_by_username,
		created_at, updated_at, deleted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.MainAccountID, node.NodeSequence, node.Name, node.BaseURL, node.ClientIP, node.DockerAPIURL,
		node.OS, node.Arch, node.CPUCores, node.MemoryTotalMB, node.DockerVersion, node.HealthStatus, node.DiscoveryStatus,
		node.LastHeartbeatAt, node.LastHeartbeatReportedAt, node.LastHeartbeatSource, node.LastCheckedAt, node.LastError, node.CreatedByUserID, node.CreatedByUsername,
		node.CreatedAt, node.UpdatedAt, node.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert edge_clients failed: %w", err)
	}
	return nil
}

// ListByMainAccount 查询主账号下未删除的 Edge Client。
func (Repository) ListByMainAccount(ctx context.Context, mainAccountID string) ([]NodeModel.EdgeClient, error) {
	rows, err := Rom.DB().QueryContext(ctx, `SELECT
		id, main_account_id, client_sequence, name, base_url, client_ip, docker_api_url,
		os, arch, cpu_cores, memory_total_mb, docker_version, health_status, discovery_status,
		last_heartbeat_at, last_heartbeat_reported_at, last_heartbeat_source, last_checked_at, last_error, created_by_user_id, created_by_username,
		created_at, updated_at, deleted_at
		FROM edge_clients
		WHERE main_account_id = ? AND deleted_at = 0
		ORDER BY created_at DESC`, mainAccountID)
	if err != nil {
		return nil, fmt.Errorf("query edge_clients failed: %w", err)
	}
	defer rows.Close()

	nodes := make([]NodeModel.EdgeClient, 0)
	for rows.Next() {
		node, scanErr := scanNode(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		nodes = append(nodes, node)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate edge_clients failed: %w", err)
	}
	return nodes, nil
}

// GetByID 查询主账号下的单个 Edge Client。
func (Repository) GetByID(ctx context.Context, mainAccountID string, id string) (*NodeModel.EdgeClient, error) {
	row := Rom.DB().QueryRowContext(ctx, `SELECT
		id, main_account_id, client_sequence, name, base_url, client_ip, docker_api_url,
		os, arch, cpu_cores, memory_total_mb, docker_version, health_status, discovery_status,
		last_heartbeat_at, last_heartbeat_reported_at, last_heartbeat_source, last_checked_at, last_error, created_by_user_id, created_by_username,
		created_at, updated_at, deleted_at
		FROM edge_clients
		WHERE main_account_id = ? AND id = ? AND deleted_at = 0`, mainAccountID, id)
	node, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// UpdateDeviceInfo 保存 Docker 2375 探测到的 Edge Client 设备事实。
//
// 该方法只更新设备能力摘要和健康字段，不修改 baseUrl/clientIp/mainAccountId，
// 避免探测过程自动覆盖 clientId 身份，后续 identity_changed/ip_mismatch 规则可继续加在 Service 层。
func (Repository) UpdateDeviceInfo(ctx context.Context, node *NodeModel.EdgeClient) error {
	result, err := Rom.DB().ExecContext(ctx, `UPDATE edge_clients SET
		os = ?, arch = ?, cpu_cores = ?, memory_total_mb = ?, docker_version = ?,
		health_status = ?, discovery_status = ?, last_checked_at = ?, last_error = ?, updated_at = ?
		WHERE id = ? AND main_account_id = ? AND deleted_at = 0`,
		node.OS, node.Arch, node.CPUCores, node.MemoryTotalMB, node.DockerVersion,
		node.HealthStatus, node.DiscoveryStatus, node.LastCheckedAt, node.LastError, node.UpdatedAt,
		node.ID, node.MainAccountID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients device info failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update edge_clients rows affected failed: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateVerifyResult 保存 verify 最终状态。
//
// verify 是 Node Server 决定 Client 能否进入业务动作的关键流程；Repository 这里只落库，
// 不调用 HTTP、不判断 heartbeat、不拼业务错误文案，避免把状态机塞进数据库层。
func (Repository) UpdateVerifyResult(ctx context.Context, node *NodeModel.EdgeClient) error {
	result, err := Rom.DB().ExecContext(ctx, `UPDATE edge_clients SET
		os = ?, arch = ?, cpu_cores = ?, memory_total_mb = ?, docker_version = ?,
		health_status = ?, discovery_status = ?, last_checked_at = ?, last_error = ?, updated_at = ?
		WHERE id = ? AND main_account_id = ? AND deleted_at = 0`,
		node.OS, node.Arch, node.CPUCores, node.MemoryTotalMB, node.DockerVersion,
		node.HealthStatus, node.DiscoveryStatus, node.LastCheckedAt, node.LastError, node.UpdatedAt,
		node.ID, node.MainAccountID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients verify result failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update edge_clients verify rows affected failed: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateHeartbeat 保存已匹配 Client 的最后心跳事实。
//
// 设计边界：
// - last_heartbeat_at 只保存 Node Server 实际接收时间，用它驱动 heartbeatStatus；
// - last_heartbeat_reported_at 只保留 Client 自报时间，服务端不拿它直接判断 online/stale/offline；
// - 这个方法不修改 health_status/discovery_status，避免心跳把节点偷偷放行；
// - WHERE 条件要求接收时间递增，避免旧包、乱序包或重放包把在线事实回滚。
func (Repository) UpdateHeartbeat(ctx context.Context, mainAccountID string, clientID string, receivedAt int64, reportedAt int64, source string) error {
	if receivedAt <= 0 {
		return nil
	}
	source = strings.TrimSpace(source)
	_, err := Rom.DB().ExecContext(ctx, `UPDATE edge_clients SET
		last_heartbeat_at = ?, last_heartbeat_reported_at = ?, last_heartbeat_source = ?, updated_at = ?
		WHERE id = ? AND main_account_id = ? AND deleted_at = 0 AND last_heartbeat_at < ?`,
		receivedAt, reportedAt, source, receivedAt, clientID, mainAccountID, receivedAt,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients heartbeat failed: %w", err)
	}
	return nil
}

// UpdateHeartbeatByDiscovery 根据 UDP discovery 的服务入口回写已注册 Edge Client 心跳。
//
// 设计来源：用户希望不用手动刷新 discovered，也能在 SQLite 里看到 last_heartbeat_at 持续更新。
// 这里仍然只更新已经注册过的 edge_clients：匹配 baseUrl/clientIp/sourceIp 任一命中才写入；
// 未注册的 UDP beacon 只留在内存 discovery 缓存，不自动创建 Client，也不产生主账号归属。
func (Repository) UpdateHeartbeatByDiscovery(ctx context.Context, baseURL string, clientIP string, sourceIP string, receivedAt int64, reportedAt int64) error {
	if receivedAt <= 0 {
		return nil
	}
	baseURL = strings.TrimSpace(baseURL)
	clientIP = strings.TrimSpace(clientIP)
	sourceIP = strings.TrimSpace(sourceIP)
	if baseURL == "" && clientIP == "" && sourceIP == "" {
		return nil
	}
	_, err := Rom.DB().ExecContext(ctx, `UPDATE edge_clients SET
		last_heartbeat_at = ?, last_heartbeat_reported_at = ?, last_heartbeat_source = ?, updated_at = ?
		WHERE deleted_at = 0
		  AND last_heartbeat_at < ?
		  AND (
			(? <> '' AND base_url = ?)
			OR (? <> '' AND client_ip = ?)
			OR (? <> '' AND client_ip = ?)
		  )`,
		receivedAt, reportedAt, "udp", receivedAt,
		receivedAt,
		baseURL, baseURL,
		clientIP, clientIP,
		sourceIP, sourceIP,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients heartbeat by discovery failed: %w", err)
	}
	return nil
}

// GetByHeartbeatLookup 根据心跳报文中的 baseUrl/clientIp/sourceIp 查找已注册 Edge Client。
//
// 设计来源：
// - Client 不生成也不保存 clientId，正式 heartbeat 不能强依赖中心身份；
// - Node Server 在内网模式下只能用已登记的 baseUrl/clientIp/sourceIp 去匹配现有节点；
// - 匹配成功后才能回写 last_heartbeat_at，但仍然不能把节点自动变成 healthy 或 verified。
//
// 匹配优先级：
// - baseUrl 优先，因为它最接近后续实际访问的 Edge API 入口；
// - clientIp/sourceIp 只作为 baseUrl 缺失或手工录入不完整时的兜底。
func (Repository) GetByHeartbeatLookup(ctx context.Context, baseURL string, clientIP string, sourceIP string) (*NodeModel.EdgeClient, error) {
	baseURL = strings.TrimSpace(baseURL)
	clientIP = strings.TrimSpace(clientIP)
	sourceIP = strings.TrimSpace(sourceIP)
	if baseURL == "" && clientIP == "" && sourceIP == "" {
		return nil, ErrNotFound
	}

	row := Rom.DB().QueryRowContext(ctx, `SELECT
		id, main_account_id, client_sequence, name, base_url, client_ip, docker_api_url,
		os, arch, cpu_cores, memory_total_mb, docker_version, health_status, discovery_status,
		last_heartbeat_at, last_heartbeat_reported_at, last_heartbeat_source, last_checked_at, last_error, created_by_user_id, created_by_username,
		created_at, updated_at, deleted_at
		FROM edge_clients
		WHERE deleted_at = 0
		  AND (
			(? <> '' AND base_url = ?)
			OR (? <> '' AND client_ip = ?)
			OR (? <> '' AND client_ip = ?)
		  )
		ORDER BY
		  CASE WHEN (? <> '' AND base_url = ?) THEN 0 ELSE 1 END,
		  CASE WHEN (? <> '' AND client_ip = ?) THEN 0 ELSE 1 END,
		  created_at DESC
		LIMIT 1`,
		baseURL, baseURL,
		clientIP, clientIP,
		sourceIP, sourceIP,
		baseURL, baseURL,
		clientIP, clientIP,
	)
	node, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// NextSequence 返回主账号下下一个 Edge Client 序号。
//
// 序号只用于展示和人工排查，不参与权限或 clientId 身份；真正身份仍是 id/baseUrl/clientIp 校验结果。
func (Repository) NextSequence(ctx context.Context, mainAccountID string) (int, error) {
	var current sql.NullInt64
	err := Rom.DB().QueryRowContext(ctx, `SELECT MAX(client_sequence) FROM edge_clients WHERE main_account_id = ?`, mainAccountID).Scan(&current)
	if err != nil {
		return 0, fmt.Errorf("query next client sequence failed: %w", err)
	}
	if !current.Valid {
		return 1, nil
	}
	return int(current.Int64) + 1, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNode(row rowScanner) (NodeModel.EdgeClient, error) {
	var node NodeModel.EdgeClient
	err := row.Scan(
		&node.ID, &node.MainAccountID, &node.NodeSequence, &node.Name, &node.BaseURL, &node.ClientIP, &node.DockerAPIURL,
		&node.OS, &node.Arch, &node.CPUCores, &node.MemoryTotalMB, &node.DockerVersion, &node.HealthStatus, &node.DiscoveryStatus,
		&node.LastHeartbeatAt, &node.LastHeartbeatReportedAt, &node.LastHeartbeatSource, &node.LastCheckedAt, &node.LastError, &node.CreatedByUserID, &node.CreatedByUsername,
		&node.CreatedAt, &node.UpdatedAt, &node.DeletedAt,
	)
	if err != nil {
		return NodeModel.EdgeClient{}, err
	}
	return node, nil
}
