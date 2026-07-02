package Node

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	NodeDAO "private_browser_server/Dao/Node"
	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
	EdgeClientService "private_browser_server/Service/EdgeClient"
)

var slotIDPattern = regexp.MustCompile(`^slot[0-9]{3}$`)

// CreateClientSlot 是 Node 管理端新增 Client slot 的正式入口。
//
// 设计来源：
// - Client 才是本机 slot 资源事实源，Node 不能只改 `edge_client_slots` 假装创建成功；
// - 但 Admin 页面也不能直接绕过 Node 调 Client，所以这里由 Node 代理调用 Edge `/api/v1/edge/slots`；
// - 用户已经明确 slot 命名收口为 `slot001/slot002` 这类三位编号，因此 Node 层先做一次协议校验。
//
// 职责边界：
// - 这里只做单个 slot 的同步新增，不做批量扩容；
// - 新增成功后重新全量读取 Client slots，并把 `target_slot_count` 同步为动作后的实际数量；
// - 这样表达“管理员本次新增就是调整目标容量”，避免立刻触发 target/actual 不一致导致 run admission 被拦。
func CreateClientSlot(ctx *gin.Context) {
	clientID := strings.TrimSpace(ctx.Param("clientId"))
	var request NodeModel.CreateClientSlotRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "create client slot request body 非法")
		return
	}
	slotID := strings.TrimSpace(request.SlotID)
	if !slotIDPattern.MatchString(slotID) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "slotId 必须使用 slot001 这类三位编号")
		return
	}

	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 30*time.Second)
	defer cancel()

	node, ok := loadHealthyVerifiedNode(ctx, requestCtx, clientID)
	if !ok {
		return
	}
	if _, err := EdgeClientService.New().CreateSlot(requestCtx, node.BaseURL, &EdgeClientService.CreateSlotRequest{SlotID: slotID}); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, fmt.Sprintf("create client slot failed: %v", err))
		return
	}

	response, err := refreshSlotCacheFromClient(requestCtx, node.ClientID, node.BaseURL, -1)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}
	response.Action = "create_slot"
	response.SlotID = slotID
	_ = NodeRepo.NewRepository().CreateSlotLog(requestCtx, &NodeDAO.SlotLogRow{
		ClientID:  node.ClientID,
		SlotID:    slotID,
		Action:    "create_slot",
		Result:    "success",
		Message:   firstNonEmpty(strings.TrimSpace(request.Source), "admin create slot"),
		CreatedAt: response.UpdatedAt,
	})

	HttpResponse.ResponseSuccess(ctx, response)
}

// DeleteClientSlot 是 Node 管理端删除 Client slot 的正式入口。
//
// 设计来源：
// - 删除真实 slot 必须先让 Client 执行本机 Docker/SQLite 清理；
// - Node 删除成功后再全量对账，确保 `edge_client_slots` 没有残留；
// - 默认 `force=false`，页面只允许删除 waiting slot，避免把运行中的账号环境误删。
//
// 职责边界：
// - 这里只删除单个 slot，不做“按目标数自动批量缩容”；
// - 删除成功后把 `target_slot_count` 同步为新的实际数量，表示管理员本次删除就是调整目标容量；
// - 如果 Edge 因 slot 正在运行而拒绝，Node 直接透出错误，不绕过状态机。
func DeleteClientSlot(ctx *gin.Context) {
	clientID := strings.TrimSpace(ctx.Param("clientId"))
	slotID := strings.TrimSpace(ctx.Param("slotId"))
	var request NodeModel.DeleteClientSlotRequest
	if err := ctx.ShouldBindJSON(&request); err != nil && strings.TrimSpace(err.Error()) != "EOF" {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "delete client slot request body 非法")
		return
	}
	if !slotIDPattern.MatchString(slotID) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "slotId 必须使用 slot001 这类三位编号")
		return
	}

	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 30*time.Second)
	defer cancel()

	node, ok := loadHealthyVerifiedNode(ctx, requestCtx, clientID)
	if !ok {
		return
	}
	if _, err := EdgeClientService.New().DestroySlot(requestCtx, node.BaseURL, slotID, &EdgeClientService.DestroySlotRequest{Force: request.Force}); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, fmt.Sprintf("delete client slot failed: %v", err))
		return
	}

	response, err := refreshSlotCacheFromClient(requestCtx, node.ClientID, node.BaseURL, -1)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}
	response.Action = "delete_slot"
	response.SlotID = slotID
	_ = NodeRepo.NewRepository().CreateSlotLog(requestCtx, &NodeDAO.SlotLogRow{
		ClientID:  node.ClientID,
		SlotID:    slotID,
		Action:    "delete_slot",
		Result:    "success",
		Message:   firstNonEmpty(strings.TrimSpace(request.Source), "admin delete slot"),
		CreatedAt: response.UpdatedAt,
	})

	HttpResponse.ResponseSuccess(ctx, response)
}

// loadHealthyVerifiedNode 统一校验 slot 管理动作的节点前置条件。
//
// slot 新增/删除会真实修改 Client 本机资源，因此不能在 discovered/stale/offline/unhealthy 状态下带病操作。
// 这里集中校验，避免 create/delete/reinit 后续各写一份不同错误口径。
func loadHealthyVerifiedNode(ctx *gin.Context, requestCtx context.Context, clientID string) (*NodeModel.EdgeClient, bool) {
	if strings.TrimSpace(clientID) == "" {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "clientId 不能为空")
		return nil, false
	}
	node, err := NodeRepo.NewRepository().GetByClientID(requestCtx, clientID)
	if err == NodeRepo.ErrNotFound {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "edge client not found")
		return nil, false
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return nil, false
	}
	if node.HealthStatus != "healthy" || node.DiscoveryStatus != "verified" {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, "edge client is not healthy and verified")
		return nil, false
	}
	return node, true
}
