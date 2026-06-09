package Auth

import (
	"private_browser_server/Pkg/HttpResponse"

	"github.com/gin-gonic/gin"
)

// RegisterUser 是 Server V1 Auth 注册入口。
//
// 当前先返回规划中，下一步实现时必须完成密码哈希、雪花 ID 和用户状态校验。
func RegisterUser(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "Auth 注册接口已规划，下一阶段接入 users 表、密码哈希和 JWT")
}

// LoginUser 是 Server V1 Auth 登录入口。
//
// JWT 只能由 Server 签发，不能下沉到 Client。
func LoginUser(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "Auth 登录接口已规划，下一阶段接入密码校验和 JWT 签发")
}

// CurrentUser 返回当前登录用户信息。
func CurrentUser(ctx *gin.Context) {
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotImplemented, "Auth me 接口已规划，下一阶段接入 JWT 解析和 RBAC")
}
