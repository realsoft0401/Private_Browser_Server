package PlatformContext

import (
	"strings"

	"github.com/gin-gonic/gin"

	"private_browser_server/Pkg/HttpResponse"
)

const (
	headerMainAccountID  = "X-Main-Account-Id"
	headerPlatformUserID = "X-Platform-User-Id"
	headerUsername       = "X-Platform-Username"
	headerRole           = "X-Platform-Role"

	contextKey = "platformContext"
)

// Context 是 PlatformServer 传给 Node Server 的商业操作上下文。
//
// 设计来源：V1 demo 为了先跑通整体商业链路，商业前端登录 PlatformServer 后，
// 再把主账号、操作者和角色通过内网 Header 传给 Node Server。
// 它不是最终安全鉴权方案；后续 verify-token 或 mTLS 接入时，可以保留这个结构作为业务上下文载体。
type Context struct {
	MainAccountID string `json:"mainAccountId"`
	UserID        string `json:"userId"`
	Username      string `json:"username"`
	Role          string `json:"role"`
}

// Middleware 校验并注入 Platform 操作上下文。
//
// 当前只保护 Node/Env/Task/Dashboard 等商业 API，不保护 /health、Swagger 和文档。
// 缺少 mainAccountId 或 userId 时直接拒绝，避免环境包、节点和任务写入无归属数据。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := Context{
			MainAccountID: strings.TrimSpace(c.GetHeader(headerMainAccountID)),
			UserID:        strings.TrimSpace(c.GetHeader(headerPlatformUserID)),
			Username:      strings.TrimSpace(c.GetHeader(headerUsername)),
			Role:          strings.TrimSpace(c.GetHeader(headerRole)),
		}
		if ctx.MainAccountID == "" || ctx.UserID == "" {
			HttpResponse.ResponseErrorWithMsg(c, HttpResponse.CodeUnauthorized,
				"缺少 Platform 操作上下文: 请由商业前端或 PlatformServer 调用 Node Server，并携带 X-Main-Account-Id 与 X-Platform-User-Id；V1 demo 暂不做 token 鉴权，但不能写入无归属数据。")
			c.Abort()
			return
		}
		c.Set(contextKey, ctx)
		c.Next()
	}
}

// FromGin 从 gin.Context 中读取标准化后的 Platform 上下文。
//
// Service 层应使用这个函数获取归属信息，不要重复解析 Header；
// 这样后续切换到 verify-token 后，只需要修改中间件，不需要重写业务接口。
func FromGin(c *gin.Context) (Context, bool) {
	value, ok := c.Get(contextKey)
	if !ok {
		return Context{}, false
	}
	ctx, ok := value.(Context)
	return ctx, ok
}
