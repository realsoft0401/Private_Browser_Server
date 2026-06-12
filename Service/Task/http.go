package Task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	"private_browser_server/Pkg/TaskStream"
	NodeRepo "private_browser_server/Repository/Node"
)

type edgeBrowserEnvDetail struct {
	Index *edgeBrowserEnvIndex `json:"index"`
}

type edgeBrowserEnvIndex struct {
	EnvID           string  `json:"envId"`
	RPAType         string  `json:"rpaType"`
	Name            string  `json:"name"`
	CDPPort         int     `json:"cdpPort"`
	WebVNCURL       string  `json:"webVncUrl"`
	Status          string  `json:"status"`
	ContainerStatus string  `json:"containerStatus"`
	MonitorStatus   string  `json:"monitorStatus"`
	LastError       *string `json:"lastError,omitempty"`
	UpdatedAt       int64   `json:"updatedAt"`
}

// ListTasks 返回当前主账号下的中心任务列表。
//
// 第一版先提供调试和链路联调用的最小查询能力；
// 任务实时刷新仍以单个 task detail 为主，避免列表接口一次性触发大量 Edge 查询。
func ListTasks(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	query := normalizeListTaskQuery(ctx)
	items, total, err := taskDao.NewModelHandler().ListServerTasks(ctx.Request.Context(), platformCtx.MainAccountID, query)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "查询中心任务列表失败: "+err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, taskModel.ListTasksResponse{
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
		Items:    items,
	})
}

// GetTask 返回单个中心任务详情，并在需要时尝试同步 Edge 当前任务状态。
//
// 设计来源：
// - Edge task 是短期内存事实，只有在查看任务详情时才能知道当前 running/success/failed；
// - 当前阶段先按“读取即刷新”的方式补任务状态，不额外引入后台轮询器；
// - 如果 Edge task 丢失，不能默认成功，必须再用环境包状态确认动作是否真的完成。
func GetTask(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	task, err := taskDao.NewModelHandler().GetServerTaskByID(
		ctx.Request.Context(),
		platformCtx.MainAccountID,
		strings.TrimSpace(ctx.Param("taskId")),
	)
	if errors.Is(err, taskDao.ErrTaskNotFound) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "中心任务不存在或不属于当前主账号")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "读取中心任务失败: "+err.Error())
		return
	}

	edgeSnapshot, syncErr := syncServerTaskFromEdge(ctx.Request.Context(), task)
	if syncErr != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "刷新任务状态失败: "+syncErr.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, taskModel.TaskDetailResponse{
		Task: task,
		Edge: edgeSnapshot,
	})
}

// StreamTaskEvents 返回中心任务 SSE 事件流。
//
// 设计来源：
// - run 现在会先经过 Node Server 自己的镜像预检和后台编排，前端需要先看到 image_check/pulling_image/edge_run 这类中心阶段；
// - 这些事件发生在 Edge taskId 产生之前，因此中心层必须优先输出自己的事件流；
// - 对于 stop 等仍然直接创建 Edge task 的接口，当前继续回退到旧的 Edge SSE 代理模式，保持兼容。
func StreamTaskEvents(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	task, err := taskDao.NewModelHandler().GetServerTaskByID(
		ctx.Request.Context(),
		platformCtx.MainAccountID,
		strings.TrimSpace(ctx.Param("taskId")),
	)
	if errors.Is(err, taskDao.ErrTaskNotFound) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "中心任务不存在或不属于当前主账号")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "读取中心任务失败: "+err.Error())
		return
	}
	if streamTaskEventsFromNode(ctx, task.TaskID) {
		return
	}
	if strings.TrimSpace(task.EdgeTaskID) == "" {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeConflict, "当前中心任务尚未绑定 Edge taskId，无法代理 SSE 事件")
		return
	}

	node, err := NodeRepo.Repository{}.GetByID(ctx.Request.Context(), platformCtx.MainAccountID, task.ClientID)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "读取目标 Edge Client 失败，无法代理 SSE: "+err.Error())
		return
	}
	endpoint, err := buildEdgeTaskEventsURL(node.BaseURL, task.EdgeTaskID)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "拼接 Edge task SSE 地址失败: "+err.Error())
		return
	}
	if err = proxyEdgeTaskStream(ctx, endpoint); err != nil && !errors.Is(err, context.Canceled) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, "代理 Edge task SSE 失败: "+err.Error())
		return
	}
}

// streamTaskEventsFromNode 优先输出 Node Server 自己维护的中心任务事件流。
//
// 只有 run 这类存在中心前置阶段的任务才会命中这里；
// 如果当前 task 没有本地事件流，则由上层继续回退到旧的 Edge SSE 代理逻辑。
func streamTaskEventsFromNode(ctx *gin.Context, taskID string) bool {
	history, events, cancel, ok := TaskStream.Subscribe(taskID)
	if !ok {
		return false
	}
	defer cancel()

	header := ctx.Writer.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	ctx.Status(http.StatusOK)

	for _, event := range history {
		writeServerTaskSSE(ctx, event)
		if isTerminalServerTaskEvent(event) {
			return true
		}
	}

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Request.Context().Done():
			return true
		case <-heartbeat.C:
			writeServerTaskSSE(ctx, taskModel.ServerTaskEvent{
				TaskID:    strings.TrimSpace(taskID),
				Event:     "heartbeat",
				Status:    taskModel.TaskStatusRunning,
				Stage:     "server_sse",
				Message:   "Node Server SSE 连接保持中",
				CreatedAt: time.Now().Unix(),
			})
		case event := <-events:
			writeServerTaskSSE(ctx, event)
			if isTerminalServerTaskEvent(event) {
				return true
			}
		}
	}
}

func normalizeListTaskQuery(ctx *gin.Context) taskModel.ListTaskQuery {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return taskModel.ListTaskQuery{
		ClientID: strings.TrimSpace(ctx.Query("clientId")),
		EnvID:    strings.TrimSpace(ctx.Query("envId")),
		Type:     strings.TrimSpace(ctx.Query("type")),
		Status:   strings.TrimSpace(ctx.Query("status")),
		Page:     page,
		PageSize: pageSize,
	}
}

func syncServerTaskFromEdge(ctx context.Context, task *taskModel.ServerTask) (*taskModel.EdgeTaskSnapshot, error) {
	if task == nil || isTerminalTask(task.Status) || strings.TrimSpace(task.EdgeTaskID) == "" {
		return nil, nil
	}

	node, err := NodeRepo.Repository{}.GetByID(ctx, task.MainAccountID, task.ClientID)
	if err != nil {
		return nil, confirmTaskWithoutEdge(ctx, task, "", "读取目标 Edge Client 失败: "+err.Error())
	}

	edgeTask, err := EdgeAPI.New().GetEdgeTask(ctx, node.BaseURL, "", task.EdgeTaskID)
	if err != nil {
		return nil, confirmTaskWithoutEdge(ctx, task, node.BaseURL, "查询 Edge task 失败: "+err.Error())
	}

	edgeSnapshot := buildEdgeTaskSnapshot(edgeTask)
	applyEdgeTaskToServerTask(task, edgeTask)
	if err = taskDao.NewModelHandler().UpdateServerTask(ctx, task); err != nil {
		return edgeSnapshot, err
	}
	if isTerminalTask(task.Status) {
		_ = syncEnvCacheAfterTask(ctx, task, node.BaseURL)
	}
	return edgeSnapshot, nil
}

func confirmTaskWithoutEdge(ctx context.Context, task *taskModel.ServerTask, baseURL string, reason string) error {
	reason = strings.TrimSpace(reason)
	confirmedSuccess := false
	if strings.TrimSpace(baseURL) != "" {
		snapshot, err := fetchEdgeBrowserEnvSnapshot(ctx, baseURL, task.EnvID)
		if err == nil {
			confirmedSuccess = canConfirmTaskFromEnv(task.Type, snapshot)
			if confirmedSuccess && strings.TrimSpace(task.Type) == taskModel.TaskTypeDeleteEnvPackage {
				_ = markEnvDeletedAfterPackage(ctx, task)
			} else {
				_ = syncEnvCacheFromSnapshot(ctx, task, baseURL, snapshot)
			}
		} else if strings.TrimSpace(task.Type) == taskModel.TaskTypeDeleteEnvPackage && isEdgeBrowserEnvMissing(err) {
			confirmedSuccess = true
			_ = markEnvDeletedAfterPackage(ctx, task)
		}
	}

	now := time.Now().Unix()
	if confirmedSuccess {
		task.Status = taskModel.TaskStatusSuccess
		task.ErrorMessage = ""
		task.UpdatedAt = now
		task.FinishedAt = now
	} else {
		task.Status = taskModel.TaskStatusFailed
		task.ErrorMessage = "Edge task 丢失或不可读，且无法从环境包状态确认动作完成: " + reason
		task.UpdatedAt = now
		task.FinishedAt = now
		_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx, task.MainAccountID, task.EnvID, task.TaskID, task.ErrorMessage, now)
	}
	return taskDao.NewModelHandler().UpdateServerTask(ctx, task)
}

func buildEdgeTaskSnapshot(edgeTask *EdgeAPI.EdgeTask) *taskModel.EdgeTaskSnapshot {
	if edgeTask == nil {
		return nil
	}
	return &taskModel.EdgeTaskSnapshot{
		TaskID:     strings.TrimSpace(edgeTask.TaskID),
		TaskType:   strings.TrimSpace(edgeTask.TaskType),
		Status:     strings.TrimSpace(edgeTask.Status),
		Message:    strings.TrimSpace(edgeTask.Message),
		LastError:  strings.TrimSpace(edgeTask.LastError),
		Result:     edgeTask.Result,
		CreatedAt:  edgeTask.CreatedAt,
		UpdatedAt:  edgeTask.UpdatedAt,
		FinishedAt: edgeTask.FinishedAt,
	}
}

func applyEdgeTaskToServerTask(task *taskModel.ServerTask, edgeTask *EdgeAPI.EdgeTask) {
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
	if isTerminalTask(task.Status) {
		if edgeTask.FinishedAt != nil && *edgeTask.FinishedAt > 0 {
			task.FinishedAt = *edgeTask.FinishedAt
		} else {
			task.FinishedAt = task.UpdatedAt
		}
	}
}

func syncEnvCacheFromEdge(ctx context.Context, task *taskModel.ServerTask, baseURL string) error {
	snapshot, err := fetchEdgeBrowserEnvSnapshot(ctx, baseURL, task.EnvID)
	if err != nil {
		return err
	}
	return syncEnvCacheFromSnapshot(ctx, task, baseURL, snapshot)
}

// syncEnvCacheAfterTask 根据任务类型选择合适的中心缓存收口方式。
//
// 设计来源：
// - run/stop 成功后仍能从 Edge detail 拉到 env 摘要，适合继续走“按详情刷新”；
// - `/package` 成功后 Edge 本地索引会被物理删除，此时再读 detail 本来就会失败；
// - 因此 package 成功要直接把中心缓存改成 deleted，而不是把“读不到详情”误判成同步失败。
func syncEnvCacheAfterTask(ctx context.Context, task *taskModel.ServerTask, baseURL string) error {
	if task == nil {
		return nil
	}
	if strings.TrimSpace(task.Type) == taskModel.TaskTypeDeleteEnvPackage && strings.TrimSpace(task.Status) == taskModel.TaskStatusSuccess {
		return markEnvDeletedAfterPackage(ctx, task)
	}
	return syncEnvCacheFromEdge(ctx, task, baseURL)
}

func syncEnvCacheFromSnapshot(ctx context.Context, task *taskModel.ServerTask, baseURL string, snapshot *edgeBrowserEnvIndex) error {
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
	env.CDPURL = buildCDPURL(baseURL, snapshot.CDPPort, snapshot.Status)
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

func fetchEdgeBrowserEnvSnapshot(ctx context.Context, baseURL string, envID string) (*edgeBrowserEnvIndex, error) {
	var detail edgeBrowserEnvDetail
	if err := EdgeAPI.New().DoJSON(ctx, baseURL, http.MethodGet, "/api/v1/edge/browser-envs/"+url.PathEscape(envID), "", nil, &detail); err != nil {
		return nil, err
	}
	if detail.Index == nil {
		return nil, fmt.Errorf("Edge 环境包详情缺少 index，无法刷新中心缓存")
	}
	return detail.Index, nil
}

func canConfirmTaskFromEnv(taskType string, snapshot *edgeBrowserEnvIndex) bool {
	if snapshot == nil {
		return false
	}
	switch strings.TrimSpace(taskType) {
	case taskModel.TaskTypeRunEnv:
		return strings.EqualFold(strings.TrimSpace(snapshot.Status), envModel.EnvStatusRunning)
	case taskModel.TaskTypeStopEnv:
		status := strings.TrimSpace(snapshot.Status)
		containerStatus := strings.TrimSpace(snapshot.ContainerStatus)
		return status == envModel.EnvStatusStopped || status == envModel.EnvStatusCreated || status == envModel.EnvStatusBackedUp || containerStatus == "exited" || containerStatus == "missing"
	case taskModel.TaskTypeRevalidateEnv:
		status := strings.TrimSpace(snapshot.Status)
		return status == envModel.EnvStatusCreated || status == envModel.EnvStatusStopped
	case taskModel.TaskTypeDeleteEnvPackage:
		return strings.TrimSpace(snapshot.Status) == envModel.EnvStatusDeleted
	default:
		return false
	}
}

// markEnvDeletedAfterPackage 把中心缓存显式收口到 deleted。
//
// 设计来源：
// - Edge `/package` 成功后，本机环境目录和 SQLite 索引都会被删除，Node Server 不可能再从 Edge 拉到详情；
// - 但中心层仍需要保留一条“这个 envId 已被删除”的聚合历史，方便任务查询、审计和排障；
// - 因此这里更新 status=deleted、清空连接入口，并保留 deleted_at=0，避免列表层直接把整条中心记录过滤掉。
func markEnvDeletedAfterPackage(ctx context.Context, task *taskModel.ServerTask) error {
	if task == nil {
		return nil
	}
	env, err := envDao.NewModelHandler().GetServerBrowserEnvByID(ctx, task.MainAccountID, task.EnvID)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	env.Status = envModel.EnvStatusDeleted
	env.ContainerStatus = "missing"
	env.MonitorStatus = envModel.EnvFactUnknown
	env.CDPURL = ""
	env.WebVNCURL = ""
	env.LastTaskID = task.TaskID
	env.LastError = ""
	env.UpdatedAt = now
	env.DeletedAt = 0
	return envDao.NewModelHandler().UpdateServerBrowserEnvSnapshot(ctx, env)
}

func isEdgeBrowserEnvMissing(err error) bool {
	var edgeErr *EdgeAPI.EdgeError
	if errors.As(err, &edgeErr) {
		if edgeErr.EdgeCode == 1002 || edgeErr.HTTPStatus == http.StatusNotFound {
			return true
		}
		message := strings.ToLower(strings.TrimSpace(edgeErr.Message))
		if strings.Contains(message, "不存在") || strings.Contains(message, "not found") {
			return true
		}
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "不存在") || strings.Contains(message, "not found")
}

func buildEdgeTaskEventsURL(baseURL string, edgeTaskID string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("Edge baseUrl 不能为空")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("Edge baseUrl 非法: %s", baseURL)
	}
	return baseURL + "/api/v1/edge/tasks/" + url.PathEscape(strings.TrimSpace(edgeTaskID)) + "/events", nil
}

func proxyEdgeTaskStream(ctx *gin.Context, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx.Request.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Edge task SSE 返回 HTTP %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	header := ctx.Writer.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	ctx.Status(http.StatusOK)

	flusher, _ := ctx.Writer.(http.Flusher)
	buffer := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := ctx.Writer.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, context.Canceled) {
				return nil
			}
			return readErr
		}
	}
}

func buildCDPURL(baseURL string, cdpPort int, envStatus string) string {
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

func isTerminalTask(status string) bool {
	status = strings.TrimSpace(status)
	return status == taskModel.TaskStatusSuccess || status == taskModel.TaskStatusFailed || status == taskModel.TaskStatusCanceled
}

func mapEdgeTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success":
		return taskModel.TaskStatusSuccess
	case "failed":
		return taskModel.TaskStatusFailed
	case "running":
		return taskModel.TaskStatusRunning
	default:
		return taskModel.TaskStatusPending
	}
}

func writeServerTaskSSE(ctx *gin.Context, event taskModel.ServerTaskEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(ctx.Writer, "event:%s\n", event.Event)
	_, _ = fmt.Fprintf(ctx.Writer, "data:%s\n\n", payload)
	if flusher, ok := ctx.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func isTerminalServerTaskEvent(event taskModel.ServerTaskEvent) bool {
	status := strings.TrimSpace(event.Status)
	return status == taskModel.TaskStatusSuccess || status == taskModel.TaskStatusFailed || status == taskModel.TaskStatusCanceled
}
