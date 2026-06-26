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

// ResolveListenAddress 返回当前 UDP discovery 应监听的标准地址。
//
// 设计来源：
// - Server 启动前需要先做端口可用性预检查，避免出现“UDP 报错、HTTP 又起一半”的半启动状态；
// - 因此把监听地址解析单独抽出来，让预检查和正式监听共用同一套地址口径；
// - 这样后续 discovery 配置变更时，也不会出现检查一套地址、实际监听另一套地址的漂移。
func ResolveListenAddress(config *Settings.DiscoveryConfig) *net.UDPAddr {
	addr := &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 43000}
	if config == nil {
		return addr
	}
	addr.Port = config.Port
	if addr.Port <= 0 {
		addr.Port = 43000
	}
	if ip := net.ParseIP(strings.TrimSpace(config.ListenAddress)); ip != nil {
		addr.IP = ip
	}
	return addr
}

// BeaconPayload 是 Node Server 监听到的 Client UDP discovery 最小报文。
//
// 设计来源：
// - 当前 Client 已经按独立内网模式持续发送 discovery beacon；
// - Server 不能只记 source ip，还要把 Client 自报的 baseUrl/hostname/capabilities 一并保留；
// - 但 discovered 只是发现线索，不是正式节点事实源，因此这里仍只接收非敏感摘要。
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

// Listener 管理 Node Server 对独立内网 UDP beacon 的监听循环。
//
// 职责边界：
// - 只负责接收 discovery 报文、校验平台识别字段、再转给 HTTP probe 收 discovery 视图；
// - 不直接落正式节点表，不自动生成 clientId，不自动升级为 verified；
// - UDP 只是发现线索，真正设备事实仍以 `/health` 和 `/api/v1/edge/device-info` 探测结果为准。
type Listener struct {
	config *Settings.DiscoveryConfig
	conn   *net.UDPConn
	stopCh chan struct{}
	doneCh chan struct{}
}

// StartListener 启动全局 UDP discovery listener。
//
// 这里保持和 Client Broadcaster 同样的全局单例口径，避免多次启动把同一个端口监听两次。
func StartListener() *Listener {
	listenerMu.Lock()
	defer listenerMu.Unlock()
	if listener != nil {
		return listener
	}

	instance := NewListener(Settings.Conf.DiscoveryConfig)
	listener = instance
	instance.Start()
	return instance
}

// StopListener 停止全局 UDP listener。
//
// Server 退出时先停监听，避免进程正在关闭但后台 goroutine 还继续读 UDP。
func StopListener() {
	listenerMu.Lock()
	instance := listener
	listener = nil
	listenerMu.Unlock()
	if instance != nil {
		instance.Stop()
	}
}

func NewListener(config *Settings.DiscoveryConfig) *Listener {
	return &Listener{
		config: config,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (l *Listener) Start() {
	if l == nil || l.config == nil || !l.config.Enabled {
		close(l.doneCh)
		return
	}

	addr := ResolveListenAddress(l.config)

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Printf("start udp discovery listener failed, addr=%s:%d, err=%v\n", addr.IP.String(), addr.Port, err)
		close(l.doneCh)
		return
	}
	l.conn = conn
	go l.loop()
}

func (l *Listener) Stop() {
	if l == nil {
		return
	}
	select {
	case <-l.doneCh:
		return
	default:
	}

	close(l.stopCh)
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
	buffer := make([]byte, 64<<10)

	for {
		if l.conn == nil {
			return
		}
		_ = l.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		size, remoteAddr, err := l.conn.ReadFromUDP(buffer)
		if err != nil {
			select {
			case <-l.stopCh:
				return
			default:
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("read udp discovery packet failed: %v\n", err)
			continue
		}
		l.handlePacket(buffer[:size], remoteAddr)
	}
}

func (l *Listener) handlePacket(packet []byte, remoteAddr *net.UDPAddr) {
	var payload BeaconPayload
	if err := json.Unmarshal(packet, &payload); err != nil {
		log.Printf("decode udp discovery payload failed, source=%s, err=%v\n", remoteAddr.String(), err)
		return
	}
	if err := ValidateBeaconPayload(&payload); err != nil {
		log.Printf("discard udp discovery payload, source=%s, err=%v\n", remoteAddr.String(), err)
		return
	}

	sourceIP := ""
	if remoteAddr != nil && remoteAddr.IP != nil {
		sourceIP = strings.TrimSpace(remoteAddr.IP.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	discovered, err := ObserveClient(ctx, ObserveRequest{
		Source:          "udp",
		SourceIP:        sourceIP,
		ClientIP:        payload.ClientIP,
		BaseURL:         payload.BaseURL,
		Hostname:        payload.Hostname,
		LastHeartbeatAt: payload.LastHeartbeatAt,
	})
	if err != nil {
		log.Printf("probe udp discovered client failed, source=%s, baseUrl=%s, err=%v\n", sourceIP, strings.TrimSpace(payload.BaseURL), err)
		return
	}
	Upsert(*discovered)
}

// ValidateBeaconPayload 校验当前 UDP 报文是否属于本平台发现域。
//
// 这里把判断单独抽出来，是为了让 UDP listener 和 HTTP heartbeat 复用同一套最小协议口径，
// 避免一个入口要求 `Private_Browser_Client`，另一个入口又要求 `private-browser-client`。
func ValidateBeaconPayload(payload *BeaconPayload) error {
	if payload == nil {
		return fmt.Errorf("payload 不能为空")
	}
	payload.DiscoveryMagic = strings.TrimSpace(payload.DiscoveryMagic)
	payload.Service = strings.TrimSpace(payload.Service)
	payload.DiscoveryGroup = strings.TrimSpace(payload.DiscoveryGroup)
	payload.ClientIP = strings.TrimSpace(payload.ClientIP)
	payload.BaseURL = strings.TrimSpace(payload.BaseURL)
	payload.Hostname = strings.TrimSpace(payload.Hostname)

	if payload.DiscoveryMagic != strings.TrimSpace(Settings.Conf.DiscoveryConfig.Magic) {
		return fmt.Errorf("discoveryMagic 不匹配当前 Node Server 发现域")
	}
	if payload.ProtocolVersion != Settings.Conf.DiscoveryConfig.ProtocolVersion {
		return fmt.Errorf("protocolVersion 不匹配当前 Node Server 配置")
	}
	if !isAllowedClientService(payload.Service) {
		return fmt.Errorf("service 必须是 Private_Browser_Client")
	}
	if payload.DiscoveryGroup != strings.TrimSpace(Settings.Conf.DiscoveryConfig.Group) {
		return fmt.Errorf("discoveryGroup 不匹配当前 Node Server 发现域")
	}
	if payload.BaseURL == "" && payload.ClientIP == "" {
		return fmt.Errorf("baseUrl 和 clientIp 不能同时为空")
	}
	return nil
}

func isAllowedClientService(service string) bool {
	switch strings.TrimSpace(service) {
	case "Private_Browser_Client", "private-browser-client":
		return true
	default:
		return false
	}
}
