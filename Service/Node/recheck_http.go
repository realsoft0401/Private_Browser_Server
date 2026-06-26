package Node

import (
	"context"
	"errors"
	"fmt"
	"io"
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

// RecheckClient 会话校验接口入口。
//
// 设计来源：
// - 你已经把 `recheck` 的业务语义明确成“会话校验”；
// - 这条接口不是重新绑定，而是管理员手动重新确认当前节点会话是否仍有效；
// - 因此 HTTP 层只做请求解析、超时控制和统一响应，不在这里掺杂治理判断细节。
func RecheckClient(ctx *gin.Context) {
	clientID := strings.TrimSpace(ctx.Param("clientId"))
	var request NodeModel.RecheckRequest
	if err := ctx.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "recheck request body 非法")
		return
	}

	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 20*time.Second)
	defer cancel()

	response, err := runSessionCheck(requestCtx, clientID, request)
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

// runSessionCheck 执行一次最小完整的会话校验。
//
// 职责边界：
// - 先读取当前正式节点，再重探 `/health + /device-info`；
// - 然后把结果收口成 `healthy/verified`、`blocked/identity_changed` 或 `blocked/probe_failed`；
// - 不重新 bind，不自动确认地址漂移，也不修改中心身份字段。
func runSessionCheck(ctx context.Context, clientID string, request NodeModel.RecheckRequest) (*NodeModel.RecheckResponse, error) {
	if clientID == "" {
		return nil, fmt.Errorf("clientId 不能为空")
	}
	_ = request

	repo := NodeRepo.NewRepository()
	node, err := repo.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	baseURL := strings.TrimRight(strings.TrimSpace(node.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("current node baseUrl 不能为空")
	}

	client := EdgeClientService.New()
	health, err := client.GetHealth(ctx, baseURL)
	if err != nil {
		return nil, finalizeFailedSessionCheck(ctx, repo, node, now, fmt.Sprintf("recheck probe failed: %v", err))
	}
	deviceInfo, err := client.GetDeviceInfo(ctx, baseURL)
	if err != nil {
		return nil, finalizeFailedSessionCheck(ctx, repo, node, now, fmt.Sprintf("recheck probe failed: %v", err))
	}

	healthStatus := DiscoveryService.NormalizeNodeHealthStatus(health.Status)
	discoveryStatus := "verified"
	discoveryReason := ""
	lastError := ""

	if reason := detectSessionCheckConflict(node, deviceInfo); reason != "" {
		discoveryStatus = "blocked"
		discoveryReason = reason
		lastError = reason
	}

	updateRow := &NodeDAO.Row{
		ClientID:        node.ClientID,
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
	if err = repo.UpdateSessionCheckResult(ctx, updateRow); err != nil {
		return nil, err
	}

	_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
		ClientID:      node.ClientID,
		MainAccountID: node.MainAccountID,
		ClientIP:      node.ClientIP,
		Action:        "recheck",
		Result:        "success",
		Message:       successMessageForSessionCheck(discoveryStatus, discoveryReason),
		CreatedAt:     now,
	})

	return &NodeModel.RecheckResponse{
		ClientID:        node.ClientID,
		Status:          "rechecked",
		HealthStatus:    healthStatus,
		DiscoveryStatus: discoveryStatus,
		DiscoveryReason: discoveryReason,
		CheckedAt:       now,
	}, nil
}

// finalizeFailedSessionCheck 负责把一次失败的会话校验明确收口成中心可见事实。
//
// 设计来源：
// - 会话校验失败后，不能只把错误留在 HTTP 返回里，否则节点中心摘要还是旧的；
// - 用户已经明确要求“有据可查”，因此 probe 失败也必须回写节点错误摘要和审计；
// - 这里统一把失败收口成 `offline + blocked + probe_failed`，避免 Service 层各自写一套失败语义。
func finalizeFailedSessionCheck(
	ctx context.Context,
	repo *NodeRepo.Repository,
	node *NodeModel.EdgeClient,
	now int64,
	message string,
) error {
	if repo == nil || node == nil {
		return fmt.Errorf("%s", message)
	}
	writeCtx, cancel := withGovernanceWriteContext(ctx)
	defer cancel()
	_ = repo.UpdateSessionCheckResult(writeCtx, &NodeDAO.Row{
		ClientID:        node.ClientID,
		HealthStatus:    "offline",
		DiscoveryStatus: "blocked",
		DiscoveryReason: "probe_failed",
		LastCheckedAt:   now,
		LastError:       message,
		UpdatedAt:       now,
	})
	_ = BindRepo.NewRepository().CreateLog(writeCtx, &BindDAO.LogRow{
		ClientID:      node.ClientID,
		MainAccountID: node.MainAccountID,
		ClientIP:      node.ClientIP,
		Action:        "recheck",
		Result:        "failed",
		Message:       message,
		CreatedAt:     now,
	})
	return fmt.Errorf("%s", message)
}

// withGovernanceWriteContext 为节点治理失败收口提供独立短上下文。
//
// 设计来源：
// - `recheck` / `confirm-address-update` 的核心价值之一，就是即使远端探测失败，也要把失败事实落到中心库和审计；
// - 但如果直接复用已经被 HTTP 超时耗尽的上游 context，后续数据库写入会一起被取消，结果就是“接口报错了，但中心没有留痕”；
// - 因此治理失败收口必须切一层短生命周期写上下文，专门保证摘要和审计能落盘。
func withGovernanceWriteContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

// detectSessionCheckConflict 把“探测成功但事实冲突”的情况统一归一成治理原因。
//
// 设计来源：
// - `recheck` 被你收口成“会话校验”后，它的职责就是确认“当前登记地址上的会话还是不是原来那台 Client”；
// - 因此这里既要识别设备身份变化，也要识别地址漂移；
// - 但它只负责发现问题，不负责自动修正地址，地址确认仍必须走 `confirm-address-update`。
func detectSessionCheckConflict(node *NodeModel.EdgeClient, deviceInfo *EdgeClientService.DeviceInfoResponse) string {
	if node == nil || deviceInfo == nil {
		return ""
	}
	if reason := detectIdentityConflict(node, deviceInfo); reason != "" {
		return reason
	}

	newDockerHost := extractHost(strings.TrimSpace(deviceInfo.DockerAPIURL))
	oldDockerHost := extractHost(strings.TrimSpace(node.DockerAPIURL))
	oldClientIP := strings.TrimSpace(node.ClientIP)
	if newDockerHost != "" {
		if oldClientIP != "" && !sameHost(oldClientIP, newDockerHost) {
			return "ip_mismatch"
		}
		if oldDockerHost != "" && !sameHost(oldDockerHost, newDockerHost) {
			return "ip_mismatch"
		}
	}
	return ""
}

// detectIdentityConflict 只判断“这还是不是原来那台机器”。
//
// 职责边界：
// - 这里只看不会因为换 IP 而自然变化的设备事实；
// - 当前第一阶段先锁定 `os/arch` 这两个最稳定的主机事实；
// - 后续如果再增加 hostname、machine-id 等字段，也应继续放在这层，而不是掺进地址漂移判断。
func detectIdentityConflict(node *NodeModel.EdgeClient, deviceInfo *EdgeClientService.DeviceInfoResponse) string {
	if node == nil || deviceInfo == nil {
		return ""
	}
	oldOS := strings.ToLower(strings.TrimSpace(node.OS))
	newOS := strings.ToLower(strings.TrimSpace(deviceInfo.OS))
	if oldOS != "" && oldOS != "unknown" && newOS != "" && newOS != "unknown" && oldOS != newOS {
		return "identity_changed"
	}

	oldArch := strings.ToLower(strings.TrimSpace(node.Arch))
	newArch := strings.ToLower(strings.TrimSpace(deviceInfo.Arch))
	if oldArch != "" && oldArch != "unknown" && newArch != "" && newArch != "unknown" && oldArch != newArch {
		return "identity_changed"
	}
	return ""
}

// isConfirmedAddressConsistent 负责确认管理员给出的“新地址”与 Client 当前自报地址是否一致。
//
// 设计来源：
// - `confirm-address-update` 的目标不是再拿旧地址比较一遍，而是确认“新地址已生效且 Client 也以此地址对外报告”；
// - 如果管理员填的是 `192.168.111.120`，但 Client 仍自报 `dockerApiUrl=http://192.168.111.119:2375`，那说明地址确认还没收口；
// - 这里单独抽出来，避免 `confirm-address-update` 继续复用 `recheck` 的旧地址漂移判断，导致正常换 IP 也被误判。
func isConfirmedAddressConsistent(confirmedClientIP string, deviceInfo *EdgeClientService.DeviceInfoResponse) bool {
	if strings.TrimSpace(confirmedClientIP) == "" || deviceInfo == nil {
		return true
	}
	dockerHost := extractHost(strings.TrimSpace(deviceInfo.DockerAPIURL))
	if dockerHost == "" {
		return true
	}
	return sameHost(strings.TrimSpace(confirmedClientIP), dockerHost)
}

func successMessageForSessionCheck(discoveryStatus, discoveryReason string) string {
	if strings.TrimSpace(discoveryStatus) == "blocked" && strings.TrimSpace(discoveryReason) != "" {
		return fmt.Sprintf("recheck success with blocked reason=%s", discoveryReason)
	}
	return "recheck success"
}

func extractHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		host, _, splitErr := net.SplitHostPort(raw)
		if splitErr == nil {
			return strings.TrimSpace(host)
		}
		return ""
	}
	host := parsed.Hostname()
	return strings.TrimSpace(host)
}

func sameHost(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
