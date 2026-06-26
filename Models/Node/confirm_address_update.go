package Node

// ConfirmAddressUpdateRequest 描述一次管理员发起的地址确认更新请求。
//
// 设计来源：
// - 会话校验只负责发现 `ip_mismatch`，不自动改地址；
// - 因此地址更新必须走单独治理动作，并显式带上管理员确认后的新地址；
// - 第一版只收最小必要字段，不把账号、配额、browser-env 等无关信息混进来。
type ConfirmAddressUpdateRequest struct {
	ClientIP string `json:"clientIp"`
	BaseURL  string `json:"baseUrl"`
	Source   string `json:"source"`
}

// ConfirmAddressUpdateResponse 描述一次地址确认更新同步收口后的中心结论。
//
// 职责边界：
// - 这里只表达“地址更新前后发生了什么，以及 Node Server 最终怎么判断这台节点”；
// - 不返回 Client 原始 health/device-info 详情，避免节点治理接口膨胀成调试接口；
// - 更细排障仍然走结构化日志和后续诊断接口。
type ConfirmAddressUpdateResponse struct {
	ClientID        string `json:"clientId"`
	OldClientIP     string `json:"oldClientIp"`
	NewClientIP     string `json:"newClientIp"`
	OldBaseURL      string `json:"oldBaseUrl"`
	NewBaseURL      string `json:"newBaseUrl"`
	HealthStatus    string `json:"healthStatus"`
	DiscoveryStatus string `json:"discoveryStatus"`
	DiscoveryReason string `json:"discoveryReason"`
	UpdatedAt       int64  `json:"updatedAt"`
}
