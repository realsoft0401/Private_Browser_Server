package Routes

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"private_browser_server/Rom"
	AuthService "private_browser_server/Service/Auth"
	DashboardService "private_browser_server/Service/Dashboard"
	EnvService "private_browser_server/Service/Env"
	NodeService "private_browser_server/Service/Node"
	TaskService "private_browser_server/Service/Task"
	"private_browser_server/Settings"
)

// Setup 统一注册 Private_Browser_Server 的 HTTP 路由。
//
// Server 只注册中心能力：Auth、Node、Env 聚合、Task、Dashboard。
// Edge 本机 Docker、WebVNC 页面、/api/v1/edge/* 不应在这里实现。
func Setup() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, fmt.Sprintf("Private_Browser_Server RESTful service\nversion=%s\nmode=%s\n", Settings.Conf.Version, Settings.Conf.Mode))
	})
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"ok":         true,
			"service":    Settings.Conf.Name,
			"mode":       Settings.Conf.Mode,
			"version":    Settings.Conf.Version,
			"configFile": Settings.Conf.ConfigFile,
			"mysql": gin.H{
				"host":     Settings.Conf.MySQLConfig.Host,
				"port":     Settings.Conf.MySQLConfig.Port,
				"database": Settings.Conf.MySQLConfig.Database,
			},
			"romInitialized": Rom.IsInitialized(),
		})
	})
	registerDocs(r)

	apiV1 := r.Group("/api/v1")
	auth := apiV1.Group("/auth")
	auth.POST("/register", AuthService.RegisterUser)
	auth.POST("/login", AuthService.LoginUser)
	auth.GET("/me", AuthService.CurrentUser)

	nodes := apiV1.Group("/nodes")
	nodes.POST("/probe-docker", NodeService.ProbeDocker)
	nodes.POST("", NodeService.RegisterNode)
	nodes.GET("", NodeService.ListNodes)
	nodes.GET("/:id", NodeService.GetNodeDetail)
	nodes.POST("/:id/device-info/refresh", NodeService.RefreshNodeDeviceInfo)

	envs := apiV1.Group("/envs")
	envs.POST("", EnvService.CreateEnv)
	envs.GET("", EnvService.ListEnvs)
	envs.GET("/:envId", EnvService.GetEnvDetail)
	envs.POST("/:envId/run", EnvService.RunEnv)
	envs.POST("/:envId/stop", EnvService.StopEnv)

	server := apiV1.Group("/server")
	server.GET("/dashboard", DashboardService.GetDashboard)
	server.POST("/nodes/heartbeat", NodeService.ReceiveHeartbeat)
	server.GET("/tasks", TaskService.ListTasks)
	server.GET("/tasks/:taskId", TaskService.GetTask)
	server.GET("/tasks/:taskId/events", TaskService.StreamTaskEvents)

	return r
}

// registerDocs 挂载 OpenAPI 文档占位。
//
// 这里先保留 /openapi.yaml，后续接口实现时以 docs/openapi.yaml 作为 Apifox/Swagger 的事实来源。
func registerDocs(r *gin.Engine) {
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.File(filepath.Join(Settings.Conf.ProjectRoot, "docs", "openapi.yaml"))
	})
}
