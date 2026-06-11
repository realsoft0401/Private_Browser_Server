package Env

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	envDao "private_browser_server/Dao/Env"
	taskDao "private_browser_server/Dao/Task"
	EdgeAPI "private_browser_server/EdgeClient"
	"private_browser_server/Middleware/PlatformContext"
	taskModel "private_browser_server/Models/Task"
	"private_browser_server/Pkg/HttpResponse"
	"private_browser_server/Pkg/TaskStream"
	NodeService "private_browser_server/Service/Node"
)

// RunEnv 在中心层创建“启动环境包”任务，并立即返回中心 taskId/eventsUrl。
//
// 设计来源：
//   - Edge run 自身不会替调用方自动拉镜像，缺镜像时必须明确失败；
//   - 用户要求 Node Server 在 run 前必须先检查目标 runtime.image 是否已在 Client 本机存在，
//     如果不存在，就先调用 Edge `/api/v1/edge/docker/pull-image`；
//   - 同时用户要求 run 作为任务接口，前端能通过 SSE 看到 image_check / pulling_image / edge_run 全过程，
//     因此这里改成“快速返回 + 后台任务编排 + 中心 SSE 事件流”。
//
// 职责边界：
// - HTTP 入口只负责校验、创建 server task、返回 eventsUrl，不在请求线程里长时间阻塞；
// - 后台任务会为同一次 run 做受控镜像预检，不会偷偷改 runtime.image，也不会自动选择别的镜像；
// - pull-image 或 Edge run 失败时，本次 run 必须 failed，不自动重试，调用方修复后再重新发起新任务。
func RunEnv(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	param, ok := bindOptionalRunRequest(ctx)
	if !ok {
		return
	}
	env, err := envDao.NewModelHandler().GetServerBrowserEnvByID(
		ctx.Request.Context(),
		platformCtx.MainAccountID,
		strings.TrimSpace(ctx.Param("envId")),
	)
	if err != nil {
		writeServiceError(ctx, mapDaoError(err))
		return
	}
	client, err := NodeService.EnsureClientReadyForBusiness(ctx.Request.Context(), platformCtx.MainAccountID, env.ClientID)
	if err != nil {
		respondClientNotReady(ctx, err)
		return
	}
	task := newServerTask(platformCtx, env.ClientID, env.EnvID, taskModel.TaskTypeRunEnv, buildServerTaskEventsURL(ctx, ""))
	task.EventsURL = buildServerTaskEventsURL(ctx, task.TaskID)
	if err = taskDao.NewModelHandler().CreateServerTask(ctx.Request.Context(), task); err != nil {
		writeServiceError(ctx, internalError("创建中心 run 任务失败: "+err.Error()))
		return
	}
	if err = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(
		ctx.Request.Context(),
		platformCtx.MainAccountID,
		env.EnvID,
		task.TaskID,
		"",
		task.UpdatedAt,
	); err != nil {
		_ = failServerTask(ctx, task, "更新环境包最近任务摘要失败: "+err.Error())
		writeServiceError(ctx, mapDaoError(err))
		return
	}
	TaskStream.Ensure(task.TaskID)
	TaskStream.Emit(task.TaskID, "queued", taskModel.TaskStatusPending, "queued", "环境包启动任务已创建，等待 Node Server 后台执行", map[string]any{
		"clientId": client.ID,
		"envId":    env.EnvID,
	})

	taskCopy := *task
	paramCopy := *param
	go runEnvTaskAsync(platformCtx, client.BaseURL, env.EnvID, &taskCopy, &paramCopy)

	HttpResponse.ResponseSuccess(ctx, buildStartTaskResponse(task, "环境包启动任务已创建"))
}

// StopEnv 在中心层创建“停止环境包”任务，并绑定 Edge stop task。
//
// 当前 stop 仍然遵守统一前置条件：必须先通过 verified/healthy/heartbeat 检查，
// 不因为它是“收尾动作”就绕过节点状态门槛，避免带病节点造成中心事实进一步失真。
func StopEnv(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	param, ok := bindOptionalStopRequest(ctx)
	if !ok {
		return
	}
	env, err := envDao.NewModelHandler().GetServerBrowserEnvByID(
		ctx.Request.Context(),
		platformCtx.MainAccountID,
		strings.TrimSpace(ctx.Param("envId")),
	)
	if err != nil {
		writeServiceError(ctx, mapDaoError(err))
		return
	}
	client, err := NodeService.EnsureClientReadyForBusiness(ctx.Request.Context(), platformCtx.MainAccountID, env.ClientID)
	if err != nil {
		respondClientNotReady(ctx, err)
		return
	}
	task := newServerTask(platformCtx, env.ClientID, env.EnvID, taskModel.TaskTypeStopEnv, buildServerTaskEventsURL(ctx, ""))
	task.EventsURL = buildServerTaskEventsURL(ctx, task.TaskID)
	if err = taskDao.NewModelHandler().CreateServerTask(ctx.Request.Context(), task); err != nil {
		writeServiceError(ctx, internalError("创建中心 stop 任务失败: "+err.Error()))
		return
	}
	if err = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(
		ctx.Request.Context(),
		platformCtx.MainAccountID,
		env.EnvID,
		task.TaskID,
		"",
		task.UpdatedAt,
	); err != nil {
		_ = failServerTask(ctx, task, "更新环境包最近任务摘要失败: "+err.Error())
		writeServiceError(ctx, mapDaoError(err))
		return
	}

	edgeResp, err := EdgeAPI.New().StopBrowserEnvTask(ctx.Request.Context(), client.BaseURL, "", env.EnvID, param)
	if err != nil {
		message := "调用 Edge 停止环境包失败: " + err.Error()
		_ = failServerTask(ctx, task, message)
		_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx.Request.Context(), platformCtx.MainAccountID, env.EnvID, task.TaskID, message, time.Now().Unix())
		writeServiceError(ctx, mapEdgeActionError("停止环境包", err))
		return
	}
	applyEdgeStartToServerTask(task, edgeResp)
	if err = taskDao.NewModelHandler().UpdateServerTask(ctx.Request.Context(), task); err != nil {
		writeServiceError(ctx, internalError("保存中心 stop 任务 Edge 绑定失败: "+err.Error()))
		return
	}

	HttpResponse.ResponseSuccess(ctx, buildStartTaskResponse(task, "环境包停止任务已创建"))
}
