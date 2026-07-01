package EdgeClient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"private_browser_server/Settings"
)

type Client struct {
	httpClient *http.Client
}

type Response[T any] struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type HealthResponse struct {
	OK     bool   `json:"ok"`
	Status string `json:"status"`
}

type DeviceInfoResponse struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	CPUCores      int64  `json:"cpuCores"`
	MemoryTotalMB int64  `json:"memoryTotalMb"`
	DockerAPIURL  string `json:"dockerApiUrl"`
	DockerVersion string `json:"dockerVersion"`
	DiscoveryMode string `json:"discoveryMode"`
}

// SlotResponse 是 Node Server 从 Client 读取 slot 当前态时使用的最小模型。
//
// 设计边界：
// - 这里只拿 Node 对账所需的当前态字段；
// - 不把 Client 侧额外展示字段全部复制进中心；
// - slot 正式事实仍以这次 Client 返回结果为准，Node 只做缓存和摘要。
type SlotResponse struct {
	SlotID           string `json:"slotId"`
	Status           string `json:"status"`
	CurrentPackageID string `json:"currentPackageId"`
	CurrentRunID     string `json:"currentRunId"`
	ContainerID      string `json:"containerId"`
	ContainerName    string `json:"containerName"`
	CDPPort          int64  `json:"cdpPort"`
	VNCPort          int64  `json:"vncPort"`
	LastError        string `json:"lastError"`
	InitializedAt    int64  `json:"initializedAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

// BrowserEnvRunRequest 是 Node 调用 Edge run 接口时使用的最小正式请求体。
//
// 设计边界：
// - 当前中心 run 只允许透传已经确认过的正式字段：`slotId` 和 `forceRecreate`；
// - 不允许在 Node 层临时覆盖 image/proxy/fingerprint 等关键配置；
// - 这样可以保证中心层不会重新长出一套“偷偷改 Edge 运行参数”的旁路协议。
type BrowserEnvRunRequest struct {
	SlotID        string `json:"slotId"`
	ForceRecreate bool   `json:"forceRecreate"`
}

// BrowserEnvCreateRequest 是 Node 调用 Edge 创建环境包时透传的正式配置。
//
// 这里刻意不包含 clientId/accountId：目标 Client 已由 Node 根据中心节点表选定，
// Edge 只负责本机环境包文件和 SQLite 索引，不能再承担中心归属判断。
type BrowserEnvCreateRequest struct {
	UserID      string                      `json:"userId"`
	RPAType     string                      `json:"rpaType"`
	Name        string                      `json:"name"`
	Runtime     BrowserEnvCreateRuntime     `json:"runtime"`
	Environment BrowserEnvCreateEnvironment `json:"environment"`
	Proxy       BrowserEnvCreateProxy       `json:"proxy"`
}

type BrowserEnvCreateRuntime struct {
	Image      string `json:"image"`
	StartupURL string `json:"startupUrl"`
	ShmSize    string `json:"shmSize"`
}

type BrowserEnvCreateEnvironment struct {
	Timezone string                 `json:"timezone"`
	Language string                 `json:"language"`
	Screen   BrowserEnvCreateScreen `json:"screen"`
}

type BrowserEnvCreateScreen struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Depth  int `json:"depth"`
}

type BrowserEnvCreateProxy struct {
	Enabled      *bool  `json:"enabled"`
	Type         string `json:"type"`
	ConfigBase64 string `json:"configBase64"`
}

// BrowserEnvCreateResponse 是 Edge 创建环境包成功后的同步结果。
type BrowserEnvCreateResponse struct {
	EnvID       string `json:"envId"`
	UserID      string `json:"userId"`
	RPAType     string `json:"rpaType"`
	EnvSequence int    `json:"envSequence"`
	Ports       struct {
		CDP int `json:"cdp"`
		VNC int `json:"vnc"`
	} `json:"ports"`
	EnvPath      string            `json:"envPath"`
	Files        map[string]string `json:"files"`
	IdentityHash string            `json:"identityHash"`
	CreatedAt    int64             `json:"createdAt"`
}

// BrowserEnvStopRequest 是 Node 调用 Edge stop 接口时复用的正式请求体。
type BrowserEnvStopRequest struct {
	TimeoutSeconds int `json:"timeoutSeconds"`
}

// BrowserEnvRuntimeImageRequest 是 Node 调用 Edge 修改 runtime.image 的最小请求体。
//
// 这条链路只更新环境包运行契约，不触发拉镜像、run 或 slot reinit。
type BrowserEnvRuntimeImageRequest struct {
	Image string `json:"image"`
}

// BrowserEnvRuntimeImageResponse 是 Edge runtime.image 修改接口的同步结果。
type BrowserEnvRuntimeImageResponse struct {
	EnvID         string `json:"envId"`
	Status        string `json:"status"`
	PreviousImage string `json:"previousImage"`
	Image         string `json:"image"`
	UpdatedAt     int64  `json:"updatedAt"`
}

// BrowserEnvBackupResponse 是 Edge backup 接口的接单结果。
//
// 设计边界：
// - Client backup 是正式 SSE 任务接口；
// - Node 这里只拿最小 task 标识，不复制 Edge 侧事件细节模型；
// - 最终成功或失败必须继续通过 Edge task detail 轮询确认。
type BrowserEnvBackupResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
}

// BrowserEnvRestoreResponse 是 Edge restore 接口的接单结果。
type BrowserEnvRestoreResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
}

// BrowserEnvDeletePackageResponse 是 Edge package delete 接口的接单结果。
type BrowserEnvDeletePackageResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
}

// BrowserEnvDeleteImageResult 是 Edge `/del` 同步结果里的单条镜像删除明细。
type BrowserEnvDeleteImageResult struct {
	Image    string `json:"image"`
	Deleted  string `json:"deleted,omitempty"`
	Untagged string `json:"untagged,omitempty"`
}

// BrowserEnvDeleteImageResponse 是 Edge `/del` 接口的同步结果。
type BrowserEnvDeleteImageResponse struct {
	EnvID          string                        `json:"envId"`
	Image          string                        `json:"image"`
	ImageRemoved   bool                          `json:"imageRemoved"`
	Results        []BrowserEnvDeleteImageResult `json:"results"`
	WarningMessage string                        `json:"warningMessage"`
	DeletedAt      int64                         `json:"deletedAt"`
}

// TaskAcceptedResponse 是 Edge SSE 任务接口的统一接单返回。
type TaskAcceptedResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
}

// TaskDetailResponse 是 Node 轮询 Edge task 终态时使用的最小模型。
type TaskDetailResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	Status       string `json:"status"`
	CurrentStage string `json:"currentStage"`
	Message      string `json:"message"`
	EventsURL    string `json:"eventsUrl"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	FinishedAt   string `json:"finishedAt"`
	Error        string `json:"error"`
	Suggestion   string `json:"suggestion"`
}

// BrowserEnvDetailResponse 只保留 Node 同步中心 env 摘要需要的最小字段。
//
// 职责边界：
// - Node 不复制 Edge 的完整详情结构，只读取中心缓存同步所需的运行态摘要；
// - 这能避免 Server 把 Client 细节协议整份镜像进来，后续维护成本失控；
// - 真正排障仍应直接看 Edge 详情接口，中心这里只收口“够用的聚合视图”。
type BrowserEnvDetailResponse struct {
	Index struct {
		EnvID           string `json:"envId"`
		UserID          string `json:"userId"`
		RPAType         string `json:"rpaType"`
		Name            string `json:"name"`
		Status          string `json:"status"`
		ContainerStatus string `json:"containerStatus"`
		WebVNCURL       string `json:"webVncUrl"`
	} `json:"index"`
	VNC struct {
		WebVNCURL string `json:"webVncUrl"`
	} `json:"vnc"`
}

// BrowserEnvStopResponse 是 Edge stop 接口的同步结果。
type BrowserEnvStopResponse struct {
	EnvID           string `json:"envId"`
	Status          string `json:"status"`
	ContainerStatus string `json:"containerStatus"`
	StoppedAt       int64  `json:"stoppedAt"`
}

func New() *Client {
	timeout := time.Duration(Settings.Conf.EdgeConfig.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Client{httpClient: &http.Client{Timeout: timeout}}
}

func (c *Client) GetHealth(ctx context.Context, baseURL string) (*HealthResponse, error) {
	var response HealthResponse
	if err := c.doJSON(ctx, http.MethodGet, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/health", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetDeviceInfo(ctx context.Context, baseURL string) (*DeviceInfoResponse, error) {
	var response DeviceInfoResponse
	if err := c.doJSON(ctx, http.MethodGet, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/device-info", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListSlots(ctx context.Context, baseURL string) ([]SlotResponse, error) {
	var response []SlotResponse
	if err := c.doJSON(ctx, http.MethodGet, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/slots", "", nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func (c *Client) RunBrowserEnv(ctx context.Context, baseURL, envID string, request *BrowserEnvRunRequest) (*TaskAcceptedResponse, error) {
	var response TaskAcceptedResponse
	if err := c.doJSON(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/browser-envs/"+strings.TrimSpace(envID)+"/run", "", request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// CreateBrowserEnv 调用目标 Edge 创建本机环境包。
//
// 这是短链路同步接口：Edge 成功返回 envId 后，Node 会立即写入 `server_browser_envs`。
func (c *Client) CreateBrowserEnv(ctx context.Context, baseURL string, request *BrowserEnvCreateRequest) (*BrowserEnvCreateResponse, error) {
	var response BrowserEnvCreateResponse
	if err := c.doJSON(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/browser-envs", "", request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ImportBrowserEnvPackage 把上传到 Node 的标准 tgz 包流式转发给目标 Edge。
//
// 设计边界：
// - Node 不解析包内容、不落真实资产；
// - Edge 负责解压、校验、重分配本机端口并返回 Edge task；
// - Node 后台等待 Edge task 成功后再回读 env detail 写中心缓存。
func (c *Client) ImportBrowserEnvPackage(ctx context.Context, baseURL, filename string, file io.Reader) (*TaskAcceptedResponse, error) {
	var response TaskAcceptedResponse
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/api/v1/edge/browser-envs/import-package"
	if err := c.doMultipartFile(ctx, endpoint, "file", filename, file, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) StopBrowserEnv(ctx context.Context, baseURL, envID string, request *BrowserEnvStopRequest) (*BrowserEnvStopResponse, error) {
	var response BrowserEnvStopResponse
	if err := c.doJSON(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/browser-envs/"+strings.TrimSpace(envID)+"/stop", "", request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// UpdateBrowserEnvRuntimeImage 调用目标 Edge 的 runtime.image 修改接口。
//
// 这条调用保持“单次 HTTP、无重试、无 SSE”的边界，避免中心底层客户端偷偷把配置修改升级成生命周期动作。
func (c *Client) UpdateBrowserEnvRuntimeImage(
	ctx context.Context,
	baseURL string,
	envID string,
	request *BrowserEnvRuntimeImageRequest,
) (*BrowserEnvRuntimeImageResponse, error) {
	var response BrowserEnvRuntimeImageResponse
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/") +
		"/api/v1/edge/browser-envs/" + strings.TrimSpace(envID) + "/runtime-image"
	if err := c.doJSON(ctx, http.MethodPatch, endpoint, "", request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) BackupBrowserEnv(ctx context.Context, baseURL, envID string) (*BrowserEnvBackupResponse, error) {
	var response BrowserEnvBackupResponse
	if err := c.doJSON(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/browser-envs/"+strings.TrimSpace(envID)+"/backup", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) RestoreBrowserEnv(ctx context.Context, baseURL, envID string) (*BrowserEnvRestoreResponse, error) {
	var response BrowserEnvRestoreResponse
	if err := c.doJSON(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/browser-envs/"+strings.TrimSpace(envID)+"/restore", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) RevalidateBrowserEnv(ctx context.Context, baseURL, envID string) (*TaskAcceptedResponse, error) {
	var response TaskAcceptedResponse
	if err := c.doJSON(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/browser-envs/"+strings.TrimSpace(envID)+"/revalidate", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DeleteBrowserEnvPackage(ctx context.Context, baseURL, envID string) (*BrowserEnvDeletePackageResponse, error) {
	var response BrowserEnvDeletePackageResponse
	if err := c.doJSON(ctx, http.MethodDelete, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/browser-envs/"+strings.TrimSpace(envID)+"/package", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DeleteBrowserEnvImage(ctx context.Context, baseURL, envID string) (*BrowserEnvDeleteImageResponse, error) {
	var response BrowserEnvDeleteImageResponse
	if err := c.doJSON(ctx, http.MethodDelete, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/browser-envs/"+strings.TrimSpace(envID)+"/del", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetTaskDetail(ctx context.Context, baseURL, taskID string) (*TaskDetailResponse, error) {
	var response TaskDetailResponse
	if err := c.doJSON(ctx, http.MethodGet, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/tasks/"+strings.TrimSpace(taskID), "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetBrowserEnvDetail(ctx context.Context, baseURL, envID string) (*BrowserEnvDetailResponse, error) {
	var response BrowserEnvDetailResponse
	if err := c.doJSON(ctx, http.MethodGet, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/browser-envs/"+strings.TrimSpace(envID), "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// AssignClientID 负责第一阶段 Node -> Client 的 assign 调用。
//
// 当前即使还没把真实 Client assign 对上，这里也先把协议边界固定下来：
// - 请求走 JSON
// - Header 用 `X-Edge-API-Key`
// - 不做底层自动重试
func (c *Client) AssignClientID(ctx context.Context, baseURL, apiKey string, body any) error {
	return c.doJSON(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/node-registration/assign", apiKey, body, nil)
}

// ClearClientID 负责在 unbind 后通知 Client 清空本地 node-registration.json 留痕。
//
// 职责边界：
// - 这里只调用 Edge 受控清理接口；
// - 不在这里改中心 bind 结果，不做自动重试，也不把 clear 失败包装成中心解绑失败；
// - unbind 的最终收口由上层 Service 负责。
func (c *Client) ClearClientID(ctx context.Context, baseURL, apiKey string, body any) error {
	return c.doJSON(ctx, http.MethodPost, strings.TrimRight(strings.TrimSpace(baseURL), "/")+"/api/v1/edge/node-registration/clear", apiKey, body, nil)
}

func (c *Client) doJSON(ctx context.Context, method, endpoint, apiKey string, body any, target any) error {
	if c == nil {
		c = New()
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request payload failed: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("build request failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-Edge-API-Key", strings.TrimSpace(apiKey))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("http status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	var envelope Response[json.RawMessage]
	if err = json.Unmarshal(bodyBytes, &envelope); err != nil {
		return fmt.Errorf("decode response failed: %w", err)
	}
	if envelope.Code != 1000 {
		return fmt.Errorf("%s", envelope.Message)
	}
	if target != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err = json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("decode response data failed: %w", err)
		}
	}
	return nil
}

func (c *Client) doMultipartFile(ctx context.Context, endpoint, fieldName, filename string, file io.Reader, target any) error {
	if c == nil {
		c = New()
	}
	if file == nil {
		return fmt.Errorf("multipart file reader is nil")
	}

	bodyReader, bodyWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(bodyWriter)
	writeErr := make(chan error, 1)
	go func() {
		defer close(writeErr)
		part, err := multipartWriter.CreateFormFile(fieldName, filename)
		if err != nil {
			_ = bodyWriter.CloseWithError(err)
			writeErr <- err
			return
		}
		if _, err = io.Copy(part, file); err != nil {
			_ = bodyWriter.CloseWithError(err)
			writeErr <- err
			return
		}
		if err = multipartWriter.Close(); err != nil {
			_ = bodyWriter.CloseWithError(err)
			writeErr <- err
			return
		}
		writeErr <- bodyWriter.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bodyReader)
	if err != nil {
		_ = bodyReader.Close()
		return fmt.Errorf("build multipart request failed: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		_ = bodyReader.Close()
		return fmt.Errorf("multipart request failed: %w", err)
	}
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	_ = resp.Body.Close()
	_ = bodyReader.Close()
	select {
	case err = <-writeErr:
		if err != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return fmt.Errorf("write multipart payload failed: %w", err)
		}
	case <-time.After(5 * time.Second):
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return fmt.Errorf("write multipart payload timeout")
		}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("http status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	var envelope Response[json.RawMessage]
	if err = json.Unmarshal(bodyBytes, &envelope); err != nil {
		return fmt.Errorf("decode response failed: %w", err)
	}
	if envelope.Code != 1000 {
		return fmt.Errorf("%s", envelope.Message)
	}
	if target != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err = json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("decode response data failed: %w", err)
		}
	}
	return nil
}
