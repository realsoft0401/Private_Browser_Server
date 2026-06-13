package Node

import (
	"strings"

	NodeModel "private_browser_server/Models/Node"
)

const (
	nodeActionVerify               = "verify"
	nodeActionRefreshDeviceInfo    = "refresh_device_info"
	nodeActionConfirmAddressUpdate = "confirm_address_update"
)

// attachNodeGovernanceActions 根据当前节点状态附加前端可直接消费的治理动作建议。
//
// 设计来源：
// - 当前代码库里还没有正式的节点管理前端，但用户已经要求把按钮口径和状态机收紧；
// - 如果继续只返回 blocked/verified，后续页面仍然容易把“blocked 的不同原因”误解成同一种按钮；
// - 因此这里由后端统一算出 primaryAction/allowedActions，让前端按字段渲染即可。
//
// 职责边界：
// - 这里只表达节点治理动作，不表达 env/run/stop 等业务动作；
// - 只输出已经正式落地的 API 动作；
// - 不把“查看详情”“人工排查”这类页面行为编码进返回值。
func attachNodeGovernanceActions(node *NodeModel.EdgeClient) {
	if node == nil {
		return
	}
	node.PrimaryAction = ""
	node.AllowedActions = nil

	reason := strings.TrimSpace(node.DiscoveryReason)
	switch {
	case node.DiscoveryStatus == NodeModel.NodeDiscoveryVerified:
		return
	case node.DiscoveryStatus == NodeModel.NodeDiscoveryBlocked && reason == NodeModel.NodeDiscoveryReasonNone:
		node.PrimaryAction = nodeActionVerify
		node.AllowedActions = []string{nodeActionVerify, nodeActionRefreshDeviceInfo}
	case node.DiscoveryStatus == NodeModel.NodeDiscoveryBlocked && reason == NodeModel.NodeDiscoveryReasonIPMismatch:
		node.PrimaryAction = nodeActionConfirmAddressUpdate
		node.AllowedActions = []string{nodeActionConfirmAddressUpdate}
	case node.DiscoveryStatus == NodeModel.NodeDiscoveryBlocked && reason == NodeModel.NodeDiscoveryReasonDeviceFactChanged:
		return
	default:
		if node.DiscoveryStatus == NodeModel.NodeDiscoveryBlocked {
			node.AllowedActions = []string{nodeActionRefreshDeviceInfo}
		}
	}
}

func attachNodeGovernanceActionsList(nodes []NodeModel.EdgeClient) {
	for i := range nodes {
		attachNodeGovernanceActions(&nodes[i])
	}
}
