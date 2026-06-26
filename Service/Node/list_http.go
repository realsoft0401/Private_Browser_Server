package Node

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
)

func ListBoundClients(ctx *gin.Context) {
	accountID := strings.TrimSpace(ctx.Query("accountId"))
	if accountID == "" {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "accountId 不能为空")
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()
	items, err := NodeRepo.NewRepository().ListByAccountID(requestCtx, accountID)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, gin.H{
		"items": items,
		"total": len(items),
	})
}
