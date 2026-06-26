package Node

// RecheckRequest 描述一次管理员发起的“会话校验”请求。
//
// 设计来源：
// - 这次已经明确把节点治理里的 `recheck` 收口成“会话校验”；
// - 第一版只需要一个最小来源字段做审计留痕，不把地址更新、账号变更等治理动作混进来；
// - 因此请求体保持极简，避免后续实现时把它误扩成 bind 或 confirm-address-update。
type RecheckRequest struct {
	Source string `json:"source"`
}

// RecheckResponse 描述一次会话校验同步收口后的中心结论。
//
// 职责边界：
// - 这里只表达“这次会话校验完成后，Node Server 对当前节点的最新判断”；
// - 不返回 Client 原始 `/health` 明细，不返回 device-info 全文，避免把中心摘要接口膨胀成调试接口；
// - 更细排障信息继续走日志、Client 受控诊断接口或后续管理员页面。
type RecheckResponse struct {
	ClientID        string `json:"clientId"`
	Status          string `json:"status"`
	HealthStatus    string `json:"healthStatus"`
	DiscoveryStatus string `json:"discoveryStatus"`
	DiscoveryReason string `json:"discoveryReason"`
	CheckedAt       int64  `json:"checkedAt"`
}
