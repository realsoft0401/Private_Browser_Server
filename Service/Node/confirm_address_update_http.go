package Node

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	BindDAO "private_browser_server/Dao/Bind"
	NodeDAO "private_browser_server/Dao/Node"
	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	BindRepo "private_browser_server/Repository/Bind"
	NodeRepo "private_browser_server/Repository/Node"
	DiscoveryService "private_browser_server/Service/Discovery"
	EdgeClientService "private_browser_server/Service/EdgeClient"
)

// ConfirmAddressUpdate 管理员确认节点新地址后的治理入口。
//
// 设计来源：
// - `recheck` 已经收口成“会话校验”，只负责发现 `ip_mismatch`，不自动改地址；
// - 因此地址修正必须单独走一条受控入口，由管理员明确确认后再更新中心记录；
// - 这里仍然保持“薄 HTTP”，只做请求解析、超时控制和统一响应，治理细节下沉到执行函数。
func ConfirmAddressUpdate(ctx *gin.Context) {
	clientID := strings.TrimSpace(ctx.Param("clientId"))
	var request NodeModel.ConfirmAddressUpdateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "confirm-address-update request body 非法")
		return
	}

	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 20*time.Second)
	defer cancel()

	response, err := runConfirmAddressUpdate(requestCtx, clientID, request)
	if err != nil {
		if err == NodeRepo.ErrNotFound {
			HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "edge client not found")
			return
		}
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, response)
}

// runConfirmAddressUpdate 执行一次最小完整的地址确认更新。
//
// 职责边界：
// - 先校验管理员给出的新地址，再对新地址执行 `/health + /device-info`；
// - 只有在拿到可信探测结果后，才允许更新中心记录；
// - 这里要把“同一台机器换 IP”与“根本不是原机器”分开：前者应恢复 `verified`，后者才收口成阻断状态。
func runConfirmAddressUpdate(ctx context.Context, clientID string, request NodeModel.ConfirmAddressUpdateRequest) (*NodeModel.ConfirmAddressUpdateResponse, error) {
	if clientID == "" {
		return nil, fmt.Errorf("clientId 不能为空")
	}

	repo := NodeRepo.NewRepository()
	node, err := repo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	newClientIP, newBaseURL, err := resolveConfirmedAddress(strings.TrimSpace(request.ClientIP), strings.TrimSpace(request.BaseURL))
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	client := EdgeClientService.New()
	health, err := client.GetHealth(ctx, newBaseURL)
	if err != nil {
		return nil, fmt.Errorf("confirm-address-update probe failed: %v", err)
	}
	deviceInfo, err := client.GetDeviceInfo(ctx, newBaseURL)
	if err != nil {
		return nil, fmt.Errorf("confirm-address-update probe failed: %v", err)
	}

	healthStatus := DiscoveryService.NormalizeNodeHealthStatus(health.Status)
	discoveryStatus := "verified"
	discoveryReason := ""
	lastError := ""
	if reason := detectIdentityConflict(node, deviceInfo); reason != "" {
		discoveryStatus = "blocked"
		discoveryReason = reason
		lastError = reason
	} else if !isConfirmedAddressConsistent(newClientIP, deviceInfo) {
		discoveryStatus = "blocked"
		discoveryReason = "ip_mismatch"
		lastError = "ip_mismatch"
	}

	updateRow := &NodeDAO.Row{
		ClientID:        node.ClientID,
		ClientIP:        newClientIP,
		BaseURL:         newBaseURL,
		DockerAPIURL:    strings.TrimSpace(deviceInfo.DockerAPIURL),
		OS:              strings.TrimSpace(deviceInfo.OS),
		Arch:            strings.TrimSpace(deviceInfo.Arch),
		CPUCores:        deviceInfo.CPUCores,
		MemoryTotalMB:   deviceInfo.MemoryTotalMB,
		DockerVersion:   strings.TrimSpace(deviceInfo.DockerVersion),
		HealthStatus:    healthStatus,
		DiscoveryStatus: discoveryStatus,
		DiscoveryReason: discoveryReason,
		LastCheckedAt:   now,
		LastError:       lastError,
		UpdatedAt:       now,
	}
	if err = repo.UpdateAddressAndSessionCheckResult(ctx, updateRow); err != nil {
		return nil, err
	}

	_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
		ClientID:      node.ClientID,
		MainAccountID: node.MainAccountID,
		ClientIP:      newClientIP,
		Action:        "confirm_address_update",
		Result:        "success",
		Message:       successMessageForConfirmAddressUpdate(discoveryStatus, discoveryReason, node.ClientIP, newClientIP),
		CreatedAt:     now,
	})

	return &NodeModel.ConfirmAddressUpdateResponse{
		ClientID:        node.ClientID,
		OldClientIP:     node.ClientIP,
		NewClientIP:     newClientIP,
		OldBaseURL:      node.BaseURL,
		NewBaseURL:      newBaseURL,
		HealthStatus:    healthStatus,
		DiscoveryStatus: discoveryStatus,
		DiscoveryReason: discoveryReason,
		UpdatedAt:       now,
	}, nil
}

func resolveConfirmedAddress(clientIP, baseURL string) (string, string, error) {
	clientIP = strings.TrimSpace(clientIP)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if clientIP == "" && baseURL == "" {
		return "", "", fmt.Errorf("clientIp 和 baseUrl 不能同时为空")
	}

	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s:3300", clientIP)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", fmt.Errorf("baseUrl 非法")
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", "", fmt.Errorf("baseUrl 非法")
	}
	if clientIP == "" {
		clientIP = host
	}
	if !sameHost(clientIP, host) {
		return "", "", fmt.Errorf("clientIp 与 baseUrl 主机不一致")
	}
	if net.ParseIP(clientIP) == nil {
		return "", "", fmt.Errorf("clientIp 非法")
	}
	return clientIP, parsed.String(), nil
}

func successMessageForConfirmAddressUpdate(discoveryStatus, discoveryReason, oldClientIP, newClientIP string) string {
	base := fmt.Sprintf("confirm address update success old=%s new=%s", strings.TrimSpace(oldClientIP), strings.TrimSpace(newClientIP))
	if strings.TrimSpace(discoveryStatus) == "blocked" && strings.TrimSpace(discoveryReason) != "" {
		return fmt.Sprintf("%s with blocked reason=%s", base, discoveryReason)
	}
	return base
}
