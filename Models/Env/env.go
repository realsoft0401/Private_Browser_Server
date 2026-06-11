package Env

import "encoding/json"

// ServerBrowserEnv 是 Node Server 聚合保存的中心环境包索引。
//
// 设计来源：
// - Edge Client 本地已经有完整环境包文件和 SQLite 索引，但 Node Server 不能直接读取这些本地事实；
// - 因此中心层只缓存 API 已确认的摘要：归属主账号、所属 Client、状态、连接入口和审计字段；
// - 真实 profile、proxy 明文、fingerprint raw、browser-data/profile 仍然只留在 Edge。
//
// 职责边界：
// - 负责给管理端提供跨节点环境包列表、详情和后续任务归属；
// - 不负责保存浏览器环境实体，也不替代 Edge 本地生命周期事实源；
// - 运行状态过期时应由 Service 重新调用 Edge API 刷新，而不是在这里猜测或修复环境包。
type ServerBrowserEnv struct {
	EnvID string `json:"envId"`
	// MainAccountID 是环境包真正归属的主账号。
	//
	// 这是用户明确确认过的商业口径：子账号只是操作人，不能改变 envId 第一段和资产归属。
	MainAccountID string `json:"mainAccountId"`
	// ClientID 是当前环境包绑定的中心 Client 身份。
	//
	// 环境包在哪台 Edge 创建，就固定在哪台 Edge 后续 run/stop/backup/restore/delete。
	ClientID        string `json:"clientId"`
	RPAType         string `json:"rpaType"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	ContainerStatus string `json:"containerStatus"`
	MonitorStatus   string `json:"monitorStatus"`
	CDPURL          string `json:"cdpUrl"`
	WebVNCURL       string `json:"webVncUrl"`
	LastTaskID      string `json:"lastTaskId"`
	LastError       string `json:"lastError"`
	// CreatedBy* 保存 Platform Header 中的操作人，服务审计和排障展示。
	CreatedByUserID   string `json:"createdByUserId"`
	CreatedByUsername string `json:"createdByUsername"`
	CreatedAt         int64  `json:"createdAt"`
	UpdatedAt         int64  `json:"updatedAt"`
	DeletedAt         int64  `json:"deletedAt,omitempty"`
}

const (
	EnvStatusCreated  = "created"
	EnvStatusRunning  = "running"
	EnvStatusStopped  = "stopped"
	EnvStatusBackedUp = "backed_up"
	EnvStatusDeleted  = "deleted"
	EnvStatusArchived = "archived"
	EnvStatusError    = "error"

	// EnvFactUnknown 表示中心层尚未从 Edge 拿到可靠的容器或监控摘要。
	//
	// 创建环境包时还没有容器事实，因此不能伪造 running/stopped，只能先保留 unknown。
	EnvFactUnknown = "unknown"
)

// BrowserEnvPorts 复用 Edge 环境包对外暴露的本机端口摘要。
//
// Node Server 创建环境包后会把这些端口回给前端，方便后续定位 CDP/VNC 连接入口；
// 但中心 SQLite 当前不把端口拆列保存，而是通过 cdpUrl/webVncUrl 聚合展示连接地址。
type BrowserEnvPorts struct {
	CDP int `json:"cdp"`
	VNC int `json:"vnc"`
}

// CreateEnvRequest 是 Node Server 创建环境包的商业入口请求。
//
// 设计来源：
// - 前端不应直接把 Edge CreateBrowserEnvRequest 原样发给 Client；
// - Node Server 需要先校验 clientId、根据节点 arch 选择 runtime.image，再把请求转换给 Edge；
// - userId 由 Node Server 固定使用 mainAccountId，保证 envId 第一段稳定，不受子账号影响。
type CreateEnvRequest struct {
	ClientID    string                    `json:"clientId"`
	Name        string                    `json:"name"`
	RPAType     string                    `json:"rpaType"`
	Runtime     CreateEnvRuntimeRequest   `json:"runtime"`
	Environment CreateEnvEnvironment      `json:"environment"`
	Proxy       CreateEnvProxyRequest     `json:"proxy"`
	Fingerprint json.RawMessage           `json:"fingerprint,omitempty"`
	Metadata    *CreateEnvMetadataRequest `json:"metadata,omitempty"`
}

// CreateEnvRuntimeRequest 是 Node Server 对前端暴露的受控运行配置。
//
// 职责边界：
// - 普通前端或用户不能随意拼装镜像字符串；imagePolicy 应由 Platform 或中心管理服务受控下发；
// - 当前迁移阶段，Platform 先把“镜像策略值”直接下发成已登记镜像字符串，例如 amd64 的 1.1-amd64；
// - 真正的 runtime.image 仍由 Node Server 根据 arch + imagePolicy 校验后决定，避免调用方绕过中心层随意指定镜像。
type CreateEnvRuntimeRequest struct {
	ImagePolicy string `json:"imagePolicy"`
	StartupURL  string `json:"startupUrl"`
	ShmSize     string `json:"shmSize"`
}

// CreateEnvEnvironment 保存浏览器稳定环境参数。
type CreateEnvEnvironment struct {
	Timezone string          `json:"timezone"`
	Language string          `json:"language"`
	Screen   CreateEnvScreen `json:"screen"`
}

// CreateEnvScreen 是浏览器屏幕参数。
type CreateEnvScreen struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Depth  int `json:"depth"`
}

// CreateEnvProxyRequest 是 Node Server 暴露给前端的代理配置输入。
//
// configBase64 原样转发给 Edge；Node Server 不解码、不保存代理明文。
type CreateEnvProxyRequest struct {
	Enabled      *bool  `json:"enabled"`
	Type         string `json:"type"`
	Mode         string `json:"mode"`
	ConfigBase64 string `json:"configBase64"`
}

// CreateEnvMetadataRequest 保存环境包轻量描述。
//
// 它只保留说明类字段，不参与 imagePolicy 或身份判断。
type CreateEnvMetadataRequest struct {
	Source      string `json:"source"`
	Description string `json:"description"`
}

// CreateEnvResponse 是 Node Server 创建环境包成功后的统一摘要。
//
// 这里同时返回中心缓存记录和本次创建中最关键的 Edge 结果，方便前端直接展示。
type CreateEnvResponse struct {
	EnvID         string            `json:"envId"`
	MainAccountID string            `json:"mainAccountId"`
	ClientID      string            `json:"clientId"`
	Status        string            `json:"status"`
	Ports         BrowserEnvPorts   `json:"ports"`
	IdentityHash  string            `json:"identityHash"`
	CDPURL        string            `json:"cdpUrl"`
	WebVNCURL     string            `json:"webVncUrl"`
	CreatedAt     int64             `json:"createdAt"`
	Env           *ServerBrowserEnv `json:"env"`
}

// ListEnvQuery 是 Node Server 环境包列表查询条件。
//
// 列表主事实源是 server_browser_envs SQLite 缓存，不直接去 Edge 扫描环境包。
type ListEnvQuery struct {
	ClientID string
	RPAType  string
	Status   string
	Page     int
	PageSize int
}

// ListEnvsResponse 是中心环境包列表响应。
type ListEnvsResponse struct {
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	Items    []ServerBrowserEnv `json:"items"`
}

// RunEnvRequest 是 Node Server 启动环境包的可选参数。
//
// 第一版只透出 forceRecreate 这类生命周期开关；
// Docker 参数、镜像、端口、挂载和代理正文都必须继续以环境包资产为准。
type RunEnvRequest struct {
	ForceRecreate bool `json:"forceRecreate"`
}

// StopEnvRequest 是 Node Server 停止环境包的可选参数。
//
// timeoutSeconds 只是允许调用方明确给浏览器写盘缓冲时间；
// 其它危险 Docker stop 细节不向平台前端暴露。
type StopEnvRequest struct {
	TimeoutSeconds *int `json:"timeoutSeconds"`
}

// DeleteEnvImageResponse 是 Node Server 代理 `/envs/:envId/del` 后返回的同步摘要。
//
// 设计来源：
// - Edge `/api/v1/edge/browser-envs/:envId/del` 当前就是同步接口，Node Server 不额外包装成中心 task；
// - 该动作只删除运行镜像，不会销毁环境包目录，因此中心层只回传镜像删除结果和 warning；
// - 这样前端可以清楚区分“镜像清理”和“环境包销毁”两条危险动作链路。
type DeleteEnvImageResponse struct {
	EnvID          string                  `json:"envId"`
	ClientID       string                  `json:"clientId"`
	Image          string                  `json:"image"`
	ImageRemoved   bool                    `json:"imageRemoved"`
	Results        []DockerImageRemoveItem `json:"results,omitempty"`
	WarningMessage string                  `json:"warningMessage,omitempty"`
	DeletedAt      int64                   `json:"deletedAt"`
}

// DockerImageRemoveItem 是镜像删除结果的最小摘要。
//
// Node Server 不直接暴露 Docker 原始响应，只保留前端联调和排障需要的 deleted/untagged 字段。
type DockerImageRemoveItem struct {
	Image    string `json:"image"`
	Deleted  string `json:"deleted,omitempty"`
	Untagged string `json:"untagged,omitempty"`
}
