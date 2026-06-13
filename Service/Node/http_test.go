package Node

import (
	"testing"

	nodeModel "private_browser_server/Models/Node"
	"private_browser_server/Settings"
)

// TestValidateHeartbeatRequest 保证正式 heartbeat 继续沿用 discovery 域校验。
//
// 这个测试的目的不是覆盖数据库，而是防止后续维护时把 discoveryMagic / group / protocolVersion
// 这些平台识别字段去掉，导致 Node Server 把任意内网请求误认成 Edge Client 心跳。
func TestValidateHeartbeatRequest(t *testing.T) {
	original := Settings.Conf.DiscoveryConfig
	Settings.Conf.DiscoveryConfig = &Settings.DiscoveryConfig{
		Magic:           "PRIVATE_BROWSER_CLIENT_DISCOVERY",
		ProtocolVersion: 1,
		Group:           "default",
	}
	t.Cleanup(func() {
		Settings.Conf.DiscoveryConfig = original
	})

	req := &heartbeatRequest{
		DiscoveryMagic:  "PRIVATE_BROWSER_CLIENT_DISCOVERY",
		ProtocolVersion: 1,
		Service:         "Private_Browser_Client",
		DiscoveryGroup:  "default",
		BaseURL:         "http://192.168.10.119:3300/",
	}
	if err := validateHeartbeatRequest(req); err != nil {
		t.Fatalf("validateHeartbeatRequest returned error: %v", err)
	}
	if req.BaseURL != "http://192.168.10.119:3300" {
		t.Fatalf("baseURL should be normalized, got %s", req.BaseURL)
	}
}

// TestAttachHeartbeatStatusUsesReceivedAt 保证 heartbeatStatus 只根据服务端接收时间判断。
//
// 用户已经明确要求把 Client 自报时间和服务端接收时间拆开：
// - last_heartbeat_at 用于 online/stale/offline
// - last_heartbeat_reported_at 只做辅助排障
// 这里用纯函数测试锁住这个边界，避免后续维护时又把两者混回去。
func TestAttachHeartbeatStatusUsesReceivedAt(t *testing.T) {
	original := Settings.Conf.DiscoveryConfig
	Settings.Conf.DiscoveryConfig = &Settings.DiscoveryConfig{
		StaleAfterSeconds:   30,
		OfflineAfterSeconds: 90,
	}
	t.Cleanup(func() {
		Settings.Conf.DiscoveryConfig = original
	})

	node := &nodeModel.EdgeClient{
		LastHeartbeatAt:         100,
		LastHeartbeatReportedAt: 1000,
	}

	attachHeartbeatStatus(node, 120)
	if node.HeartbeatStatus != nodeModel.NodeHeartbeatOnline {
		t.Fatalf("expected online, got %s", node.HeartbeatStatus)
	}

	attachHeartbeatStatus(node, 150)
	if node.HeartbeatStatus != nodeModel.NodeHeartbeatStale {
		t.Fatalf("expected stale, got %s", node.HeartbeatStatus)
	}

	attachHeartbeatStatus(node, 200)
	if node.HeartbeatStatus != nodeModel.NodeHeartbeatOffline {
		t.Fatalf("expected offline, got %s", node.HeartbeatStatus)
	}
}

// TestDiscoveryIdentityBlock 保证 discoveryReason 命中的节点不会被普通 verify/业务放行当成一般健康异常。
//
// 这个测试锁住当前收敛口径：
// - discoveryStatus=blocked + discoveryReason=ip_mismatch 必须单独阻断；
// - discoveryReason=ip_mismatch 也要给出明确提示；
// - 不能只依赖 lastError 文案模糊表达，否则后续很容易又被 verify 自动恢复。
func TestDiscoveryIdentityBlock(t *testing.T) {
	node := &nodeModel.EdgeClient{
		DiscoveryStatus: nodeModel.NodeDiscoveryBlocked,
		DiscoveryReason: nodeModel.NodeDiscoveryReasonIPMismatch,
	}

	blocked, message, nextAction := discoveryIdentityBlock(node)
	if !blocked {
		t.Fatal("expected discoveryIdentityBlock to block blocked node with discoveryReason")
	}
	if message == "" || nextAction == "" {
		t.Fatalf("expected non-empty message and nextAction, got message=%q nextAction=%q", message, nextAction)
	}
}

// TestDiscoveryIdentityBlockAllowsBlockedWithoutReason 保证“尚未 verify”的 blocked 节点不会被误判成身份异常。
//
// 收口后的 blocked 承担两种设备阶段：
// - discoveryReason 为空：只是未验证，还可以继续 verify；
// - discoveryReason 非空：身份连续性异常，必须人工确认。
func TestDiscoveryIdentityBlockAllowsBlockedWithoutReason(t *testing.T) {
	node := &nodeModel.EdgeClient{
		DiscoveryStatus: nodeModel.NodeDiscoveryBlocked,
	}

	blocked, message, nextAction := discoveryIdentityBlock(node)
	if blocked {
		t.Fatalf("expected blocked-without-reason to pass, got message=%q nextAction=%q", message, nextAction)
	}
}

// TestValidateAddressConfirmPrecondition 保证地址确认接口只接受 blocked + ip_mismatch。
//
// 这条规则是当前收口的关键边界：
// - 普通 blocked 节点还应继续走 verify；
// - 只有明确 ip_mismatch 的节点，才允许管理员确认地址更新。
func TestValidateAddressConfirmPrecondition(t *testing.T) {
	cases := []struct {
		name    string
		node    *nodeModel.EdgeClient
		wantErr bool
	}{
		{
			name: "blocked ip mismatch allowed",
			node: &nodeModel.EdgeClient{
				DiscoveryStatus: nodeModel.NodeDiscoveryBlocked,
				DiscoveryReason: nodeModel.NodeDiscoveryReasonIPMismatch,
			},
		},
		{
			name: "verified denied",
			node: &nodeModel.EdgeClient{
				DiscoveryStatus: nodeModel.NodeDiscoveryVerified,
			},
			wantErr: true,
		},
		{
			name: "blocked empty reason denied",
			node: &nodeModel.EdgeClient{
				DiscoveryStatus: nodeModel.NodeDiscoveryBlocked,
			},
			wantErr: true,
		},
		{
			name: "blocked device fact changed denied",
			node: &nodeModel.EdgeClient{
				DiscoveryStatus: nodeModel.NodeDiscoveryBlocked,
				DiscoveryReason: nodeModel.NodeDiscoveryReasonDeviceFactChanged,
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateAddressConfirmPrecondition(tc.node)
			if tc.wantErr && got == "" {
				t.Fatal("expected precondition error, got empty")
			}
			if !tc.wantErr && got != "" {
				t.Fatalf("expected no precondition error, got %q", got)
			}
		})
	}
}

// TestDetectDeviceFactChangeAfterAddressConfirm 保证地址确认后的宿主事实变化会继续阻断节点。
//
// 当前实现先用最稳定的宿主事实做最小保护，避免管理员把另一台机器错绑到旧 clientId。
func TestDetectDeviceFactChangeAfterAddressConfirm(t *testing.T) {
	before := &nodeModel.EdgeClient{
		OS:            "Ubuntu 24.04.4 LTS",
		Arch:          nodeModel.NodeArchAMD64,
		CPUCores:      4,
		MemoryTotalMB: 31815,
	}
	after := &nodeModel.EdgeClient{
		OS:            "Debian 12",
		Arch:          nodeModel.NodeArchAMD64,
		CPUCores:      4,
		MemoryTotalMB: 31815,
	}

	changed, detail := detectDeviceFactChangeAfterAddressConfirm(before, after)
	if !changed {
		t.Fatal("expected device fact change to be detected")
	}
	if detail == "" {
		t.Fatal("expected non-empty detail for device fact change")
	}
}

// TestAttachNodeGovernanceActions 保证后端返回的动作建议与当前收口后的状态机一致。
//
// 这样即使前端页面还没落地，后端也会持续输出稳定的按钮语义，避免未来再回到“按文案猜状态”。
func TestAttachNodeGovernanceActions(t *testing.T) {
	cases := []struct {
		name        string
		node        *nodeModel.EdgeClient
		wantPrimary string
		wantCount   int
	}{
		{
			name: "blocked empty reason",
			node: &nodeModel.EdgeClient{
				DiscoveryStatus: nodeModel.NodeDiscoveryBlocked,
			},
			wantPrimary: nodeActionVerify,
			wantCount:   2,
		},
		{
			name: "blocked ip mismatch",
			node: &nodeModel.EdgeClient{
				DiscoveryStatus: nodeModel.NodeDiscoveryBlocked,
				DiscoveryReason: nodeModel.NodeDiscoveryReasonIPMismatch,
			},
			wantPrimary: nodeActionConfirmAddressUpdate,
			wantCount:   1,
		},
		{
			name: "blocked device fact changed",
			node: &nodeModel.EdgeClient{
				DiscoveryStatus: nodeModel.NodeDiscoveryBlocked,
				DiscoveryReason: nodeModel.NodeDiscoveryReasonDeviceFactChanged,
			},
			wantPrimary: "",
			wantCount:   0,
		},
		{
			name: "verified",
			node: &nodeModel.EdgeClient{
				DiscoveryStatus: nodeModel.NodeDiscoveryVerified,
			},
			wantPrimary: "",
			wantCount:   0,
		},
		{
			name: "blocked unknown reason only refresh",
			node: &nodeModel.EdgeClient{
				DiscoveryStatus: nodeModel.NodeDiscoveryBlocked,
				DiscoveryReason: "legacy_reason",
			},
			wantPrimary: "",
			wantCount:   1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attachNodeGovernanceActions(tc.node)
			if tc.node.PrimaryAction != tc.wantPrimary {
				t.Fatalf("expected primary action %q, got %q", tc.wantPrimary, tc.node.PrimaryAction)
			}
			if len(tc.node.AllowedActions) != tc.wantCount {
				t.Fatalf("expected %d allowed actions, got %d", tc.wantCount, len(tc.node.AllowedActions))
			}
			if tc.name == "blocked unknown reason only refresh" && tc.node.AllowedActions[0] != nodeActionRefreshDeviceInfo {
				t.Fatalf("expected only %q, got %v", nodeActionRefreshDeviceInfo, tc.node.AllowedActions)
			}
		})
	}
}
