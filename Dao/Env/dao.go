package Env

import (
	"context"

	model "private_browser_server/Models/Env"
	repository "private_browser_server/Repository/Env"
)

var ErrEnvNotFound = repository.ErrEnvNotFound

// ModelHandler 保留 Node Server 既有的 Dao 业务动作入口风格。
//
// 设计来源：
// - 用户要求继续保留 `ModelHandler -> Repository` 这种直观调用链；
// - Env 这层后续会逐步接入 run/stop/task 刷新，如果直接让 Service 依赖 Repository 细节会越来越散；
// - 因此先把中心环境包索引的增查收敛到 Dao，后面补更新动作时能保持同一入口。
//
// 职责边界：
// - 只负责把 Service 传来的业务参数整理为 Repository 调用；
// - 不做 imagePolicy 选择，不解析 Platform Header，不调用 Edge；
// - 查无记录等数据库细节继续沿用 Repository 归一化结果。
type ModelHandler struct {
	repo repository.Repository
}

func NewModelHandler() *ModelHandler {
	return &ModelHandler{repo: repository.Repository{}}
}

// CreateServerBrowserEnv 写入中心环境包聚合索引。
func (h *ModelHandler) CreateServerBrowserEnv(ctx context.Context, env *model.ServerBrowserEnv) error {
	return h.repo.Create(ctx, env)
}

// GetServerBrowserEnvByID 查询单个中心环境包索引。
func (h *ModelHandler) GetServerBrowserEnvByID(ctx context.Context, mainAccountID string, envID string) (*model.ServerBrowserEnv, error) {
	return h.repo.GetByID(ctx, mainAccountID, envID)
}

// ListServerBrowserEnvs 查询主账号下的中心环境包列表。
func (h *ModelHandler) ListServerBrowserEnvs(ctx context.Context, mainAccountID string, query model.ListEnvQuery) ([]model.ServerBrowserEnv, int, error) {
	return h.repo.ListByMainAccount(ctx, mainAccountID, query)
}

func (h *ModelHandler) UpdateServerBrowserEnvTaskSummary(ctx context.Context, mainAccountID string, envID string, taskID string, lastError string, updatedAt int64) error {
	return h.repo.UpdateTaskSummary(ctx, mainAccountID, envID, taskID, lastError, updatedAt)
}

func (h *ModelHandler) UpdateServerBrowserEnvSnapshot(ctx context.Context, env *model.ServerBrowserEnv) error {
	return h.repo.UpdateSnapshot(ctx, env)
}
