package Node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	EdgeAPI "private_browser_server/EdgeClient"
	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Settings"
)

// verifyClientHealth 读取 Client /health 原始健康响应。
//
// /health 不是 Edge 统一 code/message/data 响应，所以这里直接使用 http.Client；
// 只读取 ok/status 两个业务必要字段，避免 Server 依赖 Client health 的完整内部结构。
func verifyClientHealth(parent context.Context, baseURL string) (*edgeHealthResponse, error) {
	endpoint, err := buildClientURL(baseURL, "/health")
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(Settings.Conf.EdgeConfig.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Client /health 请求失败: %w", err)
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Client /health 失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 Client /health 响应失败: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Client /health 返回 HTTP %d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var result edgeHealthResponse
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 Client /health JSON 失败: %w", err)
	}
	return &result, nil
}

// verifyClientDeviceInfo 读取 Client 设备事实。
//
// device-info 使用 Edge 统一响应，调用方只取 arch/dockerVersion 等校验必要字段；
// 真实 Docker 能力仍以 Docker 2375 probe 为另一条独立事实来源，两边必须一致。
func verifyClientDeviceInfo(ctx context.Context, baseURL string) (*edgeDeviceInfoResponse, error) {
	var result edgeDeviceInfoResponse
	err := EdgeAPI.New().DoJSON(ctx, baseURL, http.MethodGet, "/api/v1/edge/device-info", "", nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func buildClientURL(baseURL string, path string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("Client baseUrl 不能为空")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("Client baseUrl 非法: %s", baseURL)
	}
	return baseURL + path, nil
}

// normalizeLookupURL 归一化 discovery/baseUrl 的展示匹配键。
//
// 这里故意复用 URL 解析语义，但失败时不返回错误给用户，因为发现列表是只读展示；
// 非法 baseUrl 只会导致无法匹配 clientId，后续注册或探测接口仍会给出明确错误。
func normalizeLookupURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	normalized, err := normalizeHTTPURL(value)
	if err != nil {
		return value
	}
	return normalized
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
