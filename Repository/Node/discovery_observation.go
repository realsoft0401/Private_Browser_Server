package Node

import (
	"context"
	"fmt"
	"strings"

	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Rom"
)

// UpdateObservedDiscovery 保存一次 discovery/heartbeat 观测结果。
//
// 设计来源：
// - 新一轮节点身份状态机需要把“最近一次被发现”与“最近一次收到心跳”一起保存；
// - 同时 IP 不一致等情况不能再只写临时日志，而要沉淀到 discoveryStatus/discoveryReason；
// - 但这一层仍只负责落库，不负责判断 mismatch 规则，具体规则由 Service 层决定。
//
// 职责边界：
// - 允许更新 lastDiscoveredAt、lastHeartbeatAt、discoveryStatus、discoveryReason、lastError；
// - 不更新 baseUrl/clientIp，避免在底层把节点地址悄悄改掉；
// - 不自动把节点恢复成 verified，只有上层明确赋值才会落库。
func (Repository) UpdateObservedDiscovery(ctx context.Context, node *NodeModel.EdgeClient) error {
	if node == nil {
		return nil
	}
	source := strings.TrimSpace(node.LastHeartbeatSource)
	result, err := Rom.DB().ExecContext(ctx, `UPDATE edge_clients SET
		discovery_status = ?, discovery_reason = ?, last_discovered_at = ?,
		last_heartbeat_at = ?, last_heartbeat_reported_at = ?, last_heartbeat_source = ?,
		last_error = ?, updated_at = ?
		WHERE id = ? AND main_account_id = ? AND deleted_at = 0`,
		node.DiscoveryStatus, node.DiscoveryReason, node.LastDiscoveredAt,
		node.LastHeartbeatAt, node.LastHeartbeatReportedAt, source,
		node.LastError, node.UpdatedAt,
		node.ID, node.MainAccountID,
	)
	if err != nil {
		return fmt.Errorf("update edge_clients discovery observation failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read update edge_clients discovery rows affected failed: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
