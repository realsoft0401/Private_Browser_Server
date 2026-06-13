package Discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"private_browser_server/Settings"
)

var (
	listenerMu sync.Mutex
	listener   *Listener
)

// BeaconPayload 是 Client UDP discovery 广播的非敏感摘要。
//
// 设计来源：Client 只广播服务入口和基础能力，Server 不能把 beacon 当成节点事实源。
// 这里的字段必须保持白名单：禁止加入用户、环境包状态、proxy 明文、fingerprint raw、Cookies 或 browser-data 路径。
type BeaconPayload struct {
	DiscoveryMagic  string   `json:"discoveryMagic"`
	ProtocolVersion int      `json:"protocolVersion"`
	Service         string   `json:"service"`
	DiscoveryGroup  string   `json:"discoveryGroup"`
	ClientIP        string   `json:"clientIp"`
	BaseURL         string   `json:"baseUrl"`
	Hostname        string   `json:"hostname"`
	Mode            string   `json:"mode"`
	Version         string   `json:"version"`
	StartedAt       int64    `json:"startedAt"`
	LastHeartbeatAt int64    `json:"lastHeartbeatAt"`
	Capabilities    []string `json:"capabilities"`
}

// DiscoveredClient 是 Server 侧缓存的发现线索。
//
// SourceIP 来自 UDP 包源地址，比 payload.clientIp 更接近网络事实；
// 但它仍不能直接作为 clientId 身份，后续必须通过 Client HTTP 探测确认 baseUrl/clientIp。
type DiscoveredClient struct {
	SourceIP      string        `json:"sourceIp"`
	SourcePort    int           `json:"sourcePort"`
	Payload       BeaconPayload `json:"payload"`
	FirstSeenAt   int64         `json:"firstSeenAt"`
	LastSeenAt    int64         `json:"lastSeenAt"`
	ReceiveCount  int64         `json:"receiveCount"`
	DiscardReason string        `json:"discardReason,omitempty"`
}

// Listener 管理 UDP discovery 监听和内存缓存。
//
// V1 只做测试和发现列表，不自动写 edge_clients；自动登记必须复用 Edge Client 注册、HTTP 探测、
// Docker 2375 探测和 blocked/ip_mismatch 状态机，不能藏在 UDP 收包循环里。
type Listener struct {
	config  *Settings.DiscoveryConfig
	conn    net.PacketConn
	stopCh  chan struct{}
	doneCh  chan struct{}
	mu      sync.RWMutex
	clients map[string]DiscoveredClient
}

// StartListener 启动全局 UDP discovery 监听器。
func StartListener() *Listener {
	listenerMu.Lock()
	defer listenerMu.Unlock()
	if listener != nil {
		return listener
	}
	l := NewListener(Settings.Conf.DiscoveryConfig)
	listener = l
	l.Start()
	return l
}

// StopListener 停止全局 UDP discovery 监听器。
func StopListener() {
	listenerMu.Lock()
	l := listener
	listener = nil
	listenerMu.Unlock()
	if l != nil {
		l.Stop()
	}
}

// Current 返回当前全局监听器，供 HTTP 查询接口读取缓存。
func Current() *Listener {
	listenerMu.Lock()
	defer listenerMu.Unlock()
	return listener
}

func NewListener(config *Settings.DiscoveryConfig) *Listener {
	return &Listener{
		config:  config,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		clients: make(map[string]DiscoveredClient),
	}
}

// Start 绑定 UDP 端口并启动收包循环。
//
// 如果端口占用或系统权限不足，Server 主服务仍可启动，管理员可以继续手动添加节点；
// 但日志会明确提示 discovery 不可用，避免误以为自动发现已经工作。
func (l *Listener) Start() {
	if l == nil || l.config == nil || !l.config.Enabled {
		close(l.doneCh)
		return
	}
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(l.config.ListenAddress), l.config.Port)
	conn, err := net.ListenPacket("udp4", addr)
	if err != nil {
		log.Printf("udp discovery listen failed, addr=%s, err=%v\n", addr, err)
		close(l.doneCh)
		return
	}
	l.conn = conn
	go l.loop()
	log.Printf("UDP discovery listening on %s\n", addr)
}

func (l *Listener) Stop() {
	if l == nil {
		return
	}
	if l.conn != nil {
		_ = l.conn.Close()
	}
	select {
	case <-l.doneCh:
	case <-time.After(3 * time.Second):
		log.Printf("udp discovery listener stop timeout\n")
	}
}

func (l *Listener) loop() {
	defer close(l.doneCh)
	bufferSize := l.config.MaxPacketBytes
	if bufferSize <= 0 {
		bufferSize = 8192
	}
	buffer := make([]byte, bufferSize)
	for {
		n, addr, err := l.conn.ReadFrom(buffer)
		if err != nil {
			select {
			case <-l.stopCh:
			default:
			}
			return
		}
		l.handlePacket(addr, buffer[:n])
	}
}

func (l *Listener) handlePacket(addr net.Addr, data []byte) {
	udpAddr, _ := addr.(*net.UDPAddr)
	sourceIP := ""
	sourcePort := 0
	if udpAddr != nil {
		sourceIP = udpAddr.IP.String()
		sourcePort = udpAddr.Port
	}

	var payload BeaconPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		l.recordDiscard(sourceIP, sourcePort, "invalid_json")
		return
	}
	if reason := l.validate(payload); reason != "" {
		l.recordDiscard(sourceIP, sourcePort, reason)
		return
	}

	key := sourceIP
	if key == "" {
		key = strings.TrimSpace(payload.BaseURL)
	}
	now := time.Now().Unix()
	l.mu.Lock()
	item := l.clients[key]
	if item.FirstSeenAt == 0 {
		item.FirstSeenAt = now
	}
	item.SourceIP = sourceIP
	item.SourcePort = sourcePort
	item.Payload = payload
	item.LastSeenAt = now
	item.ReceiveCount++
	item.DiscardReason = ""
	l.clients[key] = item
	l.mu.Unlock()
	syncRegisteredHeartbeat(sourceIP, payload, now)
}

func (l *Listener) validate(payload BeaconPayload) string {
	if strings.TrimSpace(payload.DiscoveryMagic) != strings.TrimSpace(l.config.Magic) {
		return "magic_mismatch"
	}
	if payload.ProtocolVersion != l.config.ProtocolVersion {
		return "protocol_mismatch"
	}
	service := strings.TrimSpace(payload.Service)
	if service != "private-browser-client" && service != "Private_Browser_Client" {
		return "service_mismatch"
	}
	if strings.TrimSpace(payload.DiscoveryGroup) != strings.TrimSpace(l.config.Group) {
		return "group_mismatch"
	}
	if strings.TrimSpace(payload.BaseURL) == "" {
		return "base_url_empty"
	}
	return ""
}

func (l *Listener) recordDiscard(sourceIP string, sourcePort int, reason string) {
	if sourceIP == "" {
		return
	}
	now := time.Now().Unix()
	key := "discard:" + sourceIP
	l.mu.Lock()
	item := l.clients[key]
	if item.FirstSeenAt == 0 {
		item.FirstSeenAt = now
	}
	item.SourceIP = sourceIP
	item.SourcePort = sourcePort
	item.LastSeenAt = now
	item.ReceiveCount++
	item.DiscardReason = reason
	l.clients[key] = item
	l.mu.Unlock()
}

// List 返回当前缓存的发现结果。
//
// 返回前复制一份数据，避免 HTTP 层持有内部锁或修改监听器缓存。
func (l *Listener) List(_ context.Context) []DiscoveredClient {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	items := make([]DiscoveredClient, 0, len(l.clients))
	for _, item := range l.clients {
		items = append(items, item)
	}
	return items
}
