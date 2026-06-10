package Node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"private_browser_server/Middleware/PlatformContext"
	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
	DiscoveryService "private_browser_server/Service/Discovery"
	"private_browser_server/Settings"
)

type registerNodeRequest struct {
	Name         string `json:"name"`
	BaseURL      string `json:"baseUrl"`
	ClientIP     string `json:"clientIp"`
	DockerAPIURL string `json:"dockerApiUrl"`
}

type probeDockerRequest struct {
	DockerAPIURL string `json:"dockerApiUrl"`
}

type dockerProbeResult struct {
	Reachable        bool   `json:"reachable"`
	DockerAPIURL     string `json:"dockerApiUrl"`
	OS               string `json:"os"`
	Arch             string `json:"arch"`
	RawArch          string `json:"rawArch"`
	CPUCores         int    `json:"cpuCores"`
	MemoryTotalMB    int64  `json:"memoryTotalMb"`
	DockerVersion    string `json:"dockerVersion"`
	DockerAPIVersion string `json:"dockerApiVersion"`
	Containers       int    `json:"containers"`
	Images           int    `json:"images"`
	CheckedAt        int64  `json:"checkedAt"`
	Troubleshooting  string `json:"troubleshooting,omitempty"`
}

type dockerInfoResponse struct {
	OperatingSystem string `json:"OperatingSystem"`
	OSType          string `json:"OSType"`
	Architecture    string `json:"Architecture"`
	NCPU            int    `json:"NCPU"`
	MemTotal        int64  `json:"MemTotal"`
	ServerVersion   string `json:"ServerVersion"`
	Containers      int    `json:"Containers"`
	Images          int    `json:"Images"`
}

type dockerVersionResponse struct {
	Version    string `json:"Version"`
	APIVersion string `json:"ApiVersion"`
	OS         string `json:"Os"`
	Arch       string `json:"Arch"`
}

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
	node := &NodeModel.ControlNode{
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
	HttpResponse.ResponseSuccess(ctx, gin.H{"items": nodes, "total": len(nodes)})
}

// ListDiscoveredClients 返回 UDP discovery 当前收到的 Edge Client beacon。
//
// 这是自动发现的测试入口：只展示发现线索，不创建节点、不标记 verified。
// 管理员确认后仍应走注册/刷新流程，完成 Client HTTP 和 Docker 2375 探测。
func ListDiscoveredClients(ctx *gin.Context) {
	listener := DiscoveryService.Current()
	items := listener.List(ctx.Request.Context())
	HttpResponse.ResponseSuccess(ctx, gin.H{"items": items, "total": len(items)})
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
	HttpResponse.ResponseSuccess(ctx, gin.H{"node": node, "probe": probe})
}

// ReceiveHeartbeat 接收 Edge 心跳。
//
// 心跳只保存节点、Docker、环境包状态摘要，不接收 proxy 明文、fingerprint raw 或 browser-data。
func ReceiveHeartbeat(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "节点心跳接口已规划，下一阶段接入状态摘要落库；心跳不得携带 proxy 明文、fingerprint raw 或 browser-data")
}

func probeDocker(parent context.Context, dockerAPIURL string) (*dockerProbeResult, error) {
	baseURL, err := normalizeHTTPURL(dockerAPIURL)
	if err != nil {
		return nil, fmt.Errorf("Docker API 地址非法: %w；需要形如 http://192.168.10.119:2375，且 Docker 2375 只能暴露在可信内网", err)
	}
	timeout := time.Duration(Settings.Conf.EdgeConfig.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	client := &http.Client{Timeout: timeout}
	if err = dockerPing(ctx, client, baseURL); err != nil {
		return nil, err
	}
	info, err := dockerGetJSON[dockerInfoResponse](ctx, client, baseURL, "/info")
	if err != nil {
		return nil, err
	}
	version, err := dockerGetJSON[dockerVersionResponse](ctx, client, baseURL, "/version")
	if err != nil {
		return nil, err
	}
	rawArch := firstNonEmpty(info.Architecture, version.Arch)
	return &dockerProbeResult{
		Reachable:        true,
		DockerAPIURL:     baseURL,
		OS:               firstNonEmpty(info.OperatingSystem, info.OSType, version.OS),
		Arch:             normalizeArch(rawArch),
		RawArch:          rawArch,
		CPUCores:         info.NCPU,
		MemoryTotalMB:    info.MemTotal / 1024 / 1024,
		DockerVersion:    firstNonEmpty(info.ServerVersion, version.Version),
		DockerAPIVersion: version.APIVersion,
		Containers:       info.Containers,
		Images:           info.Images,
		CheckedAt:        time.Now().Unix(),
	}, nil
}

func dockerPing(ctx context.Context, client *http.Client, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/_ping", nil)
	if err != nil {
		return fmt.Errorf("创建 Docker _ping 请求失败: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Docker 2375 不可达: %w；影响范围：Node Server 无法确认节点 Docker 能力，不能安全下发浏览器容器动作；修复方式：确认 Docker daemon 已开启 tcp://0.0.0.0:2375 且仅限可信内网访问", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Docker _ping 返回 HTTP %d；请检查 Docker 2375、防火墙和 daemon 配置", resp.StatusCode)
	}
	return nil
}

func dockerGetJSON[T any](ctx context.Context, client *http.Client, baseURL string, path string) (T, error) {
	var target T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return target, fmt.Errorf("创建 Docker %s 请求失败: %w", path, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return target, fmt.Errorf("请求 Docker %s 失败: %w；请检查 Docker 2375 是否只在可信内网开放，且当前 Node Server 可以访问", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return target, fmt.Errorf("读取 Docker %s 响应失败: %w", path, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return target, fmt.Errorf("Docker %s 返回 HTTP %d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err = json.Unmarshal(body, &target); err != nil {
		return target, fmt.Errorf("解析 Docker %s JSON 失败: %w", path, err)
	}
	return target, nil
}

func normalizeHTTPURL(value string) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return "", fmt.Errorf("不能为空")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("必须包含 http/https scheme 和 host")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("只支持 http 或 https")
	}
	return value, nil
}

func normalizeArch(value string) string {
	arch := strings.ToLower(strings.TrimSpace(value))
	switch arch {
	case "amd64", "x86_64":
		return NodeModel.NodeArchAMD64
	case "arm64", "aarch64", "armv8":
		return NodeModel.NodeArchARM64
	default:
		return NodeModel.NodeArchUnknown
	}
}

func newClientID(mainAccountID string, sequence int) string {
	return strings.TrimSpace(mainAccountID) + fmt.Sprintf("%04d", sequence)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
