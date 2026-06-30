package BrowserEnv

// DeletePackageTaskAcceptedResponse 返回中心 browser-env package delete 的接单结果。
//
// 设计来源：
// - package delete 是彻底销毁当前节点环境资产的正式生命周期动作；
// - 它和 backup/restore 一样是长链路任务，不能用同步 HTTP 假装已经完成；
// - 删除成功后中心缓存会被移除，因此调用方拿到的 taskId 会成为后续最重要的审计入口。
type DeletePackageTaskAcceptedResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
}
