package Discovery

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	NodeRepo "private_browser_server/Repository/Node"
	EdgeClientService "private_browser_server/Service/EdgeClient"
)

// ProbeClientByIP 在第一阶段 bind 前做真实 HTTP 探测。
//
// 设计来源：
// - 你已经拍板 discovered 不单独入正式表，但 bind 前仍然必须先确认 Client 在线；
// - 因此这里按 `clientIp -> /health -> /api/v1/edge/device-info` 收最小在线事实；
// - 当前默认把 `baseUrl` 推成 `http://<clientIp>:3300`，后续如果恢复管理员手填 baseUrl，可继续扩这里。
func ProbeClientByIP(ctx context.Context, clientIP string) (*DiscoveredClient, error) {
	return ObserveClient(ctx, ObserveRequest{
		Source:          "bind",
		ClientIP:        clientIP,
		LastHeartbeatAt: 0,
	})
}

// ObserveRequest 描述一次来自 UDP/heartbeat/bind 的最小发现输入。
//
// 设计来源：
// - 现在 discovered 既可能来自 UDP，也可能来自 HTTP heartbeat，还可能来自 bind 前即时探测；
// - 三条入口都要做同一套 `/health` + `/device-info` 收口，不应该各自拷一遍逻辑；
// - 这里把“我观察到了哪个入口地址”收成统一请求，后续新增手动探测时也能继续复用。
type ObserveRequest struct {
	Source          string
	SourceIP        string
	ClientIP        string
	BaseURL         string
	Hostname        string
	LastHeartbeatAt int64
}

// ObserveClient 根据 discovery 输入收一个统一 discovered 视图。
//
// 职责边界：
// - 这里只做 Edge HTTP 探测、地址归一化和 discovered 内存视图组装；
// - 不直接写正式 edge_clients，不生成 clientId；
// - 但如果已经存在已绑定节点，会把 clientId/accountId 只读带回 discovered 视图，方便前端区分“已绑定发现项”和“未绑定发现项”。
func ObserveClient(ctx context.Context, request ObserveRequest) (*DiscoveredClient, error) {
	clientIP := chooseObservedClientIP(request)
	if clientIP == "" {
		return nil, fmt.Errorf("clientIp 不能为空")
	}
	baseURL := normalizeObservedBaseURL(request.BaseURL, clientIP)
	client := EdgeClientService.New()
	health, err := client.GetHealth(ctx, baseURL)
	if err != nil {
		return nil, fmt.Errorf("probe client health failed: %w", err)
	}
	deviceInfo, err := client.GetDeviceInfo(ctx, baseURL)
	if err != nil {
		return nil, fmt.Errorf("probe client device-info failed: %w", err)
	}
	now := time.Now().Unix()
	discovered := &DiscoveredClient{
		DiscoveryID:     fmt.Sprintf("disc_%d", time.Now().UnixNano()),
		ClientIP:        clientIP,
		BaseURL:         baseURL,
		Hostname:        strings.TrimSpace(request.Hostname),
		OS:              firstNonEmpty(deviceInfo.OS, "unknown"),
		Arch:            firstNonEmpty(deviceInfo.Arch, "unknown"),
		CPUCores:        deviceInfo.CPUCores,
		MemoryTotalMB:   deviceInfo.MemoryTotalMB,
		DockerAPIURL:    firstNonEmpty(deviceInfo.DockerAPIURL, fmt.Sprintf("http://%s:2375", clientIP)),
		DockerVersion:   firstNonEmpty(deviceInfo.DockerVersion, ""),
		HealthStatus:    NormalizeNodeHealthStatus(health.Status),
		Status:          "discovered",
		DiscoveredAt:    now,
		LastHeartbeatAt: firstHeartbeatAt(request.LastHeartbeatAt, now),
	}
	attachBoundIdentity(ctx, discovered)
	return discovered, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeObservedBaseURL(rawBaseURL, clientIP string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(rawBaseURL), "/")
	if baseURL == "" {
		return fmt.Sprintf("http://%s:3300", clientIP)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Sprintf("http://%s:3300", clientIP)
	}
	return parsed.String()
}

func firstHeartbeatAt(reportedAt, fallback int64) int64 {
	if reportedAt > 0 {
		return reportedAt
	}
	return fallback
}

func attachBoundIdentity(ctx context.Context, discovered *DiscoveredClient) {
	if discovered == nil {
		return
	}
	repo := NodeRepo.NewRepository()
	if node, err := repo.GetByClientIP(ctx, discovered.ClientIP); err == nil && node != nil {
		discovered.ClientID = node.ClientID
		discovered.AccountID = node.MainAccountID
		return
	}
}

func chooseObservedClientIP(request ObserveRequest) string {
	sourceIP := strings.TrimSpace(request.SourceIP)
	clientIP := strings.TrimSpace(request.ClientIP)
	if strings.EqualFold(strings.TrimSpace(request.Source), "udp") {
		return firstNonEmpty(sourceIP, clientIP)
	}
	if isLoopbackIP(sourceIP) && clientIP != "" {
		return clientIP
	}
	return firstNonEmpty(clientIP, sourceIP)
}

func isLoopbackIP(ip string) bool {
	switch strings.TrimSpace(ip) {
	case "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}
