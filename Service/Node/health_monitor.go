package Node

import (
	"context"
	"log"
	"sync"
	"time"

	NodeRepo "private_browser_server/Repository/Node"
	"private_browser_server/Settings"
)

var (
	healthMonitorMu sync.Mutex
	healthMonitor   *HealthMonitor
)

// HealthMonitor 负责把“长时间没有 heartbeat 的已知节点”统一收口成 offline。
//
// 设计来源：
// - 这次已经正式收口为 `healthy / offline` 两态；
// - Client 掉线后无法自己声明 `offline`，只能由 Node 按 heartbeat 超时来兜底判断；
// - 因此这里做成独立后台巡检，不把离线判断散落在 bind、discovery、HTTP handler 里各自猜。
type HealthMonitor struct {
	interval     time.Duration
	offlineAfter time.Duration
	stopCh       chan struct{}
	doneCh       chan struct{}
}

// StartHealthMonitor 启动全局节点在线状态巡检器。
//
// 这里使用和 UDP listener 一样的全局单例口径，避免重复启动多个定时器同时扫同一张表。
func StartHealthMonitor() *HealthMonitor {
	healthMonitorMu.Lock()
	defer healthMonitorMu.Unlock()
	if healthMonitor != nil {
		return healthMonitor
	}

	instance := NewHealthMonitor(
		time.Duration(Settings.Conf.NodeHealthConfig.MonitorIntervalSeconds)*time.Second,
		time.Duration(Settings.Conf.NodeHealthConfig.OfflineAfterSeconds)*time.Second,
	)
	healthMonitor = instance
	instance.Start()
	return instance
}

// StopHealthMonitor 在 Server 退出时停止在线状态巡检。
func StopHealthMonitor() {
	healthMonitorMu.Lock()
	instance := healthMonitor
	healthMonitor = nil
	healthMonitorMu.Unlock()
	if instance != nil {
		instance.Stop()
	}
}

func NewHealthMonitor(interval, offlineAfter time.Duration) *HealthMonitor {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if offlineAfter <= 0 {
		offlineAfter = 90 * time.Second
	}
	return &HealthMonitor{
		interval:     interval,
		offlineAfter: offlineAfter,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

func (m *HealthMonitor) Start() {
	if m == nil {
		return
	}
	go m.loop()
}

func (m *HealthMonitor) Stop() {
	if m == nil {
		return
	}
	select {
	case <-m.doneCh:
		return
	default:
	}

	close(m.stopCh)
	select {
	case <-m.doneCh:
	case <-time.After(3 * time.Second):
		log.Printf("node health monitor stop timeout\n")
	}
}

func (m *HealthMonitor) loop() {
	defer close(m.doneCh)

	m.runOnce()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runOnce()
		}
	}
}

func (m *HealthMonitor) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	affected, err := NodeRepo.NewRepository().MarkOfflineByHeartbeatTimeout(
		ctx,
		time.Now().Unix(),
		int64(m.offlineAfter/time.Second),
	)
	if err != nil {
		log.Printf("mark edge clients offline by heartbeat timeout failed: %v\n", err)
		return
	}
	if affected > 0 {
		log.Printf("node health monitor marked %d edge client(s) offline by heartbeat timeout\n", affected)
	}
}
