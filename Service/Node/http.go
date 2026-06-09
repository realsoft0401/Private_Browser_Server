package Node

import (
	"private_browser_server/Pkg/HttpResponse"

	"github.com/gin-gonic/gin"
)

// ProbeDocker 通过 Docker Engine HTTP API 探测节点能力。
//
// 当前只保留入口；实现时应请求 _ping/info/version，并把架构归一化为 amd64/arm64/unknown。
func ProbeDocker(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "节点 Docker 探测接口已规划，下一阶段接入 Docker 2375 _ping/info/version")
}

// RegisterNode 注册 Edge 节点。
func RegisterNode(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "节点注册接口已规划，下一阶段生成设备号和 API Key")
}

// ListNodes 返回当前用户可见节点列表。
func ListNodes(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "节点列表接口已规划，下一阶段接入 control_nodes Repository")
}

// GetNodeDetail 返回节点详情。
func GetNodeDetail(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "节点详情接口已规划，下一阶段接入 control_nodes Repository")
}

// RefreshNodeDeviceInfo 重新探测并保存节点设备能力。
func RefreshNodeDeviceInfo(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "节点设备信息刷新接口已规划，下一阶段接入 Docker 2375 探测并落库")
}

// ReceiveHeartbeat 接收 Edge 心跳。
//
// 心跳只保存节点、Docker、环境包状态摘要，不接收 proxy 明文、fingerprint raw 或 browser-data。
func ReceiveHeartbeat(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "节点心跳接口已规划，下一阶段接入 API Key 校验和状态摘要落库")
}
