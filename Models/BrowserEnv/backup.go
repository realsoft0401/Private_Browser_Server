package BrowserEnv

// BackupTaskAcceptedResponse 返回中心 browser-env backup 的接单结果。
//
// 设计来源：
// - backup 在中心层被定义为长链路任务，立即返回的不是最终备份结果，而是中心 task 入口；
// - 返回结构与 run/restore 一致，便于前端和排障工具统一处理任务型生命周期动作；
// - 这里单独建模而不是直接复用 run 命名，是为了避免后续维护时误把 backup 当成 run 的变体。
type BackupTaskAcceptedResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
}
