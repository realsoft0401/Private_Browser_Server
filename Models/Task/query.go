package Task

// ListQuery 是中心任务列表的过滤条件。
//
// 设计来源：
// - Server task 是平台级长期任务事实，管理员需要按节点、环境、任务类型和终态回看；
// - 列表接口只读 `server_tasks`，不补发 SSE、不访问 Edge、不触发生命周期动作；
// - page/pageSize 用于限制单次读取规模，避免任务表增长后 Swagger 或管理端一次性拉爆。
type ListQuery struct {
	ClientID   string
	EnvID      string
	ResourceID string
	TaskType   string
	Status     string
	Page       int
	PageSize   int
}

// ListResponse 是中心任务列表响应。
type ListResponse struct {
	Items    []DetailResponse `json:"items"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
}
