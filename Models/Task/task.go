package Task

const StatusPending = "pending"
const StatusRunning = "running"
const StatusSuccess = "success"
const StatusFailed = "failed"

const EventProgress = "task.progress"
const EventCompleted = "task.completed"
const EventFailed = "task.failed"

// ServerTask 是平台级持久任务模型。
//
// 设计来源：
// - 你已经拍板：Client task 只做边缘短期观察，Server task 才是平台长期事实；
// - 因此这里必须把账号、节点、环境、动作类型和最终错误都绑在一起；
// - 后续即使 Edge task 丢失，中心也要靠这张表继续审计和收口。
type ServerTask struct {
	ID               string `json:"id"`
	MainAccountID    string `json:"accountId"`
	OperatorUserID   string `json:"operatorUserId"`
	OperatorUsername string `json:"operatorUsername"`
	ClientID         string `json:"clientId"`
	EnvID            string `json:"envId"`
	TaskType         string `json:"taskType"`
	ResourceType     string `json:"resourceType"`
	ResourceID       string `json:"resourceId"`
	Status           string `json:"status"`
	EdgeTaskID       string `json:"edgeTaskId"`
	EventsURL        string `json:"eventsUrl"`
	ErrorMessage     string `json:"errorMessage"`
	Suggestion       string `json:"suggestion"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
	FinishedAt       int64  `json:"finishedAt"`
}

// Event 是 Node Server 统一 SSE 事件模型。
type Event struct {
	Event        string `json:"event"`
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	ClientID     string `json:"clientId,omitempty"`
	EnvID        string `json:"envId,omitempty"`
	SlotID       string `json:"slotId,omitempty"`
	Stage        string `json:"stage"`
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
	Error        string `json:"error,omitempty"`
	Suggestion   string `json:"suggestion,omitempty"`
	Timestamp    string `json:"timestamp"`
}

type DetailResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	Status       string `json:"status"`
	CurrentStage string `json:"currentStage"`
	Message      string `json:"message"`
	EventsURL    string `json:"eventsUrl"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	FinishedAt   string `json:"finishedAt,omitempty"`
	Error        string `json:"error,omitempty"`
	Suggestion   string `json:"suggestion,omitempty"`
}

func (e Event) GetEvent() string {
	return e.Event
}
