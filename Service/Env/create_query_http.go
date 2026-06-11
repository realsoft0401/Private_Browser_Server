package Env

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	envDao "private_browser_server/Dao/Env"
	EdgeAPI "private_browser_server/EdgeClient"
	"private_browser_server/Middleware/PlatformContext"
	envModel "private_browser_server/Models/Env"
	"private_browser_server/Pkg/HttpResponse"
	ImagePolicyService "private_browser_server/Service/ImagePolicy"
	NodeService "private_browser_server/Service/Node"
)

// CreateEnv 通过 Node Server 代理 Edge 创建环境包，并写入中心聚合索引。
//
// 设计来源：
//   - 当前节点接入、verify 和 EdgeClient 已经落地，下一步必须让 Node Server 真正进入“业务代理”阶段；
//   - 用户确认环境包永远归属主账号，因此这里固定把 Platform Header 里的 mainAccountId 作为 Edge userId；
//   - imagePolicy 当前由 Platform 受控下发；过渡期该值先直接等于已登记镜像字符串，但真正的 runtime.image
//     仍由 Node Server 根据 verified 节点 arch 做兼容校验后再下发，避免普通调用方绕过中心层随意指定镜像。
//
// 职责边界：
// - 负责参数校验、Client ready 校验、imagePolicy 解析、调用 Edge create、写入 server_browser_envs；
// - 不直接访问 Edge SQLite，不扫描 Edge 环境包目录，不代替后续 run/stop/task 编排；
// - Edge 创建成功但中心写库失败时，当前版本只明确报错，不自动回滚删除 Edge 环境包，避免在失败分支里隐式执行资产动作。
func CreateEnv(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	param := new(envModel.CreateEnvRequest)
	if !bindStrictJSON(ctx, param) {
		return
	}
	if err := validateCreateEnvRequest(param); err != nil {
		writeServiceError(ctx, err)
		return
	}

	client, err := NodeService.EnsureClientReadyForBusiness(ctx.Request.Context(), platformCtx.MainAccountID, param.ClientID)
	if err != nil {
		respondClientNotReady(ctx, err)
		return
	}
	image, err := ImagePolicyService.ResolveRuntimeImage(param.Runtime.ImagePolicy, client.Arch)
	if err != nil {
		writeServiceError(ctx, conflictError("镜像策略解析失败: "+err.Error()))
		return
	}

	edgeReq := buildEdgeCreateRequest(platformCtx.MainAccountID, param, image)
	var edgeResp edgeCreateBrowserEnvResponse
	if err = EdgeAPI.New().DoJSON(
		ctx.Request.Context(),
		client.BaseURL,
		http.MethodPost,
		"/api/v1/edge/browser-envs",
		"",
		edgeReq,
		&edgeResp,
	); err != nil {
		writeServiceError(ctx, mapEdgeError(err))
		return
	}
	if strings.TrimSpace(edgeResp.EnvID) == "" {
		writeServiceError(ctx, internalError("Edge 创建环境包成功响应缺少 envId，中心层不能写入聚合索引"))
		return
	}

	now := edgeResp.CreatedAt
	if now <= 0 {
		now = time.Now().Unix()
	}
	cdpURL, webVNCURL := buildPreviewURLs(client.BaseURL, edgeResp.EnvID, edgeResp.Ports)
	env := &envModel.ServerBrowserEnv{
		EnvID:             edgeResp.EnvID,
		MainAccountID:     platformCtx.MainAccountID,
		ClientID:          client.ID,
		RPAType:           edgeResp.RPAType,
		Name:              strings.TrimSpace(param.Name),
		Status:            envModel.EnvStatusCreated,
		ContainerStatus:   envModel.EnvFactUnknown,
		MonitorStatus:     envModel.EnvFactUnknown,
		CDPURL:            cdpURL,
		WebVNCURL:         webVNCURL,
		LastTaskID:        "",
		LastError:         "",
		CreatedByUserID:   platformCtx.UserID,
		CreatedByUsername: platformCtx.Username,
		CreatedAt:         now,
		UpdatedAt:         now,
		DeletedAt:         0,
	}
	if err = envDao.NewModelHandler().CreateServerBrowserEnv(ctx.Request.Context(), env); err != nil {
		writeServiceError(ctx, mapDaoError(err))
		return
	}

	HttpResponse.ResponseSuccess(ctx, &envModel.CreateEnvResponse{
		EnvID:         edgeResp.EnvID,
		MainAccountID: platformCtx.MainAccountID,
		ClientID:      client.ID,
		Status:        envModel.EnvStatusCreated,
		Ports:         edgeResp.Ports,
		IdentityHash:  edgeResp.IdentityHash,
		CDPURL:        cdpURL,
		WebVNCURL:     webVNCURL,
		CreatedAt:     now,
		Env:           env,
	})
}

// ListEnvs 返回当前主账号下的中心环境包缓存列表。
//
// 当前列表主事实源是 Node Server SQLite，不直接到 Edge 拉全量列表；
// 这样可以保证 Platform Header 归属过滤、分页和后续任务关联都围绕中心索引完成。
func ListEnvs(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	query := normalizeListEnvQuery(ctx)
	items, total, err := envDao.NewModelHandler().ListServerBrowserEnvs(ctx.Request.Context(), platformCtx.MainAccountID, query)
	if err != nil {
		writeServiceError(ctx, mapDaoError(err))
		return
	}
	HttpResponse.ResponseSuccess(ctx, envModel.ListEnvsResponse{
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
		Items:    items,
	})
}

// GetEnvDetail 返回单个中心环境包缓存详情。
//
// 这里先返回 SQLite 聚合摘要；后续 run/stop/task 落地后，再补 Edge 状态刷新或任务摘要拼装。
func GetEnvDetail(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	env, err := envDao.NewModelHandler().GetServerBrowserEnvByID(
		ctx.Request.Context(),
		platformCtx.MainAccountID,
		strings.TrimSpace(ctx.Param("envId")),
	)
	if err != nil {
		writeServiceError(ctx, mapDaoError(err))
		return
	}
	HttpResponse.ResponseSuccess(ctx, env)
}

// buildEdgeCreateRequest 负责把中心创建参数折叠成 Edge 创建协议。
//
// 这个转换单独抽出，是为了把 Platform 层字段约束和 Edge 协议字段稳定分开；
// 后续如果 Edge 创建协议演进，只改这里，不要把字段拼装散落到 HTTP 入口里。
func buildEdgeCreateRequest(mainAccountID string, param *envModel.CreateEnvRequest, image string) *edgeCreateBrowserEnvRequest {
	return &edgeCreateBrowserEnvRequest{
		UserID:      mainAccountID,
		RPAType:     strings.TrimSpace(param.RPAType),
		Name:        strings.TrimSpace(param.Name),
		Runtime:     edgeCreateBrowserEnvRuntime{Image: image, StartupURL: strings.TrimSpace(param.Runtime.StartupURL), ShmSize: strings.TrimSpace(param.Runtime.ShmSize)},
		Environment: param.Environment,
		Proxy:       param.Proxy,
		Fingerprint: param.Fingerprint,
		Metadata:    param.Metadata,
	}
}
