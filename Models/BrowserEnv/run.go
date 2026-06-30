package BrowserEnv

// RunRequest 是中心 browser-env run 的最小正式请求体。
//
// 设计来源：
// - Client 正式 run 协议已经收口为 `slotId + forceRecreate`；
// - Node Server 当前阶段先做显式 slot 调用，不做自动选 slot，避免调度规则未完全定型时把猜测写死；
// - 因此这里严格复用这两个字段，不再增加 image / proxy / fingerprint 等旁路覆盖参数。
type RunRequest struct {
	SlotID        string `json:"slotId"`
	ForceRecreate bool   `json:"forceRecreate"`
}

// RunTaskAcceptedResponse 返回中心 run 任务接单结果。
type RunTaskAcceptedResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
}
