package Node

import (
	"strings"

	NodeModel "private_browser_server/Models/Node"
	DiscoveryService "private_browser_server/Service/Discovery"
)

type registerNodeRequest struct {
	Name         string `json:"name"`
	BaseURL      string `json:"baseUrl"`
	ClientIP     string `json:"clientIp"`
	DockerAPIURL string `json:"dockerApiUrl"`
}

type probeDockerRequest struct {
	DockerAPIURL string `json:"dockerApiUrl"`
}

// heartbeatRequest 是 Edge Client 主动上报心跳的最小请求体。
//
// 设计来源：
// - 之前 Node Server 只有 UDP discovery 的被动回写，没有正式的 HTTP 心跳入口；
// - 用户确认 last_heartbeat_at 应当表达“心跳时间”，但更准确的做法是把“服务端实际收到时间”和
//   “Client 自报时间”拆开保存；
// - 因此这个请求体只保留发现域和接入地址等非敏感摘要，不接收任何登录态、proxy 明文、fingerprint raw。
type heartbeatRequest struct {
	DiscoveryMagic  string `json:"discoveryMagic"`
	ProtocolVersion int    `json:"protocolVersion"`
	Service         string `json:"service"`
	DiscoveryGroup  string `json:"discoveryGroup"`
	BaseURL         string `json:"baseUrl"`
	ClientIP        string `json:"clientIp"`
	LastHeartbeatAt int64  `json:"lastHeartbeatAt"`
}

// heartbeatResponse 是心跳上报成功后的中心摘要。
//
// 它只返回匹配到的中心 clientId 和最新心跳时间，不回显任何敏感业务资产。
type heartbeatResponse struct {
	ClientID                string                `json:"clientId"`
	MainAccountID           string                `json:"mainAccountId"`
	BaseURL                 string                `json:"baseUrl"`
	ClientIP                string                `json:"clientIp"`
	LastHeartbeatAt         int64                 `json:"lastHeartbeatAt"`
	LastHeartbeatReportedAt int64                 `json:"lastHeartbeatReportedAt"`
	HeartbeatStatus         string                `json:"heartbeatStatus"`
	UpdatedAt               int64                 `json:"updatedAt"`
	Client                  *NodeModel.EdgeClient `json:"client,omitempty"`
}

type dockerProbeResult struct {
	Reachable        bool   `json:"reachable"`
	DockerAPIURL     string `json:"dockerApiUrl"`
	OS               string `json:"os"`
	Arch             string `json:"arch"`
	RawArch          string `json:"rawArch"`
	CPUCores         int    `json:"cpuCores"`
	MemoryTotalMB    int64  `json:"memoryTotalMb"`
	DockerVersion    string `json:"dockerVersion"`
	DockerAPIVersion string `json:"dockerApiVersion"`
	Containers       int    `json:"containers"`
	Images           int    `json:"images"`
	CheckedAt        int64  `json:"checkedAt"`
	Troubleshooting  string `json:"troubleshooting,omitempty"`
}

type dockerInfoResponse struct {
	OperatingSystem string `json:"OperatingSystem"`
	OSType          string `json:"OSType"`
	Architecture    string `json:"Architecture"`
	NCPU            int    `json:"NCPU"`
	MemTotal        int64  `json:"MemTotal"`
	ServerVersion   string `json:"ServerVersion"`
	Containers      int    `json:"Containers"`
	Images          int    `json:"Images"`
}

type dockerVersionResponse struct {
	Version    string `json:"Version"`
	APIVersion string `json:"ApiVersion"`
	OS         string `json:"Os"`
	Arch       string `json:"Arch"`
}

type edgeHealthResponse struct {
	OK     bool   `json:"ok"`
	Status string `json:"status"`
}

type edgeDeviceInfoResponse struct {
	DeviceIP            string `json:"deviceIp"`
	DockerAPIURL        string `json:"dockerApiUrl"`
	DeviceOS            string `json:"deviceOs"`
	DeviceArch          string `json:"deviceArch"`
	DeviceRawArch       string `json:"deviceRawArch"`
	CPUCores            int    `json:"cpuCores"`
	MemoryTotalBytes    int64  `json:"memoryTotalBytes"`
	DockerVersion       string `json:"dockerVersion"`
	DockerAPIVersion    string `json:"dockerApiVersion"`
	LastDockerStatus    string `json:"lastDockerStatus"`
	LastDockerMessage   string `json:"lastDockerMessage"`
	LastImagesCount     int    `json:"lastImagesCount"`
	LastContainersCount int    `json:"lastContainersCount"`
	CheckedAt           int64  `json:"checkedAt"`
}

type verifyCheck struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type verifyResponse struct {
	Client     *NodeModel.EdgeClient  `json:"client,omitempty"`
	ClientID   string                 `json:"clientId,omitempty"`
	NextAction string                 `json:"nextAction,omitempty"`
	Checks     map[string]verifyCheck `json:"checks"`
}

// BusinessReadyError 表示 Client 未满足业务动作前置条件。
//
// 它只用于 Service 间传递稳定错误语义，HTTP 层可以把 Message/NextAction 直接返回给前端；
// 不在这里写响应，避免 Node Service 依赖具体业务入口。
type BusinessReadyError struct {
	Message    string
	NextAction string
}

func (e *BusinessReadyError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.NextAction) == "" {
		return e.Message
	}
	return e.Message + "；" + e.NextAction
}

// discoveredClientView 是 `/edge-clients/discovered` 的展示模型。
//
// 设计来源：用户明确要求 UDP 发现列表里可以直接区分“已经绑定过”和“未绑定”。
// Client UDP beacon 仍然不携带商业身份，也不携带 Node Server 分配的 clientId；
// Node Server 在返回发现列表时，按当前 Platform 主账号下已注册的 baseUrl/clientIp 做只读匹配，
// 匹配成功才补充 clientId。前端只需要看 clientId 是否为空，不再引入 bindingStatus 之类第二套状态。
type discoveredClientView struct {
	ClientID      string                         `json:"clientId"`
	SourceIP      string                         `json:"sourceIp"`
	SourcePort    int                            `json:"sourcePort"`
	Payload       DiscoveryService.BeaconPayload `json:"payload"`
	FirstSeenAt   int64                          `json:"firstSeenAt"`
	LastSeenAt    int64                          `json:"lastSeenAt"`
	ReceiveCount  int64                          `json:"receiveCount"`
	DiscardReason string                         `json:"discardReason,omitempty"`
}
