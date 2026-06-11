package Node

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"private_browser_server/Middleware/PlatformContext"
	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
	DiscoveryService "private_browser_server/Service/Discovery"
)

// ProbeDocker 通过 Docker Engine HTTP API 探测节点能力。
//
// 该接口只读取 Docker 2375 的 _ping/info/version，不写数据库；
// 注册和设备刷新会复用相同探测逻辑，避免前端或管理员看到的架构判断不一致。
func ProbeDocker(ctx *gin.Context) {
	var req probeDockerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "请求体格式错误，需要 JSON: {\"dockerApiUrl\":\"http://节点IP:2375\"}")
		return
	}
	result, err := probeDocker(ctx.Request.Context(), req.DockerAPIURL)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, result)
}

// RegisterNode 注册 Edge Client。
//
// V1 demo 下归属来自 Platform Header；Client 不生成 clientId，也不要求 Edge API 携带 clientId。
// clientId 由 Node Server 按主账号和序号生成，注册时只保存人工输入的 baseUrl/dockerApiUrl，
// 设备事实必须通过刷新探测确认后再使用。
func RegisterNode(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	var req registerNodeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "请求体格式错误，需要 JSON: {\"name\":\"节点名\",\"baseUrl\":\"http://ClientIP:3300\",\"dockerApiUrl\":\"http://ClientIP:2375\"}")
		return
	}
	baseURL, err := normalizeHTTPURL(req.BaseURL)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "baseUrl 非法: "+err.Error()+"；示例 http://192.168.10.119:3300")
		return
	}
	dockerURL := strings.TrimSpace(req.DockerAPIURL)
	if dockerURL != "" {
		dockerURL, err = normalizeHTTPURL(dockerURL)
		if err != nil {
			HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "dockerApiUrl 非法: "+err.Error()+"；示例 http://192.168.10.119:2375")
			return
		}
	}

	repo := NodeRepo.Repository{}
	sequence, err := repo.NextSequence(ctx.Request.Context(), platformCtx.MainAccountID)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "生成节点序号失败: "+err.Error())
		return
	}
	now := time.Now().Unix()
	node := &NodeModel.EdgeClient{
		ID:                newClientID(platformCtx.MainAccountID, sequence),
		MainAccountID:     platformCtx.MainAccountID,
		NodeSequence:      sequence,
		Name:              strings.TrimSpace(req.Name),
		BaseURL:           baseURL,
		ClientIP:          strings.TrimSpace(req.ClientIP),
		DockerAPIURL:      dockerURL,
		Arch:              NodeModel.NodeArchUnknown,
		HealthStatus:      NodeModel.NodeHealthStale,
		DiscoveryStatus:   NodeModel.NodeDiscoveryManual,
		CreatedByUserID:   platformCtx.UserID,
		CreatedByUsername: platformCtx.Username,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if node.Name == "" {
		node.Name = "node-" + strconv.Itoa(sequence)
	}
	if err = repo.Create(ctx.Request.Context(), node); err != nil {
		if strings.Contains(err.Error(), "constraint failed") || strings.Contains(err.Error(), "UNIQUE") {
			HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeConflict, "Edge Client baseUrl 已存在，不能重复注册；如 Client IP 变化，需要走后续 IP 更新确认流程，不能自动覆盖原 clientId 身份。")
			return
		}
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "注册节点失败: "+err.Error())
		return
	}
	attachHeartbeatStatus(node, time.Now().Unix())
	HttpResponse.ResponseSuccess(ctx, node)
}

// ListNodes 返回当前主账号可见 Edge Client 列表。
func ListNodes(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	nodes, err := (NodeRepo.Repository{}).ListByMainAccount(ctx.Request.Context(), platformCtx.MainAccountID)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "查询节点列表失败: "+err.Error())
		return
	}
	attachHeartbeatStatusList(nodes, time.Now().Unix())
	HttpResponse.ResponseSuccess(ctx, gin.H{"items": nodes, "total": len(nodes)})
}

// ListDiscoveredClients 返回 UDP discovery 当前收到的 Edge Client beacon。
//
// 这是自动发现的测试入口：只展示发现线索，不创建节点、不标记 verified。
// 管理员确认后仍应走注册/刷新流程，完成 Client HTTP 和 Docker 2375 探测。
func ListDiscoveredClients(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	listener := DiscoveryService.Current()
	items := listener.List(ctx.Request.Context())
	nodes, err := (NodeRepo.Repository{}).ListByMainAccount(ctx.Request.Context(), platformCtx.MainAccountID)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "查询已注册 Edge Client 失败: "+err.Error())
		return
	}
	views := attachClientIDToDiscovered(items, nodes)
	if err = syncDiscoveredHeartbeats(ctx.Request.Context(), platformCtx.MainAccountID, views); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "回写 Client UDP 心跳失败: "+err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, gin.H{"items": views, "total": len(views)})
}

// GetNodeDetail 返回 Edge Client 详情。
func GetNodeDetail(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	node, err := (NodeRepo.Repository{}).GetByID(ctx.Request.Context(), platformCtx.MainAccountID, strings.TrimSpace(ctx.Param("clientId")))
	if errors.Is(err, NodeRepo.ErrNotFound) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "Edge Client 不存在或不属于当前主账号")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "查询 Edge Client 详情失败: "+err.Error())
		return
	}
	attachHeartbeatStatus(node, time.Now().Unix())
	HttpResponse.ResponseSuccess(ctx, node)
}

// RefreshNodeDeviceInfo 重新探测并保存 Edge Client 设备能力。
//
// 当前只确认 Docker 2375 能力，后续还要叠加 Client /health 和 /api/v1/edge/device-info 才能进入 verified。
// 因此这里不自动修改 baseUrl/clientIp，也不把 clientId 身份校验规则藏在刷新动作里。
func RefreshNodeDeviceInfo(ctx *gin.Context) {
	platformCtx, ok := PlatformContext.FromGin(ctx)
	if !ok {
		HttpResponse.ResponseError(ctx, HttpResponse.CodeUnauthorized)
		return
	}
	repo := NodeRepo.Repository{}
	node, err := repo.GetByID(ctx.Request.Context(), platformCtx.MainAccountID, strings.TrimSpace(ctx.Param("clientId")))
	if errors.Is(err, NodeRepo.ErrNotFound) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "Edge Client 不存在或不属于当前主账号")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "读取节点失败: "+err.Error())
		return
	}
	if strings.TrimSpace(node.DockerAPIURL) == "" {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "Edge Client 未配置 dockerApiUrl，无法探测 Docker 2375；请在注册时填写 http://ClientIP:2375，或后续通过受控接口更新。")
		return
	}
	probe, err := probeDocker(ctx.Request.Context(), node.DockerAPIURL)
	if err != nil {
		node.HealthStatus = NodeModel.NodeHealthUnhealthy
		node.LastCheckedAt = time.Now().Unix()
		node.LastError = err.Error()
		node.UpdatedAt = node.LastCheckedAt
		_ = repo.UpdateDeviceInfo(ctx.Request.Context(), node)
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, err.Error())
		return
	}
	node.OS = probe.OS
	node.Arch = probe.Arch
	node.CPUCores = probe.CPUCores
	node.MemoryTotalMB = probe.MemoryTotalMB
	node.DockerVersion = probe.DockerVersion
	node.HealthStatus = NodeModel.NodeHealthHealthy
	node.DiscoveryStatus = NodeModel.NodeDiscoveryManual
	node.LastCheckedAt = probe.CheckedAt
	node.LastError = ""
	node.UpdatedAt = probe.CheckedAt
	if err = repo.UpdateDeviceInfo(ctx.Request.Context(), node); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "保存节点设备信息失败: "+err.Error())
		return
	}
	attachHeartbeatStatus(node, time.Now().Unix())
	HttpResponse.ResponseSuccess(ctx, gin.H{"node": node, "probe": probe})
}
