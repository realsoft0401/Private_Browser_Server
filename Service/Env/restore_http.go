package Env

import (
	"context"
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
	NodeModel "private_browser_server/Models/Node"
	taskModel "private_browser_server/Models/Task"
	"private_browser_server/Pkg/HttpResponse"
	"private_browser_server/Pkg/TaskStream"
	NodeService "private_browser_server/Service/Node"
)

// RestoreEnv 在中心层创建“从本机备份包恢复环境目录”任务，并用同步 Edge restore 结果直接收口。
//
// 设计来源：
// - 用户已经确认 restore 是 backup 的配对正式资产动作，必须先有中心 task、错误留痕和 SSE 事件；
// - 当前 Edge restore 是同步接口，不返回 edgeTaskId，因此 Node Server 不能伪装成“先创建异步任务再等待”；
// - 同时 restore 只能读取 Edge SQLite 已登记的 backupPath，不能让 Node Server 或前端重新上传同一份备份包。
//
// 职责边界：
// - HTTP 入口负责中心 env 查询、Client ready 校验、Edge detail 预检、创建 server task 和最终 success/failed 收口；
// - 不直接读取 Edge SQLite、备份 tar 或环境目录，只以 Edge HTTP 同步返回为恢复事实；
// - restore 成功后只把中心缓存推进回 created，不自动 run、不自动拉镜像，也不把 Edge 本地备份路径复制进中心表。
func RestoreEnv(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}

	env, client, snapshot, err := prepareRestoreEnvAction(ctx, platformCtx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	task := newServerTask(platformCtx, env.ClientID, env.EnvID, taskModel.TaskTypeRestoreEnv, buildServerTaskEventsURL(ctx, ""))
	task.EventsURL = buildServerTaskEventsURL(ctx, task.TaskID)
	if err = taskDao.NewModelHandler().CreateServerTask(ctx.Request.Context(), task); err != nil {
		writeServiceError(ctx, internalError("创建中心 restore 任务失败: "+err.Error()))
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
	TaskStream.Emit(task.TaskID, "queued", taskModel.TaskStatusPending, "queued", "环境包恢复任务已创建，等待 Node Server 执行", map[string]any{
		"clientId": client.ID,
		"envId":    env.EnvID,
	})
	TaskStream.Emit(task.TaskID, "running", taskModel.TaskStatusRunning, "edge_precheck", "Edge 环境包预检通过，开始调用 Edge restore", map[string]any{
		"clientId":        client.ID,
		"envId":           env.EnvID,
		"status":          strings.TrimSpace(snapshot.Status),
		"containerStatus": strings.TrimSpace(snapshot.ContainerStatus),
	})
	TaskStream.Emit(task.TaskID, "running", taskModel.TaskStatusRunning, "edge_restore", "Node Server 正在调用 Edge restore，同步等待 created 结果", map[string]any{
		"clientId": client.ID,
		"envId":    env.EnvID,
	})

	result, err := EdgeAPI.New().RestoreBrowserEnv(ctx.Request.Context(), client.BaseURL, "", env.EnvID)
	if err != nil {
		message := "调用 Edge 恢复环境包失败: " + err.Error()
		now := time.Now().Unix()
		_ = failServerTask(ctx, task, message)
		_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx.Request.Context(), platformCtx.MainAccountID, env.EnvID, task.TaskID, message, now)
		TaskStream.Emit(task.TaskID, "error", taskModel.TaskStatusFailed, "edge_restore", message, map[string]any{
			"clientId": client.ID,
			"envId":    env.EnvID,
		})
		writeServiceError(ctx, mapEdgeActionError("恢复环境包", err))
		return
	}

	if err = finalizeRestoreSuccess(ctx, env, task, result); err != nil {
		message := "中心 restore 成功收口失败: " + err.Error()
		_ = failServerTask(ctx, task, message)
		_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx.Request.Context(), platformCtx.MainAccountID, env.EnvID, task.TaskID, message, time.Now().Unix())
		TaskStream.Emit(task.TaskID, "error", taskModel.TaskStatusFailed, "finalize", message, map[string]any{"envId": env.EnvID})
		writeServiceError(ctx, internalError(message))
		return
	}

	HttpResponse.ResponseSuccess(ctx, buildStartTaskResponse(task, "环境包恢复完成"))
}

// prepareRestoreEnvAction 统一准备 restore 所需的中心 env、目标 client 和 Edge 当前快照。
//
// 设计来源：
// - restore 只能从 Edge 已登记的本机备份恢复，入口阶段必须先确认环境归属、节点可用和 Edge 当前状态；
// - 这里单独抽出准备逻辑，是为了把“查中心索引”和“读 Edge 详情做准入”清晰分层，避免请求入口继续膨胀；
// - 该函数只负责 restore 前置校验，不创建 task，也不直接恢复目录。
func prepareRestoreEnvAction(ctx *gin.Context, platformCtx PlatformContext.Context) (*envModel.ServerBrowserEnv, *NodeModel.EdgeClient, *edgeRunBrowserEnvIndex, error) {
	env, err := envDao.NewModelHandler().GetServerBrowserEnvByID(
		ctx.Request.Context(),
		platformCtx.MainAccountID,
		strings.TrimSpace(ctx.Param("envId")),
	)
	if err != nil {
		return nil, nil, nil, mapDaoError(err)
	}
	client, err := NodeService.EnsureClientReadyForBusiness(ctx.Request.Context(), platformCtx.MainAccountID, env.ClientID)
	if err != nil {
		return nil, nil, nil, err
	}
	snapshot, err := fetchRestoreBrowserEnvIndexSnapshot(ctx.Request.Context(), client.BaseURL, env.EnvID)
	if err != nil {
		return nil, nil, nil, remoteError("读取 Edge 环境包详情失败，无法判断是否允许 restore: " + err.Error())
	}
	if err = validateRestoreSnapshot(snapshot); err != nil {
		return nil, nil, nil, err
	}
	return env, client, snapshot, nil
}

// fetchRestoreBrowserEnvIndexSnapshot 通过 Edge 正式详情接口读取 restore 所需的生命周期快照。
//
// 这个函数与 backup 的详情读取分开保留，是为了让后续 restore 如果需要额外校验 backupPath/archived 语义时不影响其它动作；
// 它不猜测默认值，缺少 index 就直接失败。
func fetchRestoreBrowserEnvIndexSnapshot(ctx context.Context, baseURL string, envID string) (*edgeRunBrowserEnvIndex, error) {
	var detail edgeRunBrowserEnvSnapshotDetail
	if err := EdgeAPI.New().DoJSON(ctx, baseURL, http.MethodGet, "/api/v1/edge/browser-envs/"+url.PathEscape(envID), "", nil, &detail); err != nil {
		return nil, err
	}
	if detail.Index == nil {
		return nil, internalError("Edge 环境包详情缺少 index，Node Server 无法判断是否允许 restore")
	}
	return detail.Index, nil
}

// validateRestoreSnapshot 锁住 restore 允许的 Edge 生命周期状态。
//
// restore 的职责是把 `backed_up/archived` 重新恢复为本机可运行目录；
// 如果目录本来就在，例如 `created/stopped/running`，中心层必须直接拒绝，而不是让调用方误以为 restore 可以重复覆盖运行目录。
func validateRestoreSnapshot(snapshot *edgeRunBrowserEnvIndex) error {
	if snapshot == nil {
		return internalError("Edge 环境包详情为空，无法判断是否允许 restore")
	}
	status := strings.TrimSpace(snapshot.Status)
	switch status {
	case envModel.EnvStatusBackedUp, envModel.EnvStatusArchived:
		return nil
	case envModel.EnvStatusCreated:
		return conflictError("环境包当前状态为 created，说明运行目录已经存在，不需要重复 restore")
	case envModel.EnvStatusStopped:
		return conflictError("环境包当前状态为 stopped，说明运行目录已经存在，不需要 restore")
	case envModel.EnvStatusRunning:
		return conflictError("环境包当前状态为 running，说明运行目录已经存在，不能执行 restore")
	case envModel.EnvStatusDeleted:
		return conflictError("环境包当前状态为 deleted，不能执行 restore")
	case envModel.EnvStatusError:
		return conflictError("环境包当前状态为 error，请先修复环境一致性后再 restore")
	default:
		return conflictError("环境包当前状态为 " + firstNonEmpty(status, "unknown") + "，当前不允许执行 restore")
	}
}

// finalizeRestoreSuccess 把 Edge restore 的同步成功结果收口为中心 task + env 摘要事实。
//
// 只有 Edge 明确返回 `status=created`，中心才允许把 restore 记为成功；
// 这里负责恢复环境聚合到 created，但不证明浏览器已经启动，后续仍需显式 run。
func finalizeRestoreSuccess(ctx *gin.Context, env *envModel.ServerBrowserEnv, task *taskModel.ServerTask, result *EdgeAPI.RestoreBrowserEnvResponse) error {
	if env == nil || task == nil {
		return internalError("restore 收口失败：中心环境或任务对象为空")
	}
	if result == nil {
		return internalError("Edge restore 成功响应为空，中心无法确认 created 事实")
	}
	if strings.TrimSpace(result.Status) != envModel.EnvStatusCreated {
		return conflictError("Edge restore 未返回 created 状态，中心拒绝把任务记为成功")
	}

	now := firstPositive(result.RestoredAt, time.Now().Unix())
	task.Status = taskModel.TaskStatusSuccess
	task.ErrorMessage = ""
	task.UpdatedAt = now
	task.FinishedAt = now
	if err := taskDao.NewModelHandler().UpdateServerTask(ctx.Request.Context(), task); err != nil {
		return err
	}

	env.Status = envModel.EnvStatusCreated
	env.ContainerStatus = envModel.EnvFactUnknown
	env.MonitorStatus = envModel.EnvFactUnknown
	env.CDPURL = ""
	env.WebVNCURL = ""
	env.LastTaskID = task.TaskID
	env.LastError = ""
	env.UpdatedAt = now
	if err := envDao.NewModelHandler().UpdateServerBrowserEnvSnapshot(ctx.Request.Context(), env); err != nil {
		return err
	}

	TaskStream.Emit(task.TaskID, "done", taskModel.TaskStatusSuccess, "finalize", firstNonEmpty(strings.TrimSpace(result.Message), "环境包恢复完成"), map[string]any{
		"envId":      env.EnvID,
		"status":     result.Status,
		"envPath":    strings.TrimSpace(result.EnvPath),
		"restoredAt": result.RestoredAt,
	})
	return nil
}
