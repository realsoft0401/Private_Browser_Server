package Task

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	TaskModel "private_browser_server/Models/Task"
	"private_browser_server/Pkg/HttpResponse"
	TaskRepo "private_browser_server/Repository/Task"
)

// List 返回中心 task 历史列表。
//
// 职责边界：
// - 普通 HTTP 查询，不是 SSE；
// - 只读 `server_tasks` 持久化摘要，不读取 SSE 事件明细；
// - 支持按 client/env/resource/task/status 过滤，方便管理员定位失败动作。
func List(c *gin.Context) {
	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := GetService().List(requestCtx, TaskModel.ListQuery{
		ClientID:   c.Query("clientId"),
		EnvID:      c.Query("envId"),
		ResourceID: c.Query("resourceId"),
		TaskType:   c.Query("taskType"),
		Status:     c.Query("status"),
		Page:       parsePositiveInt(c.Query("page")),
		PageSize:   parsePositiveInt(c.Query("pageSize")),
	})
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInternalError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// GetDetail 返回中心 task 的当前持久化摘要。
//
// 职责边界：
// - 这里只查 SQLite 中的 `server_tasks` 主记录；
// - 不返回 SSE 全量事件历史；
// - 调用方如果要看过程事件，必须继续订阅 `/api/v1/server-tasks/{taskId}/events`。
func GetDetail(c *gin.Context) {
	requestCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := GetService().GetDetail(requestCtx, c.Param("taskId"))
	if err != nil {
		if err == TaskRepo.ErrNotFound {
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeNotFound, "server task not found")
			return
		}
		HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeInternalError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(c, result)
}

// SubscribeEvents 订阅中心 task 的 SSE 事件流。
//
// 设计来源：
// - 平台口径已经定下：Server task 是长期事实，SSE 只做短期过程观察；
// - 因此这里先输出当前进程内缓存快照，再继续把后续事件流式推给调用方；
// - 如果任务已结束，就只返回快照，不再保持长连接。
func SubscribeEvents(c *gin.Context) {
	snapshot, stream, cancel, err := GetService().Subscribe(c.Param("taskId"))
	if err != nil {
		if err == TaskRepo.ErrNotFound {
			c.JSON(http.StatusOK, gin.H{
				"code":    1004,
				"message": "server task not found",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    1005,
			"message": err.Error(),
		})
		return
	}
	defer cancel()

	writer := c.Writer
	header := writer.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	for _, event := range snapshot.Events {
		if err = writeSSEEvent(writer, event); err != nil {
			return
		}
	}
	if snapshot.Done || stream == nil {
		return
	}

	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case event, ok := <-stream:
			if !ok {
				return
			}
			if err = writeSSEEvent(writer, event); err != nil {
				return
			}
		}
	}
}

// parsePositiveInt 解析查询参数里的正整数。
//
// 非法输入统一按 0 处理，由 Service 层套默认值；这样列表接口不会因为 page 为空或误填字符串
// 直接返回 500，也不会把参数清洗逻辑散在多个 handler 里。
func parsePositiveInt(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

// writeSSEEvent 统一把事件编码成正式 SSE 报文。
//
// 维护原则：
// - 这里不要偷改事件名字、不要把 JSON 再包一层；
// - `event:` 必须和任务事件里的 `event` 字段保持一致；
// - 这样 Client/前端/调试脚本才能共用同一套订阅解析逻辑。
func writeSSEEvent(writer gin.ResponseWriter, event any) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal task event failed: %w", err)
	}
	if _, err = fmt.Fprintf(writer, "event: %s\n", extractEventName(event)); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(writer, "data: %s\n\n", body); err != nil {
		return err
	}
	writer.Flush()
	return nil
}

// extractEventName 提取 SSE `event:` 名称。
//
// 当前所有正式任务事件都实现了 `GetEvent()`；这里保留兜底 `"message"`，
// 是为了避免后续新增事件类型时因为漏实现接口而直接写出非法 SSE 格式。
func extractEventName(event any) string {
	type eventNamer interface {
		GetEvent() string
	}
	if named, ok := event.(eventNamer); ok {
		return named.GetEvent()
	}
	return "message"
}
