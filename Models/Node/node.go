package Node

// EdgeClient 保存 Edge Client 在 Node Server 侧的中心索引。
//
// 设计背景：用户确认三层服务统一把 Node Server 分配给 Client 的中心身份叫 clientId。
// SQLite 表名同步收敛为 edge_clients，避免再出现 node/control_node/client 多套叫法。
// 对外 API、Swagger 和业务文档统一暴露 clientId，避免再出现多套中心身份叫法。
// Client 真实 Docker 操作仍由 Edge 完成；Server 只保存接入信息、设备能力和健康摘要。
type EdgeClient struct {
	ID                string `json:"clientId"`
	MainAccountID     string `json:"mainAccountId"`
	NodeSequence      int    `json:"clientSequence"`
	Name              string `json:"name"`
	BaseURL           string `json:"baseUrl"`
	ClientIP          string `json:"clientIp"`
	DockerAPIURL      string `json:"dockerApiUrl"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	CPUCores          int    `json:"cpuCores"`
	MemoryTotalMB     int64  `json:"memoryTotalMb"`
	DockerVersion     string `json:"dockerVersion"`
	HealthStatus      string `json:"healthStatus"`
	DiscoveryStatus   string `json:"discoveryStatus"`
	HeartbeatStatus   string `json:"heartbeatStatus"`
	// LastHeartbeatAt 表示 Node Server 最近一次确认收到该 Client 心跳的服务端时间。
	//
	// 设计来源：
	// - 用户实测后确认，之前把 Client 自报时间直接塞进 last_heartbeat_at，容易把“Client 自己认为何时发过心跳”
	//   和 “Node Server 何时真正收到心跳”混成一件事；
	// - heartbeatStatus 是 Node Server 的调度前置判断，应该以中心服务真正收到心跳的时间为准，
	//   不能完全信任 Client 自报时钟，否则时钟漂移会让 online/stale/offline 判断失真。
	//
	// 职责边界：
	// - 这里只表示服务端接收事实；
	// - 不代表 Client 本地 startedAt，也不代表 verify/health 的探测时间；
	// - heartbeatStatus 只能根据该字段动态计算，不能根据 reportedAt 直接放行业务动作。
	LastHeartbeatAt   int64  `json:"lastHeartbeatAt"`
	// LastHeartbeatReportedAt 表示 Client 在心跳报文里自报的最近心跳时间。
	//
	// 这个字段只做排障和时钟偏差观察：
	// - UDP beacon 和 HTTP heartbeat 仍然允许携带 Client 自报时间；
	// - 但中心调度只把它当附加诊断字段，不能拿它替代服务端实际接收时间。
	LastHeartbeatReportedAt int64  `json:"lastHeartbeatReportedAt"`
	// LastHeartbeatSource 记录最近一次把心跳写入中心库的来源。
	//
	// 当前只允许：
	// - udp  : 来自 discovery/beacon 的被动发现
	// - http : 来自 Client 主动调用 /api/v1/server/edge-clients/heartbeat
	//
	// 保留来源是为了后续排查“为什么 UDP 没有更新但 HTTP 在更新”这类链路问题，
	// 不把它参与 verified/healthy 判断。
	LastHeartbeatSource string `json:"lastHeartbeatSource"`
	LastCheckedAt     int64  `json:"lastCheckedAt"`
	LastError         string `json:"lastError"`
	CreatedByUserID   string `json:"createdByUserId"`
	CreatedByUsername string `json:"createdByUsername"`
	CreatedAt         int64  `json:"createdAt"`
	UpdatedAt         int64  `json:"updatedAt"`
	DeletedAt         int64  `json:"deletedAt,omitempty"`
}

const (
	NodeArchAMD64   = "amd64"
	NodeArchARM64   = "arm64"
	NodeArchUnknown = "unknown"

	NodeHealthHealthy   = "healthy"
	NodeHealthUnhealthy = "unhealthy"
	NodeHealthOffline   = "offline"
	NodeHealthStale     = "stale"

	NodeDiscoveryManual   = "manual"
	NodeDiscoveryVerified = "verified"

	NodeHeartbeatOnline  = "online"
	NodeHeartbeatStale   = "stale"
	NodeHeartbeatOffline = "offline"
)
