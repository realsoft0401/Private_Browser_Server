package BrowserEnv

// RevalidateTaskAcceptedResponse 返回中心 browser-env revalidate 的接单结果。
//
// 设计来源：
// - revalidate 是异常环境包的受控重新校验，不是普通同步查询；
// - Edge 侧会经历加载索引、校验状态、校验原子材料、回写索引等阶段；
// - 因此中心层必须以 task + SSE 暴露，避免调用方把“接单成功”误读为“环境已经修复”。
type RevalidateTaskAcceptedResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
}
