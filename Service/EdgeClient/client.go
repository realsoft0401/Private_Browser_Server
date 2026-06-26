package EdgeClient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
