package Routes

import (
	"fmt"
	"path/filepath"

	"github.com/gin-gonic/gin"

	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	BindService "private_browser_server/Service/Bind"
	HealthService "private_browser_server/Service/Health"
	NodeService "private_browser_server/Service/Node"
	"private_browser_server/Settings"
)

// Setup 注册新的第一阶段路由。
//
// 当前只挂三类入口：
// - 基础入口：`/`、`/health`
// - 文档入口：`/swagger`、`/openapi.yaml`
// - 节点主线：heartbeat、discovered、bind、push、list、detail
//
// 这里已经按当前定案去掉旧的 `/api/v1/server/*` 命名。
// 原因是 Server 现在就是中心控制面本身，继续保留 `/server` 只会把正式 API 和历史过渡命名混在一起。
func Setup() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/", func(c *gin.Context) {
		HttpResponse.ResponseSuccess(c, gin.H{
			"service": fmt.Sprintf("Private_Browser_Server RESTful service"),
			"version": Settings.Conf.Version,
			"mode":    Settings.Conf.Mode,
		})
	})

	r.GET("/health", func(c *gin.Context) {
		HttpResponse.ResponseSuccess(c, HealthService.BuildHealthResponse())
	})

	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.File(filepath.Join(Settings.Conf.ProjectRoot, "docs", "openapi.yaml"))
	})
	r.GET("/swagger", func(c *gin.Context) {
		c.File(filepath.Join(Settings.Conf.ProjectRoot, "public", "swagger.html"))
	})
	r.GET("/swagger/", func(c *gin.Context) {
		c.File(filepath.Join(Settings.Conf.ProjectRoot, "public", "swagger.html"))
	})

	apiV1 := r.Group("/api/v1")
	edgeClients := apiV1.Group("/edge-clients")
	edgeClients.POST("/heartbeat", NodeService.ReceiveHeartbeat)
	edgeClients.GET("/discovered", NodeService.ListDiscovered)
	edgeClients.POST("/bind", BindService.BindClient)
	edgeClients.POST("/:clientId/unbind", BindService.UnbindClient)
	edgeClients.POST("/:clientId/push-client-id", BindService.PushClientID)
	edgeClients.GET("", NodeService.ListBoundClients)
	edgeClients.GET("/:clientId", NodeService.GetBoundClient)

	_ = NodeModel.EdgeClient{}
	return r
}
