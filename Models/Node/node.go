package Node

// ControlNode 保存 Edge 节点的中心侧索引。
//
// 节点真实 Docker 操作仍由 Edge 完成；Server 只保存接入信息、设备能力和健康摘要。
type ControlNode struct {
	ID              string `json:"id"`
	UserID          string `json:"userId"`
	DeviceSequence  int    `json:"deviceSequence"`
	Name            string `json:"name"`
	BaseURL         string `json:"baseUrl"`
	DockerAPIURL    string `json:"dockerApiUrl"`
	APIKeyHash      string `json:"-"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	CPUCores        int    `json:"cpuCores"`
	MemoryTotalMB   int64  `json:"memoryTotalMb"`
	DockerVersion   string `json:"dockerVersion"`
	HealthStatus    string `json:"healthStatus"`
	LastHeartbeatAt int64  `json:"lastHeartbeatAt"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

const (
	NodeArchAMD64   = "amd64"
	NodeArchARM64   = "arm64"
	NodeArchUnknown = "unknown"

	NodeHealthHealthy   = "healthy"
	NodeHealthUnhealthy = "unhealthy"
	NodeHealthOffline   = "offline"
)
