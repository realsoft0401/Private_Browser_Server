package Node

import (
	"net"
	"net/url"
	"strings"

	NodeModel "private_browser_server/Models/Node"
)

// fillNodeClientIPIfMissing 只在当前节点记录缺少正式 client_ip 时尝试补齐。
//
// 设计来源：
//   - 这轮联调确认，很多老节点记录虽然 baseUrl 已存在，但 client_ip 仍为空，
//     这会让后续 ip_mismatch 只能依赖 baseUrl，判断力不够稳定；
//   - 但用户已经明确，节点地址不能被底层流程悄悄覆盖，所以这里采取最保守策略：
//     只补齐空值，不自动改写已有 client_ip。
//
// 职责边界：
// - 优先使用显式传入的 candidate，例如注册请求、heartbeat/discovery 里的 clientIp；
// - candidate 不可用时，再尝试从 baseUrl 解析出 IP 字面量；
// - 只在 node.ClientIP 为空时生效，已有值一律保持不变。
func fillNodeClientIPIfMissing(node *NodeModel.EdgeClient, candidate string) {
	if node == nil || strings.TrimSpace(node.ClientIP) != "" {
		return
	}
	if ip := normalizeIPAddress(candidate); ip != "" {
		node.ClientIP = ip
		return
	}
	node.ClientIP = extractIPFromBaseURL(node.BaseURL)
}

func normalizeIPAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed := net.ParseIP(value)
	if parsed == nil {
		return ""
	}
	return parsed.String()
}

func extractIPFromBaseURL(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := parsed.Hostname()
	return normalizeIPAddress(host)
}
