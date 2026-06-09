package Env

// ServerBrowserEnv 是中心侧环境包聚合索引。
//
// 它只保存状态摘要和连接入口，不保存 profile、proxy 明文、fingerprint raw 或 browser-data。
type ServerBrowserEnv struct {
	EnvID           string `json:"envId"`
	UserID          string `json:"userId"`
	NodeID          string `json:"nodeId"`
	RPAType         string `json:"rpaType"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	ContainerStatus string `json:"containerStatus"`
	MonitorStatus   string `json:"monitorStatus"`
	CDPURL          string `json:"cdpUrl"`
	WebVNCURL       string `json:"webVncUrl"`
	LastTaskID      string `json:"lastTaskId"`
	LastError       string `json:"lastError"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
	DeletedAt       int64  `json:"deletedAt,omitempty"`
}

const (
	EnvStatusCreated  = "created"
	EnvStatusRunning  = "running"
	EnvStatusStopped  = "stopped"
	EnvStatusBackedUp = "backed_up"
	EnvStatusDeleted  = "deleted"
	EnvStatusArchived = "archived"
	EnvStatusError    = "error"
)
