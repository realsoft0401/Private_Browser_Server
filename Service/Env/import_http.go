package Env

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path"
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
	NodeService "private_browser_server/Service/Node"
)

// ImportEnvPackage 在中心层创建“导入标准环境包”任务，并通过后台同步 Edge multipart 导入结果直接收口。
//
// 设计来源：
// - 当前 Edge import-package 是同步 multipart 接口，不返回 edgeTaskId，但导入包可能较大，Node Server 不能把整个导入动作都塞进 HTTP 请求线程；
// - 用户已经明确导入必须指定 clientId，不能自动选节点，也不能覆盖同名 envId；
// - 用户同时要求失败要留下中心 task、SSE 事件和管理员排障信息，因此这里采用“先创建中心 task，再后台同步导入并直接收口”的模式。
//
// 职责边界：
// - HTTP 入口负责节点准入、接收上传、预解析 envId/userId/rpaType、创建中心 task 并快速返回；
// - 后台任务只通过 Edge import-package API 导入标准包，不直接写 Edge 目录或 SQLite；
// - 导入成功后只写入中心聚合索引为 created，不自动 run、不自动拉镜像、不自动重试。
func ImportEnvPackage(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}

	clientID := strings.TrimSpace(ctx.PostForm("clientId"))
	if clientID == "" {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "clientId 不能为空")
		return
	}
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "file 不能为空")
		return
	}

	client, err := NodeService.EnsureClientReadyForBusiness(ctx.Request.Context(), platformCtx.MainAccountID, clientID)
	if err != nil {
		respondClientNotReady(ctx, err)
		return
	}

	artifact, err := stageImportPackageUpload(fileHeader)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	if err = validateImportPackageIdentity(platformCtx.MainAccountID, artifact.Identity); err != nil {
		_ = os.Remove(artifact.FilePath)
		writeServiceError(ctx, err)
		return
	}
	if err = validateImportConflict(ctx.Request.Context(), platformCtx.MainAccountID, client.ID, artifact.Identity.EnvID); err != nil {
		_ = os.Remove(artifact.FilePath)
		writeServiceError(ctx, err)
		return
	}

	task := newServerTask(platformCtx, client.ID, artifact.Identity.EnvID, taskModel.TaskTypeImportEnvPackage, buildServerTaskEventsURL(ctx, ""))
	task.EventsURL = buildServerTaskEventsURL(ctx, task.TaskID)
	if err = taskDao.NewModelHandler().CreateServerTask(ctx.Request.Context(), task); err != nil {
		_ = os.Remove(artifact.FilePath)
		writeServiceError(ctx, internalError("创建中心 import-package 任务失败: "+err.Error()))
		return
	}

	TaskStream.Ensure(task.TaskID)
	TaskStream.Emit(task.TaskID, "queued", taskModel.TaskStatusPending, "queued", "环境包导入任务已创建，等待 Node Server 后台执行", map[string]any{
		"clientId":    client.ID,
		"envId":       artifact.Identity.EnvID,
		"fileName":    artifact.baseName(),
		"packageSize": artifact.OriginalSize,
	})

	taskCopy := *task
	artifactCopy := *artifact
	go importEnvPackageTaskAsync(platformCtx, client.BaseURL, &taskCopy, &artifactCopy)

	HttpResponse.ResponseSuccess(ctx, buildStartTaskResponse(task, "环境包导入任务已创建"))
}

// importEnvPackageTaskAsync 在后台串行执行 import-package 的真正导入与中心收口。
//
// 设计来源：
// - HTTP 入口需要尽快返回 taskId，但 Edge import-package 仍然是同步 multipart 动作，因此必须由本地 goroutine 承接后半段；
// - 这里统一负责 running 过渡、Edge 调用、成功收口、清理暂存包与失败事件发射，避免状态分散在多个入口里；
// - 该流程不自动重试，任何阶段失败都直接写 failed，等待管理员修复后重新发起新任务。
func importEnvPackageTaskAsync(platformCtx PlatformContext.Context, clientBaseURL string, task *taskModel.ServerTask, artifact *importPackageUploadArtifact) {
	if task == nil || artifact == nil {
		return
	}
	reporter := func(event string, status string, stage string, message string, data map[string]any) {
		TaskStream.Emit(strings.TrimSpace(task.TaskID), event, status, stage, message, data)
	}
	cleanupHandled := false
	defer func() {
		if recovered := recover(); recovered != nil {
			message := fmt.Sprintf("Node Server import-package 后台任务异常崩溃: %v", recovered)
			reporter("error", taskModel.TaskStatusFailed, "server_import", message, nil)
			_ = persistFailedImportTask(context.Background(), task, message)
		}
		if cleanupHandled || artifact.FilePath == "" {
			return
		}
		if err := os.Remove(artifact.FilePath); err != nil && !os.IsNotExist(err) {
			log.Printf("import-package temp cleanup failed, taskId=%s, clientId=%s, envId=%s, path=%s, err=%v\n",
				strings.TrimSpace(task.TaskID),
				strings.TrimSpace(task.ClientID),
				strings.TrimSpace(task.EnvID),
				artifact.FilePath,
				err,
			)
		}
	}()

	backgroundCtx := context.Background()
	task.Status = taskModel.TaskStatusRunning
	task.UpdatedAt = time.Now().Unix()
	if err := taskDao.NewModelHandler().UpdateServerTask(backgroundCtx, task); err != nil {
		message := "更新中心 import-package 任务为 running 失败: " + err.Error()
		reporter("error", taskModel.TaskStatusFailed, "server_import", message, nil)
		_ = persistFailedImportTask(backgroundCtx, task, message)
		return
	}
	reporter("running", taskModel.TaskStatusRunning, "server_import", "Node Server 后台 import-package 工作流开始执行", map[string]any{
		"clientId": task.ClientID,
		"envId":    task.EnvID,
		"fileName": artifact.baseName(),
	})
	reporter("progress", taskModel.TaskStatusRunning, "edge_import", "标准包暂存完成，开始调用 Edge import-package", map[string]any{
		"envId":       task.EnvID,
		"fileName":    artifact.baseName(),
		"packageSize": artifact.OriginalSize,
	})

	edgeResp, err := EdgeAPI.New().ImportBrowserEnvPackage(backgroundCtx, clientBaseURL, "", artifact.FilePath, artifact.baseName())
	if err != nil {
		message := "调用 Edge 导入环境包失败: " + err.Error()
		reporter("error", taskModel.TaskStatusFailed, "edge_import", message, nil)
		_ = persistFailedImportTask(backgroundCtx, task, message)
		return
	}

	env, err := finalizeImportPackageSuccess(backgroundCtx, platformCtx, clientBaseURL, task, artifact, edgeResp)
	if err != nil {
		message := "中心 import-package 成功收口失败: " + err.Error()
		reporter("error", taskModel.TaskStatusFailed, "finalize", message, map[string]any{
			"envId": task.EnvID,
		})
		_ = persistFailedImportTask(backgroundCtx, task, message)
		if env != nil {
			_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(backgroundCtx, env.MainAccountID, env.EnvID, task.TaskID, message, time.Now().Unix())
		}
		return
	}
	if err = cleanupImportPackageUpload(backgroundCtx, task, env, artifact); err != nil {
		reporter("error", taskModel.TaskStatusFailed, "cleanup", err.Error(), map[string]any{
			"clientId": task.ClientID,
			"envId":    task.EnvID,
			"fileName": artifact.baseName(),
		})
		return
	}
	cleanupHandled = true

	reporter("done", taskModel.TaskStatusSuccess, "finalize", "环境包导入完成", map[string]any{
		"envId":       env.EnvID,
		"status":      env.Status,
		"clientId":    env.ClientID,
		"packageSize": artifact.OriginalSize,
	})
}

// persistFailedImportTask 把 import-package 本地任务统一收口为 failed，并留下服务端错误日志。
//
// 设计来源：
// - import-package 当前没有 edgeTaskId，失败事实主要依赖中心 task/SSE/服务日志三处互相印证；
// - 这里单独收敛 failed 更新，避免不同失败阶段各自改 task 字段时出现漏写 finishedAt 或 error_message；
// - 日志里只保留 task/client/env 和错误摘要，不记录代理明文、指纹原文或浏览器数据内容。
func persistFailedImportTask(ctx context.Context, task *taskModel.ServerTask, message string) error {
	if task == nil {
		return nil
	}
	task.Status = taskModel.TaskStatusFailed
	task.ErrorMessage = strings.TrimSpace(message)
	task.UpdatedAt = time.Now().Unix()
	task.FinishedAt = task.UpdatedAt
	log.Printf("import-package task failed, taskId=%s, clientId=%s, envId=%s, err=%s\n",
		strings.TrimSpace(task.TaskID),
		strings.TrimSpace(task.ClientID),
		strings.TrimSpace(task.EnvID),
		task.ErrorMessage,
	)
	return taskDao.NewModelHandler().UpdateServerTask(ctx, task)
}

// finalizeImportPackageSuccess 把 Edge 已确认导入成功的事实写回中心环境聚合与中心 task。
//
// 设计来源：
// - 当前 import-package 的成功判定不是“文件上传完”或“Edge 返回过 HTTP 200”，而是 Edge data 明确返回 `status=created`；
// - 导入成功后中心必须立即拥有可查询的 env 聚合记录，供后续 run/list/detail/task 审计继续使用；
// - 这里负责把 Edge 同步返回折叠为中心 created 摘要，但不负责声明浏览器已经 running。
func finalizeImportPackageSuccess(ctx context.Context, platformCtx PlatformContext.Context, clientBaseURL string, task *taskModel.ServerTask, artifact *importPackageUploadArtifact, edgeResp *EdgeAPI.ImportBrowserEnvPackageResponse) (*envModel.ServerBrowserEnv, error) {
	if task == nil || artifact == nil {
		return nil, internalError("import-package 收口失败：中心任务或上传暂存对象为空")
	}
	if edgeResp == nil {
		return nil, internalError("Edge import-package 成功响应为空，中心无法确认导入事实")
	}
	if strings.TrimSpace(edgeResp.Status) != envModel.EnvStatusCreated {
		return nil, conflictError("Edge import-package 未返回 created 状态，中心拒绝把任务记为成功")
	}
	if strings.TrimSpace(edgeResp.EnvID) == "" {
		return nil, internalError("Edge import-package 成功响应缺少 envId，中心无法写入环境聚合记录")
	}

	now := firstPositive(edgeResp.ImportedAt, time.Now().Unix())
	cdpURL, webVNCURL := buildPreviewURLs(clientBaseURL, edgeResp.EnvID, envModel.BrowserEnvPorts{
		CDP: edgeResp.Ports.CDP,
		VNC: edgeResp.Ports.VNC,
	})
	env := &envModel.ServerBrowserEnv{
		EnvID:             strings.TrimSpace(edgeResp.EnvID),
		MainAccountID:     platformCtx.MainAccountID,
		ClientID:          task.ClientID,
		RPAType:           firstNonEmpty(strings.TrimSpace(edgeResp.RPAType), artifact.Identity.RPAType),
		Name:              firstNonEmpty(artifact.Identity.Name, edgeResp.EnvID),
		Status:            envModel.EnvStatusCreated,
		ContainerStatus:   envModel.EnvFactUnknown,
		MonitorStatus:     envModel.EnvFactUnknown,
		CDPURL:            cdpURL,
		WebVNCURL:         webVNCURL,
		LastTaskID:        task.TaskID,
		LastError:         "",
		CreatedByUserID:   platformCtx.UserID,
		CreatedByUsername: platformCtx.Username,
		CreatedAt:         now,
		UpdatedAt:         now,
		DeletedAt:         0,
	}
	if err := envDao.NewModelHandler().CreateOrReviveDeletedServerBrowserEnv(ctx, env); err != nil {
		return nil, mapDaoError(err)
	}

	task.Status = taskModel.TaskStatusSuccess
	task.ErrorMessage = ""
	task.UpdatedAt = now
	task.FinishedAt = now
	if err := taskDao.NewModelHandler().UpdateServerTask(ctx, task); err != nil {
		return env, err
	}
	return env, nil
}

// cleanupImportPackageUpload 清理 Node Server 本地暂存包，并在失败时回滚中心任务成功结论。
//
// 设计来源：
// - 用户已经明确正式资产接口失败不能“假装成功”，因此暂存文件清理失败也必须对管理员可见；
// - 这里不回滚 Edge 已导入成功的事实，只把中心 task 改成 failed，并把 env 摘要写入 last_error，提醒管理员清理残留；
// - 该函数只处理 Node Server 本地临时文件，不接触 Edge 环境目录、SQLite 或 browser-data 资产内容。
func cleanupImportPackageUpload(ctx context.Context, task *taskModel.ServerTask, env *envModel.ServerBrowserEnv, artifact *importPackageUploadArtifact) error {
	if artifact == nil || strings.TrimSpace(artifact.FilePath) == "" {
		return nil
	}
	if err := os.Remove(artifact.FilePath); err != nil && !os.IsNotExist(err) {
		message := "环境包导入完成，但 Node Server 清理本地暂存包失败；请管理员检查磁盘权限或残留临时文件后再继续排障: " + err.Error()
		if task != nil {
			_ = persistFailedImportTask(ctx, task, message)
		}
		if env != nil {
			_ = envDao.NewModelHandler().UpdateServerBrowserEnvTaskSummary(ctx, env.MainAccountID, env.EnvID, task.TaskID, message, time.Now().Unix())
		}
		log.Printf("import-package temp cleanup failed after edge success, taskId=%s, clientId=%s, envId=%s, path=%s, err=%v\n",
			strings.TrimSpace(task.TaskID),
			strings.TrimSpace(task.ClientID),
			strings.TrimSpace(task.EnvID),
			artifact.FilePath,
			err,
		)
		return internalError(message)
	}
	return nil
}

// stageImportPackageUpload 负责把上传文件落到 Node Server 本地暂存区，并做第一层大小与格式预检。
//
// 设计来源：
// - import-package 是 multipart 上传接口，Node Server 需要先把流式内容暂存下来，后续才能复用同一份文件做身份预解析与 Edge 转发；
// - 这里先锁住文件非空、大小上限和最小 tar/gzip 身份校验，避免把明显错误的大包直接转发给 Edge；
// - 这里只处理 Node Server 本地 staging，不生成中心 task，也不判断目标 client 是否允许导入。
func stageImportPackageUpload(fileHeader *multipart.FileHeader) (*importPackageUploadArtifact, error) {
	if fileHeader == nil {
		return nil, invalidError("导入文件不能为空")
	}
	if strings.TrimSpace(fileHeader.Filename) == "" {
		return nil, invalidError("导入文件名不能为空")
	}
	if fileHeader.Size <= 0 {
		return nil, invalidError("导入文件为空")
	}
	if fileHeader.Size > importPackageMaxUploadBytes {
		return nil, invalidError("导入文件超过大小限制")
	}

	src, err := fileHeader.Open()
	if err != nil {
		return nil, internalError("打开上传文件失败: " + err.Error())
	}
	defer src.Close()

	tmpFile, err := os.CreateTemp("", "private-browser-server-import-*.tar.gz")
	if err != nil {
		return nil, internalError("创建导入暂存文件失败: " + err.Error())
	}
	defer tmpFile.Close()

	written, err := io.Copy(tmpFile, io.LimitReader(src, importPackageMaxUploadBytes+1))
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, internalError("写入导入暂存文件失败: " + err.Error())
	}
	if written > importPackageMaxUploadBytes {
		_ = os.Remove(tmpFile.Name())
		return nil, invalidError("导入文件超过大小限制")
	}
	identity, err := inspectImportPackageIdentity(tmpFile.Name())
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, err
	}
	return &importPackageUploadArtifact{
		FilePath:     tmpFile.Name(),
		FileName:     strings.TrimSpace(fileHeader.Filename),
		OriginalSize: written,
		Identity:     *identity,
	}, nil
}

// inspectImportPackageIdentity 只读取标准包里的 `profile.json`，提取导入前身份摘要。
//
// 设计来源：
// - 用户要求 import-package 必须保留原 `envId/userId/rpaType`，因此 Node Server 需要在创建中心 task 前先知道它是谁；
// - 这里故意只做最小读取，不提前展开整个 tar，也不解析 browser-data，避免把导入入口做成新的资产读取通道；
// - 真正完整的原子材料与 checksum 校验仍由 Edge import-package 继续负责，中心层只做最小准入拦截。
func inspectImportPackageIdentity(filePath string) (*importPackageProfileIdentity, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, internalError("打开导入暂存文件失败: " + err.Error())
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, invalidError("导入文件不是有效的 gzip 包")
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	rootDir := ""
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, invalidError("读取 tar 包结构失败: " + nextErr.Error())
		}
		cleaned, err := normalizeImportTarPath(header.Name)
		if err != nil {
			return nil, err
		}
		segments := strings.Split(cleaned, "/")
		if len(segments) == 0 || strings.TrimSpace(segments[0]) == "" {
			return nil, invalidError("导入包缺少合法 envId 根目录")
		}
		if rootDir == "" {
			rootDir = strings.TrimSpace(segments[0])
		} else if rootDir != strings.TrimSpace(segments[0]) {
			return nil, invalidError("导入包包含多个环境根目录，Node Server 不允许继续转发到 Edge")
		}
		if len(segments) == 2 && segments[1] == "profile.json" && header.Typeflag == tar.TypeReg {
			var profile importPackageProfileIdentity
			if err = json.NewDecoder(io.LimitReader(tarReader, 2<<20)).Decode(&profile); err != nil {
				return nil, invalidError("导入包 profile.json 不可解析: " + err.Error())
			}
			if strings.TrimSpace(profile.EnvID) == "" || strings.TrimSpace(profile.UserID) == "" || strings.TrimSpace(profile.RPAType) == "" {
				return nil, invalidError("导入包 profile.json 缺少 envId/userId/rpaType，不能导入")
			}
			if strings.TrimSpace(profile.EnvID) != rootDir {
				return nil, invalidError("导入包根目录名与 profile.envId 不一致，不能导入")
			}
			return &profile, nil
		}
	}
	return nil, invalidError("导入包缺少 profile.json，不能导入")
}

// normalizeImportTarPath 归一化 tar 条目路径，阻止空路径、绝对路径和目录穿越。
//
// 这个函数单独存在，是为了把 tar 包结构安全校验集中到一个入口；
// 后续如果标准包协议继续收紧，例如禁止额外顶层目录或隐藏文件，也应继续在这里演进，而不是散落在遍历循环里。
func normalizeImportTarPath(value string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	normalized = strings.TrimPrefix(normalized, "./")
	if normalized == "" {
		return "", invalidError("导入包包含空路径条目，不能导入")
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return "", invalidError("导入包路径非法，不能导入")
	}
	return cleaned, nil
}

// validateImportPackageIdentity 锁住导入包与当前主账号的归属关系，禁止跨主账号导入。
//
// 设计来源：
// - 当前平台口径里 `mainAccountId` 是中心资产分区边界，Node Server 不能把一个主账号的包静默导到另一个主账号下；
// - 用户还要求 `envId` 第一段必须与 `userId/mainAccountId` 一致，避免中心索引和 Edge 目录身份漂移；
// - 这里只判断归属关系，不判断目标节点运行能力；节点准入仍由 EnsureClientReadyForBusiness 负责。
func validateImportPackageIdentity(mainAccountID string, identity importPackageProfileIdentity) error {
	if strings.TrimSpace(identity.EnvID) == "" || strings.TrimSpace(identity.UserID) == "" {
		return invalidError("导入包身份字段缺失，不能导入")
	}
	if strings.TrimSpace(identity.UserID) != strings.TrimSpace(mainAccountID) {
		return conflictError("导入包 userId 不属于当前 mainAccountId，Node Server 不允许跨主账号导入")
	}
	envPrefix := strings.TrimSpace(identity.EnvID)
	if cut := strings.Index(envPrefix, "_"); cut > 0 {
		envPrefix = envPrefix[:cut]
	}
	if envPrefix != strings.TrimSpace(mainAccountID) {
		return conflictError("导入包 envId 第一段与当前 mainAccountId 不一致，Node Server 不允许导入到当前主账号")
	}
	return nil
}

// validateImportConflict 提前拦截 import-package 与中心 tombstone 的冲突关系。
//
// 设计来源：
// - 实测发现 `/package` 删除后的中心 status=deleted 记录会把同 envId 的重新导入挡住，导致 Edge 已支持的恢复链路无法走通；
// - 用户当前商业口径又明确“不做自动跨节点迁移”，所以 deleted 记录只能在同一 clientId 上被受控复活；
// - 这个前置校验要在创建 task 前完成，避免用户拿到一个最终必然失败的 taskId。
//
// 职责边界：
// - 只判断中心层是否允许继续导入，不校验 tar 包结构，也不访问 Edge 文件系统；
// - 活跃记录一律拒绝覆盖；只有 status=deleted 且 clientId 与本次请求一致时才允许继续；
// - 如果中心查库本身失败，这里只返回统一业务错误，不偷偷放行。
func validateImportConflict(ctx context.Context, mainAccountID string, clientID string, envID string) error {
	existingEnv, err := envDao.NewModelHandler().GetServerBrowserEnvByID(ctx, mainAccountID, envID)
	if err == nil {
		if strings.TrimSpace(existingEnv.Status) != envModel.EnvStatusDeleted {
			return conflictError("中心已存在相同 envId 的活跃环境记录，禁止重复导入；如确需替换，请先由管理员明确处理原环境")
		}
		if strings.TrimSpace(existingEnv.ClientID) != strings.TrimSpace(clientID) {
			return conflictError("中心已存在相同 envId 的 deleted 历史，但它绑定的是另一台 client；当前版本不允许借 import-package 自动跨节点迁移")
		}
		return nil
	}
	if errors.Is(err, envDao.ErrEnvNotFound) {
		return nil
	}
	return mapDaoError(err)
}
