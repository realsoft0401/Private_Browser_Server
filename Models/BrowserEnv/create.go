package BrowserEnv

// CreateRequest 是中心创建 browser-env 的正式请求体。
//
// 设计来源：
// - Server 必须明确指定目标 clientId，不能自动选机器；
// - Client 只负责在本机创建环境包，不理解中心 clientId；
// - 因此 clientId 只服务 Node 调度，传给 Edge 时会被剥离。
type CreateRequest struct {
	ClientID    string            `json:"clientId"`
	UserID      string            `json:"userId"`
	RPAType     string            `json:"rpaType"`
	Name        string            `json:"name"`
	Runtime     CreateRuntime     `json:"runtime"`
	Environment CreateEnvironment `json:"environment"`
	Proxy       CreateProxy       `json:"proxy"`
}

type CreateRuntime struct {
	Image      string `json:"image"`
	StartupURL string `json:"startupUrl"`
	ShmSize    string `json:"shmSize"`
}

type CreateEnvironment struct {
	Timezone string       `json:"timezone"`
	Language string       `json:"language"`
	Screen   CreateScreen `json:"screen"`
}

type CreateScreen struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Depth  int `json:"depth"`
}

// CreateProxy 只承载中心下发给 Edge 的代理配置摘要和配置正文 Base64。
//
// 这里不解析 proxy 明文，也不持久化到 Server SQLite；真实文件仍只落在 Client 环境包目录。
type CreateProxy struct {
	Enabled      *bool  `json:"enabled"`
	Type         string `json:"type"`
	ConfigBase64 string `json:"configBase64"`
}

// CreateResponse 是中心创建 browser-env 成功后的同步响应。
//
// 它把 Edge 创建结果和中心缓存摘要一起返回，便于调用方立刻拿 envId 进入后续 run/backup 流程。
type CreateResponse struct {
	EnvID        string            `json:"envId"`
	ClientID     string            `json:"clientId"`
	AccountID    string            `json:"accountId"`
	UserID       string            `json:"userId"`
	RPAType      string            `json:"rpaType"`
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	EnvSequence  int               `json:"envSequence"`
	Ports        BrowserEnvPorts   `json:"ports"`
	EnvPath      string            `json:"envPath"`
	Files        map[string]string `json:"files"`
	IdentityHash string            `json:"identityHash"`
	CreatedAt    int64             `json:"createdAt"`
}

type BrowserEnvPorts struct {
	CDP int `json:"cdp"`
	VNC int `json:"vnc"`
}
