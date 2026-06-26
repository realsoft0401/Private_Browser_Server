package Discovery

import (
	"sync"
	"time"
)

// DiscoveredClient 表示第一阶段临时 discovered 视图。
//
// 设计来源：
// - 你已经明确拍板 discovered 不单独落正式表；
// - 但前端和联调仍然需要一个“当前看到了哪些待绑定 Client”的视图；
// - 因此第一阶段先用内存缓存承载 discovered 过程，不把它升级成正式中心实体。
type DiscoveredClient struct {
	DiscoveryID     string `json:"discoveryId"`
	ClientID        string `json:"clientId"`
	AccountID       string `json:"accountId"`
	Status          string `json:"status"`
	ClientIP        string `json:"clientIp"`
	BaseURL         string `json:"baseUrl"`
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	CPUCores        int64  `json:"cpuCores"`
	MemoryTotalMB   int64  `json:"memoryTotalMb"`
	DockerAPIURL    string `json:"dockerApiUrl"`
	DockerVersion   string `json:"dockerVersion"`
	HealthStatus    string `json:"healthStatus"`
	DiscoveredAt    int64  `json:"discoveredAt"`
	LastHeartbeatAt int64  `json:"lastHeartbeatAt"`
}

var (
	discoveredMu      sync.RWMutex
	discoveredClients = make(map[string]DiscoveredClient)
)

func List() []DiscoveredClient {
	pruneExpired()
	discoveredMu.RLock()
	defer discoveredMu.RUnlock()
	result := make([]DiscoveredClient, 0, len(discoveredClients))
	for _, item := range discoveredClients {
		result = append(result, item)
	}
	return result
}

func Upsert(item DiscoveredClient) {
	discoveredMu.Lock()
	defer discoveredMu.Unlock()
	if item.DiscoveredAt <= 0 {
		item.DiscoveredAt = time.Now().Unix()
	}
	if item.Status == "" {
		item.Status = "discovered"
	}
	if existing, ok := discoveredClients[item.ClientIP]; ok {
		item = mergeDiscoveredClient(existing, item)
	}
	discoveredClients[item.ClientIP] = item
}

func RemoveByClientIP(clientIP string) {
	discoveredMu.Lock()
	defer discoveredMu.Unlock()
	delete(discoveredClients, clientIP)
}

func pruneExpired() {
	discoveredMu.Lock()
	defer discoveredMu.Unlock()
	now := time.Now().Unix()
	for clientIP, item := range discoveredClients {
		lastSeenAt := item.LastHeartbeatAt
		if lastSeenAt <= 0 {
			lastSeenAt = item.DiscoveredAt
		}
		if lastSeenAt <= 0 {
			continue
		}
		if now-lastSeenAt > 120 {
			delete(discoveredClients, clientIP)
		}
	}
}

func mergeDiscoveredClient(existing, incoming DiscoveredClient) DiscoveredClient {
	if incoming.DiscoveryID == "" {
		incoming.DiscoveryID = existing.DiscoveryID
	}
	if incoming.ClientID == "" {
		incoming.ClientID = existing.ClientID
	}
	if incoming.AccountID == "" {
		incoming.AccountID = existing.AccountID
	}
	if incoming.BaseURL == "" {
		incoming.BaseURL = existing.BaseURL
	}
	if incoming.Hostname == "" {
		incoming.Hostname = existing.Hostname
	}
	if incoming.OS == "" {
		incoming.OS = existing.OS
	}
	if incoming.Arch == "" {
		incoming.Arch = existing.Arch
	}
	if incoming.CPUCores == 0 {
		incoming.CPUCores = existing.CPUCores
	}
	if incoming.MemoryTotalMB == 0 {
		incoming.MemoryTotalMB = existing.MemoryTotalMB
	}
	if incoming.DockerAPIURL == "" {
		incoming.DockerAPIURL = existing.DockerAPIURL
	}
	if incoming.DockerVersion == "" {
		incoming.DockerVersion = existing.DockerVersion
	}
	if incoming.HealthStatus == "" {
		incoming.HealthStatus = existing.HealthStatus
	}
	if incoming.DiscoveredAt <= 0 || (existing.DiscoveredAt > 0 && existing.DiscoveredAt < incoming.DiscoveredAt) {
		incoming.DiscoveredAt = existing.DiscoveredAt
	}
	if incoming.LastHeartbeatAt <= 0 {
		incoming.LastHeartbeatAt = existing.LastHeartbeatAt
	}
	return incoming
}
