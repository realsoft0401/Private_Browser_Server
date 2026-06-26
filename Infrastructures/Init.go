package Infrastructures

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	SQLiteInfra "private_browser_server/Infrastructures/SQLite"
	"private_browser_server/Routes"
	DiscoveryService "private_browser_server/Service/Discovery"
	NodeService "private_browser_server/Service/Node"
	"private_browser_server/Settings"
)

type serverOptions struct {
	host         string
	port         int
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// Init 统一完成新的第一阶段基础设施初始化和 HTTP 服务启动。
//
// 设计边界：
// - 这里只负责配置、SQLite、路由和优雅退出；
// - bind / push / discovery 的业务判断都不写在这里；
// - 后续扩第一阶段实现时，优先扩 Service / Repository，不要让基础设施层膨胀成业务层。
func Init(projectRoot string) error {
	if err := initDependencies(projectRoot); err != nil {
		return err
	}

	options := buildServerOptions()
	if err := preflightPorts(options); err != nil {
		return err
	}
	server := newHTTPServer(options)
	DiscoveryService.StartListener()
	NodeService.StartHealthMonitor()
	if err := startHTTPServer(server, options); err != nil {
		DiscoveryService.StopListener()
		NodeService.StopHealthMonitor()
		return err
	}
	waitForShutdownSignal()
	return shutdownHTTPServer(server)
}

func initDependencies(projectRoot string) error {
	if err := Settings.Init(projectRoot); err != nil {
		return fmt.Errorf("init settings failed: %w", err)
	}
	if err := SQLiteInfra.Init(); err != nil {
		return fmt.Errorf("init sqlite failed: %w", err)
	}
	return nil
}

func buildServerOptions() serverOptions {
	return serverOptions{
		host:         Settings.Conf.ServerConfig.Host,
		port:         Settings.Conf.ServerConfig.Port,
		readTimeout:  time.Duration(Settings.Conf.ServerConfig.ReadTimeoutSeconds) * time.Second,
		writeTimeout: time.Duration(Settings.Conf.ServerConfig.WriteTimeoutSeconds) * time.Second,
	}
}

func newHTTPServer(options serverOptions) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf("%s:%d", options.host, options.port),
		Handler:           Routes.Setup(),
		ReadHeaderTimeout: options.readTimeout,
		WriteTimeout:      options.writeTimeout,
	}
}

// preflightPorts 在真正启动前先校验 Node 关键监听端口是否可用。
//
// 设计来源：
// - 你这次实测已经踩到“43000/udp 被占用后仍继续走到 3400/tcp 启动”的半启动问题；
// - 对 Node 这种长期运行服务来说，端口占用应该在启动前一次性收口，而不是边启动边炸；
// - 因此这里先检查 TCP/UDP 两条正式入口，任一冲突都直接返回明确错误和修复建议。
func preflightPorts(options serverOptions) error {
	tcpAddr := fmt.Sprintf("%s:%d", options.host, options.port)
	if err := ensureTCPAddressAvailable(tcpAddr); err != nil {
		return fmt.Errorf("node server startup blocked: tcp %s is already in use; please stop the existing 3400 service first, then retry: %w", tcpAddr, err)
	}

	discoveryAddr := DiscoveryService.ResolveListenAddress(Settings.Conf.DiscoveryConfig)
	if Settings.Conf != nil && Settings.Conf.DiscoveryConfig != nil && Settings.Conf.DiscoveryConfig.Enabled {
		if err := ensureUDPAddressAvailable(discoveryAddr); err != nil {
			return fmt.Errorf("node server startup blocked: udp %s:%d is already in use; please stop the existing 43000 discovery listener first, then retry: %w", discoveryAddr.IP.String(), discoveryAddr.Port, err)
		}
	}
	return nil
}

func ensureTCPAddressAvailable(address string) error {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	return ln.Close()
}

func ensureUDPAddressAvailable(address *net.UDPAddr) error {
	if address == nil {
		return nil
	}
	conn, err := net.ListenUDP("udp4", address)
	if err != nil {
		return err
	}
	return conn.Close()
}

func startHTTPServer(server *http.Server, options serverOptions) error {
	if server == nil {
		return fmt.Errorf("http server 不能为空")
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("start http listener failed: %w", err)
	}

	go func() {
		log.Printf("Private_Browser_Server RESTful service listening on http://%s:%d\n", options.host, options.port)
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Printf("server serve stopped unexpectedly, err=%v\n", serveErr)
		}
	}()
	return nil
}

func waitForShutdownSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func shutdownHTTPServer(server *http.Server) error {
	DiscoveryService.StopListener()
	NodeService.StopHealthMonitor()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server failed: %w", err)
	}
	if err := SQLiteInfra.Close(); err != nil {
		return fmt.Errorf("close sqlite failed: %w", err)
	}
	return nil
}
