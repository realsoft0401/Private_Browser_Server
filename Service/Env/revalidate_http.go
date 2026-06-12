package Env

import (
	"net/http"
	"net/url"
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

// RevalidateEnv 在中心层创建“异常环境重新准入”任务，并绑定 Edge revalidate task。
//
// 设计来源：
// - 用户已经明确 `status=error` 不能靠 run/stop/proxy update 等普通动作隐式恢复，必须走独立正式接口；
// - Client 侧 revalidate 是标准 SSE task，会在后台检查原子材料、Docker 身份冲突和端口重分配，因此 Node Server 必须保留 edgeTaskId；
// - 这类动作失败后不能自动重试，也不能默认成功，Edge task 丢失时必须再用环境详情确认是否真的恢复到可准入状态。
//
// 职责边界：
// - HTTP 入口只负责中心 env 查询、Client ready 校验、Edge detail 预检、创建中心 task、绑定 Edge task 并快速返回；
// - 不直接修复文件、容器或登录态，只通过 Edge API 发起受控重新校验；
// - 最终成功/失败和中心缓存刷新由 task detail / SSE 读取时继续收口，不在请求线程里长时间等待终态。
func RevalidateEnv(ctx *gin.Context) {
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

	snapshot, err := fetchRevalidateBrowserEnvIndexSnapshot(ctx, client.BaseURL, env.EnvID)
	if err != nil {
		writeServiceError(ctx, remoteError("读取 Edge 环境包详情失败，无法判断是否允许 revalidate: "+err.Error()))
		return
	}
	if err = validateRevalidateSnapshot(snapshot); err != nil {
		writeServiceError(ctx, err)
		return
	}

	task := newServerTask(platformCtx, env.ClientID, env.EnvID, taskModel.TaskTypeRevalidateEnv, buildServerTaskEventsURL(ctx, ""))
	task.EventsURL = buildServerTaskEventsURL(ctx, task.TaskID)
	if err = taskDao.NewModelHandler().CreateServerTask(ctx.Request.Context(), task); err != nil {
		writeServiceError(ctx, internalError("创建中心 revalidate 任务失败: "+err.Error()))
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

	edgeResp, err := EdgeAPI.New().RevalidateBrowserEnvTask(ctx.Request.Context(), client.BaseURL, "", env.EnvID)
	if err != nil {
		message := "调用 Edge 重新校验环境包失败: " + err.Error()
		_ = failServerTask(ctx, task, message)
		_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx.Request.Context(), platformCtx.MainAccountID, env.EnvID, task.TaskID, message, time.Now().Unix())
		writeServiceError(ctx, mapEdgeActionError("重新校验环境包", err))
		return
	}

	applyEdgeStartToServerTask(task, edgeResp)
	if err = taskDao.NewModelHandler().UpdateServerTask(ctx.Request.Context(), task); err != nil {
		writeServiceError(ctx, internalError("保存中心 revalidate 任务 Edge 绑定失败: "+err.Error()))
		return
	}

	HttpResponse.ResponseSuccess(ctx, buildStartTaskResponse(task, "环境包重新校验任务已创建"))
}

// fetchRevalidateBrowserEnvIndexSnapshot 读取 Edge 当前环境快照，作为 revalidate 准入判断基础。
//
// revalidate 是“异常环境重新准入”，Node Server 必须先确认 Edge 视角里该环境确实还处在 error；
// 这里仍坚持只走 Edge 正式 API，不从中心缓存反推最终准入事实。
func fetchRevalidateBrowserEnvIndexSnapshot(ctx *gin.Context, baseURL string, envID string) (*edgeRunBrowserEnvIndex, error) {
	var detail edgeRunBrowserEnvSnapshotDetail
	if err := EdgeAPI.New().DoJSON(ctx.Request.Context(), baseURL, http.MethodGet, "/api/v1/edge/browser-envs/"+url.PathEscape(envID), "", nil, &detail); err != nil {
		return nil, err
	}
	if detail.Index == nil {
		return nil, internalError("Edge 环境包详情缺少 index，Node Server 无法判断是否允许 revalidate")
	}
	return detail.Index, nil
}

// validateRevalidateSnapshot 锁住 revalidate 的入口状态，只允许 error 环境进入。
//
// 设计来源：
// - 用户已经明确异常环境不能靠 run/stop 等普通动作带病恢复，必须走独立受控修复入口；
// - 因此这里拒绝 backed_up/archived/deleted 等状态，避免把 restore 与 revalidate 语义混在一起；
// - 真正是否恢复成功，仍以后续 Edge task 终态和详情回读为准。
func validateRevalidateSnapshot(snapshot *edgeRunBrowserEnvIndex) error {
	if snapshot == nil {
		return internalError("Edge 环境包详情为空，无法判断是否允许 revalidate")
	}
	status := strings.TrimSpace(snapshot.Status)
	switch status {
	case envModel.EnvStatusError:
		return nil
	case envModel.EnvStatusBackedUp:
		return conflictError("环境包当前状态为 backed_up，请先 restore 后再 revalidate")
	case envModel.EnvStatusArchived:
		return conflictError("环境包当前状态为 archived，请先 restore 后再 revalidate")
	case envModel.EnvStatusDeleted:
		return conflictError("环境包当前状态为 deleted，不能执行 revalidate")
	default:
		return conflictError("只有 status=error 的环境包允许 revalidate；当前状态为 " + firstNonEmpty(status, "unknown"))
	}
}
