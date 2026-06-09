package Task

import (
	"strings"

	"private_browser_server/EdgeClient"
	model "private_browser_server/Models/Task"
)

// FinalizeDecision 是 Server 根据 Edge task 事实得到的中心任务收口建议。
//
// 设计来源：
// - Client task 只是 Edge 进程内短期观察，服务重启或 SSE 中断后可能丢失；
// - Server task 才是平台持久事实，终态只能是 success/failed；
// - 因此这里把 Edge task 状态、查询错误和业务错误统一映射，禁止“查不到 Edge task 就默认成功”。
type FinalizeDecision struct {
	Status  string
	Message string
	Final   bool
}

// DecideFromEdgeTask 把 Edge task 查询结果归一成 Server task 状态。
//
// 维护边界：
// - edgeTask=nil 或查询错误时只能 failed，不能自动重放 run/stop/backup/restore/import-package；
// - Edge 的 queued/running 仍是执行中暂态，Server task 不应提前写终态；
// - done/success 才能收口为 success，error/failed 统一收口为 failed。
func DecideFromEdgeTask(edgeTask *EdgeClient.EdgeTask, queryErr error) FinalizeDecision {
	if queryErr != nil {
		return FinalizeDecision{
			Status:  model.TaskStatusFailed,
			Message: "Edge task 查询失败，不能默认成功，也不能自动重试资产动作: " + queryErr.Error(),
			Final:   true,
		}
	}
	if edgeTask == nil {
		return FinalizeDecision{
			Status:  model.TaskStatusFailed,
			Message: "Edge task 不存在或已丢失，Server 不能默认成功；需要重新调用 Edge 状态接口核验事实",
			Final:   true,
		}
	}
	status := strings.ToLower(strings.TrimSpace(edgeTask.Status))
	switch status {
	case "success", "done":
		return FinalizeDecision{Status: model.TaskStatusSuccess, Message: firstTaskMessage(edgeTask.Message, "Edge task completed"), Final: true}
	case "failed", "error":
		return FinalizeDecision{Status: model.TaskStatusFailed, Message: firstTaskMessage(edgeTask.LastError, edgeTask.Message, "Edge task failed"), Final: true}
	case "queued", "pending", "running":
		return FinalizeDecision{Status: model.TaskStatusRunning, Message: firstTaskMessage(edgeTask.Message, "Edge task running"), Final: false}
	default:
		return FinalizeDecision{
			Status:  model.TaskStatusFailed,
			Message: "Edge task 状态不可识别，不能作为成功事实: " + status,
			Final:   true,
		}
	}
}

func firstTaskMessage(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
