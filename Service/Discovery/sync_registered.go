package Discovery

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	NodeModel "private_browser_server/Models/Node"
	NodeRepo "private_browser_server/Repository/Node"
)

// syncRegisteredHeartbeat 在 UDP 收包时回写已注册节点的 discovery/heartbeat 摘要。
//
// 这里保持最小职责：
// - 只处理已经能匹配到的正式节点；
// - 每次命中都刷新 lastDiscoveredAt 和 lastHeartbeatAt；
// - 如果发现到的地址与登记节点不一致，则标记 blocked + ip_mismatch。
func syncRegisteredHeartbeat(sourceIP string, payload BeaconPayload, fallbackAt int64) {
	receivedAt := fallbackAt
	reportedAt := payload.LastHeartbeatAt
	if reportedAt <= 0 {
		reportedAt = fallbackAt
	}

	repo := NodeRepo.Repository{}
	node, err := repo.GetByHeartbeatLookup(context.Background(), payload.BaseURL, payload.ClientIP, sourceIP)
	if errors.Is(err, NodeRepo.ErrNotFound) {
		return
	}
	if err != nil {
		log.Printf("lookup udp discovery heartbeat failed, sourceIp=%s, baseUrl=%s, err=%v\n", sourceIP, payload.BaseURL, err)
		return
	}

	applyObservedDiscovery(node, payload.BaseURL, payload.ClientIP, sourceIP, receivedAt, reportedAt)
	if err = repo.UpdateObservedDiscovery(context.Background(), node); err != nil {
		log.Printf("udp discovery heartbeat sync failed, sourceIp=%s, baseUrl=%s, err=%v\n", sourceIP, payload.BaseURL, err)
	}
}

func applyObservedDiscovery(node *NodeModel.EdgeClient, observedBaseURL, observedClientIP, sourceIP string, observedAt, reportedAt int64) {
	if node == nil {
		return
	}
	node.LastDiscoveredAt = observedAt
	node.LastHeartbeatAt = observedAt
	node.LastHeartbeatReportedAt = reportedAt
	node.LastHeartbeatSource = "udp"
	node.UpdatedAt = observedAt
	if !hasAddressMismatch(node, observedBaseURL, observedClientIP, sourceIP) {
		return
	}
	node.DiscoveryStatus = NodeModel.NodeDiscoveryBlocked
	node.DiscoveryReason = NodeModel.NodeDiscoveryReasonIPMismatch
	node.LastError = fmt.Sprintf(
		"discovery 地址与登记节点不一致，已标记 blocked，discoveryReason=ip_mismatch；registeredBaseUrl=%s registeredClientIp=%s observedBaseUrl=%s observedClientIp=%s sourceIp=%s；请管理员确认是否为同一台 Client 的 IP 变化，确认前禁止继续放行业务动作",
		strings.TrimSpace(node.BaseURL),
		strings.TrimSpace(node.ClientIP),
		normalizeObservedURL(observedBaseURL),
		strings.TrimSpace(observedClientIP),
		strings.TrimSpace(sourceIP),
	)
}

func hasAddressMismatch(node *NodeModel.EdgeClient, observedBaseURL, observedClientIP, sourceIP string) bool {
	if node == nil {
		return false
	}
	registeredBaseURL := normalizeObservedURL(node.BaseURL)
	observedBaseURL = normalizeObservedURL(observedBaseURL)
	if registeredBaseURL != "" && observedBaseURL != "" && registeredBaseURL != observedBaseURL {
		return true
	}

	registeredClientIP := strings.TrimSpace(node.ClientIP)
	observedClientIP = strings.TrimSpace(observedClientIP)
	sourceIP = strings.TrimSpace(sourceIP)
	if registeredClientIP == "" {
		return false
	}
	if observedClientIP != "" && observedClientIP == registeredClientIP {
		return false
	}
	if sourceIP != "" && sourceIP == registeredClientIP {
		return false
	}
	return (observedClientIP != "" || sourceIP != "") && observedClientIP != registeredClientIP && sourceIP != registeredClientIP
}

func normalizeObservedURL(raw string) string {
	value := strings.TrimRight(strings.TrimSpace(raw), "/")
	return value
}
