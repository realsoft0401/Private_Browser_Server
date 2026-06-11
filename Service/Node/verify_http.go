package Node

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"private_browser_server/Middleware/PlatformContext"
	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
)

// VerifyNode 对已注册 Edge Client 执行 verified 验证。
//
// 设计来源：用户要求 Node Server 必须区分“发现/注册/验证/业务放行”。
// verify 会按固定顺序检查 UDP 心跳、Client /health、Client device-info、Docker 2375 和架构事实；
// 只有全部通过才写 discoveryStatus=verified，任何失败都只写 lastError，不允许带病进入业务动作。
func VerifyNode(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	repo := NodeRepo.Repository{}
	node, err := repo.GetByID(ctx.Request.Context(), platformCtx.MainAccountID, strings.TrimSpace(ctx.Param("clientId")))
	if errors.Is(err, NodeRepo.ErrNotFound) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "Edge Client 不存在或不属于当前主账号")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "读取 Edge Client 失败: "+err.Error())
		return
	}

	checks := make(map[string]verifyCheck)
	now := time.Now().Unix()
	attachHeartbeatStatus(node, now)
	if node.HeartbeatStatus != NodeModel.NodeHeartbeatOnline {
		message := fmt.Sprintf("UDP 心跳不是 online，当前 heartbeatStatus=%s，lastHeartbeatAt=%d", node.HeartbeatStatus, node.LastHeartbeatAt)
		checks["heartbeat"] = verifyCheck{Status: "failed", Message: message}
		failVerify(ctx, repo, node, checks, NodeModel.NodeHealthStale, message, "请先确认 Client 容器运行、UDP discovery 可达，再重新调用 verify")
		return
	}
	checks["heartbeat"] = verifyCheck{Status: "passed", Message: "UDP 心跳在线"}

	health, err := verifyClientHealth(ctx.Request.Context(), node.BaseURL)
	if err != nil {
		message := "Client /health 不可达或返回异常: " + err.Error()
		checks["clientHealth"] = verifyCheck{Status: "failed", Message: message}
		failVerify(ctx, repo, node, checks, NodeModel.NodeHealthStale, message, "请检查 baseUrl、3300 端口、防火墙和 Client 服务进程")
		return
	}
	if !health.OK || strings.TrimSpace(health.Status) != NodeModel.NodeHealthHealthy {
		message := fmt.Sprintf("Client /health 不是 healthy，ok=%v status=%s", health.OK, health.Status)
		checks["clientHealth"] = verifyCheck{Status: "failed", Message: message}
		failVerify(ctx, repo, node, checks, NodeModel.NodeHealthUnhealthy, message, "请先修复 Client /health 中失败的 checks")
		return
	}
	checks["clientHealth"] = verifyCheck{Status: "passed", Message: "Client /health healthy"}

	deviceInfo, err := verifyClientDeviceInfo(ctx.Request.Context(), node.BaseURL)
	if err != nil {
		message := "Client device-info 不可达或解析失败: " + err.Error()
		checks["clientDeviceInfo"] = verifyCheck{Status: "failed", Message: message}
		failVerify(ctx, repo, node, checks, NodeModel.NodeHealthUnhealthy, message, "请确认 Client /api/v1/edge/device-info 可用")
		return
	}
	checks["clientDeviceInfo"] = verifyCheck{Status: "passed", Message: "Client device-info 可用"}

	probe, err := probeDocker(ctx.Request.Context(), node.DockerAPIURL)
	if err != nil {
		message := "Docker 2375 探测失败: " + err.Error()
		checks["docker2375"] = verifyCheck{Status: "failed", Message: message}
		failVerify(ctx, repo, node, checks, NodeModel.NodeHealthUnhealthy, message, "请检查 dockerApiUrl、Docker daemon 2375、防火墙和内网访问权限")
		return
	}
	checks["docker2375"] = verifyCheck{Status: "passed", Message: "Docker 2375 可用"}

	deviceArch := normalizeArch(firstNonEmpty(deviceInfo.DeviceArch, deviceInfo.DeviceRawArch))
	if probe.Arch == NodeModel.NodeArchUnknown || deviceArch == NodeModel.NodeArchUnknown {
		message := fmt.Sprintf("架构无法归一化，deviceArch=%s dockerArch=%s", firstNonEmpty(deviceInfo.DeviceArch, deviceInfo.DeviceRawArch), probe.RawArch)
		checks["arch"] = verifyCheck{Status: "failed", Message: message}
		failVerify(ctx, repo, node, checks, NodeModel.NodeHealthUnhealthy, message, "请确认 Client 和 Docker 返回 amd64/x86_64 或 arm64/aarch64")
		return
	}
	if deviceArch != probe.Arch {
		message := fmt.Sprintf("Client device-info 架构与 Docker 2375 架构不一致，deviceArch=%s dockerArch=%s", deviceArch, probe.Arch)
		checks["arch"] = verifyCheck{Status: "failed", Message: message}
		failVerify(ctx, repo, node, checks, NodeModel.NodeHealthUnhealthy, message, "请检查 baseUrl/dockerApiUrl 是否指向同一台 Client 服务器")
		return
	}
	checks["arch"] = verifyCheck{Status: "passed", Message: "架构已归一化为 " + probe.Arch}

	node.OS = probe.OS
	node.Arch = probe.Arch
	node.CPUCores = probe.CPUCores
	node.MemoryTotalMB = probe.MemoryTotalMB
	node.DockerVersion = probe.DockerVersion
	node.HealthStatus = NodeModel.NodeHealthHealthy
	node.DiscoveryStatus = NodeModel.NodeDiscoveryVerified
	node.LastCheckedAt = now
	node.LastError = ""
	node.UpdatedAt = now
	attachHeartbeatStatus(node, now)
	if err = repo.UpdateVerifyResult(ctx.Request.Context(), node); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "保存 verify 成功结果失败: "+err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, verifyResponse{Client: node, Checks: checks})
}

// EnsureClientReadyForBusiness 是所有业务动作前必须复用的 Client 放行校验。
//
// 放行条件来自用户确认的三段状态：healthStatus=healthy、discoveryStatus=verified、
// heartbeatStatus=online。这个函数只读数据库并动态计算 heartbeatStatus，不主动探测、
// 不自动 verify、不修正状态；不满足条件时调用方必须拒绝业务动作。
func EnsureClientReadyForBusiness(ctx context.Context, mainAccountID string, clientID string) (*NodeModel.EdgeClient, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, &BusinessReadyError{
			Message:    "缺少 clientId，无法判断目标 Edge Client 是否允许业务动作",
			NextAction: "请先从 /api/v1/edge-clients 或 /api/v1/edge-clients/discovered 选择已绑定 Client",
		}
	}
	node, err := (NodeRepo.Repository{}).GetByID(ctx, mainAccountID, clientID)
	if errors.Is(err, NodeRepo.ErrNotFound) {
		return nil, &BusinessReadyError{
			Message:    "Edge Client 不存在或不属于当前主账号",
			NextAction: "请确认 Platform Header 和 clientId 是否匹配",
		}
	}
	if err != nil {
		return nil, err
	}
	attachHeartbeatStatus(node, time.Now().Unix())
	if node.HealthStatus != NodeModel.NodeHealthHealthy {
		return nil, &BusinessReadyError{
			Message:    fmt.Sprintf("Client healthStatus=%s，不允许业务动作", node.HealthStatus),
			NextAction: "请先调用 POST /api/v1/edge-clients/{clientId}/verify，或根据 lastError 修复 Client",
		}
	}
	if node.DiscoveryStatus != NodeModel.NodeDiscoveryVerified {
		return nil, &BusinessReadyError{
			Message:    fmt.Sprintf("Client discoveryStatus=%s，尚未 verified", node.DiscoveryStatus),
			NextAction: "请先调用 POST /api/v1/edge-clients/{clientId}/verify",
		}
	}
	if node.HeartbeatStatus != NodeModel.NodeHeartbeatOnline {
		return nil, &BusinessReadyError{
			Message:    fmt.Sprintf("Client heartbeatStatus=%s，Node Server 不能确认 Client 当前在线", node.HeartbeatStatus),
			NextAction: "请检查 Client 容器和 UDP discovery，再调用 verify",
		}
	}
	if node.Arch != NodeModel.NodeArchAMD64 && node.Arch != NodeModel.NodeArchARM64 {
		return nil, &BusinessReadyError{
			Message:    fmt.Sprintf("Client arch=%s，架构不可用于业务镜像策略", node.Arch),
			NextAction: "请修复 Docker/device-info 架构识别后重新 verify",
		}
	}
	if strings.TrimSpace(node.BaseURL) == "" || strings.TrimSpace(node.DockerAPIURL) == "" {
		return nil, &BusinessReadyError{
			Message:    "Client baseUrl 或 dockerApiUrl 为空，不允许业务动作",
			NextAction: "请重新注册 Client 或补充受控更新接口后再 verify",
		}
	}
	if strings.TrimSpace(node.LastError) != "" {
		return nil, &BusinessReadyError{
			Message:    "Client lastError 未清空，不允许业务动作: " + node.LastError,
			NextAction: "请先修复错误并重新调用 verify",
		}
	}
	return node, nil
}

// failVerify 统一收口 verify 失败写库和响应。
//
// 失败时绝不能把 discoveryStatus 改成 verified；这里只写 healthStatus、lastCheckedAt、lastError，
// 并保留当前 discoveryStatus，方便管理员修复后重新调用 verify。
func failVerify(ctx *gin.Context, repo NodeRepo.Repository, node *NodeModel.EdgeClient, checks map[string]verifyCheck, healthStatus string, message string, nextAction string) {
	now := time.Now().Unix()
	node.HealthStatus = healthStatus
	node.LastCheckedAt = now
	node.LastError = message + "；" + nextAction
	node.UpdatedAt = now
	attachHeartbeatStatus(node, now)
	if err := repo.UpdateVerifyResult(ctx.Request.Context(), node); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "保存 verify 失败结果失败: "+err.Error())
		return
	}
	ctx.JSON(http.StatusOK, &HttpResponse.ResponseData{
		Code: HttpResponse.CodeServerBusy,
		Msg:  "Client 验证失败: " + message,
		Data: verifyResponse{
			ClientID:   node.ID,
			Client:     node,
			NextAction: nextAction,
			Checks:     checks,
		},
	})
}
