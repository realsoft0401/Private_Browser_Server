package Node

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	BindDAO "private_browser_server/Dao/Bind"
	QuotaDAO "private_browser_server/Dao/Quota"
	QuotaModel "private_browser_server/Models/Quota"
	"private_browser_server/Pkg/HttpResponse"
	BindRepo "private_browser_server/Repository/Bind"
	NodeRepo "private_browser_server/Repository/Node"
	QuotaRepo "private_browser_server/Repository/Quota"
)

// GetRunQuota 返回最近一次可信平台额度快照，并附带当前 run admission 判断。
func GetRunQuota(ctx *gin.Context) {
	clientID := ctx.Param("clientId")
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	node, quota, err := loadNodeAndQuota(requestCtx, clientID)
	if err == NodeRepo.ErrNotFound {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "edge client not found")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}

	response := buildRunQuotaResponse(node.ClientID, quota, evaluateRunAdmission(node, quota))
	HttpResponse.ResponseSuccess(ctx, response)
}

// RefreshRunQuota 在平台正式接口接入前，先允许管理员手工写入最新额度快照。
//
// ******** 平台端接口接入说明：
// 后续平台提供正式 quota API 后，这里应改成：
// 1. Node 调平台接口拉取最新额度
// 2. 校验平台返回值
// 3. 再写入 `client_run_quotas`
// 当前这版只为了让 run admission 主链先落地，不是最终平台协议。
func RefreshRunQuota(ctx *gin.Context) {
	clientID := ctx.Param("clientId")
	var request QuotaModel.RefreshRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "run-quota refresh request body 非法")
		return
	}
	if request.QuotaLimit < 0 || request.QuotaUsedSnapshot < 0 || request.QuotaAvailableSnapshot < 0 {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "quota snapshot 不能为负数")
		return
	}
	if request.Status == "" {
		request.Status = "valid"
	}

	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	node, _, err := loadNodeAndQuota(requestCtx, clientID)
	if err == NodeRepo.ErrNotFound {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "edge client not found")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}

	now := time.Now().Unix()
	if request.ExpiresAt == 0 {
		request.ExpiresAt = now + 300
	}
	if err = QuotaRepo.NewRepository().Upsert(requestCtx, &QuotaDAO.Row{
		ClientID:               clientID,
		QuotaLimit:             request.QuotaLimit,
		QuotaUsedSnapshot:      request.QuotaUsedSnapshot,
		QuotaAvailableSnapshot: request.QuotaAvailableSnapshot,
		FetchedAt:              now,
		ExpiresAt:              request.ExpiresAt,
		Status:                 request.Status,
		LastError:              request.LastError,
	}); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}

	quota, err := QuotaRepo.NewRepository().GetByClientID(requestCtx, clientID)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}

	_ = BindRepo.NewRepository().CreateLog(requestCtx, &BindDAO.LogRow{
		ClientID:      clientID,
		MainAccountID: node.MainAccountID,
		ClientIP:      node.ClientIP,
		Action:        "refresh_run_quota",
		Result:        "success",
		Message:       fmt.Sprintf("quota_limit=%d quota_available_snapshot=%d source=%s", quota.QuotaLimit, quota.QuotaAvailableSnapshot, request.Source),
		CreatedAt:     now,
	})

	HttpResponse.ResponseSuccess(ctx, buildRunQuotaResponse(clientID, quota, evaluateRunAdmission(node, quota)))
}

// buildRunQuotaResponse 把额度快照和准入判断拼成统一响应。
//
// 职责边界：
// - 不重新计算 admission，只消费已经算好的结果；
// - quota 缺失时明确返回 `status=untrusted`，避免调用方误以为“空额度 = 允许 run”；
// - 这样查询接口、刷新接口和后续诊断接口都能复用同一套响应口径。
func buildRunQuotaResponse(clientID string, quota *QuotaModel.ClientRunQuota, admission QuotaModel.AdmissionResult) *QuotaModel.RunQuotaResponse {
	response := &QuotaModel.RunQuotaResponse{
		ClientID:  clientID,
		Status:    "untrusted",
		LastError: "",
		Admission: admission,
	}
	if quota == nil {
		return response
	}
	response.QuotaLimit = quota.QuotaLimit
	response.QuotaUsedSnapshot = quota.QuotaUsedSnapshot
	response.QuotaAvailableSnapshot = quota.QuotaAvailableSnapshot
	response.FetchedAt = quota.FetchedAt
	response.ExpiresAt = quota.ExpiresAt
	response.Status = quota.Status
	response.LastError = quota.LastError
	return response
}

var _ = sql.ErrNoRows
