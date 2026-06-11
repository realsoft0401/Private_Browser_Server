package TaskStream

import (
	"strings"
	"sync"
	"time"

	model "private_browser_server/Models/Task"
)

const eventLimit = 200

type streamRecord struct {
	events       []model.ServerTaskEvent
	subscribers  map[chan model.ServerTaskEvent]struct{}
	terminalSeen bool
}

type streamHub struct {
	mu    sync.Mutex
	tasks map[string]*streamRecord
}

var hub = &streamHub{tasks: make(map[string]*streamRecord)}

// Ensure 为指定 server task 准备一条中心事件流。
//
// 设计来源：
// - run 现在要把 image_check、pulling_image、edge_run 这些中心阶段通过 SSE 暴露给前端；
// - 这些事件发生在 Edge taskId 产生之前，不能继续依赖“有 edgeTaskId 后再代理 Edge SSE”的旧逻辑；
// - 因此这里提供一条 Node Server 本地内存事件流，供 Env/Task 两侧共享。
//
// 职责边界：
// - 只维护当前进程内的实时事件历史和订阅者；
// - 不持久化到 SQLite，不承担重启恢复；
// - 终态事件到达后不再接受后续写入，避免前端看到反复回滚的状态。
func Ensure(taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.tasks[taskID] != nil {
		return
	}
	hub.tasks[taskID] = &streamRecord{
		events:      make([]model.ServerTaskEvent, 0, 8),
		subscribers: make(map[chan model.ServerTaskEvent]struct{}),
	}
}

// Emit 向中心事件流追加一条事件。
func Emit(taskID, event, status, stage, message string, data map[string]any) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	hub.mu.Lock()
	record := hub.tasks[taskID]
	if record == nil {
		record = &streamRecord{
			events:      make([]model.ServerTaskEvent, 0, 8),
			subscribers: make(map[chan model.ServerTaskEvent]struct{}),
		}
		hub.tasks[taskID] = record
	}
	if record.terminalSeen {
		hub.mu.Unlock()
		return
	}
	taskEvent := model.ServerTaskEvent{
		TaskID:    taskID,
		Event:     strings.TrimSpace(event),
		Status:    strings.TrimSpace(status),
		Stage:     strings.TrimSpace(stage),
		Message:   strings.TrimSpace(message),
		Data:      data,
		CreatedAt: time.Now().Unix(),
	}
	record.events = append(record.events, taskEvent)
	if len(record.events) > eventLimit {
		record.events = record.events[len(record.events)-eventLimit:]
	}
	if isTerminal(taskEvent) {
		record.terminalSeen = true
	}
	subscribers := make([]chan model.ServerTaskEvent, 0, len(record.subscribers))
	for subscriber := range record.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	hub.mu.Unlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- taskEvent:
		default:
		}
	}
}

// Subscribe 返回当前任务事件历史和后续事件通道。
//
// 取消订阅时只从 hub 中移除，不主动关闭 channel，
// 避免并发 emit 时向已关闭 channel 发送导致 panic。
func Subscribe(taskID string) ([]model.ServerTaskEvent, <-chan model.ServerTaskEvent, func(), bool) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, nil, nil, false
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	record := hub.tasks[taskID]
	if record == nil {
		return nil, nil, nil, false
	}
	ch := make(chan model.ServerTaskEvent, 32)
	record.subscribers[ch] = struct{}{}
	history := append([]model.ServerTaskEvent(nil), record.events...)
	cancel := func() {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		if current := hub.tasks[taskID]; current != nil {
			delete(current.subscribers, ch)
		}
	}
	return history, ch, cancel, true
}

func isTerminal(event model.ServerTaskEvent) bool {
	status := strings.TrimSpace(event.Status)
	return status == model.TaskStatusSuccess || status == model.TaskStatusFailed || status == model.TaskStatusCanceled
}
