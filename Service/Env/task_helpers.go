package Env

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	taskDao "private_browser_server/Dao/Task"
	EdgeAPI "private_browser_server/EdgeClient"
	"private_browser_server/Middleware/PlatformContext"
	taskModel "private_browser_server/Models/Task"
)

func publicRequestBase(ctx *gin.Context) string {
	scheme := "http"
	if ctx.Request.TLS != nil || strings.EqualFold(ctx.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := ctx.Request.Host
	if host == "" {
		host = "127.0.0.1"
	}
	return scheme + "://" + host
}

func buildServerTaskEventsURL(ctx *gin.Context, taskID string) string {
	return strings.TrimRight(publicRequestBase(ctx), "/") + "/api/v1/server/tasks/" + strings.TrimSpace(taskID) + "/events"
}

func buildStartTaskResponse(task *taskModel.ServerTask, message string) *taskModel.StartTaskResponse {
	if task == nil {
		return &taskModel.StartTaskResponse{}
	}
	return &taskModel.StartTaskResponse{
		TaskID:     task.TaskID,
		TaskType:   task.Type,
		Status:     task.Status,
		ClientID:   task.ClientID,
		EnvID:      task.EnvID,
		EdgeTaskID: task.EdgeTaskID,
		EventsURL:  task.EventsURL,
		Message:    strings.TrimSpace(message),
		CreatedAt:  task.CreatedAt,
	}
}

// newServerTask 统一生成中心层任务骨架。
//
// 这个入口单独存在，是为了把 taskId 生成、Platform 操作人信息和初始状态统一收口；
// 后续新增 backup/restore/delete/import-package 等动作时，应该继续复用这里，避免每个接口各自拼任务对象。
func newServerTask(platformCtx PlatformContext.Context, clientID string, envID string, taskType string, eventsURL string) *taskModel.ServerTask {
	now := time.Now().Unix()
	taskID := "task_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	return &taskModel.ServerTask{
		TaskID:           taskID,
		MainAccountID:    platformCtx.MainAccountID,
		OperatorUserID:   platformCtx.UserID,
		OperatorUsername: platformCtx.Username,
		ClientID:         strings.TrimSpace(clientID),
		EnvID:            strings.TrimSpace(envID),
		Type:             strings.TrimSpace(taskType),
		Status:           taskModel.TaskStatusPending,
		EdgeTaskID:       "",
		EventsURL:        strings.TrimSpace(eventsURL),
		ErrorMessage:     "",
		CreatedAt:        now,
		UpdatedAt:        now,
		FinishedAt:       0,
	}
}

func applyEdgeStartToServerTask(task *taskModel.ServerTask, edgeResp *EdgeAPI.TaskStartResponse) {
	if task == nil || edgeResp == nil {
		return
	}
	task.EdgeTaskID = strings.TrimSpace(edgeResp.TaskID)
	task.Status = mapEdgeTaskStatus(edgeResp.Status)
	task.UpdatedAt = time.Now().Unix()
	if task.Status == taskModel.TaskStatusSuccess || task.Status == taskModel.TaskStatusFailed {
		task.FinishedAt = task.UpdatedAt
	}
}

// failServerTask 用于在 HTTP 入口阶段快速把中心任务收口为 failed。
//
// 这个函数只负责写 server_tasks，不自动刷新环境包状态，也不猜测 Edge 端真实资产结果；
// 涉及环境包摘要的更新，仍由各个动作入口按自己的上下文单独决定。
func failServerTask(ctx *gin.Context, task *taskModel.ServerTask, message string) error {
	if task == nil {
		return nil
	}
	task.Status = taskModel.TaskStatusFailed
	task.ErrorMessage = strings.TrimSpace(message)
	task.UpdatedAt = time.Now().Unix()
	task.FinishedAt = task.UpdatedAt
	return taskDao.NewModelHandler().UpdateServerTask(ctx.Request.Context(), task)
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

func runTaskFirstMessage(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
