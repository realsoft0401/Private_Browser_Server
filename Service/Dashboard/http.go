package Dashboard

import (
	"private_browser_server/Pkg/HttpResponse"

	"github.com/gin-gonic/gin"
)

// GetDashboard 返回 Server V1 最小统计。
//
// 当前只保留入口；实现时统计应来自 MySQL 聚合和 Edge 心跳摘要，不依赖 Server 内存临时状态。
func GetDashboard(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "Dashboard 接口已规划，下一阶段接入节点、环境包和任务聚合统计")
}
