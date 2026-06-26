package BrowserEnv

// ServerBrowserEnv 是中心层 browser-env 聚合缓存模型。
//
// 职责边界：
// - 它只保存 Server 用来展示、调度、审计的聚合摘要；
// - 真正的环境包资产、profile、browser-data 仍然只在 Client 本机；
// - 因此这里允许保存状态、URL、任务关联和最近错误，但不允许保存 Edge 文件正文。
type ServerBrowserEnv struct {
	EnvID           string `json:"envId"`
	MainAccountID   string `json:"accountId"`
	ClientID        string `json:"clientId"`
	UserID          string `json:"userId"`
	RPAType         string `json:"rpaType"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	ContainerStatus string `json:"containerStatus"`
	RuntimeStatus   string `json:"runtimeStatus"`
	CurrentSlotID   string `json:"currentSlotId"`
	CDPURL          string `json:"cdpUrl"`
	WebVNCURL       string `json:"webVncUrl"`
	LastTaskID      string `json:"lastTaskId"`
	LastError       string `json:"lastError"`
	LastSyncedAt    int64  `json:"lastSyncedAt"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
	DeletedAt       int64  `json:"deletedAt"`
}
