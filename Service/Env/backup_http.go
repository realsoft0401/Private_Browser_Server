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

// BackupEnv 在中心层创建“备份并释放运行目录”任务，并用同步 Edge backup 结果直接收口。
//
// 设计来源：
// - 用户已经明确确认 backup 是正式资产动作，必须先留下中心 task、错误痕迹和 SSE 事件，再调用 Edge；
// - 当前 Edge backup 是同步接口，不返回 edgeTaskId，因此 Node Server 不能沿用 stop/package 的“绑定 Edge task 后再等 detail 收口”模式；
// - 同时用户要求 backup 前必须先做 Edge env detail 预检，running/error/backed_up/deleted 等状态要在中心层直接拒绝。
//
// 职责边界：
// - HTTP 入口负责中心 env 查询、Client ready 校验、Edge detail 预检、创建 server task 和最终 success/failed 收口；
// - 不直接读取 Edge SQLite、备份包目录或 tar.gz 内容，只以 Edge HTTP 返回的同步结果为事实；
// - backup 成功后只把中心缓存推进到 backed_up，不自动 restore、不自动 run，也不把 backup 资产明细复制进中心表。
func BackupEnv(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}

	env, client, snapshot, err := prepareBackupEnvAction(ctx, platformCtx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}

	task := newServerTask(platformCtx, env.ClientID, env.EnvID, taskModel.TaskTypeBackupEnv, buildServerTaskEventsURL(ctx, ""))
	task.EventsURL = buildServerTaskEventsURL(ctx, task.TaskID)
	if err = taskDao.NewModelHandler().CreateServerTask(ctx.Request.Context(), task); err != nil {
		writeServiceError(ctx, internalError("创建中心 backup 任务失败: "+err.Error()))
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
	TaskStream.Emit(task.TaskID, "queued", taskModel.TaskStatusPending, "queued", "环境包备份任务已创建，等待 Node Server 执行", map[string]any{
		"clientId": client.ID,
		"envId":    env.EnvID,
	})
	TaskStream.Emit(task.TaskID, "running", taskModel.TaskStatusRunning, "edge_precheck", "Edge 环境包预检通过，开始调用 Edge backup", map[string]any{
		"clientId":        client.ID,
		"envId":           env.EnvID,
		"status":          strings.TrimSpace(snapshot.Status),
		"containerStatus": strings.TrimSpace(snapshot.ContainerStatus),
	})
	TaskStream.Emit(task.TaskID, "running", taskModel.TaskStatusRunning, "edge_backup", "Node Server 正在调用 Edge backup，同步等待 backed_up 结果", map[string]any{
		"clientId": client.ID,
		"envId":    env.EnvID,
	})

	result, err := EdgeAPI.New().BackupBrowserEnv(ctx.Request.Context(), client.BaseURL, "", env.EnvID)
	if err != nil {
		message := "调用 Edge 备份环境包失败: " + err.Error()
		now := time.Now().Unix()
		_ = failServerTask(ctx, task, message)
		_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx.Request.Context(), platformCtx.MainAccountID, env.EnvID, task.TaskID, message, now)
		TaskStream.Emit(task.TaskID, "error", taskModel.TaskStatusFailed, "edge_backup", message, map[string]any{
			"clientId": client.ID,
			"envId":    env.EnvID,
		})
		writeServiceError(ctx, mapEdgeActionError("备份环境包", err))
		return
	}

	if err = finalizeBackupSuccess(ctx, env, task, result); err != nil {
		message := "中心 backup 成功收口失败: " + err.Error()
		_ = failServerTask(ctx, task, message)
		_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx.Request.Context(), platformCtx.MainAccountID, env.EnvID, task.TaskID, message, time.Now().Unix())
		TaskStream.Emit(task.TaskID, "error", taskModel.TaskStatusFailed, "finalize", message, map[string]any{"envId": env.EnvID})
		writeServiceError(ctx, internalError(message))
		return
	}

	HttpResponse.ResponseSuccess(ctx, buildStartTaskResponse(task, "环境包备份完成"))
}

// prepareBackupEnvAction 统一准备 backup 所需的中心 env、目标 client 和 Edge 当前快照。
//
// 设计来源：
// - backup 是正式资产动作，入口阶段必须先把“中心归属 + 节点准入 + Edge 实时状态”三件事对齐；
// - 这里单独抽出来，是为了避免 HTTP 入口被查询和预检细节淹没，也方便后续补测试或加更细的前置校验；
// - 该函数只做准备与拒绝，不创建 task，也不修改任何中心摘要。
func prepareBackupEnvAction(ctx *gin.Context, platformCtx PlatformContext.Context) (*envModel.ServerBrowserEnv, *NodeModel.EdgeClient, *edgeRunBrowserEnvIndex, error) {
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
	snapshot, err := fetchEdgeBrowserEnvIndexSnapshot(ctx.Request.Context(), client.BaseURL, env.EnvID)
	if err != nil {
		return nil, nil, nil, remoteError("读取 Edge 环境包详情失败，无法判断是否允许 backup: " + err.Error())
	}
	if err = validateBackupSnapshot(snapshot); err != nil {
		return nil, nil, nil, err
	}
	return env, client, snapshot, nil
}

// fetchEdgeBrowserEnvIndexSnapshot 读取 Edge 环境包详情里的 index 视图，作为 backup 准入事实。
//
// Node Server 不能直接翻 Edge SQLite 或目录，因此这里必须通过正式 HTTP API 读取；
// 如果 Edge 没有返回 index，说明当前详情响应不足以支撑资产动作判断，中心层宁可失败也不猜测继续。
func fetchEdgeBrowserEnvIndexSnapshot(ctx context.Context, baseURL string, envID string) (*edgeRunBrowserEnvIndex, error) {
	var detail edgeRunBrowserEnvSnapshotDetail
	if err := EdgeAPI.New().DoJSON(ctx, baseURL, http.MethodGet, "/api/v1/edge/browser-envs/"+url.PathEscape(envID), "", nil, &detail); err != nil {
		return nil, err
	}
	if detail.Index == nil {
		return nil, internalError("Edge 环境包详情缺少 index，Node Server 无法判断是否允许 backup")
	}
	return detail.Index, nil
}

// validateBackupSnapshot 锁住 backup 允许的 Edge 生命周期状态。
//
// 设计来源：
// - 用户已经明确 backup 是“归档并释放运行目录”，因此只有 `created/stopped` 能进入；
// - 这里拒绝 running/error/backed_up/deleted 等状态，避免把“先 stop/先修复/先 restore”的业务责任推迟到更后面才爆雷；
// - 该判断只负责 backup 准入，不替代其它接口自己的状态机。
func validateBackupSnapshot(snapshot *edgeRunBrowserEnvIndex) error {
	if snapshot == nil {
		return internalError("Edge 环境包详情为空，无法判断是否允许 backup")
	}
	status := strings.TrimSpace(snapshot.Status)
	switch status {
	case envModel.EnvStatusCreated, envModel.EnvStatusStopped:
		return nil
	case envModel.EnvStatusRunning:
		return conflictError("环境包当前状态为 running，必须先 stop 后再 backup")
	case envModel.EnvStatusBackedUp:
		return conflictError("环境包当前状态为 backed_up，请先 restore 后再重新 backup")
	case envModel.EnvStatusDeleted:
		return conflictError("环境包当前状态为 deleted，不能执行 backup")
	case envModel.EnvStatusError:
		return conflictError("环境包当前状态为 error，请先修复并 revalidate 后再 backup")
	case envModel.EnvStatusArchived:
		return conflictError("环境包当前状态为 archived，不能执行 backup")
	default:
		return conflictError("环境包当前状态为 " + firstNonEmpty(status, "unknown") + "，当前不允许执行 backup")
	}
}

// finalizeBackupSuccess 把 Edge backup 的同步成功结果收口为中心 task + env 摘要事实。
//
// 这里的成功判定必须看到 Edge 明确返回 `status=backed_up`；
// 一旦中心 task 或 env 摘要回写失败，就不能把本次 backup 记为成功，避免 task 事实与环境列表状态分裂。
func finalizeBackupSuccess(ctx *gin.Context, env *envModel.ServerBrowserEnv, task *taskModel.ServerTask, result *EdgeAPI.BackupBrowserEnvResponse) error {
	if env == nil || task == nil {
		return internalError("backup 收口失败：中心环境或任务对象为空")
	}
	if result == nil {
		return internalError("Edge backup 成功响应为空，中心无法确认 backed_up 事实")
	}
	if strings.TrimSpace(result.Status) != envModel.EnvStatusBackedUp {
		return conflictError("Edge backup 未返回 backed_up 状态，中心拒绝把任务记为成功")
	}

	now := firstPositive(result.BackupAt, time.Now().Unix())
	task.Status = taskModel.TaskStatusSuccess
	task.ErrorMessage = ""
	task.UpdatedAt = now
	task.FinishedAt = now
	if err := taskDao.NewModelHandler().UpdateServerTask(ctx.Request.Context(), task); err != nil {
		return err
	}

	env.Status = envModel.EnvStatusBackedUp
	env.ContainerStatus = "missing"
	env.MonitorStatus = envModel.EnvFactUnknown
	env.CDPURL = ""
	env.WebVNCURL = ""
	env.LastTaskID = task.TaskID
	env.LastError = ""
	env.UpdatedAt = now
	if err := envDao.NewModelHandler().UpdateServerBrowserEnvSnapshot(ctx.Request.Context(), env); err != nil {
		return err
	}

	TaskStream.Emit(task.TaskID, "done", taskModel.TaskStatusSuccess, "finalize", firstNonEmpty(strings.TrimSpace(result.Message), "环境包备份完成"), map[string]any{
		"envId":          env.EnvID,
		"status":         result.Status,
		"backupPath":     strings.TrimSpace(result.BackupPath),
		"backupChecksum": strings.TrimSpace(result.BackupChecksum),
		"backupSize":     result.BackupSize,
		"backupAt":       result.BackupAt,
	})
	return nil
}
