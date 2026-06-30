package BrowserEnv

// StopRequest 是中心 browser-env stop 的最小正式请求体。
//
// 设计来源：
// - Client stop 正式协议已经收口为 `timeoutSeconds`，不接受 slotId 和 force；
// - Node Server 不应该再长出第二套 stop 参数体系，否则中心和边缘会很快漂移；
// - 因此这里严格复用 Client 的正式请求字段，只保留 Docker 优雅停止等待秒数。
type StopRequest struct {
	TimeoutSeconds int `json:"timeoutSeconds"`
}

// StopResponse 返回一次中心 browser-env stop 的同步结果。
//
// 职责边界：
// - 这是同步 stop 的最终 HTTP 结果，不是 task 接单结果；
// - 中心仍会在后台落一条 `server_task` 审计事实，但当前接口不要求调用方继续订阅 SSE；
// - 返回值只聚焦调用方最关心的停止后摘要。
type StopResponse struct {
	EnvID           string `json:"envId"`
	Status          string `json:"status"`
	ContainerStatus string `json:"containerStatus"`
	StoppedAt       int64  `json:"stoppedAt"`
}
