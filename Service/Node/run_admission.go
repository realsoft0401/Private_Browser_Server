package Node

import (
	"context"
	"strings"
	"time"

	NodeModel "private_browser_server/Models/Node"
	QuotaModel "private_browser_server/Models/Quota"
	NodeRepo "private_browser_server/Repository/Node"
	QuotaRepo "private_browser_server/Repository/Quota"
)

// AdmissionSnapshot 把一次 run admission 依赖的中心快照打包返回给调用方。
//
// 设计来源：
// - browser-env run、管理员诊断接口、后续平台调度都要复用同一套准入判断；
// - 但这些调用方不只需要 allowed/blocked，还经常需要节点摘要和额度快照本身；
// - 因此这里把 node/quota/result 一次性收口，避免上层自己再分别查表、再各写一遍拼装逻辑。
type AdmissionSnapshot struct {
	Node   *NodeModel.EdgeClient
	Quota  *QuotaModel.ClientRunQuota
	Result QuotaModel.AdmissionResult
}

// evaluateRunAdmission 统一收口“当前中心口径下，这个节点是否允许进入 run”。
//
// 设计来源：
// - 用户已经明确要求：run 不能只看 healthy + verified，还必须同时满足中心身份、平台额度和 slot 可用性；
// - 这套判断后续会被 browser-env run、管理员诊断接口和平台链路共同复用；
// - 因此这里单独抽成统一函数，避免路由层和未来业务层各写一套准入逻辑。
func evaluateRunAdmission(node *NodeModel.EdgeClient, quota *QuotaModel.ClientRunQuota) QuotaModel.AdmissionResult {
	result := QuotaModel.AdmissionResult{
		Allowed:    true,
		Status:     "allowed",
		Reasons:    make([]string, 0),
		Suggestion: "",
		CheckedAt:  time.Now().Unix(),
	}
	addBlock := func(reason string) {
		result.Allowed = false
		if result.Status != "blocked" {
			result.Status = "blocked"
		}
		result.Reasons = append(result.Reasons, reason)
	}

	if node == nil || strings.TrimSpace(node.ClientID) == "" {
		addBlock("missing_client_identity")
		result.Suggestion = "bind edge client first"
		return result
	}
	if node.HealthStatus != "healthy" {
		addBlock("client_not_healthy")
	}
	if node.DiscoveryStatus != "verified" {
		addBlock("client_not_verified")
	}
	if node.SlotExceptionStatus == "exception" {
		addBlock("slot_exception")
	}
	if node.AvailableSlotCount <= 0 {
		addBlock("no_available_slot")
	}

	if quota == nil {
		addBlock("missing_run_quota")
	} else {
		if quota.Status != "valid" {
			addBlock("run_quota_not_valid")
		}
		if quota.QuotaLimit <= 0 {
			addBlock("quota_limit_zero")
		}
		if quota.QuotaAvailableSnapshot <= 0 {
			addBlock("quota_exhausted")
		}
		if quota.ExpiresAt > 0 && quota.ExpiresAt <= result.CheckedAt {
			addBlock("run_quota_expired")
		}
	}

	if !result.Allowed {
		if len(result.Reasons) > 0 && result.Suggestion == "" {
			result.Suggestion = buildAdmissionSuggestion(result.Reasons)
		}
	}
	return result
}

func buildAdmissionSuggestion(reasons []string) string {
	reasonSet := make(map[string]bool, len(reasons))
	for _, reason := range reasons {
		reasonSet[reason] = true
	}
	switch {
	case reasonSet["missing_client_identity"]:
		return "bind edge client first"
	case reasonSet["client_not_healthy"] || reasonSet["client_not_verified"]:
		return "repair node health or run recheck first"
	case reasonSet["slot_exception"]:
		return "resolve slot exception and run slot-reconcile first"
	case reasonSet["no_available_slot"]:
		return "prepare an available slot before run"
	case reasonSet["missing_run_quota"], reasonSet["run_quota_not_valid"], reasonSet["run_quota_expired"], reasonSet["quota_limit_zero"], reasonSet["quota_exhausted"]:
		return "refresh platform run quota first"
	default:
		return "fix run admission blockers first"
	}
}

func loadNodeAndQuota(ctx context.Context, clientID string) (*NodeModel.EdgeClient, *QuotaModel.ClientRunQuota, error) {
	node, err := NodeRepo.NewRepository().GetByClientID(ctx, clientID)
	if err != nil {
		return nil, nil, err
	}
	quota, err := QuotaRepo.NewRepository().GetByClientID(ctx, clientID)
	if err == QuotaRepo.ErrNotFound {
		return node, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return node, quota, nil
}

// CheckRunAdmission 对外暴露统一的中心 run admission 入口。
//
// 职责边界：
// - 这里只读取当前中心缓存并应用统一准入规则；
// - 不直接发起 browser-env run，不直接改 slot，也不刷新平台 quota；
// - 如果后续 run 主链、预检接口、平台调度要看准入结果，都应该先走这里，避免逻辑漂移。
func CheckRunAdmission(ctx context.Context, clientID string) (*AdmissionSnapshot, error) {
	node, quota, err := loadNodeAndQuota(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return &AdmissionSnapshot{
		Node:   node,
		Quota:  quota,
		Result: evaluateRunAdmission(node, quota),
	}, nil
}
