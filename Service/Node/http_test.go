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
