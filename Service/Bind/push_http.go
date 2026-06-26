package Bind

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	BindModel "private_browser_server/Models/Bind"
	"private_browser_server/Pkg/HttpResponse"
)

func PushClientID(ctx *gin.Context) {
	clientID := ctx.Param("clientId")
	var request BindModel.PushClientIDRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "push request body 非法")
		return
	}
	request.ClientID = clientID
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 20*time.Second)
	defer cancel()
	if err := NewService().PushClientID(requestCtx, clientID, request); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, gin.H{
		"clientId":   clientID,
		"pushStatus": "success",
		"pushedAt":   time.Now().Unix(),
	})
}
