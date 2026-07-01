package BrowserEnv

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	BrowserEnvDAO "private_browser_server/Dao/BrowserEnv"
	TaskDAO "private_browser_server/Dao/Task"
	BrowserEnvModel "private_browser_server/Models/BrowserEnv"
	NodeModel "private_browser_server/Models/Node"
	TaskModel "private_browser_server/Models/Task"
	BrowserEnvRepo "private_browser_server/Repository/BrowserEnv"
	TaskRepo "private_browser_server/Repository/Task"
	EdgeClientService "private_browser_server/Service/EdgeClient"
	NodeService "private_browser_server/Service/Node"
	TaskService "private_browser_server/Service/Task"
)

var ErrInvalidParams = errors.New("invalid browser-env run params")

type Service struct{}

func NewService() *Service {
	return &Service{}
}

// List 返回中心当前缓存的 browser-env 列表视图。
//
// 设计来源：
// - API 规划已经明确：中心 `GET /browser-envs` 第一阶段先以 `server_browser_envs` 为主视图；
// - 因此这里先不夹带 Edge 实时拉取，避免普通列表查询退化成跨机探测；
// - 真正需要刷新事实时，调用方应走单条 `refresh` 接口。
func (s *Service) List(ctx context.Context, query BrowserEnvModel.ListQuery) (*BrowserEnvModel.ListResponse, error) {
	items, err := BrowserEnvRepo.NewRepository().List(ctx, query)
	if err != nil {
		return nil, err
	}
	return &BrowserEnvModel.ListResponse{
		Items: items,
		Total: len(items),
	}, nil
}

// GetDetail 返回中心缓存中的单条 browser-env 摘要。
//
// 职责边界：
// - 当前只返回 `server_browser_envs` 这一层已经保存的聚合摘要；
// - 不在 detail 查询里偷偷穿透 Edge；
// - 需要强制拉新时必须显式调用 `Refresh`，避免读接口带来不可预期的远端依赖。
func (s *Service) GetDetail(ctx context.Context, envID string) (*BrowserEnvModel.ServerBrowserEnv, error) {
	return BrowserEnvRepo.NewRepository().GetByEnvID(ctx, strings.TrimSpace(envID))
}

// Run 创建中心 browser-env run 任务，并在后台编排统一准入和 Edge run。
//
// 设计来源：
// - 前面已经把 quota / slot / health / verified 的中心准入规则独立收口；
// - 现在需要一个真正的 Server run 入口把这些规则接回主链，而不是一直停留在查询接口；
// - 这里先做“最小可用中心 run 骨架”：显式 slot、中心 task、Edge run、终态确认。
//
// 职责边界：
// - 负责读取中心 env 聚合记录、创建 server task、应用 run admission、发起 Edge run；
// - 负责在 Edge task 成功/失败后回写中心 env 摘要；
// - 不负责自动选 slot，不负责平台真实 quota 拉取，不负责跨 Client 调度。
func (s *Service) Run(ctx context.Context, envID string, request *BrowserEnvModel.RunRequest) (*BrowserEnvModel.RunTaskAcceptedResponse, error) {
	envID = strings.TrimSpace(envID)
	if envID == "" || request == nil || strings.TrimSpace(request.SlotID) == "" {
		return nil, fmt.Errorf("%w: envId 和 slotId 不能为空", ErrInvalidParams)
	}

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	env, err := BrowserEnvRepo.NewRepository().GetByEnvID(requestCtx, envID)
	if err != nil {
		return nil, err
	}

	taskID, err := TaskService.GetService().CreateTask(requestCtx, &TaskDAO.Row{
		MainAccountID: env.MainAccountID,
		ClientID:      env.ClientID,
		EnvID:         env.EnvID,
		TaskType:      "browser_env_run",
		ResourceType:  "browser_env",
		ResourceID:    env.EnvID,
	})
	if err != nil {
		return nil, err
	}

	go s.runInBackground(taskID, env, request)

	return &BrowserEnvModel.RunTaskAcceptedResponse{
		TaskID:       taskID,
		TaskType:     "browser_env_run",
		ResourceType: "browser_env",
		ResourceID:   env.EnvID,
		EventsURL:    fmt.Sprintf("/api/v1/server-tasks/%s/events", taskID),
	}, nil
}

// Stop 执行一次中心 browser-env stop，并同步返回最终结果。
//
// 设计来源：
// - Client stop 已经明确是短链路同步动作，普通 HTTP 足够表达最终结论；
// - 但中心层仍需要为 stop 留下一条 `server_task` 审计事实，避免 run/stop/backup/restore 的平台口径分裂；
// - 因此这里采用“同步返回 + 同步落 task”的组合，而不是再把 stop 强行任务化。
//
// 职责边界：
// - 负责读取中心 env 记录、校验目标节点健康、同步调用 Edge stop；
// - 负责回写 `server_browser_envs` 当前停止后摘要；
// - 不做自动重试，不做强制 kill 扩展，不做跨 Client stop。
func (s *Service) Stop(ctx context.Context, envID string, request *BrowserEnvModel.StopRequest) (*BrowserEnvModel.StopResponse, error) {
	envID = strings.TrimSpace(envID)
	if envID == "" {
		return nil, fmt.Errorf("envId 不能为空")
	}

	env, err := BrowserEnvRepo.NewRepository().GetByEnvID(ctx, envID)
	if err != nil {
		return nil, err
	}

	node, err := s.loadReadyNode(ctx, env.ClientID)
	if err != nil {
		return nil, err
	}

	taskID, err := TaskService.GetService().CreateTask(ctx, &TaskDAO.Row{
		MainAccountID: env.MainAccountID,
		ClientID:      env.ClientID,
		EnvID:         env.EnvID,
		TaskType:      "browser_env_stop",
		ResourceType:  "browser_env",
		ResourceID:    env.EnvID,
	})
	if err != nil {
		return nil, err
	}

	nowRFC3339 := time.Now().Format(time.RFC3339)
	_ = TaskService.GetService().PublishProgress(ctx, taskID, TaskModel.Event{
		Event:        TaskModel.EventProgress,
		TaskID:       taskID,
		TaskType:     "browser_env_stop",
		ResourceType: "browser_env",
		ResourceID:   env.EnvID,
		ClientID:     env.ClientID,
		EnvID:        env.EnvID,
		Stage:        "dispatch_edge_stop",
		Status:       TaskModel.StatusRunning,
		Message:      "dispatching edge stop",
		Timestamp:    nowRFC3339,
	})

	timeoutSeconds := normalizeServerBrowserEnvStopTimeout(request)
	stopResult, err := EdgeClientService.New().StopBrowserEnv(ctx, node.BaseURL, env.EnvID, &EdgeClientService.BrowserEnvStopRequest{
		TimeoutSeconds: timeoutSeconds,
	})
	if err != nil {
		_ = s.updateEnvAfterFailure(ctx, env, taskID, err.Error())
		_ = TaskService.GetService().PublishFailed(ctx, taskID, TaskModel.Event{
			Event:        TaskModel.EventFailed,
			TaskID:       taskID,
			TaskType:     "browser_env_stop",
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			Stage:        "dispatch_edge_stop_failed",
			Status:       TaskModel.StatusFailed,
			Message:      "dispatch edge stop failed",
			Error:        err.Error(),
			Suggestion:   "check edge client reachability and current env runtime state",
			Timestamp:    time.Now().Format(time.RFC3339),
		})
		return nil, err
	}

	if syncErr := s.syncEnvFromEdge(ctx, env, taskID, "", "", node.BaseURL); syncErr != nil {
		_ = s.updateEnvAfterFailure(ctx, env, taskID, syncErr.Error())
		_ = TaskService.GetService().PublishFailed(ctx, taskID, TaskModel.Event{
			Event:        TaskModel.EventFailed,
			TaskID:       taskID,
			TaskType:     "browser_env_stop",
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			Stage:        "finalize_sync_failed",
			Status:       TaskModel.StatusFailed,
			Message:      "edge stop succeeded but env sync failed",
			Error:        syncErr.Error(),
			Suggestion:   "recheck edge browser-env detail before trusting stop result",
			Timestamp:    time.Now().Format(time.RFC3339),
		})
		return nil, syncErr
	}

	_ = TaskService.GetService().PublishCompleted(ctx, taskID, TaskModel.Event{
		Event:        TaskModel.EventCompleted,
		TaskID:       taskID,
		TaskType:     "browser_env_stop",
		ResourceType: "browser_env",
		ResourceID:   env.EnvID,
		ClientID:     env.ClientID,
		EnvID:        env.EnvID,
		Stage:        "finalize_success",
		Status:       TaskModel.StatusSuccess,
		Message:      "browser env stop completed",
		Timestamp:    time.Now().Format(time.RFC3339),
	})

	return &BrowserEnvModel.StopResponse{
		EnvID:           stopResult.EnvID,
		Status:          stopResult.Status,
		ContainerStatus: stopResult.ContainerStatus,
		StoppedAt:       stopResult.StoppedAt,
	}, nil
}

// UpdateRuntimeImage 修改中心已绑定 browser-env 的正式运行镜像。
//
// 设计来源：
//   - 镜像地址属于 browser-env 运行契约，不属于 slot 默认基础镜像；
//   - created 是首次运行前配置态，stopped 是运行后已释放 slot/container 关系的干净隔离态；
//   - 因此只允许在 created/stopped 修改，不能在运行中、归档态或异常态热改。
//
// 职责边界：
// - 负责中心 env/节点准入、调用 Edge 同步修改、刷新中心缓存摘要；
// - 不创建 server task，不订阅 SSE；
// - 不拉镜像、不 run、不 reinit slot、不删除旧镜像。
func (s *Service) UpdateRuntimeImage(
	ctx context.Context,
	envID string,
	request *BrowserEnvModel.UpdateRuntimeImageRequest,
) (*BrowserEnvModel.UpdateRuntimeImageResponse, error) {
	envID = strings.TrimSpace(envID)
	if envID == "" {
		return nil, fmt.Errorf("envId 不能为空")
	}
	if request == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}
	image := strings.TrimSpace(request.Image)
	if image == "" {
		return nil, fmt.Errorf("runtime.image 不能为空")
	}

	env, err := BrowserEnvRepo.NewRepository().GetByEnvID(ctx, envID)
	if err != nil {
		return nil, err
	}
	if !isServerBrowserEnvRuntimeImageEditableState(env.Status) {
		return nil, fmt.Errorf("环境包当前状态为 %s，只有 created 或 stopped 状态才允许修改 runtime.image", env.Status)
	}

	node, err := s.loadReadyNode(ctx, env.ClientID)
	if err != nil {
		return nil, err
	}

	result, err := EdgeClientService.New().UpdateBrowserEnvRuntimeImage(
		ctx,
		node.BaseURL,
		env.EnvID,
		&EdgeClientService.BrowserEnvRuntimeImageRequest{Image: image},
	)
	if err != nil {
		_ = s.updateEnvAfterFailure(ctx, env, env.LastTaskID, err.Error())
		return nil, err
	}

	now := time.Now().Unix()
	if upsertErr := BrowserEnvRepo.NewRepository().Upsert(ctx, &BrowserEnvDAO.Row{
		EnvID:           env.EnvID,
		MainAccountID:   env.MainAccountID,
		ClientID:        env.ClientID,
		UserID:          env.UserID,
		RPAType:         env.RPAType,
		Name:            env.Name,
		Status:          result.Status,
		ContainerStatus: env.ContainerStatus,
		RuntimeStatus:   result.Status,
		CurrentSlotID:   env.CurrentSlotID,
		CDPURL:          env.CDPURL,
		WebVNCURL:       env.WebVNCURL,
		LastTaskID:      env.LastTaskID,
		LastError:       "",
		LastSyncedAt:    now,
		CreatedAt:       env.CreatedAt,
		UpdatedAt:       now,
		DeletedAt:       env.DeletedAt,
	}); upsertErr != nil {
		return nil, upsertErr
	}

	return &BrowserEnvModel.UpdateRuntimeImageResponse{
		EnvID:         result.EnvID,
		Status:        result.Status,
		PreviousImage: result.PreviousImage,
		Image:         result.Image,
		UpdatedAt:     result.UpdatedAt,
	}, nil
}

// isServerBrowserEnvRuntimeImageEditableState 集中收口中心层 runtime.image 修改准入状态。
//
// created 是首次运行前配置态，stopped 是运行后与 slot/container 完全隔离的干净态。
// 不能把 running/backed_up/deleted/error 放进来，否则会造成配置契约和运行事实漂移。
func isServerBrowserEnvRuntimeImageEditableState(status string) bool {
	switch strings.TrimSpace(status) {
	case "created", "stopped":
		return true
	default:
		return false
	}
}

// Backup 创建中心 browser-env backup 任务，并在后台编排 Edge backup 与终态同步。
//
// 设计来源：
// - backup 在 Client 侧已经是正式 SSE 长链路接口，中心层不能把它降级成同步 HTTP；
// - 同时平台要求所有生命周期动作都必须落到 `server_tasks` 作为中心审计事实；
// - 因此这里复用“中心 task + Edge task + 终态确认”的正式收口方式。
//
// 职责边界：
// - 负责读取中心 env 聚合记录、创建中心 task、发起 Edge backup、轮询 Edge task；
// - 负责在 Edge success 后再次同步 env 摘要到 `server_browser_envs`；
// - 不自动 restore，不自动 run，不绕过 Edge 正式 backup 协议。
func (s *Service) Backup(ctx context.Context, envID string) (*BrowserEnvModel.BackupTaskAcceptedResponse, error) {
	envID = strings.TrimSpace(envID)
	if envID == "" {
		return nil, fmt.Errorf("envId 不能为空")
	}

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	env, err := BrowserEnvRepo.NewRepository().GetByEnvID(requestCtx, envID)
	if err != nil {
		return nil, err
	}

	taskID, err := TaskService.GetService().CreateTask(requestCtx, &TaskDAO.Row{
		MainAccountID: env.MainAccountID,
		ClientID:      env.ClientID,
		EnvID:         env.EnvID,
		TaskType:      "browser_env_backup",
		ResourceType:  "browser_env",
		ResourceID:    env.EnvID,
	})
	if err != nil {
		return nil, err
	}

	go s.backupInBackground(taskID, env)

	return &BrowserEnvModel.BackupTaskAcceptedResponse{
		TaskID:       taskID,
		TaskType:     "browser_env_backup",
		ResourceType: "browser_env",
		ResourceID:   env.EnvID,
		EventsURL:    fmt.Sprintf("/api/v1/server-tasks/%s/events", taskID),
	}, nil
}

// Restore 创建中心 browser-env restore 任务，并在后台编排 Edge restore 与终态同步。
func (s *Service) Restore(ctx context.Context, envID string) (*BrowserEnvModel.RestoreTaskAcceptedResponse, error) {
	envID = strings.TrimSpace(envID)
	if envID == "" {
		return nil, fmt.Errorf("envId 不能为空")
	}

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	env, err := BrowserEnvRepo.NewRepository().GetByEnvID(requestCtx, envID)
	if err != nil {
		return nil, err
	}

	taskID, err := TaskService.GetService().CreateTask(requestCtx, &TaskDAO.Row{
		MainAccountID: env.MainAccountID,
		ClientID:      env.ClientID,
		EnvID:         env.EnvID,
		TaskType:      "browser_env_restore",
		ResourceType:  "browser_env",
		ResourceID:    env.EnvID,
	})
	if err != nil {
		return nil, err
	}

	go s.restoreInBackground(taskID, env)

	return &BrowserEnvModel.RestoreTaskAcceptedResponse{
		TaskID:       taskID,
		TaskType:     "browser_env_restore",
		ResourceType: "browser_env",
		ResourceID:   env.EnvID,
		EventsURL:    fmt.Sprintf("/api/v1/server-tasks/%s/events", taskID),
	}, nil
}

// DeletePackage 创建中心 browser-env package delete 任务。
//
// 设计来源：
// - package delete 的终态不是“状态变成 deleted 后继续留在主列表”，而是当前节点缓存应被移除；
// - 但删除过程本身仍然必须受 `server_tasks` 审计和 SSE 观察约束；
// - 因此这里采用“中心 task + Edge task + 删除中心缓存”的正式收口。
func (s *Service) DeletePackage(ctx context.Context, envID string) (*BrowserEnvModel.DeletePackageTaskAcceptedResponse, error) {
	envID = strings.TrimSpace(envID)
	if envID == "" {
		return nil, fmt.Errorf("envId 不能为空")
	}

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	env, err := BrowserEnvRepo.NewRepository().GetByEnvID(requestCtx, envID)
	if err != nil {
		return nil, err
	}

	taskID, err := TaskService.GetService().CreateTask(requestCtx, &TaskDAO.Row{
		MainAccountID: env.MainAccountID,
		ClientID:      env.ClientID,
		EnvID:         env.EnvID,
		TaskType:      "browser_env_delete_package",
		ResourceType:  "browser_env",
		ResourceID:    env.EnvID,
	})
	if err != nil {
		return nil, err
	}

	go s.deletePackageInBackground(taskID, env)

	return &BrowserEnvModel.DeletePackageTaskAcceptedResponse{
		TaskID:       taskID,
		TaskType:     "browser_env_delete_package",
		ResourceType: "browser_env",
		ResourceID:   env.EnvID,
		EventsURL:    fmt.Sprintf("/api/v1/server-tasks/%s/events", taskID),
	}, nil
}

// DeleteImage 执行一次中心 browser-env `/del`，同步返回镜像清理结果。
//
// 设计来源：
// - Client `/del` 已经明确是同步 HTTP，不创建 task；
// - 这条动作只清理本机镜像，不改环境资产，所以中心层也不应该强行任务化；
// - 同时中心仍要把最近一次 `/del` 的成功或失败写回 env 摘要，方便后续排障。
//
// 职责边界：
// - 负责读取中心 env 记录、校验目标节点健康、同步调用 Edge `/del`；
// - 负责回写 `lastTaskId / lastError / lastSyncedAt`；
// - 不删除中心 env 缓存，不改变 env 主状态。
func (s *Service) DeleteImage(ctx context.Context, envID string) (*BrowserEnvModel.DeleteBrowserEnvImageResponse, error) {
	envID = strings.TrimSpace(envID)
	if envID == "" {
		return nil, fmt.Errorf("envId 不能为空")
	}

	env, err := BrowserEnvRepo.NewRepository().GetByEnvID(ctx, envID)
	if err != nil {
		return nil, err
	}

	node, err := s.loadReadyNode(ctx, env.ClientID)
	if err != nil {
		return nil, err
	}

	result, err := EdgeClientService.New().DeleteBrowserEnvImage(ctx, node.BaseURL, env.EnvID)
	if err != nil {
		_ = s.updateEnvAfterFailure(ctx, env, env.LastTaskID, err.Error())
		return nil, err
	}

	now := time.Now().Unix()
	if upsertErr := BrowserEnvRepo.NewRepository().Upsert(ctx, &BrowserEnvDAO.Row{
		EnvID:           env.EnvID,
		MainAccountID:   env.MainAccountID,
		ClientID:        env.ClientID,
		UserID:          env.UserID,
		RPAType:         env.RPAType,
		Name:            env.Name,
		Status:          env.Status,
		ContainerStatus: env.ContainerStatus,
		RuntimeStatus:   env.RuntimeStatus,
		CurrentSlotID:   env.CurrentSlotID,
		CDPURL:          env.CDPURL,
		WebVNCURL:       env.WebVNCURL,
		LastTaskID:      env.LastTaskID,
		LastError:       "",
		LastSyncedAt:    now,
		CreatedAt:       env.CreatedAt,
		UpdatedAt:       now,
		DeletedAt:       env.DeletedAt,
	}); upsertErr != nil {
		return nil, upsertErr
	}

	mappedResults := make([]BrowserEnvModel.DeleteBrowserEnvImageResult, 0, len(result.Results))
	for _, item := range result.Results {
		mappedResults = append(mappedResults, BrowserEnvModel.DeleteBrowserEnvImageResult{
			Image:    item.Image,
			Deleted:  item.Deleted,
			Untagged: item.Untagged,
		})
	}

	return &BrowserEnvModel.DeleteBrowserEnvImageResponse{
		EnvID:          result.EnvID,
		Image:          result.Image,
		ImageRemoved:   result.ImageRemoved,
		Results:        mappedResults,
		WarningMessage: result.WarningMessage,
		DeletedAt:      result.DeletedAt,
	}, nil
}

// Refresh 主动从目标 Edge 拉取一次 env 详情，并更新中心缓存。
//
// 设计来源：
// - 中心列表/detail 当前都先读 `server_browser_envs`，这能保证主视图稳定；
// - 但管理员和后续 run 收口仍需要一个显式“现在就去同步一次”的入口；
// - 因此 refresh 被设计成同步 HTTP：单条 env、单次 detail 拉取、即时回写，不必上 SSE。
//
// 职责边界：
// - 只允许刷新中心已存在的 env；
// - 只在目标节点 `healthy + verified` 时执行；
// - 不创建 server task，不发任何生命周期动作。
func (s *Service) Refresh(ctx context.Context, envID string) (*BrowserEnvModel.RefreshResponse, error) {
	env, err := BrowserEnvRepo.NewRepository().GetByEnvID(ctx, strings.TrimSpace(envID))
	if err != nil {
		return nil, err
	}

	admission, err := NodeService.CheckRunAdmission(ctx, env.ClientID)
	if err != nil {
		return nil, err
	}
	if admission.Node == nil {
		return nil, fmt.Errorf("edge client not found")
	}
	if admission.Node.HealthStatus != "healthy" || admission.Node.DiscoveryStatus != "verified" {
		return nil, fmt.Errorf("edge client is not healthy and verified")
	}

	if err = s.syncEnvFromEdge(ctx, env, env.LastTaskID, env.CurrentSlotID, "", admission.Node.BaseURL); err != nil {
		return nil, err
	}

	updated, err := BrowserEnvRepo.NewRepository().GetByEnvID(ctx, env.EnvID)
	if err != nil {
		return nil, err
	}
	return &BrowserEnvModel.RefreshResponse{
		EnvID:           updated.EnvID,
		ClientID:        updated.ClientID,
		Status:          updated.Status,
		RuntimeStatus:   updated.RuntimeStatus,
		ContainerStatus: updated.ContainerStatus,
		CurrentSlotID:   updated.CurrentSlotID,
		WebVNCURL:       updated.WebVNCURL,
		LastTaskID:      updated.LastTaskID,
		LastError:       updated.LastError,
		LastSyncedAt:    updated.LastSyncedAt,
	}, nil
}

func (s *Service) runInBackground(taskID string, env *BrowserEnvModel.ServerBrowserEnv, request *BrowserEnvModel.RunRequest) {
	taskSvc := TaskService.GetService()
	now := func() string { return time.Now().Format(time.RFC3339) }
	publishProgress := func(stage, status, message string) {
		_ = taskSvc.PublishProgress(context.Background(), taskID, TaskModel.Event{
			Event:        TaskModel.EventProgress,
			TaskID:       taskID,
			TaskType:     "browser_env_run",
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			SlotID:       request.SlotID,
			Stage:        stage,
			Status:       status,
			Message:      message,
			Timestamp:    now(),
		})
	}
	publishFailed := func(stage, message, errMsg, suggestion string) {
		_ = taskSvc.PublishFailed(context.Background(), taskID, TaskModel.Event{
			Event:        TaskModel.EventFailed,
			TaskID:       taskID,
			TaskType:     "browser_env_run",
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			SlotID:       request.SlotID,
			Stage:        stage,
			Status:       TaskModel.StatusFailed,
			Message:      message,
			Error:        errMsg,
			Suggestion:   suggestion,
			Timestamp:    now(),
		})
	}
	publishCompleted := func(message string) {
		_ = taskSvc.PublishCompleted(context.Background(), taskID, TaskModel.Event{
			Event:        TaskModel.EventCompleted,
			TaskID:       taskID,
			TaskType:     "browser_env_run",
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			SlotID:       request.SlotID,
			Stage:        "finalize_success",
			Status:       TaskModel.StatusSuccess,
			Message:      message,
			Timestamp:    now(),
		})
	}

	publishProgress("load_server_env", TaskModel.StatusPending, "task accepted")

	requestCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	admission, err := NodeService.CheckRunAdmission(requestCtx, env.ClientID)
	if err != nil {
		_ = s.updateEnvAfterFailure(requestCtx, env, taskID, err.Error())
		publishFailed("check_run_admission_failed", "load run admission failed", err.Error(), "check node cache and quota snapshot first")
		return
	}
	if !admission.Result.Allowed {
		errorMessage := strings.Join(admission.Result.Reasons, ",")
		_ = s.updateEnvAfterFailure(requestCtx, env, taskID, errorMessage)
		publishFailed("finalize_admission_failed", "browser env run blocked by admission", errorMessage, admission.Result.Suggestion)
		return
	}

	publishProgress("dispatch_edge_run", TaskModel.StatusRunning, "run admission passed, dispatching edge run")

	edgeClient := EdgeClientService.New()
	edgeAccepted, err := edgeClient.RunBrowserEnv(requestCtx, admission.Node.BaseURL, env.EnvID, &EdgeClientService.BrowserEnvRunRequest{
		SlotID:        request.SlotID,
		ForceRecreate: request.ForceRecreate,
	})
	if err != nil {
		_ = s.updateEnvAfterFailure(requestCtx, env, taskID, err.Error())
		publishFailed("dispatch_edge_run_failed", "dispatch edge run failed", err.Error(), "check edge client reachability and env runtime state")
		return
	}

	_ = TaskRepo.NewRepository().UpdateStatus(requestCtx, &TaskDAO.Row{
		ID:         taskID,
		Status:     TaskModel.StatusRunning,
		EdgeTaskID: edgeAccepted.TaskID,
		UpdatedAt:  time.Now().Unix(),
	})
	publishProgress("edge_task_accepted", TaskModel.StatusRunning, fmt.Sprintf("edge task accepted: %s", edgeAccepted.TaskID))

	lastStage := ""
	lastStatus := ""
	for {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 20*time.Second)
		detail, detailErr := edgeClient.GetTaskDetail(pollCtx, admission.Node.BaseURL, edgeAccepted.TaskID)
		pollCancel()
		if detailErr != nil {
			_ = s.updateEnvAfterFailure(context.Background(), env, taskID, detailErr.Error())
			publishFailed("edge_task_detail_failed", "load edge task detail failed", detailErr.Error(), "check edge task state and edge logs")
			return
		}

		if detail.CurrentStage != "" && (detail.CurrentStage != lastStage || detail.Status != lastStatus) {
			lastStage = detail.CurrentStage
			lastStatus = detail.Status
			publishProgress("edge."+detail.CurrentStage, detail.Status, detail.Message)
		}

		switch detail.Status {
		case TaskModel.StatusSuccess:
			syncCtx, syncCancel := context.WithTimeout(context.Background(), 20*time.Second)
			syncErr := s.syncEnvFromEdge(syncCtx, env, taskID, request.SlotID, edgeAccepted.TaskID, admission.Node.BaseURL)
			syncCancel()
			if syncErr != nil {
				_ = s.updateEnvAfterFailure(context.Background(), env, taskID, syncErr.Error())
				publishFailed("finalize_sync_failed", "edge run succeeded but env sync failed", syncErr.Error(), "recheck edge browser-env detail before trusting success")
				return
			}
			publishCompleted("browser env run completed")
			return
		case TaskModel.StatusFailed:
			errMsg := detail.Error
			if strings.TrimSpace(errMsg) == "" {
				errMsg = detail.Message
			}
			_ = s.updateEnvAfterFailure(context.Background(), env, taskID, errMsg)
			publishFailed("finalize_edge_failed", "browser env run failed", errMsg, detail.Suggestion)
			return
		}

		time.Sleep(2 * time.Second)
	}
}

// backupInBackground 负责中心 backup 的后台编排。
//
// 维护原则：
// - 这里只做中心编排，不重写 Edge backup 业务规则；
// - 中心只能观察、透传和收口 Edge task，不能在这里偷偷生成第二套 backup 状态机；
// - 一旦 Edge success 后中心无法再次确认 env 摘要，仍必须按 failed 收口。
func (s *Service) backupInBackground(taskID string, env *BrowserEnvModel.ServerBrowserEnv) {
	s.runEdgeTaskLifecycle(taskID, env, edgeLifecycleTaskConfig{
		TaskType:            "browser_env_backup",
		StartStage:          "load_server_env",
		StartMessage:        "task accepted",
		DispatchStage:       "dispatch_edge_backup",
		DispatchMessage:     "dispatching edge backup",
		AcceptedStage:       "edge_task_accepted",
		AcceptedMessage:     "edge task accepted: %s",
		DispatchFailedStage: "dispatch_edge_backup_failed",
		DispatchFailedMsg:   "dispatch edge backup failed",
		DispatchSuggestion:  "check edge client reachability and current env backup preconditions",
		EdgeFailedStage:     "finalize_edge_failed",
		EdgeFailedMsg:       "browser env backup failed",
		SyncFailedStage:     "finalize_sync_failed",
		SyncFailedMsg:       "edge backup succeeded but env sync failed",
		SyncSuggestion:      "recheck edge browser-env detail before trusting backup success",
		SuccessMessage:      "browser env backup completed",
		Dispatch: func(ctx context.Context, nodeBaseURL string) (*EdgeClientService.TaskAcceptedResponse, error) {
			accepted, err := EdgeClientService.New().BackupBrowserEnv(ctx, nodeBaseURL, env.EnvID)
			if err != nil {
				return nil, err
			}
			return &EdgeClientService.TaskAcceptedResponse{
				TaskID:       accepted.TaskID,
				TaskType:     accepted.TaskType,
				ResourceType: accepted.ResourceType,
				ResourceID:   accepted.ResourceID,
				EventsURL:    accepted.EventsURL,
			}, nil
		},
	})
}

// restoreInBackground 负责中心 restore 的后台编排。
func (s *Service) restoreInBackground(taskID string, env *BrowserEnvModel.ServerBrowserEnv) {
	s.runEdgeTaskLifecycle(taskID, env, edgeLifecycleTaskConfig{
		TaskType:            "browser_env_restore",
		StartStage:          "load_server_env",
		StartMessage:        "task accepted",
		DispatchStage:       "dispatch_edge_restore",
		DispatchMessage:     "dispatching edge restore",
		AcceptedStage:       "edge_task_accepted",
		AcceptedMessage:     "edge task accepted: %s",
		DispatchFailedStage: "dispatch_edge_restore_failed",
		DispatchFailedMsg:   "dispatch edge restore failed",
		DispatchSuggestion:  "check edge client reachability and current env restore preconditions",
		EdgeFailedStage:     "finalize_edge_failed",
		EdgeFailedMsg:       "browser env restore failed",
		SyncFailedStage:     "finalize_sync_failed",
		SyncFailedMsg:       "edge restore succeeded but env sync failed",
		SyncSuggestion:      "recheck edge browser-env detail before trusting restore success",
		SuccessMessage:      "browser env restore completed",
		Dispatch: func(ctx context.Context, nodeBaseURL string) (*EdgeClientService.TaskAcceptedResponse, error) {
			accepted, err := EdgeClientService.New().RestoreBrowserEnv(ctx, nodeBaseURL, env.EnvID)
			if err != nil {
				return nil, err
			}
			return &EdgeClientService.TaskAcceptedResponse{
				TaskID:       accepted.TaskID,
				TaskType:     accepted.TaskType,
				ResourceType: accepted.ResourceType,
				ResourceID:   accepted.ResourceID,
				EventsURL:    accepted.EventsURL,
			}, nil
		},
	})
}

// deletePackageInBackground 负责中心 package delete 的后台编排。
//
// 与 backup/restore 的关键区别：
// - 删除成功后，Edge 侧 detail 已经不再可读；
// - 因此中心不能再走“重新拉 detail 同步摘要”的收口方式；
// - 正式成功口径改成：Edge task success + 中心缓存删除成功。
func (s *Service) deletePackageInBackground(taskID string, env *BrowserEnvModel.ServerBrowserEnv) {
	taskSvc := TaskService.GetService()
	now := func() string { return time.Now().Format(time.RFC3339) }
	publishProgress := func(stage, status, message string) {
		_ = taskSvc.PublishProgress(context.Background(), taskID, TaskModel.Event{
			Event:        TaskModel.EventProgress,
			TaskID:       taskID,
			TaskType:     "browser_env_delete_package",
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			Stage:        stage,
			Status:       status,
			Message:      message,
			Timestamp:    now(),
		})
	}
	publishFailed := func(stage, message, errMsg, suggestion string) {
		_ = taskSvc.PublishFailed(context.Background(), taskID, TaskModel.Event{
			Event:        TaskModel.EventFailed,
			TaskID:       taskID,
			TaskType:     "browser_env_delete_package",
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			Stage:        stage,
			Status:       TaskModel.StatusFailed,
			Message:      message,
			Error:        errMsg,
			Suggestion:   suggestion,
			Timestamp:    now(),
		})
	}
	publishCompleted := func(message string) {
		_ = taskSvc.PublishCompleted(context.Background(), taskID, TaskModel.Event{
			Event:        TaskModel.EventCompleted,
			TaskID:       taskID,
			TaskType:     "browser_env_delete_package",
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			Stage:        "finalize_success",
			Status:       TaskModel.StatusSuccess,
			Message:      message,
			Timestamp:    now(),
		})
	}

	publishProgress("load_server_env", TaskModel.StatusPending, "task accepted")

	requestCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	node, err := s.loadReadyNode(requestCtx, env.ClientID)
	if err != nil {
		_ = s.updateEnvAfterFailure(requestCtx, env, taskID, err.Error())
		publishFailed("load_ready_node_failed", "load lifecycle node failed", err.Error(), "check node health and verify status first")
		return
	}

	publishProgress("dispatch_edge_delete_package", TaskModel.StatusRunning, "dispatching edge package delete")

	edgeAccepted, err := EdgeClientService.New().DeleteBrowserEnvPackage(requestCtx, node.BaseURL, env.EnvID)
	if err != nil {
		_ = s.updateEnvAfterFailure(requestCtx, env, taskID, err.Error())
		publishFailed("dispatch_edge_delete_package_failed", "dispatch edge package delete failed", err.Error(), "check edge client reachability and current env delete preconditions")
		return
	}

	_ = TaskRepo.NewRepository().UpdateStatus(requestCtx, &TaskDAO.Row{
		ID:         taskID,
		Status:     TaskModel.StatusRunning,
		EdgeTaskID: edgeAccepted.TaskID,
		UpdatedAt:  time.Now().Unix(),
	})
	publishProgress("edge_task_accepted", TaskModel.StatusRunning, fmt.Sprintf("edge task accepted: %s", edgeAccepted.TaskID))

	lastStage := ""
	lastStatus := ""
	for {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 20*time.Second)
		detail, detailErr := EdgeClientService.New().GetTaskDetail(pollCtx, node.BaseURL, edgeAccepted.TaskID)
		pollCancel()
		if detailErr != nil {
			_ = s.updateEnvAfterFailure(context.Background(), env, taskID, detailErr.Error())
			publishFailed("edge_task_detail_failed", "load edge task detail failed", detailErr.Error(), "check edge task state and edge logs")
			return
		}

		if detail.CurrentStage != "" && (detail.CurrentStage != lastStage || detail.Status != lastStatus) {
			lastStage = detail.CurrentStage
			lastStatus = detail.Status
			publishProgress("edge."+detail.CurrentStage, detail.Status, detail.Message)
		}

		switch detail.Status {
		case TaskModel.StatusSuccess:
			deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 10*time.Second)
			deleteErr := BrowserEnvRepo.NewRepository().DeleteByEnvID(deleteCtx, env.EnvID)
			deleteCancel()
			if deleteErr != nil {
				_ = s.updateEnvAfterFailure(context.Background(), env, taskID, deleteErr.Error())
				publishFailed("finalize_cache_delete_failed", "edge package delete succeeded but center cache delete failed", deleteErr.Error(), "repair server browser-env cache before trusting delete result")
				return
			}
			publishCompleted("browser env package delete completed")
			return
		case TaskModel.StatusFailed:
			errMsg := detail.Error
			if strings.TrimSpace(errMsg) == "" {
				errMsg = detail.Message
			}
			_ = s.updateEnvAfterFailure(context.Background(), env, taskID, errMsg)
			publishFailed("finalize_edge_failed", "browser env package delete failed", errMsg, detail.Suggestion)
			return
		}

		time.Sleep(2 * time.Second)
	}
}

type edgeLifecycleTaskConfig struct {
	TaskType            string
	StartStage          string
	StartMessage        string
	DispatchStage       string
	DispatchMessage     string
	AcceptedStage       string
	AcceptedMessage     string
	DispatchFailedStage string
	DispatchFailedMsg   string
	DispatchSuggestion  string
	EdgeFailedStage     string
	EdgeFailedMsg       string
	SyncFailedStage     string
	SyncFailedMsg       string
	SyncSuggestion      string
	SuccessMessage      string
	Dispatch            func(ctx context.Context, nodeBaseURL string) (*EdgeClientService.TaskAcceptedResponse, error)
}

// runEdgeTaskLifecycle 把 backup / restore 这类“中心接单 + Edge SSE task + 中心二次确认”链路统一收口。
//
// 设计来源：
// - run、backup、restore 后续都会走“中心 task + Edge task + 再次同步事实”这条骨架；
// - 如果每条生命周期都复制一遍轮询和失败收口逻辑，后续非常容易在 stage、错误口径和任务终态上漂移；
// - 因此把公共编排抽成一个受控 helper，上层只传各动作自己的阶段命名和 dispatch 方法。
//
// 不负责什么：
// - 不做 run admission；
// - 不自行决定 Edge 业务参数；
// - 不替代各动作自己的前置条件文档。
func (s *Service) runEdgeTaskLifecycle(taskID string, env *BrowserEnvModel.ServerBrowserEnv, config edgeLifecycleTaskConfig) {
	taskSvc := TaskService.GetService()
	now := func() string { return time.Now().Format(time.RFC3339) }
	publishProgress := func(stage, status, message string) {
		_ = taskSvc.PublishProgress(context.Background(), taskID, TaskModel.Event{
			Event:        TaskModel.EventProgress,
			TaskID:       taskID,
			TaskType:     config.TaskType,
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			Stage:        stage,
			Status:       status,
			Message:      message,
			Timestamp:    now(),
		})
	}
	publishFailed := func(stage, message, errMsg, suggestion string) {
		_ = taskSvc.PublishFailed(context.Background(), taskID, TaskModel.Event{
			Event:        TaskModel.EventFailed,
			TaskID:       taskID,
			TaskType:     config.TaskType,
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			Stage:        stage,
			Status:       TaskModel.StatusFailed,
			Message:      message,
			Error:        errMsg,
			Suggestion:   suggestion,
			Timestamp:    now(),
		})
	}
	publishCompleted := func(message string) {
		_ = taskSvc.PublishCompleted(context.Background(), taskID, TaskModel.Event{
			Event:        TaskModel.EventCompleted,
			TaskID:       taskID,
			TaskType:     config.TaskType,
			ResourceType: "browser_env",
			ResourceID:   env.EnvID,
			ClientID:     env.ClientID,
			EnvID:        env.EnvID,
			Stage:        "finalize_success",
			Status:       TaskModel.StatusSuccess,
			Message:      message,
			Timestamp:    now(),
		})
	}

	publishProgress(config.StartStage, TaskModel.StatusPending, config.StartMessage)

	requestCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	node, err := s.loadReadyNode(requestCtx, env.ClientID)
	if err != nil {
		_ = s.updateEnvAfterFailure(requestCtx, env, taskID, err.Error())
		publishFailed("load_ready_node_failed", "load lifecycle node failed", err.Error(), "check node health and verify status first")
		return
	}

	publishProgress(config.DispatchStage, TaskModel.StatusRunning, config.DispatchMessage)

	edgeAccepted, err := config.Dispatch(requestCtx, node.BaseURL)
	if err != nil {
		_ = s.updateEnvAfterFailure(requestCtx, env, taskID, err.Error())
		publishFailed(config.DispatchFailedStage, config.DispatchFailedMsg, err.Error(), config.DispatchSuggestion)
		return
	}

	_ = TaskRepo.NewRepository().UpdateStatus(requestCtx, &TaskDAO.Row{
		ID:         taskID,
		Status:     TaskModel.StatusRunning,
		EdgeTaskID: edgeAccepted.TaskID,
		UpdatedAt:  time.Now().Unix(),
	})
	publishProgress(config.AcceptedStage, TaskModel.StatusRunning, fmt.Sprintf(config.AcceptedMessage, edgeAccepted.TaskID))

	lastStage := ""
	lastStatus := ""
	for {
		pollCtx, pollCancel := context.WithTimeout(context.Background(), 20*time.Second)
		detail, detailErr := EdgeClientService.New().GetTaskDetail(pollCtx, node.BaseURL, edgeAccepted.TaskID)
		pollCancel()
		if detailErr != nil {
			_ = s.updateEnvAfterFailure(context.Background(), env, taskID, detailErr.Error())
			publishFailed("edge_task_detail_failed", "load edge task detail failed", detailErr.Error(), "check edge task state and edge logs")
			return
		}

		if detail.CurrentStage != "" && (detail.CurrentStage != lastStage || detail.Status != lastStatus) {
			lastStage = detail.CurrentStage
			lastStatus = detail.Status
			publishProgress("edge."+detail.CurrentStage, detail.Status, detail.Message)
		}

		switch detail.Status {
		case TaskModel.StatusSuccess:
			syncCtx, syncCancel := context.WithTimeout(context.Background(), 20*time.Second)
			syncErr := s.syncEnvFromEdge(syncCtx, env, taskID, "", edgeAccepted.TaskID, node.BaseURL)
			syncCancel()
			if syncErr != nil {
				_ = s.updateEnvAfterFailure(context.Background(), env, taskID, syncErr.Error())
				publishFailed(config.SyncFailedStage, config.SyncFailedMsg, syncErr.Error(), config.SyncSuggestion)
				return
			}
			publishCompleted(config.SuccessMessage)
			return
		case TaskModel.StatusFailed:
			errMsg := detail.Error
			if strings.TrimSpace(errMsg) == "" {
				errMsg = detail.Message
			}
			_ = s.updateEnvAfterFailure(context.Background(), env, taskID, errMsg)
			publishFailed(config.EdgeFailedStage, config.EdgeFailedMsg, errMsg, detail.Suggestion)
			return
		}

		time.Sleep(2 * time.Second)
	}
}

// loadReadyNode 只校验“这个中心节点当前是否 still healthy + verified 且可被生命周期动作使用”。
//
// 设计来源：
// - 用户已经明确：run 要额外受 quota/slot 准入控制，但 stop/backup/restore 只需要节点本身可用；
// - 之前 stop 复用了 run admission，这会把 quota 误带进非 run 动作；
// - 因此这里单独抽出最小节点可用性校验，后续所有非 run 生命周期都应走这里。
func (s *Service) loadReadyNode(ctx context.Context, clientID string) (*NodeModel.EdgeClient, error) {
	admission, err := NodeService.CheckRunAdmission(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if admission.Node == nil {
		return nil, fmt.Errorf("edge client not found")
	}
	if admission.Node.HealthStatus != "healthy" || admission.Node.DiscoveryStatus != "verified" {
		return nil, fmt.Errorf("edge client is not healthy and verified")
	}
	return admission.Node, nil
}

// syncEnvFromEdge 把目标 Edge 的当前 env 摘要回写到中心缓存。
//
// 维护原则：
// - 中心只同步“够用的聚合字段”，不镜像 Edge 全量详情；
// - 当前 `currentSlotId` 仍以中心编排链路传入的 slotId 为准，因为 Edge detail 还没有单独返回这个字段；
// - `edgeTaskID` 先作为扩展位保留，当前中心主事实仍以 `server_task` 为准，不把 Edge taskId 写成新的事实源。
func (s *Service) syncEnvFromEdge(ctx context.Context, env *BrowserEnvModel.ServerBrowserEnv, taskID, slotID, edgeTaskID, baseURL string) error {
	detail, err := EdgeClientService.New().GetBrowserEnvDetail(ctx, baseURL, env.EnvID)
	if err != nil {
		return err
	}
	webVNCURL := strings.TrimSpace(detail.VNC.WebVNCURL)
	if webVNCURL == "" {
		webVNCURL = strings.TrimSpace(detail.Index.WebVNCURL)
	}
	now := time.Now().Unix()
	status := strings.TrimSpace(detail.Index.Status)
	if status == "" {
		status = env.Status
	}
	containerStatus := strings.TrimSpace(detail.Index.ContainerStatus)
	if containerStatus == "" {
		containerStatus = env.ContainerStatus
	}
	name := strings.TrimSpace(detail.Index.Name)
	if name == "" {
		name = env.Name
	}
	row := &BrowserEnvDAO.Row{
		EnvID:           env.EnvID,
		MainAccountID:   env.MainAccountID,
		ClientID:        env.ClientID,
		UserID:          coalesce(detail.Index.UserID, env.UserID),
		RPAType:         coalesce(detail.Index.RPAType, env.RPAType),
		Name:            name,
		Status:          status,
		ContainerStatus: containerStatus,
		RuntimeStatus:   status,
		CurrentSlotID:   slotID,
		CDPURL:          env.CDPURL,
		WebVNCURL:       webVNCURL,
		LastTaskID:      taskID,
		LastError:       "",
		LastSyncedAt:    now,
		CreatedAt:       env.CreatedAt,
		UpdatedAt:       now,
		DeletedAt:       env.DeletedAt,
	}
	return BrowserEnvRepo.NewRepository().Upsert(ctx, row)
}

func (s *Service) updateEnvAfterFailure(ctx context.Context, env *BrowserEnvModel.ServerBrowserEnv, taskID, lastError string) error {
	now := time.Now().Unix()
	return BrowserEnvRepo.NewRepository().Upsert(ctx, &BrowserEnvDAO.Row{
		EnvID:           env.EnvID,
		MainAccountID:   env.MainAccountID,
		ClientID:        env.ClientID,
		UserID:          env.UserID,
		RPAType:         env.RPAType,
		Name:            env.Name,
		Status:          env.Status,
		ContainerStatus: env.ContainerStatus,
		RuntimeStatus:   env.RuntimeStatus,
		CurrentSlotID:   env.CurrentSlotID,
		CDPURL:          env.CDPURL,
		WebVNCURL:       env.WebVNCURL,
		LastTaskID:      taskID,
		LastError:       strings.TrimSpace(lastError),
		LastSyncedAt:    now,
		CreatedAt:       env.CreatedAt,
		UpdatedAt:       now,
		DeletedAt:       env.DeletedAt,
	})
}

func coalesce(primary, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}

func normalizeServerBrowserEnvStopTimeout(request *BrowserEnvModel.StopRequest) int {
	if request == nil || request.TimeoutSeconds <= 0 {
		return 10
	}
	if request.TimeoutSeconds > 3600 {
		return 3600
	}
	return request.TimeoutSeconds
}
