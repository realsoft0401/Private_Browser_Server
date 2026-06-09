package Env

import (
	"private_browser_server/Pkg/HttpResponse"

	"github.com/gin-gonic/gin"
)

// CreateEnv 由 Server 选择节点后代理 Edge 创建环境包。
//
// 当前只保留入口；实现时必须先确认节点 healthy 且 arch 不是 unknown，再通过 EdgeClient 调用 /api/v1/edge/browser-envs。
func CreateEnv(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "环境包创建接口已规划，下一阶段接入节点选择和 EdgeClient")
}

func ListEnvs(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "环境包列表接口已规划，下一阶段接入 server_browser_envs Repository")
}

func GetEnvDetail(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "环境包详情接口已规划，下一阶段聚合 Server 索引和 Edge 状态")
}

func RunEnv(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "环境包启动接口已规划，下一阶段创建 Server task 并代理 Edge run")
}

func StopEnv(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "环境包停止接口已规划，下一阶段创建 Server task 并代理 Edge stop")
}
