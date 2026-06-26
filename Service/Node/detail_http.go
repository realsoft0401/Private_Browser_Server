package Node

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"private_browser_server/Pkg/HttpResponse"
	NodeRepo "private_browser_server/Repository/Node"
)

func GetBoundClient(ctx *gin.Context) {
	clientID := ctx.Param("clientId")
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()
	node, err := NodeRepo.NewRepository().GetByClientID(requestCtx, clientID)
	if err == NodeRepo.ErrNotFound {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeNotFound, "edge client not found")
		return
	}
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInternalError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, node)
}
