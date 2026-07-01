package BrowserEnv

// ImportPackageTaskAcceptedResponse 返回中心 import-package 的接单结果。
//
// import-package 是上传、解压、校验、写入 SQLite 的长链路动作，所以中心层必须返回 taskId/eventsUrl，
// 调用方不能把接单成功理解成环境包已经可用。
type ImportPackageTaskAcceptedResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	ClientID     string `json:"clientId"`
	EventsURL    string `json:"eventsUrl"`
}
