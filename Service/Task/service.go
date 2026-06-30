package Task

import (
	"context"
	"fmt"
	"sync"
	"time"

	TaskDAO "private_browser_server/Dao/Task"
	TaskModel "private_browser_server/Models/Task"
	TaskRepo "private_browser_server/Repository/Task"
)

type record struct {
	id           string
	taskType     string
	resourceType string
	resourceID   string
	createdAt    string
	updatedAt    string
	finishedAt   string

	mu          sync.Mutex
	events      []TaskModel.Event
	done        bool
	subscribers map[int]chan TaskModel.Event
	nextSubID   int
}

type Snapshot struct {
	Events []TaskModel.Event
	Done   bool
}

// Service 管理 Node Server 当前进程内的 SSE 事件流，并把正式任务终态写入 SQLite。
//
// 设计来源：
// - 文档已经收口：Server task 是长期事实，SSE 事件只是过程观察；
// - 因此这里采用“双层结构”：SQLite 持久化任务主记录，内存只保留短期事件流；
// - 这样既满足审计，又避免把每条 SSE 事件都写进库里。
type Service struct {
	mu    sync.RWMutex
	tasks map[string]*record
	repo  *TaskRepo.Repository
}

var (
	defaultService *Service
	once           sync.Once
)

func GetService() *Service {
	once.Do(func() {
		defaultService = &Service{
			tasks: make(map[string]*record),
			repo:  TaskRepo.NewRepository(),
		}
	})
	return defaultService
}

// CreateTask 创建一条新的中心任务主记录，并初始化进程内 SSE 缓存。
//
// 职责边界：
// - 负责生成 `server-task-*` 主键；
// - 负责写入 SQLite 主记录；
// - 负责在当前进程里建立后续 SSE 事件缓冲；
// - 不在这里偷发第一条 progress 事件，阶段事件必须由具体业务自己决定。
func (s *Service) CreateTask(ctx context.Context, row *TaskDAO.Row) (string, error) {
	if s == nil {
		s = GetService()
	}
	taskID := fmt.Sprintf("server-task-%d", time.Now().UnixNano())
	now := time.Now()
	nowRFC3339 := now.Format(time.RFC3339)
	if row == nil {
		row = &TaskDAO.Row{}
	}
	row.ID = taskID
	if row.Status == "" {
		row.Status = TaskModel.StatusPending
	}
	row.CreatedAt = now.Unix()
	row.UpdatedAt = now.Unix()
	if row.EventsURL == "" {
		row.EventsURL = fmt.Sprintf("/api/v1/server-tasks/%s/events", taskID)
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return "", err
	}

	s.mu.Lock()
	s.tasks[taskID] = &record{
		id:           taskID,
		taskType:     row.TaskType,
		resourceType: row.ResourceType,
		resourceID:   row.ResourceID,
		createdAt:    nowRFC3339,
		updatedAt:    nowRFC3339,
		subscribers:  make(map[int]chan TaskModel.Event),
	}
	s.mu.Unlock()
	return taskID, nil
}

func (s *Service) PublishProgress(ctx context.Context, taskID string, event TaskModel.Event) error {
	return s.publish(ctx, taskID, event, false, TaskModel.StatusRunning)
}

// PublishCompleted 把中心 task 收口为成功终态。
func (s *Service) PublishCompleted(ctx context.Context, taskID string, event TaskModel.Event) error {
	return s.publish(ctx, taskID, event, true, TaskModel.StatusSuccess)
}

// PublishFailed 把中心 task 收口为失败终态。
func (s *Service) PublishFailed(ctx context.Context, taskID string, event TaskModel.Event) error {
	return s.publish(ctx, taskID, event, true, TaskModel.StatusFailed)
}

// publish 是中心任务事件写入的统一收口点。
//
// 维护原则：
// - 先更新内存事件流，再更新 SQLite 主状态；
// - markDone=true 时必须关闭所有订阅者，避免 SSE 长连接悬挂；
// - 这里不做自动重试，数据库写失败必须明确暴露给上层。
func (s *Service) publish(ctx context.Context, taskID string, event TaskModel.Event, markDone bool, taskStatus string) error {
	item, err := s.getRecord(taskID)
	if err != nil {
		return err
	}

	item.mu.Lock()
	item.events = append(item.events, event)
	item.updatedAt = event.Timestamp
	for _, subscriber := range item.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
	if markDone && !item.done {
		item.done = true
		item.finishedAt = event.Timestamp
		for id, subscriber := range item.subscribers {
			close(subscriber)
			delete(item.subscribers, id)
		}
	}
	item.mu.Unlock()

	updateRow := &TaskDAO.Row{
		ID:           taskID,
		Status:       taskStatus,
		ErrorMessage: event.Error,
		Suggestion:   event.Suggestion,
		UpdatedAt:    mustParseRFC3339Unix(event.Timestamp),
	}
	if markDone {
		updateRow.FinishedAt = updateRow.UpdatedAt
	}
	if updateErr := s.repo.UpdateStatus(ctx, updateRow); updateErr != nil {
		return updateErr
	}
	return nil
}

// Subscribe 为某个中心 task 建立一次 SSE 订阅。
//
// 设计来源：
// - 新订阅者需要先看到历史快照，再接实时事件；
// - 已结束任务不需要保持通道，只返回静态快照即可；
// - SQLite 不保存逐条事件，因此这里的实时事件只依赖当前进程内缓存。
func (s *Service) Subscribe(taskID string) (Snapshot, <-chan TaskModel.Event, func(), error) {
	item, err := s.getRecord(taskID)
	if err != nil {
		if err == TaskRepo.ErrNotFound {
			_, repoErr := s.repo.GetByID(context.Background(), taskID)
			if repoErr != nil {
				return Snapshot{}, nil, nil, err
			}
			return Snapshot{Done: true}, nil, func() {}, nil
		}
		return Snapshot{}, nil, nil, err
	}

	item.mu.Lock()
	defer item.mu.Unlock()

	base := Snapshot{
		Events: append([]TaskModel.Event(nil), item.events...),
		Done:   item.done,
	}
	if item.done {
		return base, nil, func() {}, nil
	}

	item.nextSubID++
	subID := item.nextSubID
	channel := make(chan TaskModel.Event, 16)
	item.subscribers[subID] = channel

	cancel := func() {
		item.mu.Lock()
		defer item.mu.Unlock()
		if subscriber, ok := item.subscribers[subID]; ok {
			close(subscriber)
			delete(item.subscribers, subID)
		}
	}
	return base, channel, cancel, nil
}

// GetDetail 读取中心任务主记录，并尽量补上最近一条内存事件摘要。
//
// 这样做的原因是：
// - SQLite 主记录能保证进程重启后仍可查询；
// - 内存里最近事件能补充 `currentStage/message` 这种更适合排障的信息；
// - 两层合起来，detail 才既稳定又可读。
func (s *Service) GetDetail(ctx context.Context, taskID string) (*TaskModel.DetailResponse, error) {
	item, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	result := &TaskModel.DetailResponse{
		TaskID:       item.ID,
		TaskType:     item.TaskType,
		ResourceType: item.ResourceType,
		ResourceID:   item.ResourceID,
		Status:       item.Status,
		EventsURL:    item.EventsURL,
		CreatedAt:    formatUnix(item.CreatedAt),
		UpdatedAt:    formatUnix(item.UpdatedAt),
		FinishedAt:   formatUnix(item.FinishedAt),
		Error:        item.ErrorMessage,
		Suggestion:   item.Suggestion,
	}
	if record, getErr := s.getRecord(taskID); getErr == nil {
		record.mu.Lock()
		if len(record.events) > 0 {
			last := record.events[len(record.events)-1]
			result.CurrentStage = last.Stage
			result.Message = last.Message
			if last.Status != "" {
				result.Status = last.Status
			}
			if last.Error != "" {
				result.Error = last.Error
			}
			if last.Suggestion != "" {
				result.Suggestion = last.Suggestion
			}
			if last.Timestamp != "" {
				result.UpdatedAt = last.Timestamp
			}
		}
		record.mu.Unlock()
	}
	return result, nil
}

// getRecord 读取当前进程内的任务事件缓存。
func (s *Service) getRecord(taskID string) (*record, error) {
	if s == nil {
		s = GetService()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.tasks[taskID]
	if !ok {
		return nil, TaskRepo.ErrNotFound
	}
	return item, nil
}

// formatUnix 统一把数据库秒级时间戳转成 RFC3339。
func formatUnix(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.Unix(value, 0).Format(time.RFC3339)
}

// mustParseRFC3339Unix 把事件时间转回数据库秒级时间戳。
//
// 这里使用“失败回落到当前时间”的策略，是为了避免单条事件格式异常时把整条任务更新完全打断；
// 但这不是时间协议放松，正式事件仍应始终写 RFC3339。
func mustParseRFC3339Unix(value string) int64 {
	value = fmt.Sprintf("%s", value)
	if value == "" {
		return time.Now().Unix()
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now().Unix()
	}
	return parsed.Unix()
}
