package Env

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	EdgeAPI "private_browser_server/EdgeClient"
	taskModel "private_browser_server/Models/Task"
)

// ensureRuntimeImageReadyForRun 负责在真正调用 Edge run 前确认 runtime.image 已经在 Client 本机可用。
//
// 设计来源：
// - 当前环境包真正要运行哪个镜像，只能以 Edge 环境包 profile.runtime.image 为准；
// - 用户明确要求 Node Server 在 run 前闭环处理“镜像不存在则先 pull-image”；
// - `/edge/docker/containers` 只能看容器事实，不能证明镜像已经存在，所以这里必须查 `/edge/docker/images`。
//
// 维护边界：
// - 这里只检查和拉取一个已经写进环境包的镜像，不负责镜像策略选择，也不替换镜像引用；
// - pull-image 成功后还会再次核对 docker/images，避免“任务成功但镜像仍不可见”的假阳性；
// - 失败时返回可执行的错误，让上层把 run 任务明确收口为 failed。
func ensureRuntimeImageReadyForRun(parentCtx context.Context, baseURL string, envID string, reporter runTaskProgressReporter) error {
	if reporter != nil {
		reporter("running", taskModel.TaskStatusRunning, "image_check", "开始检查运行镜像是否已在 Edge 本机可用", map[string]any{"envId": envID})
	}
	detail, err := fetchEdgeBrowserEnvRunDetail(parentCtx, baseURL, envID)
	if err != nil {
		return fmt.Errorf("读取 Edge 环境包详情失败: %w", err)
	}
	runtimeImage := strings.TrimSpace(detail.Profile.Runtime.Image)
	if runtimeImage == "" {
		return fmt.Errorf("Edge 环境包 profile.runtime.image 为空，Node Server 无法判断需要检查哪个镜像")
	}

	images, err := EdgeAPI.New().GetDockerImages(parentCtx, baseURL, "")
	if err != nil {
		return fmt.Errorf("读取 Edge Docker 镜像列表失败: %w", err)
	}
	if runRuntimeImageExists(images, runtimeImage) {
		if reporter != nil {
			reporter("progress", taskModel.TaskStatusRunning, "image_check", "运行镜像已存在，无需拉取", map[string]any{"image": runtimeImage})
		}
		return nil
	}
	if reporter != nil {
		reporter("progress", taskModel.TaskStatusRunning, "pulling_image", "运行镜像缺失，开始调用 Edge pull-image", map[string]any{"image": runtimeImage})
	}

	preflightCtx, cancel := context.WithTimeout(parentCtx, runImagePreflightTimeout)
	defer cancel()
	pullResp, err := EdgeAPI.New().PullDockerImageTask(preflightCtx, baseURL, "", &EdgeAPI.PullImageRequest{Image: runtimeImage})
	if err != nil {
		return fmt.Errorf("缺少运行镜像 %s，触发 Edge pull-image 失败: %w", runtimeImage, err)
	}
	if pullResp == nil || strings.TrimSpace(pullResp.TaskID) == "" {
		return fmt.Errorf("缺少运行镜像 %s，但 Edge pull-image 响应缺少 taskId，无法确认拉取结果", runtimeImage)
	}
	if reporter != nil {
		reporter("progress", taskModel.TaskStatusRunning, "pulling_image", "Edge pull-image 任务已创建", map[string]any{
			"image":      runtimeImage,
			"edgeTaskId": strings.TrimSpace(pullResp.TaskID),
		})
	}
	terminalTask, err := waitEdgeTaskTerminal(preflightCtx, baseURL, strings.TrimSpace(pullResp.TaskID), "pulling_image", reporter)
	if err != nil {
		return fmt.Errorf("缺少运行镜像 %s，Edge pull-image task=%s 未成功完成: %w", runtimeImage, strings.TrimSpace(pullResp.TaskID), err)
	}

	images, err = EdgeAPI.New().GetDockerImages(parentCtx, baseURL, "")
	if err != nil {
		return fmt.Errorf("Edge pull-image task=%s 已成功，但重新读取 Docker 镜像列表失败: %w", terminalTask.TaskID, err)
	}
	if !runRuntimeImageExists(images, runtimeImage) {
		return fmt.Errorf("Edge pull-image task=%s 返回 success，但本机 Docker 镜像列表仍未找到 %s", terminalTask.TaskID, runtimeImage)
	}
	if reporter != nil {
		reporter("progress", taskModel.TaskStatusRunning, "pulling_image", "运行镜像已拉取完成并通过复核", map[string]any{
			"image":      runtimeImage,
			"edgeTaskId": terminalTask.TaskID,
		})
	}
	return nil
}

// fetchEdgeBrowserEnvRunDetail 读取 run 前必需的最小 Edge 环境包详情。
//
// 这里只拿 profile.runtime.image，不把完整详情模型搬到 Node Server，
// 避免中心层和 Edge 详情结构耦合过深。
func fetchEdgeBrowserEnvRunDetail(ctx context.Context, baseURL string, envID string) (*edgeRunBrowserEnvDetail, error) {
	var detail edgeRunBrowserEnvDetail
	if err := EdgeAPI.New().DoJSON(ctx, baseURL, http.MethodGet, "/api/v1/edge/browser-envs/"+url.PathEscape(envID), "", nil, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// waitEdgeTaskTerminal 轮询 Edge 内存任务，直到拿到 success/failed 等终态。
//
// pull-image 是 Edge 端异步长任务，Node Server 只有明确看到终态，才能决定这次 run 是否可以继续。
func waitEdgeTaskTerminal(ctx context.Context, baseURL string, edgeTaskID string, stage string, reporter runTaskProgressReporter) (*EdgeAPI.EdgeTask, error) {
	ticker := time.NewTicker(runImagePreflightPollInterval)
	defer ticker.Stop()
	lastSignature := ""

	for {
		edgeTask, err := EdgeAPI.New().GetEdgeTask(ctx, baseURL, "", edgeTaskID)
		if err != nil {
			return nil, err
		}
		signature := strings.ToLower(strings.TrimSpace(edgeTask.Status)) + "|" + firstNonEmpty(edgeTask.Message, edgeTask.LastError)
		if reporter != nil && signature != lastSignature {
			reporter("progress", mapEdgeTaskStatus(edgeTask.Status), stage, firstNonEmpty(edgeTask.Message, edgeTask.LastError, "Edge task running"), map[string]any{
				"edgeTaskId": edgeTask.TaskID,
				"edgeStatus": strings.TrimSpace(edgeTask.Status),
			})
			lastSignature = signature
		}
		switch strings.ToLower(strings.TrimSpace(edgeTask.Status)) {
		case "success", "done":
			return edgeTask, nil
		case "failed", "error":
			return edgeTask, fmt.Errorf(runTaskFirstMessage(edgeTask.LastError, edgeTask.Message, "Edge task failed"))
		case "queued", "pending", "running":
		default:
			return edgeTask, fmt.Errorf("Edge task 状态不可识别，不能作为成功事实: %s", strings.TrimSpace(edgeTask.Status))
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("等待 Edge task 超时或被取消: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// runRuntimeImageExists 判断目标 runtime.image 是否已经出现在 Edge 的 Docker 镜像列表里。
//
// 这里同时比对 repoTag、repoDigest 和 imageID，兼容后续平台把镜像以 tag、digest 或 image id 落进环境包。
func runRuntimeImageExists(images []EdgeAPI.DockerImage, runtimeImage string) bool {
	runtimeImage = strings.TrimSpace(runtimeImage)
	if runtimeImage == "" {
		return false
	}
	for _, item := range images {
		if strings.EqualFold(strings.TrimSpace(item.ID), runtimeImage) {
			return true
		}
		for _, repoTag := range item.RepoTags {
			if strings.TrimSpace(repoTag) == runtimeImage {
				return true
			}
		}
		for _, repoDigest := range item.RepoDigests {
			if strings.TrimSpace(repoDigest) == runtimeImage {
				return true
			}
		}
	}
	return false
}
