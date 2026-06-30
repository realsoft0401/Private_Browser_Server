package BrowserEnv

// RestoreTaskAcceptedResponse 返回中心 browser-env restore 的接单结果。
//
// 职责边界：
// - 这只是中心 restore 任务接单成功，不是 restore 已经最终成功；
// - 调用方必须继续订阅中心 `/api/v1/server-tasks/{taskId}/events`；
// - restore 的目录恢复、备份包删除和索引回写终态，都由后续任务阶段决定。
type RestoreTaskAcceptedResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
}
