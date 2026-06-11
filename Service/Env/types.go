package Env

import (
	"encoding/json"
	"time"

	envModel "private_browser_server/Models/Env"
)

const (
	// runImagePreflightTimeout 是 Node Server 在 run 前等待 Edge pull-image 完成的最大时长。
	//
	// 设计来源：
	// - 用户明确要求 run 不能直接把“镜像未拉取”甩给前端，而要先尝试调用 Edge pull-image；
	// - pull-image 本身是 Edge 内存长任务，HTTP 入口只会快速返回 taskId，如果 Server 不等待终态，
	//   同一次 run 仍然会命中旧的“镜像未拉取”失败；
	// - 因此这里给 run 前置镜像预检一个受控超时，既不无限阻塞，也不把自动预拉镜像做成隐式后台重试。
	runImagePreflightTimeout = 10 * time.Minute

	// runImagePreflightPollInterval 是 Node Server 轮询 Edge pull-image 任务状态的节奏。
	//
	// 这里只做轻量轮询，不引入第二套后台调度器；后续如果 pull-image 变成独立平台任务，再单独演进。
	runImagePreflightPollInterval = 2 * time.Second

	// runEdgeTaskPollInterval 是 Node Server 在 Edge run 已创建后观察执行进度的轮询节奏。
	//
	// 当前先用中心层轮询把 Edge run 状态桥接成统一 SSE 事件；后续如果需要更细粒度事件，可再演进成受控转发。
	runEdgeTaskPollInterval = 2 * time.Second
)

type runTaskProgressReporter func(event string, status string, stage string, message string, data map[string]any)

type edgeCreateBrowserEnvRequest struct {
	UserID      string                             `json:"userId"`
	RPAType     string                             `json:"rpaType"`
	Name        string                             `json:"name"`
	Runtime     edgeCreateBrowserEnvRuntime        `json:"runtime"`
	Environment envModel.CreateEnvEnvironment      `json:"environment"`
	Proxy       envModel.CreateEnvProxyRequest     `json:"proxy"`
	Fingerprint json.RawMessage                    `json:"fingerprint,omitempty"`
	Metadata    *envModel.CreateEnvMetadataRequest `json:"metadata,omitempty"`
}

type edgeCreateBrowserEnvRuntime struct {
	Image      string `json:"image"`
	StartupURL string `json:"startupUrl,omitempty"`
	ShmSize    string `json:"shmSize,omitempty"`
}

type edgeCreateBrowserEnvResponse struct {
	EnvID        string                   `json:"envId"`
	UserID       string                   `json:"userId"`
	RPAType      string                   `json:"rpaType"`
	EnvSequence  int                      `json:"envSequence"`
	Ports        envModel.BrowserEnvPorts `json:"ports"`
	IdentityHash string                   `json:"identityHash"`
	CreatedAt    int64                    `json:"createdAt"`
}

type edgeRunBrowserEnvDetail struct {
	Profile edgeRunBrowserEnvProfile `json:"profile"`
}

type edgeRunBrowserEnvProfile struct {
	Runtime edgeRunBrowserEnvRuntime `json:"runtime"`
}

type edgeRunBrowserEnvRuntime struct {
	Image string `json:"image"`
}

type edgeRunBrowserEnvSnapshotDetail struct {
	Index *edgeRunBrowserEnvIndex `json:"index"`
}

type edgeRunBrowserEnvIndex struct {
	EnvID           string  `json:"envId"`
	RPAType         string  `json:"rpaType"`
	Name            string  `json:"name"`
	CDPPort         int     `json:"cdpPort"`
	WebVNCURL       string  `json:"webVncUrl"`
	Status          string  `json:"status"`
	ContainerStatus string  `json:"containerStatus"`
	MonitorStatus   string  `json:"monitorStatus"`
	LastError       *string `json:"lastError,omitempty"`
	UpdatedAt       int64   `json:"updatedAt"`
}

type runFinalizeDecision struct {
	Status  string
	Message string
	Final   bool
}
