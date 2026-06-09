package Task

import (
	"private_browser_server/Pkg/HttpResponse"

	"github.com/gin-gonic/gin"
)

func ListTasks(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "任务列表接口已规划，下一阶段接入 tasks Repository")
}

func GetTask(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "任务详情接口已规划，下一阶段绑定 Server task 与 Edge task")
}

func StreamTaskEvents(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "任务 SSE 接口已规划，下一阶段代理或聚合 Edge 任务事件")
}
