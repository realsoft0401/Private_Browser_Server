package Task

// ServerTask 保存中心任务与 Edge 任务的绑定关系。
//
// Server 通过它把前端可见 taskId、目标 Edge Client、目标环境包和 Edge taskId 串起来；
// 具体 Docker/容器动作仍然由 Edge 执行。
type ServerTask struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	// ClientID 是任务目标 Client 的中心身份。
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

const (
	TaskTypeCreateEnv = "create_env"
	TaskTypeRunEnv    = "run_env"
	TaskTypeStopEnv   = "stop_env"
	TaskTypePullImage = "pull_image"
	TaskTypeBackupEnv = "backup_env"

	TaskStatusPending  = "pending"
	TaskStatusRunning  = "running"
	TaskStatusSuccess  = "success"
	TaskStatusFailed   = "failed"
	TaskStatusCanceled = "canceled"
)
