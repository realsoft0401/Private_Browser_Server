package Node

// ControlNode 保存 Edge Client 的中心侧索引。
//
// 设计背景：用户确认三层服务统一把 Node Server 分配给 Client 的中心身份叫 clientId。
// SQLite 表名同步收敛为 edge_clients，避免再出现 node/control_node/client 多套叫法。
// 对外 API、Swagger 和业务文档统一暴露 clientId，避免再出现多套中心身份叫法。
// Client 真实 Docker 操作仍由 Edge 完成；Server 只保存接入信息、设备能力和健康摘要。
type ControlNode struct {
	ID                string `json:"clientId"`
	MainAccountID     string `json:"mainAccountId"`
	NodeSequence      int    `json:"clientSequence"`
	Name              string `json:"name"`
	BaseURL           string `json:"baseUrl"`
	ClientIP          string `json:"clientIp"`
	DockerAPIURL      string `json:"dockerApiUrl"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	CPUCores          int    `json:"cpuCores"`
	MemoryTotalMB     int64  `json:"memoryTotalMb"`
	DockerVersion     string `json:"dockerVersion"`
	HealthStatus      string `json:"healthStatus"`
	DiscoveryStatus   string `json:"discoveryStatus"`
	LastHeartbeatAt   int64  `json:"lastHeartbeatAt"`
	LastCheckedAt     int64  `json:"lastCheckedAt"`
	LastError         string `json:"lastError"`
	CreatedByUserID   string `json:"createdByUserId"`
	CreatedByUsername string `json:"createdByUsername"`
	CreatedAt         int64  `json:"createdAt"`
	UpdatedAt         int64  `json:"updatedAt"`
	DeletedAt         int64  `json:"deletedAt,omitempty"`
}

const (
	NodeArchAMD64   = "amd64"
	NodeArchARM64   = "arm64"
	NodeArchUnknown = "unknown"

	NodeHealthHealthy   = "healthy"
	NodeHealthUnhealthy = "unhealthy"
	NodeHealthOffline   = "offline"
	NodeHealthStale     = "stale"

	NodeDiscoveryManual   = "manual"
	NodeDiscoveryVerified = "verified"
)
