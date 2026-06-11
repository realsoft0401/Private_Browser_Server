package EdgeClient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"private_browser_server/Settings"
)

// Client 封装 Server 到 Edge 的 HTTP 调用。
//
// 设计来源：
// - Server 只能通过 Private_Browser_Client 的 HTTP API 管理环境包，不能直接读 Edge SQLite、环境目录或 Docker socket；
// - 用户确认资产动作失败后不能自动重试，包括 run/stop/backup/restore/revalidate/delete/import-package；
// - 因此 EdgeClient 只做一次 HTTP 请求、统一超时、API Key Header 和响应错误映射。
//
// 维护边界：
// - 不要在这里加底层 retry；需要重试时必须由用户或管理员重新发起新的 Server task；
// - Service 层可以记录请求结果和刷新中心缓存，但不能绕过本 Client 直接散写 http.Client。
type Client struct {
	httpClient *http.Client
}

func New() *Client {
	timeout := time.Duration(Settings.Conf.EdgeConfig.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

// EdgeError 是 Edge API 调用失败后的稳定错误结构。
//
// HTTPStatus 表示网络层状态；EdgeCode/EdgeMessage 来自 Client 统一响应体。
// Server 对外返回时可以保留 Message 作为排障依据，但不要把 proxy 明文、fingerprint raw 或登录态内容写进错误。
type EdgeError struct {
	HTTPStatus  int
	EdgeCode    int64
	EdgeMessage string
	Message     string
}

func (e *EdgeError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Message)
}

// Response 是 Client 统一响应结构。
//
// data 由调用方决定具体类型；EdgeClient 只识别 code/message，不理解业务字段。
type Response[T any] struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// TaskStartResponse 是 Edge SSE 任务创建后的统一摘要。
type TaskStartResponse struct {
	TaskID       string `json:"taskId"`
	TaskType     string `json:"taskType"`
	Status       string `json:"status"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	EventsURL    string `json:"eventsUrl"`
	Message      string `json:"message"`
}

// EdgeTask 是 Edge 内存任务详情。
//
// Client task 只是短期观察，Server task 才是平台持久事实；如果 Edge task 丢失，
// Server 不能默认成功，应重新调用环境包状态接口确认事实。
type EdgeTask struct {
	TaskID       string          `json:"taskId"`
	TaskType     string          `json:"taskType"`
	Status       string          `json:"status"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	Message      string          `json:"message"`
	LastError    string          `json:"lastError"`
	Result       json.RawMessage `json:"result"`
	CreatedAt    int64           `json:"createdAt"`
	UpdatedAt    int64           `json:"updatedAt"`
	FinishedAt   *int64          `json:"finishedAt,omitempty"`
}

// DockerImage 是 Node Server 读取到的 Edge 本机 Docker 镜像摘要。
//
// 设计来源：
// - Edge run 明确要求镜像必须先存在，本机缺镜像时会直接失败；
// - 用户要求 Node Server 在 run 前先检查镜像是否已拉取，必要时主动调用 pull-image；
// - 因此这里复用 Edge `/api/v1/edge/docker/images` 的稳定摘要，而不是让业务层猜 Docker 原始字段。
type DockerImage struct {
	ID          string   `json:"id"`
	RepoTags    []string `json:"repoTags"`
	RepoDigests []string `json:"repoDigests"`
	Created     int64    `json:"created"`
	Size        int64    `json:"size"`
	VirtualSize int64    `json:"virtualSize"`
	SharedSize  int64    `json:"sharedSize"`
	Containers  int64    `json:"containers"`
}

// PullImageRequest 是 Node Server 发给 Edge pull-image 的最小请求体。
//
// 镜像选择权仍然在 Server；这里只负责把受控镜像引用转发给 Edge 本机 Docker。
type PullImageRequest struct {
	Image string `json:"image"`
	Tag   string `json:"tag,omitempty"`
}

// DeleteBrowserEnvImageResponse 是 Edge `/browser-envs/:envId/del` 的同步响应摘要。
//
// 设计来源：
// - `/del` 只删除运行镜像，不创建 Edge task；
// - Node Server 需要稳定读取 imageRemoved/results/warningMessage，给前端明确区分“成功删除”和“Docker 拒绝删除”；
// - 这里只保留协议层稳定字段，不复制 Edge 内部更细的 Docker 模型。
type DeleteBrowserEnvImageResponse struct {
	EnvID          string                    `json:"envId"`
	Image          string                    `json:"image"`
	ImageRemoved   bool                      `json:"imageRemoved"`
	Results        []DockerImageRemoveResult `json:"results,omitempty"`
	WarningMessage string                    `json:"warningMessage,omitempty"`
	DeletedAt      int64                     `json:"deletedAt"`
}

type DockerImageRemoveResult struct {
	Image    string `json:"image"`
	Deleted  string `json:"deleted,omitempty"`
	Untagged string `json:"untagged,omitempty"`
}

// DoJSON 发送一次 JSON 请求并解析 Edge 统一响应。
//
// body 为 nil 时不发送请求体；target 必须是指针或 nil。这个函数不做任何自动重试，
// 资产动作即使遇到网络错误也必须把错误交给上层任务收口。
func (c *Client) DoJSON(ctx context.Context, baseURL string, method string, path string, apiKey string, body any, target any) error {
	if c == nil {
		c = New()
	}
	endpoint, err := buildEdgeURL(baseURL, path)
	if err != nil {
		return err
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("序列化 Edge 请求体失败: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("创建 Edge 请求失败: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-Edge-API-Key", strings.TrimSpace(apiKey))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &EdgeError{Message: "Edge API 请求失败，未重试；请检查节点健康、网络、防火墙和 Client 服务状态: " + err.Error()}
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("读取 Edge 响应失败: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &EdgeError{
			HTTPStatus: resp.StatusCode,
			Message:    fmt.Sprintf("Edge API HTTP 状态异常 status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBytes))),
		}
	}

	var envelope Response[json.RawMessage]
	if err = json.Unmarshal(respBytes, &envelope); err != nil {
		return fmt.Errorf("解析 Edge 统一响应失败: %w", err)
	}
	if envelope.Code != 1000 {
		return &EdgeError{
			HTTPStatus:  resp.StatusCode,
			EdgeCode:    envelope.Code,
			EdgeMessage: envelope.Message,
			Message:     envelope.Message,
		}
	}
	if target == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err = json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("解析 Edge data 失败: %w", err)
	}
	return nil
}

// StartBrowserEnvTask 调用 Edge run 并返回 Edge taskId。
func (c *Client) StartBrowserEnvTask(ctx context.Context, baseURL string, apiKey string, envID string, body any) (*TaskStartResponse, error) {
	var result TaskStartResponse
	err := c.DoJSON(ctx, baseURL, http.MethodPost, "/api/v1/edge/browser-envs/"+url.PathEscape(envID)+"/run", apiKey, body, &result)
	return &result, err
}

// StopBrowserEnvTask 调用 Edge stop 并返回 Edge taskId。
func (c *Client) StopBrowserEnvTask(ctx context.Context, baseURL string, apiKey string, envID string, body any) (*TaskStartResponse, error) {
	var result TaskStartResponse
	err := c.DoJSON(ctx, baseURL, http.MethodPost, "/api/v1/edge/browser-envs/"+url.PathEscape(envID)+"/stop", apiKey, body, &result)
	return &result, err
}

// RevalidateBrowserEnvTask 调用 Edge revalidate 并返回 Edge taskId。
func (c *Client) RevalidateBrowserEnvTask(ctx context.Context, baseURL string, apiKey string, envID string) (*TaskStartResponse, error) {
	var result TaskStartResponse
	err := c.DoJSON(ctx, baseURL, http.MethodPost, "/api/v1/edge/browser-envs/"+url.PathEscape(envID)+"/revalidate", apiKey, nil, &result)
	return &result, err
}

// GetEdgeTask 读取 Edge 短期任务详情。
func (c *Client) GetEdgeTask(ctx context.Context, baseURL string, apiKey string, edgeTaskID string) (*EdgeTask, error) {
	var result EdgeTask
	err := c.DoJSON(ctx, baseURL, http.MethodGet, "/api/v1/edge/tasks/"+url.PathEscape(edgeTaskID), apiKey, nil, &result)
	return &result, err
}

// GetDockerImages 读取 Edge 本机 Docker 镜像列表。
func (c *Client) GetDockerImages(ctx context.Context, baseURL string, apiKey string) ([]DockerImage, error) {
	var result []DockerImage
	err := c.DoJSON(ctx, baseURL, http.MethodGet, "/api/v1/edge/docker/images", apiKey, nil, &result)
	return result, err
}

// PullDockerImageTask 调用 Edge pull-image 并返回 Edge taskId。
func (c *Client) PullDockerImageTask(ctx context.Context, baseURL string, apiKey string, body *PullImageRequest) (*TaskStartResponse, error) {
	var result TaskStartResponse
	err := c.DoJSON(ctx, baseURL, http.MethodPost, "/api/v1/edge/docker/pull-image", apiKey, body, &result)
	return &result, err
}

// DeleteBrowserEnvPackageTask 调用 Edge `/package` 并返回 Edge taskId。
func (c *Client) DeleteBrowserEnvPackageTask(ctx context.Context, baseURL string, apiKey string, envID string) (*TaskStartResponse, error) {
	var result TaskStartResponse
	err := c.DoJSON(ctx, baseURL, http.MethodDelete, "/api/v1/edge/browser-envs/"+url.PathEscape(envID)+"/package", apiKey, nil, &result)
	return &result, err
}

// DeleteBrowserEnvImage 调用 Edge `/del` 删除环境包关联运行镜像。
func (c *Client) DeleteBrowserEnvImage(ctx context.Context, baseURL string, apiKey string, envID string) (*DeleteBrowserEnvImageResponse, error) {
	var result DeleteBrowserEnvImageResponse
	err := c.DoJSON(ctx, baseURL, http.MethodDelete, "/api/v1/edge/browser-envs/"+url.PathEscape(envID)+"/del", apiKey, nil, &result)
	return &result, err
}

func buildEdgeURL(baseURL string, path string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("Edge baseURL 不能为空")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("Edge baseURL 非法: %s", baseURL)
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path, nil
}
