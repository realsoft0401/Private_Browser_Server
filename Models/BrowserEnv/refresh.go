package BrowserEnv

// RefreshResponse 返回一次中心 env 刷新后的最新缓存摘要。
//
// 职责边界：
// - 这是 Node Server 当前缓存视图，不是 Edge 资产原文；
// - 它只回答“这条 env 中心现在看到的摘要是什么”；
// - 后续如果需要完整排障详情，仍然应查看 Edge detail 或单独诊断接口。
type RefreshResponse struct {
	EnvID           string `json:"envId"`
	ClientID        string `json:"clientId"`
	Status          string `json:"status"`
	RuntimeStatus   string `json:"runtimeStatus"`
	ContainerStatus string `json:"containerStatus"`
	CurrentSlotID   string `json:"currentSlotId"`
	WebVNCURL       string `json:"webVncUrl"`
	LastTaskID      string `json:"lastTaskId"`
	LastError       string `json:"lastError"`
	LastSyncedAt    int64  `json:"lastSyncedAt"`
}
