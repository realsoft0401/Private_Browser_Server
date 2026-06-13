package Node

import (
	"fmt"
	"strings"

	NodeModel "private_browser_server/Models/Node"
)

// applyDiscoveryObservation 把一次 discovery/heartbeat 观测映射到节点身份状态。
//
// 当前先落最小闭环：
//   - 每次成功匹配已登记节点时，都刷新 lastDiscoveredAt；
//   - 如果登记地址与当前观测地址明显不一致，则标记 blocked + ip_mismatch；
//   - discovery/heartbeat 只负责记录观测事实，不能顺手补正式 client_ip，
//     否则会把“先发现 IP 变化、再判断 mismatch”的链路悄悄抹平；
//   - 不在这里自动恢复 verified，后续恢复动作必须通过更受控的人工确认流程。
func applyDiscoveryObservation(node *NodeModel.EdgeClient, observedBaseURL, observedClientIP, sourceIP string, observedAt, reportedAt int64, source string) {
	if node == nil {
		return
	}
	node.LastDiscoveredAt = observedAt
	node.LastHeartbeatAt = observedAt
	node.LastHeartbeatReportedAt = reportedAt
	node.LastHeartbeatSource = strings.TrimSpace(source)
	node.UpdatedAt = observedAt

	if !hasDiscoveryAddressMismatch(node, observedBaseURL, observedClientIP, sourceIP) {
		return
	}
	node.DiscoveryStatus = NodeModel.NodeDiscoveryBlocked
	node.DiscoveryReason = NodeModel.NodeDiscoveryReasonIPMismatch
	node.LastError = buildDiscoveryMismatchMessage(node, observedBaseURL, observedClientIP, sourceIP)
}

// ApplyDiscoveryObservationForExternal 暴露给 discovery 包调用，避免把整套 Node HTTP 逻辑带过去。
//
// 这个导出包装器只转发到内部 helper，方便 UDP discovery 在不引入循环依赖的前提下复用同一套最小身份判断。
func ApplyDiscoveryObservationForExternal(node *NodeModel.EdgeClient, observedBaseURL, observedClientIP, sourceIP string, observedAt, reportedAt int64, source string) {
	applyDiscoveryObservation(node, observedBaseURL, observedClientIP, sourceIP, observedAt, reportedAt, source)
}

func hasDiscoveryAddressMismatch(node *NodeModel.EdgeClient, observedBaseURL, observedClientIP, sourceIP string) bool {
	if node == nil {
		return false
	}
	registeredBaseURL := normalizeLookupURL(node.BaseURL)
	observedBaseURL = normalizeLookupURL(observedBaseURL)
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

func buildDiscoveryMismatchMessage(node *NodeModel.EdgeClient, observedBaseURL, observedClientIP, sourceIP string) string {
	return fmt.Sprintf(
		"discovery 地址与登记节点不一致，已标记 blocked，discoveryReason=ip_mismatch；registeredBaseUrl=%s registeredClientIp=%s observedBaseUrl=%s observedClientIp=%s sourceIp=%s；请管理员确认是否为同一台 Client 的 IP 变化，确认前禁止继续放行业务动作",
		strings.TrimSpace(node.BaseURL),
		strings.TrimSpace(node.ClientIP),
		normalizeLookupURL(observedBaseURL),
		strings.TrimSpace(observedClientIP),
		strings.TrimSpace(sourceIP),
	)
}
