package Node

import (
	"testing"

	nodeModel "private_browser_server/Models/Node"
)

func TestFillNodeClientIPIfMissingPrefersCandidate(t *testing.T) {
	node := &nodeModel.EdgeClient{BaseURL: "http://192.168.10.119:3300"}
	fillNodeClientIPIfMissing(node, "192.168.10.120")
	if node.ClientIP != "192.168.10.120" {
		t.Fatalf("expected candidate ip to win, got %q", node.ClientIP)
	}
}

func TestFillNodeClientIPIfMissingFallsBackToBaseURL(t *testing.T) {
	node := &nodeModel.EdgeClient{BaseURL: "http://192.168.10.119:3300"}
	fillNodeClientIPIfMissing(node, "")
	if node.ClientIP != "192.168.10.119" {
		t.Fatalf("expected baseUrl ip fallback, got %q", node.ClientIP)
	}
}

func TestFillNodeClientIPIfMissingDoesNotOverrideExisting(t *testing.T) {
	node := &nodeModel.EdgeClient{
		BaseURL:  "http://192.168.10.119:3300",
		ClientIP: "192.168.10.119",
	}
	fillNodeClientIPIfMissing(node, "192.168.10.120")
	if node.ClientIP != "192.168.10.119" {
		t.Fatalf("expected existing client_ip to remain unchanged, got %q", node.ClientIP)
	}
}
