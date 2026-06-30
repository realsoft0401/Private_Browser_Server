package BrowserEnv

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	BrowserEnvModel "private_browser_server/Models/BrowserEnv"
	"private_browser_server/Pkg/HttpResponse"
	BrowserEnvRepo "private_browser_server/Repository/BrowserEnv"
)

// List 返回中心当前缓存的 browser-env 列表。
func List(c *gin.Context) {
	accountID := strings.TrimSpace(c.Query("accountId"))
	if accountID == "" {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "accountId 不能为空")
		return
	}

	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := NewService().List(requestCtx, BrowserEnvModel.ListQuery{
		AccountID: accountID,
		ClientID:  c.Query("clientId"),
		UserID:    c.Query("userId"),
		RPAType:   c.Query("rpaType"),
		Status:    c.Query("status"),
	})
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInternalError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// GetDetail 返回中心当前缓存的单条 browser-env 摘要。
func GetDetail(c *gin.Context) {
	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := NewService().GetDetail(requestCtx, c.Param("envId"))
	if err != nil {
		switch {
		case errors.Is(err, BrowserEnvRepo.ErrNotFound):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server browser env not found")
		default:
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInternalError, err.Error())
		}
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// Run 是中心 browser-env run 接口的 HTTP 入口。
//
// HTTP 层职责边界：
// - 只负责解析 `envId` 和 JSON 请求体；
// - 真正的中心准入、任务创建、Edge 调用和任务收口全部留在 Service；
// - 这样后续 stop/backup/restore 可以复用同一套服务层结构，不会在路由层分叉出第二套编排逻辑。
func Run(c *gin.Context) {
	var request modelRunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "browser-env run request body 非法，请检查 slotId")
		return
	}

	result, err := NewService().Run(c.Request.Context(), c.Param("envId"), request.toModel())
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidParams):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, err.Error())
		case errors.Is(err, BrowserEnvRepo.ErrNotFound):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server browser env not found")
		default:
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, err.Error())
		}
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// Stop 是中心 browser-env stop 接口的 HTTP 入口。
//
// 当前 stop 采用同步 HTTP：
// - 发起后直接等待 Edge stop 完成；
// - 同步返回 stop 结果；
// - 同时在中心落一条 `server_task` 审计事实，但不要求调用方继续订阅 SSE。
func Stop(c *gin.Context) {
	var request BrowserEnvModel.StopRequest
	if err := c.ShouldBindJSON(&request); err != nil && !errors.Is(err, context.Canceled) {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "browser-env stop request body 非法，请检查 timeoutSeconds")
		return
	}

	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := NewService().Stop(requestCtx, c.Param("envId"), &request)
	if err != nil {
		switch {
		case errors.Is(err, BrowserEnvRepo.ErrNotFound):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server browser env not found")
		default:
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, err.Error())
		}
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// Backup 是中心 browser-env backup 接口的 HTTP 入口。
//
// 当前 backup 被明确定义为中心长链路任务：
// - 立即返回中心 taskId/eventsUrl；
// - 真正的打包、目录释放和缓存收口在后台继续执行；
// - 调用方必须继续订阅中心 SSE，不能把“接单成功”当成“备份完成”。
func Backup(c *gin.Context) {
	result, err := NewService().Backup(c.Request.Context(), c.Param("envId"))
	if err != nil {
		switch {
		case errors.Is(err, BrowserEnvRepo.ErrNotFound):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server browser env not found")
		default:
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, err.Error())
		}
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// Restore 是中心 browser-env restore 接口的 HTTP 入口。
//
// restore 与 backup 一样是多阶段长链路动作，因此中心层也统一收口到 task + SSE。
func Restore(c *gin.Context) {
	result, err := NewService().Restore(c.Request.Context(), c.Param("envId"))
	if err != nil {
		switch {
		case errors.Is(err, BrowserEnvRepo.ErrNotFound):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server browser env not found")
		default:
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, err.Error())
		}
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// DeletePackage 是中心 browser-env package delete 接口的 HTTP 入口。
//
// 这条接口会真正销毁目标节点上的 env 资产，因此仍采用 task + SSE，
// 避免把“接单成功”误解成“已经删除完成”。
func DeletePackage(c *gin.Context) {
	result, err := NewService().DeletePackage(c.Request.Context(), c.Param("envId"))
	if err != nil {
		switch {
		case errors.Is(err, BrowserEnvRepo.ErrNotFound):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server browser env not found")
		default:
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, err.Error())
		}
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// DeleteImage 是中心 browser-env `/del` 接口的 HTTP 入口。
//
// 当前正式口径下，`/del` 只做镜像清理，因此继续保持同步 HTTP，
// 不创建中心 task，也不要求调用方订阅 SSE。
func DeleteImage(c *gin.Context) {
	result, err := NewService().DeleteImage(c.Request.Context(), c.Param("envId"))
	if err != nil {
		switch {
		case errors.Is(err, BrowserEnvRepo.ErrNotFound):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server browser env not found")
		default:
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, err.Error())
		}
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// Refresh 让 Node 主动向目标 Edge 拉一次 env 详情，并刷新中心缓存。
func Refresh(c *gin.Context) {
	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	result, err := NewService().Refresh(requestCtx, c.Param("envId"))
	if err != nil {
		switch {
		case errors.Is(err, BrowserEnvRepo.ErrNotFound):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server browser env not found")
		default:
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, err.Error())
		}
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

type modelRunRequest struct {
	SlotID        string `json:"slotId"`
	ForceRecreate bool   `json:"forceRecreate"`
}

func (r modelRunRequest) toModel() *BrowserEnvModel.RunRequest {
	return &BrowserEnvModel.RunRequest{
		SlotID:        r.SlotID,
		ForceRecreate: r.ForceRecreate,
	}
}
