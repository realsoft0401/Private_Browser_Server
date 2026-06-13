package Node

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"private_browser_server/Middleware/PlatformContext"
	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
)

// ConfirmNodeAddressUpdate 处理管理员确认节点接入地址变化。
//
// 设计来源：
// - 节点进入 blocked + ip_mismatch 后，用户要求必须有正式恢复出口，不能靠手工改库；
// - 该出口的职责不是“强行放行”，而是“在管理员确认这还是同一台机器后，更新最新地址并重新完整探测”；
// - clientId 仍保持不变，历史任务和环境绑定不迁移到新身份。
//
// 职责边界：
// - 只允许处理 blocked + ip_mismatch 节点；
// - 只更新 baseUrl/clientIp/dockerApiUrl 这三类接入地址事实；
// - 更新后必须立即重新执行完整校验，通过才恢复 verified，否则继续保持 blocked。
func ConfirmNodeAddressUpdate(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	var req confirmAddressUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "请求体格式错误，需要 JSON: {\"baseUrl\":\"http://新的ClientIP:3300\",\"clientIp\":\"新的ClientIP\",\"dockerApiUrl\":\"http://新的ClientIP:2375\"}")
		return
	}
	baseURL, err := normalizeHTTPURL(req.BaseURL)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "baseUrl 非法: "+err.Error()+"；示例 http://192.168.10.120:3300")
		return
	}
	dockerURL, err := normalizeHTTPURL(req.DockerAPIURL)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "dockerApiUrl 非法: "+err.Error()+"；示例 http://192.168.10.120:2375")
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
	if conflictMessage := validateAddressConfirmPrecondition(node); conflictMessage != "" {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeConflict, conflictMessage)
		return
	}

	previous := *node
	node.BaseURL = baseURL
	node.DockerAPIURL = dockerURL
	node.ClientIP = ""
	fillNodeClientIPIfMissing(node, req.ClientIP)
	node.DiscoveryStatus = NodeModel.NodeDiscoveryBlocked
	node.DiscoveryReason = NodeModel.NodeDiscoveryReasonNone
	node.LastError = ""

	checks, failed, healthStatus, message, nextAction := runVerifyChecks(ctx.Request.Context(), node, true)
	if failed {
		node.DiscoveryStatus = NodeModel.NodeDiscoveryBlocked
		node.DiscoveryReason = NodeModel.NodeDiscoveryReasonNone
		if err = persistAddressConfirmFailure(ctx, repo, node, healthStatus, message, nextAction); err != nil {
			return
		}
		attachNodeGovernanceActions(node)
		ctx.JSON(200, &HttpResponse.ResponseData{
			Code: HttpResponse.CodeServerBusy,
			Msg:  "确认地址更新失败: " + message,
			Data: verifyResponse{
				Client:     node,
				ClientID:   node.ID,
				NextAction: nextAction,
				Checks:     checks,
			},
		})
		return
	}
	if changed, detail := detectDeviceFactChangeAfterAddressConfirm(&previous, node); changed {
		node.DiscoveryStatus = NodeModel.NodeDiscoveryBlocked
		node.DiscoveryReason = NodeModel.NodeDiscoveryReasonDeviceFactChanged
		node.LastError = detail
		if err = repo.UpdateAddressConfirmResult(ctx.Request.Context(), node); err != nil {
			HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "保存地址确认后的设备事实冲突结果失败: "+err.Error())
			return
		}
		attachNodeGovernanceActions(node)
		ctx.JSON(200, &HttpResponse.ResponseData{
			Code: HttpResponse.CodeConflict,
			Msg:  "确认地址更新失败: 设备事实与原节点不一致",
			Data: verifyResponse{
				Client:     node,
				ClientID:   node.ID,
				NextAction: "请确认新的接入地址是否仍然指向原节点；如果确实发生了设备重置或换机，不要继续沿用原 clientId",
				Checks:     checks,
			},
		})
		return
	}
	if err = repo.UpdateAddressConfirmResult(ctx.Request.Context(), node); err != nil {
		if strings.Contains(err.Error(), "constraint failed") || strings.Contains(err.Error(), "UNIQUE") {
			HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeConflict, "新的 baseUrl 已被其它活动节点占用，不能把两个 clientId 绑定到同一接入地址")
			return
		}
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "保存地址确认成功结果失败: "+err.Error())
		return
	}
	attachNodeGovernanceActions(node)
	HttpResponse.ResponseSuccess(ctx, verifyResponse{Client: node, Checks: checks})
}

// validateAddressConfirmPrecondition 收口“确认地址更新”接口的前置状态。
//
// 该接口只服务一个非常明确的场景：管理员确认 blocked + ip_mismatch 仍然是原节点。
// 如果节点只是普通 blocked、已经 verified，或是别的原因被拦住，都必须拒绝，避免接口职责继续膨胀。
func validateAddressConfirmPrecondition(node *NodeModel.EdgeClient) string {
	if node == nil {
		return "Edge Client 不存在，无法确认地址更新"
	}
	if node.DiscoveryStatus != NodeModel.NodeDiscoveryBlocked {
		return fmt.Sprintf("当前 discoveryStatus=%s，不允许走地址确认流程；只有 blocked 节点才需要管理员确认更新地址", node.DiscoveryStatus)
	}
	if strings.TrimSpace(node.DiscoveryReason) != NodeModel.NodeDiscoveryReasonIPMismatch {
		return fmt.Sprintf("当前 discoveryReason=%s，不允许走地址确认流程；该接口只处理 ip_mismatch", strings.TrimSpace(node.DiscoveryReason))
	}
	return ""
}

// detectDeviceFactChangeAfterAddressConfirm 判断地址确认后的设备事实是否还能连续指向原节点。
//
// 设计来源：
// - 用户明确要求：IP 更新后必须重新探测，只有设备事实仍然匹配原节点，才能恢复 verified；
// - 当前模型里还没有 hostname 等更完整的指纹，因此先用最稳定、最不容易误判的宿主事实做最小保护；
// - 这里只比较历史记录里已有的稳定字段，旧记录为空时不强行判冲突，避免误伤老节点。
func detectDeviceFactChangeAfterAddressConfirm(previous *NodeModel.EdgeClient, current *NodeModel.EdgeClient) (bool, string) {
	if previous == nil || current == nil {
		return false, ""
	}
	mismatches := make([]string, 0, 4)
	appendMismatch := func(field string, before string, after string) {
		before = strings.TrimSpace(before)
		after = strings.TrimSpace(after)
		if before == "" || after == "" || before == after {
			return
		}
		mismatches = append(mismatches, fmt.Sprintf("%s: %s -> %s", field, before, after))
	}
	appendIntMismatch := func(field string, before int64, after int64) {
		if before <= 0 || after <= 0 || before == after {
			return
		}
		mismatches = append(mismatches, fmt.Sprintf("%s: %d -> %d", field, before, after))
	}

	appendMismatch("os", previous.OS, current.OS)
	appendMismatch("arch", previous.Arch, current.Arch)
	appendIntMismatch("cpuCores", int64(previous.CPUCores), int64(current.CPUCores))
	appendIntMismatch("memoryTotalMb", previous.MemoryTotalMB, current.MemoryTotalMB)
	if len(mismatches) == 0 {
		return false, ""
	}
	return true, "地址更新后重新探测到的设备事实与原节点不一致，已标记 blocked，discoveryReason=device_fact_changed；" + strings.Join(mismatches, "；")
}

func persistAddressConfirmFailure(ctx *gin.Context, repo NodeRepo.Repository, node *NodeModel.EdgeClient, healthStatus string, message string, nextAction string) error {
	node.HealthStatus = healthStatus
	node.DiscoveryStatus = NodeModel.NodeDiscoveryBlocked
	node.DiscoveryReason = NodeModel.NodeDiscoveryReasonNone
	node.LastError = message + "；" + nextAction
	if err := repo.UpdateAddressConfirmResult(ctx.Request.Context(), node); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "保存地址确认失败结果失败: "+err.Error())
		return err
	}
	return nil
}
