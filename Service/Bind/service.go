package Bind

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	BindDAO "private_browser_server/Dao/Bind"
	NodeDAO "private_browser_server/Dao/Node"
	BindModel "private_browser_server/Models/Bind"
	NodeModel "private_browser_server/Models/Node"
	BindRepo "private_browser_server/Repository/Bind"
	NodeRepo "private_browser_server/Repository/Node"
	DiscoveryService "private_browser_server/Service/Discovery"
	EdgeClientService "private_browser_server/Service/EdgeClient"
	"private_browser_server/Settings"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

// BindByAccountAndClientIP 负责第一阶段 bind 主逻辑。
//
// 当前职责边界：
// - 按已经拍板的 `accountId + clientIp` 收口；
// - 先把正式节点写入中心库；
// - 再自动尝试 push clientId；
// - 即使 push 失败，也不回滚 bind。
//
// 当前仍然保留第一阶段 bind 输入，但内部序号策略已经收紧：
// - discovery 不落正式表，但 bind 前会真实探测 Client；
// - 普通 bind 前必须检查 Client 本地 node-registration 锁，只要已有 JSON 留痕就拒绝；
// - clientId 仍然是 `mainAccountId + 4位序号`；
// - unbind 删除当前有效绑定结果后，下一次 bind 必须重新生成新的 clientId；
// - 设备序号不再按列表数量猜，而是走 repository 里的 `MAX(client_sequence)+1`。
func (s *Service) BindByAccountAndClientIP(ctx context.Context, request BindModel.BindRequest) (*BindModel.BindResponse, error) {
	accountID := strings.TrimSpace(request.AccountID)
	clientIP := strings.TrimSpace(request.ClientIP)
	if accountID == "" {
		return nil, fmt.Errorf("accountId 不能为空")
	}
	if clientIP == "" {
		return nil, fmt.Errorf("clientIp 不能为空")
	}
	var existingNode *NodeModel.EdgeClient
	existing, err := NodeRepo.NewRepository().GetByClientIP(ctx, clientIP)
	if err == nil && existing != nil {
		existingNode = existing
	} else if err != nil && err != NodeRepo.ErrNotFound {
		return nil, err
	}
	if existingNode != nil {
		switch strings.TrimSpace(existingNode.MainAccountID) {
		case accountID:
			return nil, fmt.Errorf("该 Client 已经绑定，无需重复绑定")
		default:
			return nil, fmt.Errorf("该 Client 已被其它账号绑定，请先解绑后再重新绑定")
		}
	}

	discovered, err := DiscoveryService.ProbeClientByIP(ctx, clientIP)
	if err != nil {
		return nil, err
	}
	DiscoveryService.Upsert(*discovered)
	if err = ensureClientRegistrationUnlocked(ctx, discovered.BaseURL); err != nil {
		now := time.Now().Unix()
		_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
			ClientID:      "",
			MainAccountID: accountID,
			ClientIP:      clientIP,
			Action:        "bind",
			Result:        "failed",
			Message:       err.Error(),
			CreatedAt:     now,
		})
		return nil, err
	}

	now := time.Now().Unix()
	apiKeyHash := hashAPIKey(strings.TrimSpace(Settings.Conf.EdgeConfig.APIKey))
	clientSequence, err := NodeRepo.NewRepository().AllocateNextSequence(ctx, accountID)
	if err != nil {
		return nil, err
	}
	clientID := fmt.Sprintf("%s%04d", accountID, clientSequence)
	row := &NodeDAO.Row{
		ClientID:                clientID,
		MainAccountID:           accountID,
		ClientSequence:          clientSequence,
		Name:                    firstNonEmpty(discovered.Hostname, clientID),
		ClientIP:                discovered.ClientIP,
		BaseURL:                 discovered.BaseURL,
		DockerAPIURL:            discovered.DockerAPIURL,
		OS:                      firstNonEmpty(discovered.OS, "unknown"),
		Arch:                    firstNonEmpty(discovered.Arch, "unknown"),
		CPUCores:                discovered.CPUCores,
		MemoryTotalMB:           discovered.MemoryTotalMB,
		DockerVersion:           firstNonEmpty(discovered.DockerVersion, ""),
		HealthStatus:            DiscoveryService.NormalizeNodeHealthStatus(discovered.HealthStatus),
		DiscoveryStatus:         "verified",
		DiscoveryReason:         "",
		PushStatus:              "pending",
		APIKeyHash:              apiKeyHash,
		LastDiscoveredAt:        discovered.DiscoveredAt,
		LastHeartbeatAt:         discovered.LastHeartbeatAt,
		LastHeartbeatReportedAt: discovered.LastHeartbeatAt,
		LastHeartbeatSource:     "bind",
		LastCheckedAt:           now,
		CreatedAt:               now,
		UpdatedAt:               now,
		DeletedAt:               0,
	}
	err = NodeRepo.NewRepository().Create(ctx, row)
	if err != nil {
		_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
			ClientID:      clientID,
			MainAccountID: accountID,
			ClientIP:      clientIP,
			Action:        "bind",
			Result:        "failed",
			Message:       err.Error(),
			CreatedAt:     now,
		})
		return nil, err
	}

	response := &BindModel.BindResponse{
		ClientID:   clientID,
		AccountID:  accountID,
		Status:     "bound",
		ClientIP:   discovered.ClientIP,
		BaseURL:    discovered.BaseURL,
		BindStatus: "success",
		PushStatus: "pending",
		Node: &NodeModel.EdgeClient{
			ClientID:                clientID,
			MainAccountID:           accountID,
			ClientSequence:          row.ClientSequence,
			Name:                    row.Name,
			ClientIP:                row.ClientIP,
			BaseURL:                 row.BaseURL,
			DockerAPIURL:            row.DockerAPIURL,
			OS:                      row.OS,
			Arch:                    row.Arch,
			HealthStatus:            row.HealthStatus,
			DiscoveryStatus:         row.DiscoveryStatus,
			DiscoveryReason:         row.DiscoveryReason,
			PushStatus:              row.PushStatus,
			LastDiscoveredAt:        row.LastDiscoveredAt,
			LastHeartbeatAt:         row.LastHeartbeatAt,
			LastHeartbeatReportedAt: row.LastHeartbeatReportedAt,
			LastHeartbeatSource:     row.LastHeartbeatSource,
			LastCheckedAt:           row.LastCheckedAt,
			CreatedAt:               row.CreatedAt,
			UpdatedAt:               row.UpdatedAt,
		},
	}
	_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
		ClientID:      clientID,
		MainAccountID: accountID,
		ClientIP:      clientIP,
		Action:        "bind",
		Result:        "success",
		Message:       "bind success",
		CreatedAt:     now,
	})

	pushErr := s.PushClientID(ctx, clientID, BindModel.PushClientIDRequest{
		AccountID:         accountID,
		ClientID:          clientID,
		NodeServerBaseURL: strings.TrimRight(strings.TrimSpace(request.NodeServerBaseURL), "/"),
		Source:            "node-bind",
		AssignedAt:        now,
	})
	if pushErr != nil {
		response.PushStatus = "failed"
		response.PushMessage = pushErr.Error()
		_ = NodeRepo.NewRepository().UpdatePushStatus(ctx, clientID, "failed", time.Now().Unix())
		return response, nil
	}
	response.PushStatus = "success"
	_ = NodeRepo.NewRepository().UpdatePushStatus(ctx, clientID, "success", time.Now().Unix())
	return response, nil
}

// ensureClientRegistrationUnlocked 把 Client 本地 node-registration.json 收口成普通 bind 的本地互斥锁。
//
// 设计来源：
// - 多个 Node Server 不共享 SQLite 时，每个 Node 都只能看到自己的 `edge_clients`；
// - 如果普通 bind 只查当前 Node 数据库，就会允许另一台 Node 抢同一台 Client；
// - 用户已经明确收口：只要 Client 本地存在 `node-registration.json`，普通 bind 一律拒绝。
//
// 职责边界：
// - 这里只做普通 bind 的前置拒绝；
// - 不删除、不覆盖 Client 本地注册文件；
// - 不根据 cachedRegistration 里的 clientId/accountId 做“看起来相同就放行”的兼容；
// - 后续如果要支持旧 Node 不可用时的管理员接管，必须做单独 force 流程和审计，不能塞进这里。
func ensureClientRegistrationUnlocked(ctx context.Context, baseURL string) error {
	status, err := EdgeClientService.New().GetNodeRegistration(ctx, baseURL)
	if err != nil {
		return fmt.Errorf("检查 Client 本地 node-registration 失败，无法确认绑定锁状态: %w", err)
	}
	if status == nil {
		return nil
	}
	if strings.TrimSpace(status.CacheStatus) != "cached" || status.CachedRegistration == nil {
		return nil
	}
	registration := status.CachedRegistration
	return fmt.Errorf(
		"该 Client 已存在 node-registration.json，普通 bind 被拒绝；请先从旧 Node unbind 清理本地注册；旧 Node 不可用时由管理员手动删除 Client 本地注册文件后再重新 bind；currentNode=%s clientId=%s accountId=%s",
		strings.TrimSpace(registration.NodeServerBaseURL),
		strings.TrimSpace(registration.ClientID),
		strings.TrimSpace(registration.MainAccountID),
	)
}

// UnbindClient 删除当前有效绑定结果，并尝试清理 Client 本地 node-registration.json 留痕。
//
// 职责边界：
// - 先删除中心当前有效绑定结果，再尝试清理 Client 本地留痕；
// - 清理本地失败不回滚中心解绑；
// - 后续如果再次 bind，必须重新生成新的 clientId。
func (s *Service) UnbindClient(ctx context.Context, clientID string, request BindModel.UnbindRequest) (*BindModel.UnbindResponse, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return nil, fmt.Errorf("clientId 不能为空")
	}
	node, err := NodeRepo.NewRepository().GetByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(node.MainAccountID) == "" {
		return nil, fmt.Errorf("该 Client 当前未绑定，无需解绑")
	}

	now := time.Now().Unix()
	accountID := node.MainAccountID
	if err = NodeRepo.NewRepository().Unbind(ctx, clientID, now); err != nil {
		_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
			ClientID:      clientID,
			MainAccountID: accountID,
			ClientIP:      node.ClientIP,
			Action:        "unbind",
			Result:        "failed",
			Message:       err.Error(),
			CreatedAt:     now,
		})
		return nil, err
	}

	_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
		ClientID:      clientID,
		MainAccountID: accountID,
		ClientIP:      node.ClientIP,
		Action:        "unbind",
		Result:        "success",
		Message:       "unbind success",
		CreatedAt:     now,
	})

	response := &BindModel.UnbindResponse{
		ClientID:                clientID,
		AccountID:               accountID,
		Status:                  "unbound",
		ClearRegistrationStatus: "pending",
		UnboundAt:               now,
	}

	clearErr := EdgeClientService.New().ClearClientID(ctx, node.BaseURL, strings.TrimSpace(Settings.Conf.EdgeConfig.APIKey), map[string]any{
		"source":    firstNonEmpty(strings.TrimSpace(request.Source), "node-unbind"),
		"clearedAt": now,
	})
	if clearErr != nil {
		response.ClearRegistrationStatus = "failed"
		response.ClearRegistrationMessage = clearErr.Error()
		_ = NodeRepo.NewRepository().UpdateLastError(ctx, clientID, fmt.Sprintf("clear node registration failed: %s", clearErr.Error()), time.Now().Unix())
		_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
			ClientID:      clientID,
			MainAccountID: accountID,
			ClientIP:      node.ClientIP,
			Action:        "clear_registration",
			Result:        "failed",
			Message:       clearErr.Error(),
			CreatedAt:     time.Now().Unix(),
		})
		return response, nil
	}

	response.ClearRegistrationStatus = "success"
	_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
		ClientID:      clientID,
		MainAccountID: accountID,
		ClientIP:      node.ClientIP,
		Action:        "clear_registration",
		Result:        "success",
		Message:       "clear node registration success",
		CreatedAt:     time.Now().Unix(),
	})
	return response, nil
}

func (s *Service) PushClientID(ctx context.Context, clientID string, request BindModel.PushClientIDRequest) error {
	node, err := NodeRepo.NewRepository().GetByClientID(ctx, clientID)
	if err != nil {
		return err
	}
	err = EdgeClientService.New().AssignClientID(ctx, node.BaseURL, strings.TrimSpace(Settings.Conf.EdgeConfig.APIKey), request)
	now := time.Now().Unix()
	result := "success"
	message := "push success"
	if err != nil {
		result = "failed"
		message = err.Error()
	}
	_ = BindRepo.NewRepository().CreateLog(ctx, &BindDAO.LogRow{
		ClientID:      clientID,
		MainAccountID: request.AccountID,
		ClientIP:      node.ClientIP,
		Action:        "push_client_id",
		Result:        result,
		Message:       message,
		CreatedAt:     now,
	})
	return err
}

func hashAPIKey(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
