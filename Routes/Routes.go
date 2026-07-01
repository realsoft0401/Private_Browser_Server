package Routes

import (
	"fmt"
	"path/filepath"

	"github.com/gin-gonic/gin"

	NodeModel "private_browser_server/Models/Node"
	"private_browser_server/Pkg/HttpResponse"
	BindService "private_browser_server/Service/Bind"
	BrowserEnvService "private_browser_server/Service/BrowserEnv"
	HealthService "private_browser_server/Service/Health"
	NodeService "private_browser_server/Service/Node"
	TaskService "private_browser_server/Service/Task"
	"private_browser_server/Settings"
)

// Setup 注册当前 Node Server 对外暴露的正式路由。
//
// 当前这份路由表已经收口为 5 类正式入口：
// - 基础入口：`/`、`/health`
// - 文档入口：`/swagger`、`/openapi.yaml`
// - 节点治理：heartbeat、discovered、bind、recheck、confirm-address-update、slot、quota
// - browser-env 生命周期：query、refresh、run、stop、backup、restore、package、del
// - 中心任务观察：`/api/v1/server-tasks/*`
//
// 这里继续坚持去掉旧的 `/api/v1/server/*` 命名。
// 原因是 Server 现在就是中心控制面本身，继续保留 `/server` 只会把正式 API 和历史过渡命名混在一起，
// 后续 OpenAPI、前端 SDK 和回归文档也会更容易漂移。
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
	edgeClients.POST("/:clientId/recheck", NodeService.RecheckClient)
	edgeClients.POST("/:clientId/confirm-address-update", NodeService.ConfirmAddressUpdate)
	edgeClients.POST("/:clientId/push-client-id", BindService.PushClientID)
	edgeClients.POST("/:clientId/slot-reconcile", NodeService.SlotReconcile)
	edgeClients.POST("/:clientId/target-slot-count", NodeService.SetTargetSlotCount)
	edgeClients.GET("/:clientId/run-quota", NodeService.GetRunQuota)
	edgeClients.POST("/:clientId/run-quota/refresh", NodeService.RefreshRunQuota)
	edgeClients.GET("", NodeService.ListBoundClients)
	edgeClients.GET("/:clientId", NodeService.GetBoundClient)
	edgeClients.GET("/:clientId/slots", NodeService.ListClientSlots)
	serverTasks := apiV1.Group("/server-tasks")
	serverTasks.GET("/:taskId", TaskService.GetDetail)
	serverTasks.GET("/:taskId/events", TaskService.SubscribeEvents)
	browserEnvs := apiV1.Group("/browser-envs")
	browserEnvs.GET("", BrowserEnvService.List)
	browserEnvs.POST("", BrowserEnvService.Create)
	browserEnvs.POST("/import-package", BrowserEnvService.ImportPackage)
	browserEnvs.GET("/:envId", BrowserEnvService.GetDetail)
	browserEnvs.POST("/:envId/refresh", BrowserEnvService.Refresh)
	browserEnvs.POST("/:envId/run", BrowserEnvService.Run)
	browserEnvs.POST("/:envId/stop", BrowserEnvService.Stop)
	browserEnvs.PATCH("/:envId/runtime-image", BrowserEnvService.UpdateRuntimeImage)
	browserEnvs.POST("/:envId/backup", BrowserEnvService.Backup)
	browserEnvs.POST("/:envId/restore", BrowserEnvService.Restore)
	browserEnvs.DELETE("/:envId/del", BrowserEnvService.DeleteImage)
	browserEnvs.DELETE("/:envId/package", BrowserEnvService.DeletePackage)

	_ = NodeModel.EdgeClient{}
	return r
}
