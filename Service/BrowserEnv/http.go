package BrowserEnv

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	BrowserEnvModel "private_browser_server/Models/BrowserEnv"
	"private_browser_server/Pkg/HttpResponse"
	BrowserEnvRepo "private_browser_server/Repository/BrowserEnv"
)

// Create 是中心 browser-env 创建接口的 HTTP 入口。
//
// 当前 create 是同步短链路：Node 根据 clientId 找到目标 Client，调用 Edge 创建环境包，
// 成功后立即写入中心缓存，不返回 taskId，也不使用 SSE。
func Create(c *gin.Context) {
	var request BrowserEnvModel.CreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "browser-env create request body 非法，请检查 clientId/userId/rpaType/runtime.image")
		return
	}

	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := NewService().Create(requestCtx, &request)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCreateParams):
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, err.Error())
		default:
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, err.Error())
		}
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// ImportPackage 是中心 browser-env import-package 的 HTTP 入口。
//
// 它是 SSE 任务接口：HTTP 只负责接收 tgz 和目标 clientId，立即返回中心 taskId/eventsUrl；
// 后台负责转发 Edge、等待 Edge task 终态，并在成功后写入 server_browser_envs。
func ImportPackage(c *gin.Context) {
	clientID := strings.TrimSpace(c.PostForm("clientId"))
	if clientID == "" {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "clientId 不能为空")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "缺少导入文件字段 file")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInternalError, "打开导入文件失败，请检查上传内容")
		return
	}
	defer file.Close()

	result, err := NewService().ImportPackage(c.Request.Context(), clientID, fileHeader.Filename, file)
	if err != nil {
		if errors.Is(err, io.EOF) {
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "导入文件为空")
			return
		}
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeRemoteError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// List 返回中心当前缓存的 browser-env 列表。
func List(c *gin.Context) {
	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := NewService().List(requestCtx, BrowserEnvModel.ListQuery{
		AccountID: c.Query("accountId"),
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

// UpdateRuntimeImage 是中心 browser-env 运行镜像修改接口的 HTTP 入口。
//
// 当前它是同步短链路，不创建中心 task：
// - 只解析 envId + image；
// - 业务准入、节点校验、Edge 调用和中心缓存刷新放在 Service；
// - 不把它伪装成 SSE 接口，避免调用方把配置修改和真正 run 混在一起。
func UpdateRuntimeImage(c *gin.Context) {
	var request BrowserEnvModel.UpdateRuntimeImageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInvalidParams, "browser-env runtime-image request body 非法，请检查 image")
		return
	}

	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	result, err := NewService().UpdateRuntimeImage(requestCtx, c.Param("envId"), &request)
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

// Revalidate 是中心 browser-env revalidate 接口的 HTTP 入口。
//
// 当前 revalidate 是 task + SSE：
// - HTTP 立即返回中心 taskId/eventsUrl；
// - 后台调用 Edge 正式 revalidate 并等待 Edge task 终态；
// - 成功后刷新中心缓存，失败后写入 task 错误和 env last_error。
func Revalidate(c *gin.Context) {
	result, err := NewService().Revalidate(c.Request.Context(), c.Param("envId"))
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
