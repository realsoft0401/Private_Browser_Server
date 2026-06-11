package Env

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	envDao "private_browser_server/Dao/Env"
	taskDao "private_browser_server/Dao/Task"
	EdgeAPI "private_browser_server/EdgeClient"
	"private_browser_server/Middleware/PlatformContext"
	envModel "private_browser_server/Models/Env"
	taskModel "private_browser_server/Models/Task"
	"private_browser_server/Pkg/HttpResponse"
	NodeService "private_browser_server/Service/Node"
)

// DeleteEnvImage 代理删除环境包关联的运行镜像。
//
// 设计来源：
// - 用户今天已经确认 `/del` 只删除运行镜像，不销毁环境包资产；
// - Edge 当前把 `/del` 设计成同步接口，因此 Node Server 继续保持同步返回，避免人为再包一层假异步；
// - 该动作仍必须先经过 verified/healthy 校验，不能因为“只是删镜像”就绕过节点准入。
//
// 职责边界：
// - 只代理 Edge `/api/v1/edge/browser-envs/:envId/del`，不创建中心 task；
// - 不修改中心环境包主状态，只在有 warning 时回写 lastError 方便排障；
// - 真正的镜像删除事实仍以目标 Edge 的 Docker 结果为准。
func DeleteEnvImage(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
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

	result, err := EdgeAPI.New().DeleteBrowserEnvImage(ctx.Request.Context(), client.BaseURL, "", env.EnvID)
	if err != nil {
		writeServiceError(ctx, mapEdgeActionError("删除环境包关联镜像", err))
		return
	}

	if result != nil && strings.TrimSpace(result.WarningMessage) != "" {
		env.LastError = strings.TrimSpace(result.WarningMessage)
		env.UpdatedAt = firstPositive(result.DeletedAt, time.Now().Unix())
		if updateErr := envDao.NewModelHandler().UpdateServerBrowserEnvSnapshot(ctx.Request.Context(), env); updateErr != nil {
			writeServiceError(ctx, internalError("回写中心环境包 warning 摘要失败: "+updateErr.Error()))
			return
		}
	}

	HttpResponse.ResponseSuccess(ctx, buildDeleteEnvImageResponse(env, result))
}

// DeleteEnvPackage 在中心层创建“彻底销毁环境包资产”任务，并绑定 Edge `/package` task。
//
// 设计来源：
// - `/package` 会删除 Edge 本机环境目录、browser-data/profile 和 SQLite 索引，属于不可逆资产动作；
// - 用户要求这类动作必须落中心 task，保证任务详情、最终结论和审计都留在 Node Server；
// - Edge task 丢失后也不能默认成功，必须由中心层根据“环境已不存在”或明确状态再次确认。
//
// 职责边界：
// - HTTP 入口只负责校验、创建中心 task、绑定 Edge task 并快速返回；
// - 真正的终态同步由 Server task detail/SSE 读取时推进，不在请求线程里等待删除完成；
// - 成功后中心缓存会保留一条 status=deleted 的聚合记录，不因为 Edge 物理删除就丢掉中心历史。
func DeleteEnvPackage(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
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

	task := newServerTask(platformCtx, env.ClientID, env.EnvID, taskModel.TaskTypeDeleteEnvPackage, buildServerTaskEventsURL(ctx, ""))
	task.EventsURL = buildServerTaskEventsURL(ctx, task.TaskID)
	if err = taskDao.NewModelHandler().CreateServerTask(ctx.Request.Context(), task); err != nil {
		writeServiceError(ctx, internalError("创建中心 package 删除任务失败: "+err.Error()))
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

	edgeResp, err := EdgeAPI.New().DeleteBrowserEnvPackageTask(ctx.Request.Context(), client.BaseURL, "", env.EnvID)
	if err != nil {
		message := "调用 Edge 销毁环境包失败: " + err.Error()
		_ = failServerTask(ctx, task, message)
		_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx.Request.Context(), platformCtx.MainAccountID, env.EnvID, task.TaskID, message, time.Now().Unix())
		writeServiceError(ctx, mapEdgeActionError("销毁环境包", err))
		return
	}

	applyEdgeStartToServerTask(task, edgeResp)
	if err = taskDao.NewModelHandler().UpdateServerTask(ctx.Request.Context(), task); err != nil {
		writeServiceError(ctx, internalError("保存中心 package 删除任务 Edge 绑定失败: "+err.Error()))
		return
	}

	HttpResponse.ResponseSuccess(ctx, buildStartTaskResponse(task, "环境包销毁任务已创建"))
}

func buildDeleteEnvImageResponse(env *envModel.ServerBrowserEnv, edgeResp *EdgeAPI.DeleteBrowserEnvImageResponse) *envModel.DeleteEnvImageResponse {
	resp := &envModel.DeleteEnvImageResponse{}
	if env != nil {
		resp.EnvID = strings.TrimSpace(env.EnvID)
		resp.ClientID = strings.TrimSpace(env.ClientID)
	}
	if edgeResp == nil {
		resp.DeletedAt = time.Now().Unix()
		return resp
	}

	resp.EnvID = runTaskFirstMessage(strings.TrimSpace(edgeResp.EnvID), resp.EnvID)
	resp.Image = strings.TrimSpace(edgeResp.Image)
	resp.ImageRemoved = edgeResp.ImageRemoved
	resp.WarningMessage = strings.TrimSpace(edgeResp.WarningMessage)
	resp.DeletedAt = firstPositive(edgeResp.DeletedAt, time.Now().Unix())
	if len(edgeResp.Results) > 0 {
		resp.Results = make([]envModel.DockerImageRemoveItem, 0, len(edgeResp.Results))
		for _, item := range edgeResp.Results {
			resp.Results = append(resp.Results, envModel.DockerImageRemoveItem{
				Image:    strings.TrimSpace(item.Image),
				Deleted:  strings.TrimSpace(item.Deleted),
				Untagged: strings.TrimSpace(item.Untagged),
			})
		}
	}
	return resp
}

func firstPositive(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
