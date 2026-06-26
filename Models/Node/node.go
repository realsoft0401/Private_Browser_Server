package Node

// EdgeClient 是 Server 中心侧的正式节点模型。
//
// 设计来源：
// - 当前已经拍板：discovered 只是过程视图，不单独落正式表；
// - 因此这里必须只表达“中心正式节点身份 + 状态摘要”，不能混入 Edge 资产内容；
// - 后续 verify/health/quota/browser-env 聚合都围绕这个模型扩，但不要把 slot 真相、profile、proxy 明文塞进来。
type EdgeClient struct {
	ClientID                string `json:"clientId"`
	MainAccountID           string `json:"accountId"`
	ClientSequence          int64  `json:"clientSequence"`
	Name                    string `json:"name"`
	ClientIP                string `json:"clientIp"`
	BaseURL                 string `json:"baseUrl"`
	DockerAPIURL            string `json:"dockerApiUrl"`
	OS                      string `json:"os"`
	Arch                    string `json:"arch"`
	CPUCores                int64  `json:"cpuCores"`
	MemoryTotalMB           int64  `json:"memoryTotalMb"`
	DockerVersion           string `json:"dockerVersion"`
	HealthStatus            string `json:"healthStatus"`
	DiscoveryStatus         string `json:"discoveryStatus"`
	DiscoveryReason         string `json:"discoveryReason"`
	PushStatus              string `json:"pushStatus"`
	LastDiscoveredAt        int64  `json:"lastDiscoveredAt"`
	LastHeartbeatAt         int64  `json:"lastHeartbeatAt"`
	LastHeartbeatReportedAt int64  `json:"lastHeartbeatReportedAt"`
	LastHeartbeatSource     string `json:"lastHeartbeatSource"`
	LastCheckedAt           int64  `json:"lastCheckedAt"`
	LastError               string `json:"lastError"`
	CreatedByUserID         string `json:"createdByUserId"`
	CreatedByUsername       string `json:"createdByUsername"`
	CreatedAt               int64  `json:"createdAt"`
	UpdatedAt               int64  `json:"updatedAt"`
	DeletedAt               int64  `json:"deletedAt"`
}
