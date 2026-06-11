package Task

import (
	"context"

	model "private_browser_server/Models/Task"
	repository "private_browser_server/Repository/Task"
)

var ErrTaskNotFound = repository.ErrTaskNotFound

// ModelHandler 是中心任务记录的业务动作入口。
//
// Dao 只负责把 Service 的任务对象整理给 Repository，不负责长连接 SSE 转发或 Edge API 调用。
type ModelHandler struct {
	repo repository.Repository
}

func NewModelHandler() *ModelHandler {
	return &ModelHandler{repo: repository.Repository{}}
}

func (h *ModelHandler) CreateServerTask(ctx context.Context, task *model.ServerTask) error {
	return h.repo.Create(ctx, task)
}

func (h *ModelHandler) GetServerTaskByID(ctx context.Context, mainAccountID string, taskID string) (*model.ServerTask, error) {
	return h.repo.GetByID(ctx, mainAccountID, taskID)
}

func (h *ModelHandler) ListServerTasks(ctx context.Context, mainAccountID string, query model.ListTaskQuery) ([]model.ServerTask, int, error) {
	return h.repo.ListByMainAccount(ctx, mainAccountID, query)
}

func (h *ModelHandler) UpdateServerTask(ctx context.Context, task *model.ServerTask) error {
	return h.repo.Update(ctx, task)
}
