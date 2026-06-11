package Env

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	envModel "private_browser_server/Models/Env"
	"private_browser_server/Pkg/HttpResponse"
)

// validateCreateEnvRequest 负责锁住中心创建环境包的入参边界。
//
// 这里不只是做字段非空校验，还承担“中心层协议比 Edge 更稳定”的职责，
// 避免非法 rpaType、空 timezone 或无效 screen 尺寸直接带到 Edge，再把协议错误暴露给前端。
func validateCreateEnvRequest(param *envModel.CreateEnvRequest) error {
	if param == nil {
		return invalidError("请求体不能为空")
	}
	if strings.TrimSpace(param.ClientID) == "" {
		return invalidError("clientId 不能为空")
	}
	if strings.TrimSpace(param.Name) == "" {
		return invalidError("name 不能为空")
	}
	if strings.TrimSpace(param.RPAType) == "" {
		return invalidError("rpaType 不能为空")
	}
	normalizedRPAType, err := normalizeRPATypeForEdge(param.RPAType)
	if err != nil {
		return err
	}
	param.RPAType = normalizedRPAType
	if strings.TrimSpace(param.Environment.Timezone) == "" {
		return invalidError("environment.timezone 不能为空")
	}
	if strings.TrimSpace(param.Environment.Language) == "" {
		return invalidError("environment.language 不能为空")
	}
	if param.Environment.Screen.Width <= 0 || param.Environment.Screen.Height <= 0 || param.Environment.Screen.Depth <= 0 {
		return invalidError("environment.screen.width/height/depth 必须为正整数")
	}
	proxyEnabled := param.Proxy.Enabled != nil && *param.Proxy.Enabled
	if proxyEnabled && strings.TrimSpace(param.Proxy.ConfigBase64) == "" {
		return invalidError("proxy.enabled=true 时必须提供 proxy.configBase64")
	}
	return nil
}

func buildPreviewURLs(baseURL string, envID string, ports envModel.BrowserEnvPorts) (string, string) {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return "", ""
	}
	webVNCURL := trimmed + "/web-vnc.html?envId=" + url.QueryEscape(envID)
	if ports.CDP <= 0 {
		return "", webVNCURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Hostname() == "" {
		return "", webVNCURL
	}
	cdpHost := net.JoinHostPort(parsed.Hostname(), strconv.Itoa(ports.CDP))
	return "http://" + cdpHost, webVNCURL
}

func normalizeListEnvQuery(ctx *gin.Context) envModel.ListEnvQuery {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return envModel.ListEnvQuery{
		ClientID: strings.TrimSpace(ctx.Query("clientId")),
		RPAType:  normalizeOptionalRPATypeAlias(ctx.Query("rpaType")),
		Status:   strings.TrimSpace(ctx.Query("status")),
		Page:     page,
		PageSize: pageSize,
	}
}

// normalizeRPATypeForEdge 负责把 Platform 侧可读别名归一化成 Edge 认可的短码。
//
// 设计来源：
// - 当前商业侧示例和讨论里常用 tiktok/facebook/instagram 等全称；
// - 但 Edge 为了保持 envId、目录结构和云存储 key 短而稳定，只接受 tk/fb/ins/yt/x/other；
// - 因此中心层必须在这里做一次受控归一化，不能把这个协议差异甩给前端或让联调时随机踩坑。
//
// 职责边界：
// - 只负责把少量稳定别名映射成 Edge 短码；
// - 不在这里发明新的平台枚举，也不允许未知值静默透传到 Edge；
// - response/list/detail 里仍以归一化后的短码为准，保证中心缓存和 Edge 事实一致。
func normalizeRPATypeForEdge(value string) (string, error) {
	normalized := normalizeOptionalRPATypeAlias(value)
	if normalized == "" {
		return "", invalidError("rpaType 不能为空")
	}
	switch normalized {
	case "tk", "fb", "ins", "yt", "x", "other":
		return normalized, nil
	default:
		return "", invalidError("rpaType 仅支持 tk/fb/ins/yt/x/other，兼容别名 tiktok/facebook/instagram/youtube/twitter")
	}
}

// normalizeOptionalRPATypeAlias 用于把查询条件或创建请求中的平台别名折叠到统一短码。
//
// 这个函数允许空值直接返回空字符串，方便列表过滤继续表达“不过滤 rpaType”；
// 真正的必填校验和未知值拦截仍由 normalizeRPATypeForEdge 负责。
func normalizeOptionalRPATypeAlias(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "tk", "tiktok":
		return "tk"
	case "fb", "facebook":
		return "fb"
	case "ins", "instagram":
		return "ins"
	case "yt", "youtube":
		return "yt"
	case "x", "twitter":
		return "x"
	case "other":
		return "other"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func bindOptionalRunRequest(ctx *gin.Context) (*envModel.RunEnvRequest, bool) {
	param := new(envModel.RunEnvRequest)
	if err := ctx.ShouldBindJSON(param); err != nil {
		if errors.Is(err, io.EOF) {
			return param, true
		}
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "请求参数格式错误")
		return nil, false
	}
	return param, true
}

func bindOptionalStopRequest(ctx *gin.Context) (*envModel.StopEnvRequest, bool) {
	param := new(envModel.StopEnvRequest)
	if err := ctx.ShouldBindJSON(param); err != nil {
		if errors.Is(err, io.EOF) {
			return param, true
		}
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "请求参数格式错误")
		return nil, false
	}
	return param, true
}

func bindStrictJSON(ctx *gin.Context, target any) bool {
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "请求参数格式错误: "+err.Error())
		return false
	}
	return true
}
