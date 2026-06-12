package Env

import (
	"encoding/json"
	"path/filepath"
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

	// importPackageMaxUploadBytes 是 Node Server 第一版接收标准环境包上传的受控上限。
	//
	// 设计来源：
	// - Edge 侧标准包会携带 browser-data/profile，体积可能较大，Node Server 不能默认按“小文件表单上传”处理；
	// - 当前 Server 还没把上传上限做进配置文件，因此先和 Edge 侧保持同一数量级，避免联调阶段出现两套口径；
	// - 即使上限放宽，这个动作仍然只能在受控内网和中心 task 模式下执行，不能暴露到公网长传场景。
	importPackageMaxUploadBytes = 20 << 30
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

type edgeImportBrowserEnvPackageResponse struct {
	EnvID       string                   `json:"envId"`
	UserID      string                   `json:"userId"`
	RPAType     string                   `json:"rpaType"`
	EnvSequence int                      `json:"envSequence"`
	Ports       envModel.BrowserEnvPorts `json:"ports"`
	EnvPath     string                   `json:"envPath"`
	Status      string                   `json:"status"`
	ImportedAt  int64                    `json:"importedAt"`
}

type importPackageProfileIdentity struct {
	EnvID   string `json:"envId"`
	UserID  string `json:"userId"`
	RPAType string `json:"rpaType"`
	Name    string `json:"name"`
}

type importPackageUploadArtifact struct {
	FilePath     string
	FileName     string
	OriginalSize int64
	Identity     importPackageProfileIdentity
}

func (a importPackageUploadArtifact) baseName() string {
	return filepath.Base(a.FileName)
}
