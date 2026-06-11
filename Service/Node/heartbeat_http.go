package Node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
	DiscoveryService "private_browser_server/Service/Discovery"
	"private_browser_server/Settings"
)

// ReceiveHeartbeat 接收 Edge Client 主动上报的正式心跳。
//
// 设计来源：
// - 之前 Node Server 只有 UDP discovery 的被动回写，这会让 last_heartbeat_at 只反映 beacon 命中时间；
// - 用户确认 last_heartbeat_at 应该是“心跳时间”，但更稳妥的做法是把“服务端接收时间”作为主事实，
//   同时保留 Client 自报时间用于排障和时钟偏差分析；
// - 这个接口是 Edge Client 直接打给 Node Server 的，不依赖 Platform Header，因为它不是前端业务调用。
//
// 职责边界：
// - 只接受 discoveryMagic/service/discoveryGroup/protocolVersion/baseUrl/clientIp 这类非敏感摘要；
// - 只更新 edge_clients 的心跳事实，不修改 healthStatus/discoveryStatus，不把节点自动升为 verified；
// - 不接收 proxy 明文、fingerprint raw、browser-data 或登录态内容。
func ReceiveHeartbeat(ctx *gin.Context) {
	var req heartbeatRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "请求体格式错误，需要 JSON: {\"discoveryMagic\":\"...\",\"protocolVersion\":1,\"service\":\"Private_Browser_Client\",\"discoveryGroup\":\"default\",\"baseUrl\":\"http://127.0.0.1:3300\",\"clientIp\":\"192.168.10.119\",\"lastHeartbeatAt\":123}")
		return
	}
	if err := validateHeartbeatRequest(&req); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, err.Error())
		return
	}

	receivedAt := time.Now().Unix()
	reportedAt := req.LastHeartbeatAt
	if reportedAt <= 0 {
		reportedAt = receivedAt
	}
	sourceIP := ctx.ClientIP()

	repo := NodeRepo.Repository{}
	node, err := repo.GetByHeartbeatLookup(ctx.Request.Context(), req.BaseURL, req.ClientIP, sourceIP)
	if errors.Is(err, NodeRepo.ErrNotFound) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "找不到已注册的 Edge Client，请先通过注册或 verify 流程绑定 baseUrl/clientIp")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "查找目标 Edge Client 失败: "+err.Error())
		return
	}

	node.LastHeartbeatAt = receivedAt
	node.LastHeartbeatReportedAt = reportedAt
	node.LastHeartbeatSource = "http"
	node.UpdatedAt = receivedAt
	if err = repo.UpdateHeartbeat(ctx.Request.Context(), node.MainAccountID, node.ID, receivedAt, reportedAt, node.LastHeartbeatSource); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "保存心跳失败: "+err.Error())
		return
	}
	attachHeartbeatStatus(node, receivedAt)
	HttpResponse.ResponseSuccess(ctx, heartbeatResponse{
		ClientID:                node.ID,
		MainAccountID:           node.MainAccountID,
		BaseURL:                 node.BaseURL,
		ClientIP:                node.ClientIP,
		LastHeartbeatAt:         node.LastHeartbeatAt,
		LastHeartbeatReportedAt: node.LastHeartbeatReportedAt,
		HeartbeatStatus:         node.HeartbeatStatus,
		UpdatedAt:               node.UpdatedAt,
		Client:                  node,
	})
}

// attachClientIDToDiscovered 只在查询响应阶段把已注册 Client 身份补到 UDP 发现结果上。
//
// 这个函数不写数据库，也不把 UDP beacon 升级成 verified，避免“收到 UDP 就绑定”的错误路径。
// 匹配优先级是 baseUrl，其次 clientIp/sourceIp；baseUrl 更接近 Server 后续要访问的 HTTP 入口，
// IP 匹配只作为旧配置或手工录入不完整时的兜底展示。
func attachClientIDToDiscovered(items []DiscoveryService.DiscoveredClient, nodes []NodeModel.EdgeClient) []discoveredClientView {
	byBaseURL := make(map[string]string, len(nodes))
	byClientIP := make(map[string]string, len(nodes))
	for _, node := range nodes {
		if baseURL := normalizeLookupURL(node.BaseURL); baseURL != "" {
			byBaseURL[baseURL] = node.ID
		}
		if clientIP := strings.TrimSpace(node.ClientIP); clientIP != "" {
			byClientIP[clientIP] = node.ID
		}
	}

	views := make([]discoveredClientView, 0, len(items))
	for _, item := range items {
		clientID := ""
		if baseURL := normalizeLookupURL(item.Payload.BaseURL); baseURL != "" {
			clientID = byBaseURL[baseURL]
		}
		if clientID == "" {
			clientID = byClientIP[strings.TrimSpace(item.Payload.ClientIP)]
		}
		if clientID == "" {
			clientID = byClientIP[strings.TrimSpace(item.SourceIP)]
		}
		views = append(views, discoveredClientView{
			ClientID:      clientID,
			SourceIP:      item.SourceIP,
			SourcePort:    item.SourcePort,
			Payload:       item.Payload,
			FirstSeenAt:   item.FirstSeenAt,
			LastSeenAt:    item.LastSeenAt,
			ReceiveCount:  item.ReceiveCount,
			DiscardReason: item.DiscardReason,
		})
	}
	return views
}

// syncDiscoveredHeartbeats 把已经匹配到 clientId 的 UDP 心跳写回 edge_clients。
//
// 这一步只服务“列表页能看到最后一次 UDP 广播时间”，不承担绑定、不创建节点、不验证节点可用性。
// 未绑定发现项 clientId 为空，必须跳过，避免因为 UDP 源地址相似就产生错误归属。
func syncDiscoveredHeartbeats(ctx context.Context, mainAccountID string, views []discoveredClientView) error {
	repo := NodeRepo.Repository{}
	for _, view := range views {
		if strings.TrimSpace(view.ClientID) == "" {
			continue
		}
		receivedAt := view.LastSeenAt
		reportedAt := view.Payload.LastHeartbeatAt
		if reportedAt <= 0 {
			reportedAt = receivedAt
		}
		if err := repo.UpdateHeartbeat(ctx, mainAccountID, view.ClientID, receivedAt, reportedAt, "udp"); err != nil {
			return err
		}
	}
	return nil
}

// attachHeartbeatStatusList 给列表响应补充 heartbeatStatus。
//
// heartbeatStatus 是展示/调度前置判断字段，不直接写数据库；真实健康仍由 healthStatus 表达。
func attachHeartbeatStatusList(nodes []NodeModel.EdgeClient, now int64) {
	for i := range nodes {
		attachHeartbeatStatus(&nodes[i], now)
	}
}

// attachHeartbeatStatus 根据 lastHeartbeatAt 计算 UDP 心跳状态。
//
// online/stale/offline 只说明 Node Server 最近是否收到该 Client 的 UDP beacon；
// 它不能替代 /health、Docker 2375 和设备探测，也不能把节点自动升为 verified。
func attachHeartbeatStatus(node *NodeModel.EdgeClient, now int64) {
	if node == nil {
		return
	}
	if node.LastHeartbeatAt <= 0 {
		node.HeartbeatStatus = NodeModel.NodeHeartbeatOffline
		return
	}
	staleAfter := int64(Settings.Conf.DiscoveryConfig.StaleAfterSeconds)
	offlineAfter := int64(Settings.Conf.DiscoveryConfig.OfflineAfterSeconds)
	if staleAfter <= 0 {
		staleAfter = 30
	}
	if offlineAfter <= 0 {
		offlineAfter = 90
	}
	age := now - node.LastHeartbeatAt
	switch {
	case age <= staleAfter:
		node.HeartbeatStatus = NodeModel.NodeHeartbeatOnline
	case age <= offlineAfter:
		node.HeartbeatStatus = NodeModel.NodeHeartbeatStale
	default:
		node.HeartbeatStatus = NodeModel.NodeHeartbeatOffline
	}
}

// validateHeartbeatRequest 校验 Edge 正式心跳上报请求。
//
// 设计来源：
// - UDP beacon 和 HTTP heartbeat 都应继续复用同一套 discovery 域识别字段；
// - 否则 Node Server 很容易把其它内网探测器或错误脚本的 HTTP 请求当成私有浏览器 Client 心跳；
// - 当前接口不要求 clientId，因为 Client 自身不保存中心身份，仍然以 baseUrl/clientIp/sourceIp 做匹配。
//
// 职责边界：
// - 这里只校验平台识别字段和接入摘要格式；
// - 不在这里做数据库查找，也不判断 healthStatus/discoveryStatus；
// - baseUrl/clientIp 允许缺一，但不能同时为空。
func validateHeartbeatRequest(req *heartbeatRequest) error {
	if req == nil {
		return fmt.Errorf("请求参数不能为空")
	}
	req.DiscoveryMagic = strings.TrimSpace(req.DiscoveryMagic)
	req.Service = strings.TrimSpace(req.Service)
	req.DiscoveryGroup = strings.TrimSpace(req.DiscoveryGroup)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.ClientIP = strings.TrimSpace(req.ClientIP)

	if req.DiscoveryMagic != strings.TrimSpace(Settings.Conf.DiscoveryConfig.Magic) {
		return fmt.Errorf("discoveryMagic 不匹配当前 Node Server 发现域")
	}
	if req.ProtocolVersion != Settings.Conf.DiscoveryConfig.ProtocolVersion {
		return fmt.Errorf("protocolVersion 不匹配当前 Node Server 配置")
	}
	if req.Service != "private-browser-client" && req.Service != "Private_Browser_Client" {
		return fmt.Errorf("service 必须是 Private_Browser_Client")
	}
	if req.DiscoveryGroup != strings.TrimSpace(Settings.Conf.DiscoveryConfig.Group) {
		return fmt.Errorf("discoveryGroup 不匹配当前 Node Server 发现域")
	}
	if req.BaseURL == "" && req.ClientIP == "" {
		return fmt.Errorf("baseUrl 和 clientIp 不能同时为空")
	}
	if req.BaseURL != "" {
		baseURL, err := normalizeHTTPURL(req.BaseURL)
		if err != nil {
			return fmt.Errorf("baseUrl 非法: %w", err)
		}
		req.BaseURL = baseURL
	}
	return nil
}
