package Node

import (
	"fmt"
	"strings"

	NodeModel "private_browser_server/Models/Node"
)

// discoveryIdentityBlock 返回节点是否因身份连续性问题而必须阻断 verify/业务放行。
//
// 设计来源：
// - 当前收口后，正式节点只有 blocked / verified 两种身份结论；
// - 但 blocked 本身还分“尚未 verify”和“身份连续性异常待确认”两类；
// - 因此这里不直接看 blocked，而是看 discoveryReason 是否已给出明确异常原因；
// - 因此需要一个纯函数收口这套判断，避免 verify 和业务放行各自散写不同文案。
func discoveryIdentityBlock(node *NodeModel.EdgeClient) (bool, string, string) {
	if node == nil {
		return false, "", ""
	}
	reason := strings.TrimSpace(node.DiscoveryReason)
	if reason == "" {
		return false, "", ""
	}
	message := fmt.Sprintf("Client discoveryStatus=%s，节点身份连续性待人工确认", node.DiscoveryStatus)
	message = fmt.Sprintf("%s，discoveryReason=%s", message, reason)
	nextAction := "请先由管理员确认当前 clientIp/baseUrl 和设备事实是否仍属于原 clientId，确认前不要继续 verify 或下发生命周期动作"
	return true, message, nextAction
}
