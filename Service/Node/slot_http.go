package Node

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	BindDAO "private_browser_server/Dao/Bind"
	NodeDAO "private_browser_server/Dao/Node"
	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	BindRepo "private_browser_server/Repository/Bind"
	NodeRepo "private_browser_server/Repository/Node"
)

// ListClientSlots 返回中心当前缓存的某个节点 slot 明细。
//
// 设计来源：
// - `slot_reconcile` 已经把 slot 明细正式写入 `edge_client_slots`；
// - 如果没有查询接口，前端和管理员仍然只能直接查 SQLite，不利于后续治理页面和排障 API 收口；
// - 这条接口只返回中心缓存，不直接穿透到 Client，避免查询接口偷偷变成远端探测接口。
func ListClientSlots(ctx *gin.Context) {
	clientID := ctx.Param("clientId")
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	repo := NodeRepo.NewRepository()
	node, err := repo.GetByClientID(requestCtx, clientID)
	if err == NodeRepo.ErrNotFound {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "edge client not found")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}

	items, err := repo.ListSlotsByClientID(requestCtx, clientID)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}

	HttpResponse.ResponseSuccess(ctx, &NodeModel.EdgeClientSlotListResponse{
		ClientID:            node.ClientID,
		TargetSlotCount:     node.TargetSlotCount,
		ActualSlotCount:     node.ActualSlotCount,
		AvailableSlotCount:  node.AvailableSlotCount,
		RunningSlotCount:    node.RunningSlotCount,
		SlotExceptionStatus: node.SlotExceptionStatus,
		SlotExceptionReason: node.SlotExceptionReason,
		LastSlotCheckedAt:   node.LastSlotCheckedAt,
		Items:               items,
		Total:               len(items),
	})
}

// SetTargetSlotCount 允许管理员在平台正式接口接入前，先手工维护节点目标 slot 数。
//
// 职责边界：
// - 这里只更新中心 `target_slot_count` 和由此推导出的异常摘要；
// - 不直接调用 Client 创建 / 删除 slot；
// - 不修改 `actual_slot_count`，实际值仍由 `slot_reconcile` 和 Client 事实决定。
func SetTargetSlotCount(ctx *gin.Context) {
	clientID := ctx.Param("clientId")
	var request NodeModel.SetTargetSlotCountRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "set-target-slot-count request body 非法")
		return
	}
	if request.TargetSlotCount <= 0 {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "targetSlotCount 必须大于 0")
		return
	}

	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	repo := NodeRepo.NewRepository()
	node, err := repo.GetByClientID(requestCtx, clientID)
	if err == NodeRepo.ErrNotFound {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "edge client not found")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}

	now := time.Now().Unix()
	slotExceptionStatus := "normal"
	slotExceptionReason := ""
	if node.ActualSlotCount != request.TargetSlotCount {
		slotExceptionStatus = "exception"
		slotExceptionReason = fmt.Sprintf("target_slot_count=%d actual_slot_count=%d", request.TargetSlotCount, node.ActualSlotCount)
	}
	if err = repo.UpdateTargetSlotCount(requestCtx, clientID, request.TargetSlotCount, slotExceptionStatus, slotExceptionReason, now); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}

	_ = repo.CreateSlotLog(requestCtx, &NodeDAO.SlotLogRow{
		ClientID:  clientID,
		Action:    "update_target_slot_count",
		Result:    "success",
		Message:   fmt.Sprintf("target_slot_count updated to %d source=%s", request.TargetSlotCount, request.Source),
		CreatedAt: now,
	})
	_ = BindRepo.NewRepository().CreateLog(requestCtx, &BindDAO.LogRow{
		ClientID:      clientID,
		MainAccountID: node.MainAccountID,
		ClientIP:      node.ClientIP,
		Action:        "update_target_slot_count",
		Result:        "success",
		Message:       fmt.Sprintf("target_slot_count=%d source=%s", request.TargetSlotCount, request.Source),
		CreatedAt:     now,
	})

	HttpResponse.ResponseSuccess(ctx, &NodeModel.SetTargetSlotCountResponse{
		ClientID:            clientID,
		TargetSlotCount:     request.TargetSlotCount,
		ActualSlotCount:     node.ActualSlotCount,
		AvailableSlotCount:  node.AvailableSlotCount,
		RunningSlotCount:    node.RunningSlotCount,
		SlotExceptionStatus: slotExceptionStatus,
		SlotExceptionReason: slotExceptionReason,
		UpdatedAt:           now,
	})
}
