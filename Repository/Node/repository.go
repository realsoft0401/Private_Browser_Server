package Node

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
func (Repository) Create(ctx context.Context, node *NodeModel.ControlNode) error {
	_, err := Rom.DB().ExecContext(ctx, `INSERT INTO edge_clients (
		id, main_account_id, client_sequence, name, base_url, client_ip, docker_api_url,
		os, arch, cpu_cores, memory_total_mb, docker_version, health_status, discovery_status,
		last_heartbeat_at, last_checked_at, last_error, created_by_user_id, created_by_username,
		created_at, updated_at, deleted_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		node.ID, node.MainAccountID, node.NodeSequence, node.Name, node.BaseURL, node.ClientIP, node.DockerAPIURL,
		node.OS, node.Arch, node.CPUCores, node.MemoryTotalMB, node.DockerVersion, node.HealthStatus, node.DiscoveryStatus,
		node.LastHeartbeatAt, node.LastCheckedAt, node.LastError, node.CreatedByUserID, node.CreatedByUsername,
		node.CreatedAt, node.UpdatedAt, node.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert edge_clients failed: %w", err)
	}
	return nil
}

// ListByMainAccount 查询主账号下未删除的 Edge Client。
func (Repository) ListByMainAccount(ctx context.Context, mainAccountID string) ([]NodeModel.ControlNode, error) {
	rows, err := Rom.DB().QueryContext(ctx, `SELECT
		id, main_account_id, client_sequence, name, base_url, client_ip, docker_api_url,
		os, arch, cpu_cores, memory_total_mb, docker_version, health_status, discovery_status,
		last_heartbeat_at, last_checked_at, last_error, created_by_user_id, created_by_username,
		created_at, updated_at, deleted_at
		FROM edge_clients
		WHERE main_account_id = ? AND deleted_at = 0
		ORDER BY created_at DESC`, mainAccountID)
	if err != nil {
		return nil, fmt.Errorf("query edge_clients failed: %w", err)
	}
	defer rows.Close()

	nodes := make([]NodeModel.ControlNode, 0)
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
func (Repository) GetByID(ctx context.Context, mainAccountID string, id string) (*NodeModel.ControlNode, error) {
	row := Rom.DB().QueryRowContext(ctx, `SELECT
		id, main_account_id, client_sequence, name, base_url, client_ip, docker_api_url,
		os, arch, cpu_cores, memory_total_mb, docker_version, health_status, discovery_status,
		last_heartbeat_at, last_checked_at, last_error, created_by_user_id, created_by_username,
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
func (Repository) UpdateDeviceInfo(ctx context.Context, node *NodeModel.ControlNode) error {
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

func scanNode(row rowScanner) (NodeModel.ControlNode, error) {
	var node NodeModel.ControlNode
	err := row.Scan(
		&node.ID, &node.MainAccountID, &node.NodeSequence, &node.Name, &node.BaseURL, &node.ClientIP, &node.DockerAPIURL,
		&node.OS, &node.Arch, &node.CPUCores, &node.MemoryTotalMB, &node.DockerVersion, &node.HealthStatus, &node.DiscoveryStatus,
		&node.LastHeartbeatAt, &node.LastCheckedAt, &node.LastError, &node.CreatedByUserID, &node.CreatedByUsername,
		&node.CreatedAt, &node.UpdatedAt, &node.DeletedAt,
	)
	if err != nil {
		return NodeModel.ControlNode{}, err
	}
	return node, nil
}
