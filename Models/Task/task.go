package Task

import "encoding/json"

// ServerTask 保存 Node Server 的平台级任务事实。
//
// 设计来源：
// - Edge Client 的 task 只是进程内短期观察通道，服务重启后可能丢失；
// - Node Server 需要一条可审计、可查询的中心事实，把主账号、操作者、Client、envId 和 Edge taskId 串起来；
// - 当前阶段先围绕 run/stop/backup/restore 任务落库，后续 pull-image/RPA 继续复用同一模型。
//
// 职责边界：
// - 负责保存中心 taskId、归属、目标资源、Edge task 绑定和最终 success/failed 结论；
// - 不保存代理明文、fingerprint raw、browser-data 或任意 Docker 参数透传；
// - 结果详情只保留可展示摘要，真实运行态仍以 Edge 环境包事实和 Edge task 结果为准。
type ServerTask struct {
	TaskID string `json:"taskId"`
	// MainAccountID 是平台侧主账号归属。
	//
	// 即使操作者是子账号，历史任务也必须围绕主账号资产查询。
	MainAccountID string `json:"mainAccountId,omitempty"`
	// Operator* 保存触发动作的平台操作人摘要。
	//
	// V1 demo 先通过 Platform Header 注入，不在 Node Server 本地做账号鉴权。
	OperatorUserID   string `json:"operatorUserId,omitempty"`
	OperatorUsername string `json:"operatorUsername,omitempty"`
	// ClientID 是任务目标 Edge Client 的中心身份。
	//
	// 底层字段名和 JSON 都统一为 clientId，避免维护时把 Node Server 与 Edge Client 混淆。
	ClientID     string `json:"clientId"`
	EnvID        string `json:"envId"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	EdgeTaskID   string `json:"edgeTaskId"`
	EventsURL    string `json:"eventsUrl"`
	ErrorMessage string `json:"errorMessage"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
	FinishedAt   int64  `json:"finishedAt,omitempty"`
}

// EdgeTaskSnapshot 是 Node Server 查询到的当前 Edge 任务摘要。
//
// 这份数据只做观察用途，不替代 ServerTask 的最终平台结论。
type EdgeTaskSnapshot struct {
	TaskID     string          `json:"taskId"`
	TaskType   string          `json:"taskType"`
	Status     string          `json:"status"`
	Message    string          `json:"message"`
	LastError  string          `json:"lastError,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	CreatedAt  int64           `json:"createdAt"`
	UpdatedAt  int64           `json:"updatedAt"`
	FinishedAt *int64          `json:"finishedAt,omitempty"`
}

// ServerTaskEvent 是 Node Server 对前端输出的一条中心任务事件。
//
// 设计来源：
// - 用户要求 `/api/v1/envs/{envId}/run` 不仅返回 taskId，还要能通过 SSE 看到镜像预检、拉镜像、Edge run 等全过程；
// - 仅透传 Edge 原始 SSE 无法覆盖 Node Server 自己的前置阶段，因此需要一条中心层事件结构；
// - 字段命名尽量贴近 Edge task event，降低前端同时接入两层任务时的解析差异。
//
// 职责边界：
// - 只保存可展示的阶段、状态、摘要消息和少量数据；
// - 不在事件里放代理明文、fingerprint raw、browser-data 路径内容等敏感信息；
// - 事件是实时观察通道，不替代 ServerTask 的持久最终事实。
type ServerTaskEvent struct {
	TaskID    string         `json:"taskId"`
	Event     string         `json:"event"`
	Status    string         `json:"status"`
	Stage     string         `json:"stage"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"data,omitempty"`
	CreatedAt int64          `json:"createdAt"`
}

// StartTaskResponse 是 Node Server 对前端暴露的统一任务启动摘要。
//
// 所有长动作都应先返回这份摘要，再通过 task detail 或 SSE 观察最终结果。
type StartTaskResponse struct {
	TaskID     string `json:"taskId"`
	TaskType   string `json:"taskType"`
	Status     string `json:"status"`
	ClientID   string `json:"clientId"`
	EnvID      string `json:"envId"`
	EdgeTaskID string `json:"edgeTaskId,omitempty"`
	EventsURL  string `json:"eventsUrl"`
	Message    string `json:"message"`
	CreatedAt  int64  `json:"createdAt"`
}

// TaskDetailResponse 是单个中心任务详情。
//
// task 是平台持久事实；edge 只是当前可读到的边缘执行快照。
type TaskDetailResponse struct {
	Task *ServerTask       `json:"task"`
	Edge *EdgeTaskSnapshot `json:"edge,omitempty"`
}

// ListTaskQuery 是任务列表过滤条件。
//
// 当前主要服务调试和链路联调，保留最小过滤条件：clientId/envId/type/status。
type ListTaskQuery struct {
	ClientID string
	EnvID    string
	Type     string
	Status   string
	Page     int
	PageSize int
}

// ListTasksResponse 是任务列表响应。
type ListTasksResponse struct {
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Items    []ServerTask `json:"items"`
}

const (
	TaskTypeCreateEnv        = "create_env"
	TaskTypeRunEnv           = "run_env"
	TaskTypeStopEnv          = "stop_env"
	TaskTypePullImage        = "pull_image"
	TaskTypeBackupEnv        = "backup_env"
	TaskTypeRestoreEnv       = "restore_env"
	TaskTypeRevalidateEnv    = "revalidate_env"
	TaskTypeImportEnvPackage = "import_env_package"
	TaskTypeDeleteEnvPackage = "delete_env_package"

	TaskStatusPending  = "pending"
	TaskStatusRunning  = "running"
	TaskStatusSuccess  = "success"
	TaskStatusFailed   = "failed"
	TaskStatusCanceled = "canceled"
)
