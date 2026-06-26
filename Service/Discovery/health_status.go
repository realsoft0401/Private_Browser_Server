package Discovery

import "strings"

// NormalizeNodeHealthStatus 统一把 Node 侧在线状态收口成 `healthy / offline` 两态。
//
// 设计来源：
// - 这次已经明确废弃 `stale`、`unhealthy` 等中心在线中间态；
// - 当前这条状态只表达“Node 眼里这台 Client 现在在线还是离线”，不再承载本机细分检查结果；
// - 因此只要 HTTP probe/heartbeat 已经成功到达 Client，就视为 `healthy`；真正更细的本机检查细节继续留在 Client `/health` 明细里。
func NormalizeNodeHealthStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "offline":
		return "offline"
	default:
		return "healthy"
	}
}
