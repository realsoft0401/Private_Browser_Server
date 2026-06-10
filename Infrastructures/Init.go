package Infrastructures

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"private_browser_server/Rom"
	"private_browser_server/Routes"
	DiscoveryService "private_browser_server/Service/Discovery"
	"private_browser_server/Settings"
)

type serverOptions struct {
	host         string
	port         int
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// Init 统一完成 Server 基础设施初始化和 HTTP 服务启动。
//
// 设计边界：
// - 这里负责配置、数据库占位初始化、路由和优雅退出；
// - 不在这里写用户认证、节点调度、Edge 调用等业务规则；
// - Server V1 使用 SQLite 作为本地控制面数据库，后续迁移也应优先扩展 Rom.Init，而不是让各 Service 自己连库。
func Init(projectRoot string) error {
	if err := initDependencies(projectRoot); err != nil {
		return err
	}
	DiscoveryService.StartListener()
	defer Rom.Close()
	defer DiscoveryService.StopListener()

	options := buildServerOptions()
	server := newHTTPServer(options)
	startHTTPServer(server, options)
	waitForShutdownSignal()
	return shutdownHTTPServer(server)
}

// initDependencies 初始化 Server 运行依赖。
//
// 当前阶段在 Rom.Init 内完成 SQLite 打开和基础迁移。
// Node Server 后续新增表或切换连接策略时，应继续保持“基础设施统一初始化、业务层只调用 Repository”的边界。
func initDependencies(projectRoot string) error {
	if err := Settings.Init(projectRoot); err != nil {
		return fmt.Errorf("init settings failed: %w", err)
	}
	if err := Rom.Init(); err != nil {
		return fmt.Errorf("init rom failed: %w", err)
	}
	return nil
}

// buildServerOptions 将配置里的监听参数归一化。
//
// Node Server 默认监听 3400，端口规划为 Client=3300、Node Server=3400、PlatformServer=3500。
func buildServerOptions() serverOptions {
	options := serverOptions{
		host:         Settings.Conf.ServerConfig.Host,
		port:         Settings.Conf.ServerConfig.Port,
		readTimeout:  time.Duration(Settings.Conf.ServerConfig.ReadTimeoutSeconds) * time.Second,
		writeTimeout: time.Duration(Settings.Conf.ServerConfig.WriteTimeoutSeconds) * time.Second,
	}
	if options.host == "" {
		options.host = "0.0.0.0"
	}
	if options.port <= 0 {
		options.port = 3400
	}
	if options.readTimeout <= 0 {
		options.readTimeout = 15 * time.Second
	}
	if options.writeTimeout <= 0 {
		options.writeTimeout = 15 * time.Second
	}
	return options
}

// newHTTPServer 创建标准 HTTP Server。
//
// 路由统一来自 Routes.Setup，避免在基础设施层散写接口。
func newHTTPServer(options serverOptions) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf("%s:%d", options.host, options.port),
		Handler:           Routes.Setup(),
		ReadHeaderTimeout: options.readTimeout,
		WriteTimeout:      options.writeTimeout,
	}
}

// startHTTPServer 异步启动中心服务。
//
// Server 不沿用 Client 开发期自动 kill 端口逻辑，避免中心服务在部署环境误杀其他进程。
func startHTTPServer(server *http.Server, options serverOptions) {
	go func() {
		log.Printf("Private_Browser_Server RESTful service listening on http://%s:%d\n", options.host, options.port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen failed: %v", err)
		}
	}()
}

// waitForShutdownSignal 阻塞等待退出信号。
func waitForShutdownSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

// shutdownHTTPServer 执行优雅关机。
func shutdownHTTPServer(server *http.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server failed: %w", err)
	}
	return nil
}
