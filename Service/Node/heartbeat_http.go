package Node

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	NodeDAO "private_browser_server/Dao/Node"
	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
	DiscoveryService "private_browser_server/Service/Discovery"
	EdgeClientService "private_browser_server/Service/EdgeClient"
)

// ReceiveHeartbeat 接收 Client 主动上报的正式 HTTP 心跳。
//
// 设计来源：
// - 这条接口曾经被当成 discovery 补充链路使用，但最新定案已经收紧为“节点发现只有 UDP beacon”；
// - heartbeat 仍然必须存在，因为它承担“已知节点活性回执”的职责；
// - 因此这里现在只更新已知正式节点的 heartbeat 摘要，不创建 discovered、不自动 bind。
//
// 职责边界：
// - 只接受最小 discovery/活性摘要，不接任何业务资产或敏感数据；
// - 不参与发现，不更新 discovered，不自动落正式绑定节点，不自动生成 clientId；
// - 真正发现仍然只走 UDP beacon，真正绑定仍然走 `/api/v1/edge-clients/bind`。
func ReceiveHeartbeat(ctx *gin.Context) {
	var request DiscoveryService.BeaconPayload
	if err := ctx.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "heartbeat request body 非法")
		return
	}
	if err := DiscoveryService.ValidateBeaconPayload(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, err.Error())
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	matchedClientID, matchedBy, err := updateKnownNodeHeartbeat(requestCtx, &request)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, gin.H{
		"received":                true,
		"source":                  "heartbeat",
		"participatesInDiscovery": false,
		"knownNodeMatched":        matchedClientID != "",
		"matchedClientId":         matchedClientID,
		"matchedBy":               matchedBy,
		"message":                 heartbeatMessage(matchedClientID != ""),
		"clientIp":                request.ClientIP,
		"baseUrl":                 request.BaseURL,
		"lastHeartbeatAt":         request.LastHeartbeatAt,
	})
}

func updateKnownNodeHeartbeat(ctx context.Context, request *DiscoveryService.BeaconPayload) (string, string, error) {
	repo := NodeRepo.NewRepository()
	node, matchedBy, err := findHeartbeatNode(ctx, repo, request)
	if err == NodeRepo.ErrNotFound {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	receivedAt := time.Now().Unix()
	reportedAt := request.LastHeartbeatAt
	if reportedAt <= 0 {
		reportedAt = receivedAt
	}
	if err = repo.MarkHeartbeatHealthy(
		ctx,
		node.ClientID,
		receivedAt,
		reportedAt,
		"heartbeat_http",
		strings.TrimSpace(request.ClientIP),
		strings.TrimSpace(request.BaseURL),
	); err != nil {
		return "", "", err
	}
	if err = refreshKnownNodeDeviceFacts(ctx, repo, node, request, receivedAt); err != nil {
		return "", "", err
	}
	return node.ClientID, matchedBy, nil
}

// refreshKnownNodeDeviceFacts 在 heartbeat 命中已知节点后补刷一次设备摘要。
//
// 设计来源：
// - 仅靠 bind 写一次设备事实，会让“早期已绑定节点”长期保留空的 cpu/memory/dockerVersion；
// - 用户已经明确要求这些字段应成为中心节点摘要的一部分，因此 heartbeat 期间允许刷新；
// - 这里只更新设备摘要，不把 heartbeat 重新变成 discovery。
func refreshKnownNodeDeviceFacts(ctx context.Context, repo *NodeRepo.Repository, node *NodeModel.EdgeClient, request *DiscoveryService.BeaconPayload, now int64) error {
	if repo == nil || node == nil {
		return nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(request.BaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(node.BaseURL), "/")
	}
	if baseURL == "" {
		return nil
	}
	deviceInfo, err := EdgeClientService.New().GetDeviceInfo(ctx, baseURL)
	if err != nil {
		log.Printf("refresh node device facts skipped, clientId=%s, baseUrl=%s, err=%v\n", node.ClientID, baseURL, err)
		return nil
	}
	if deviceInfo.CPUCores <= 0 || deviceInfo.MemoryTotalMB <= 0 || strings.TrimSpace(deviceInfo.DockerVersion) == "" {
		log.Printf("refresh node device facts got incomplete payload, clientId=%s, baseUrl=%s, cpuCores=%d, memoryTotalMb=%d, dockerVersion=%q\n",
			node.ClientID,
			baseURL,
			deviceInfo.CPUCores,
			deviceInfo.MemoryTotalMB,
			strings.TrimSpace(deviceInfo.DockerVersion),
		)
	}
	return repo.UpdateDeviceFacts(ctx, &NodeDAO.Row{
		ClientID:      node.ClientID,
		ClientIP:      firstNonEmpty(strings.TrimSpace(request.ClientIP), strings.TrimSpace(node.ClientIP)),
		BaseURL:       firstNonEmpty(baseURL, strings.TrimSpace(node.BaseURL)),
		DockerAPIURL:  strings.TrimSpace(deviceInfo.DockerAPIURL),
		OS:            strings.TrimSpace(deviceInfo.OS),
		Arch:          strings.TrimSpace(deviceInfo.Arch),
		CPUCores:      deviceInfo.CPUCores,
		MemoryTotalMB: deviceInfo.MemoryTotalMB,
		DockerVersion: strings.TrimSpace(deviceInfo.DockerVersion),
		LastCheckedAt: now,
		UpdatedAt:     now,
	})
}

func findHeartbeatNode(ctx context.Context, repo *NodeRepo.Repository, request *DiscoveryService.BeaconPayload) (*NodeModel.EdgeClient, string, error) {
	baseURL := strings.TrimSpace(request.BaseURL)
	if baseURL != "" {
		node, err := repo.GetByBaseURL(ctx, baseURL)
		if err == nil {
			return node, "baseUrl", nil
		}
		if err != NodeRepo.ErrNotFound {
			return nil, "", err
		}
	}
	clientIP := strings.TrimSpace(request.ClientIP)
	if clientIP != "" {
		node, err := repo.GetByClientIP(ctx, clientIP)
		if err == nil {
			return node, "clientIp", nil
		}
		if err != NodeRepo.ErrNotFound {
			return nil, "", err
		}
	}
	return nil, "", NodeRepo.ErrNotFound
}

func heartbeatMessage(matched bool) string {
	if matched {
		return "heartbeat accepted and known node liveness updated; discovery still only relies on udp beacon"
	}
	return "heartbeat accepted but no known node matched; discovery still only relies on udp beacon"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
