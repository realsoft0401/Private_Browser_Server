package Env

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	EdgeAPI "private_browser_server/EdgeClient"
	"private_browser_server/Middleware/PlatformContext"
	envDao "private_browser_server/Dao/Env"
	taskDao "private_browser_server/Dao/Task"
	envModel "private_browser_server/Models/Env"
	taskModel "private_browser_server/Models/Task"
	"private_browser_server/Pkg/TaskStream"
)

// runEnvTaskAsync 在后台串行推进 Node Server 的 run 工作流，并把每个阶段写入中心 SSE 事件流。
//
// 设计来源：
// - `/envs/:envId/run` 需要快速返回 taskId，但镜像预检、pull-image、Edge run 都是长动作；
// - 仅靠前端轮询 task detail 无法及时看到“镜像预检中 / 拉镜像中 / Edge run 已创建”等细阶段；
// - 因此这里把 run 真正的执行逻辑移到 goroutine 中，由它统一更新 server task、env 摘要和 TaskStream。
//
// 职责边界：
// - 这里只编排一条 run 动作，不做自动重试，也不隐式恢复 error 环境；
// - 任务终态仍然只写 success/failed，避免中心层出现第三套长期状态机；
// - 如果 Edge task 查询失败或状态不可确认，本次 run 必须 failed，调用方需要重新发起新任务。
func runEnvTaskAsync(platformCtx PlatformContext.Context, clientBaseURL string, envID string, task *taskModel.ServerTask, param *envModel.RunEnvRequest) {
	reporter := func(event string, status string, stage string, message string, data map[string]any) {
		TaskStream.Emit(strings.TrimSpace(task.TaskID), event, status, stage, message, data)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			message := fmt.Sprintf("Node Server run 后台任务异常崩溃: %v", recovered)
			reporter("error", taskModel.TaskStatusFailed, "server_run", message, nil)
			_ = persistFailedServerTask(context.Background(), platformCtx.MainAccountID, envID, task, message)
		}
	}()

	backgroundCtx := context.Background()
	task.Status = taskModel.TaskStatusRunning
	task.UpdatedAt = time.Now().Unix()
	if err := taskDao.NewModelHandler().UpdateServerTask(backgroundCtx, task); err != nil {
		message := "更新中心 run 任务为 running 失败: " + err.Error()
		reporter("error", taskModel.TaskStatusFailed, "server_run", message, nil)
		_ = persistFailedServerTask(backgroundCtx, platformCtx.MainAccountID, envID, task, message)
		return
	}
	reporter("running", taskModel.TaskStatusRunning, "server_run", "Node Server 后台 run 工作流开始执行", map[string]any{
		"clientId": task.ClientID,
		"envId":    envID,
	})

	if err := ensureRuntimeImageReadyForRun(backgroundCtx, clientBaseURL, envID, reporter); err != nil {
		message := "运行前镜像预检失败: " + err.Error()
		reporter("error", taskModel.TaskStatusFailed, "image_check", message, nil)
		_ = persistFailedServerTask(backgroundCtx, platformCtx.MainAccountID, envID, task, message)
		return
	}

	reporter("progress", taskModel.TaskStatusRunning, "edge_run", "镜像预检完成，开始调用 Edge run", map[string]any{"envId": envID})
	edgeResp, err := EdgeAPI.New().StartBrowserEnvTask(backgroundCtx, clientBaseURL, "", envID, param)
	if err != nil {
		message := "调用 Edge 启动环境包失败: " + err.Error()
		reporter("error", taskModel.TaskStatusFailed, "edge_run", message, nil)
		_ = persistFailedServerTask(backgroundCtx, platformCtx.MainAccountID, envID, task, message)
		return
	}
	applyEdgeStartToServerTask(task, edgeResp)
	if err = taskDao.NewModelHandler().UpdateServerTask(backgroundCtx, task); err != nil {
		message := "保存中心 run 任务 Edge 绑定失败: " + err.Error()
		reporter("error", taskModel.TaskStatusFailed, "edge_run", message, nil)
		_ = persistFailedServerTask(backgroundCtx, platformCtx.MainAccountID, envID, task, message)
		return
	}
	reporter("progress", taskModel.TaskStatusRunning, "edge_run", "Edge run 任务已创建，开始等待最终结果", map[string]any{
		"edgeTaskId": task.EdgeTaskID,
		"edgeStatus": edgeResp.Status,
		"eventsUrl":  task.EventsURL,
	})

	if err = waitRunEdgeTaskAndFinalize(backgroundCtx, clientBaseURL, task, reporter); err != nil {
		message := "等待 Edge run 终态失败: " + err.Error()
		reporter("error", taskModel.TaskStatusFailed, "edge_run", message, nil)
		_ = persistFailedServerTask(backgroundCtx, platformCtx.MainAccountID, envID, task, message)
		return
	}
}

func waitRunEdgeTaskAndFinalize(ctx context.Context, clientBaseURL string, task *taskModel.ServerTask, reporter runTaskProgressReporter) error {
	ticker := time.NewTicker(runEdgeTaskPollInterval)
	defer ticker.Stop()
	lastSignature := ""

	for {
		edgeTask, err := EdgeAPI.New().GetEdgeTask(ctx, clientBaseURL, "", task.EdgeTaskID)
		if err != nil {
			return err
		}
		signature := strings.ToLower(strings.TrimSpace(edgeTask.Status)) + "|" + firstNonEmpty(edgeTask.Message, edgeTask.LastError)
		if signature != lastSignature && reporter != nil {
			reporter("progress", mapEdgeTaskStatus(edgeTask.Status), "edge_run", firstNonEmpty(edgeTask.Message, edgeTask.LastError, "Edge run 正在执行"), map[string]any{
				"edgeTaskId": edgeTask.TaskID,
				"edgeStatus": strings.TrimSpace(edgeTask.Status),
			})
			lastSignature = signature
		}

		decision := decideRunFinalizeFromEdgeTask(edgeTask, nil)
		if !decision.Final {
			<-ticker.C
			continue
		}

		applyRunEdgeTaskToServerTask(task, edgeTask)
		if decision.Status == taskModel.TaskStatusFailed && strings.TrimSpace(task.ErrorMessage) == "" {
			task.ErrorMessage = decision.Message
		}
		if err = taskDao.NewModelHandler().UpdateServerTask(ctx, task); err != nil {
			return fmt.Errorf("更新中心 run 任务终态失败: %w", err)
		}
		if syncErr := syncRunEnvCacheFromEdge(ctx, task, clientBaseURL); syncErr != nil && reporter != nil {
			reporter("progress", task.Status, "sync_env_cache", "Edge run 已完成，但刷新中心环境包缓存失败", map[string]any{
				"error": syncErr.Error(),
			})
		}
		if reporter != nil {
			finalEvent := "done"
			if task.Status == taskModel.TaskStatusFailed {
				finalEvent = "error"
			}
			reporter(finalEvent, task.Status, "edge_run", firstNonEmpty(decision.Message, task.ErrorMessage, "环境包启动任务已完成"), map[string]any{
				"edgeTaskId": edgeTask.TaskID,
				"edgeStatus": strings.TrimSpace(edgeTask.Status),
			})
		}
		return nil
	}
}

func persistFailedServerTask(ctx context.Context, mainAccountID string, envID string, task *taskModel.ServerTask, message string) error {
	if task == nil {
		return nil
	}
	task.Status = taskModel.TaskStatusFailed
	task.ErrorMessage = strings.TrimSpace(message)
	task.UpdatedAt = time.Now().Unix()
	task.FinishedAt = task.UpdatedAt
	if err := taskDao.NewModelHandler().UpdateServerTask(ctx, task); err != nil {
		return err
	}
	return envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx, mainAccountID, envID, task.TaskID, task.ErrorMessage, task.UpdatedAt)
}

func decideRunFinalizeFromEdgeTask(edgeTask *EdgeAPI.EdgeTask, queryErr error) runFinalizeDecision {
	if queryErr != nil {
		return runFinalizeDecision{
			Status:  taskModel.TaskStatusFailed,
			Message: "Edge run task 查询失败，Node Server 不能默认成功: " + queryErr.Error(),
			Final:   true,
		}
	}
	if edgeTask == nil {
		return runFinalizeDecision{
			Status:  taskModel.TaskStatusFailed,
			Message: "Edge run task 不存在或已丢失，Node Server 不能默认成功",
			Final:   true,
		}
	}
	switch strings.ToLower(strings.TrimSpace(edgeTask.Status)) {
	case "success", "done":
		return runFinalizeDecision{Status: taskModel.TaskStatusSuccess, Message: firstNonEmpty(edgeTask.Message, "Edge run completed"), Final: true}
	case "failed", "error":
		return runFinalizeDecision{Status: taskModel.TaskStatusFailed, Message: firstNonEmpty(edgeTask.LastError, edgeTask.Message, "Edge run failed"), Final: true}
	case "queued", "pending", "running":
		return runFinalizeDecision{Status: taskModel.TaskStatusRunning, Message: firstNonEmpty(edgeTask.Message, "Edge run running"), Final: false}
	default:
		return runFinalizeDecision{
			Status:  taskModel.TaskStatusFailed,
			Message: "Edge run task 状态不可识别，不能作为成功事实: " + strings.TrimSpace(edgeTask.Status),
			Final:   true,
		}
	}
}

func applyRunEdgeTaskToServerTask(task *taskModel.ServerTask, edgeTask *EdgeAPI.EdgeTask) {
	if task == nil || edgeTask == nil {
		return
	}
	task.Status = mapEdgeTaskStatus(edgeTask.Status)
	task.UpdatedAt = time.Now().Unix()
	if task.Status == taskModel.TaskStatusFailed {
		task.ErrorMessage = firstNonEmpty(strings.TrimSpace(edgeTask.LastError), strings.TrimSpace(edgeTask.Message))
	}
	if task.Status == taskModel.TaskStatusSuccess {
		task.ErrorMessage = ""
	}
	if task.Status == taskModel.TaskStatusSuccess || task.Status == taskModel.TaskStatusFailed || task.Status == taskModel.TaskStatusCanceled {
		if edgeTask.FinishedAt != nil && *edgeTask.FinishedAt > 0 {
			task.FinishedAt = *edgeTask.FinishedAt
		} else {
			task.FinishedAt = task.UpdatedAt
		}
	}
}

func syncRunEnvCacheFromEdge(ctx context.Context, task *taskModel.ServerTask, baseURL string) error {
	snapshot, err := fetchRunEnvSnapshot(ctx, baseURL, task.EnvID)
	if err != nil {
		return err
	}
	return syncRunEnvCacheFromSnapshot(ctx, task, baseURL, snapshot)
}

func syncRunEnvCacheFromSnapshot(ctx context.Context, task *taskModel.ServerTask, baseURL string, snapshot *edgeRunBrowserEnvIndex) error {
	if task == nil || snapshot == nil {
		return nil
	}
	env, err := envDao.NewModelHandler().GetServerBrowserEnvByID(ctx, task.MainAccountID, task.EnvID)
	if err != nil {
		return err
	}
	env.RPAType = firstNonEmpty(snapshot.RPAType, env.RPAType)
	env.Name = firstNonEmpty(snapshot.Name, env.Name)
	env.Status = firstNonEmpty(snapshot.Status, env.Status)
	env.ContainerStatus = firstNonEmpty(snapshot.ContainerStatus, env.ContainerStatus)
	env.MonitorStatus = firstNonEmpty(snapshot.MonitorStatus, env.MonitorStatus)
	env.CDPURL = buildRunCDPURL(baseURL, snapshot.CDPPort, snapshot.Status)
	env.WebVNCURL = strings.TrimSpace(snapshot.WebVNCURL)
	env.LastTaskID = task.TaskID
	env.LastError = strings.TrimSpace(optionalString(snapshot.LastError))
	if task.Status == taskModel.TaskStatusFailed && env.LastError == "" {
		env.LastError = task.ErrorMessage
	}
	if snapshot.UpdatedAt > 0 {
		env.UpdatedAt = snapshot.UpdatedAt
	} else {
		env.UpdatedAt = time.Now().Unix()
	}
	return envDao.NewModelHandler().UpdateServerBrowserEnvSnapshot(ctx, env)
}

func fetchRunEnvSnapshot(ctx context.Context, baseURL string, envID string) (*edgeRunBrowserEnvIndex, error) {
	var detail edgeRunBrowserEnvSnapshotDetail
	if err := EdgeAPI.New().DoJSON(ctx, baseURL, http.MethodGet, "/api/v1/edge/browser-envs/"+url.PathEscape(envID), "", nil, &detail); err != nil {
		return nil, err
	}
	if detail.Index == nil {
		return nil, fmt.Errorf("Edge 环境包详情缺少 index，无法刷新中心缓存")
	}
	return detail.Index, nil
}

func buildRunCDPURL(baseURL string, cdpPort int, envStatus string) string {
	if cdpPort <= 0 || !strings.EqualFold(strings.TrimSpace(envStatus), envModel.EnvStatusRunning) {
		return ""
	}
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	return "http://" + parsed.Hostname() + ":" + strconv.Itoa(cdpPort)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
