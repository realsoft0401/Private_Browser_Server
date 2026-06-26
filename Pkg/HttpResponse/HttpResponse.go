package HttpResponse

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ResponseSuccess(ctx *gin.Context, data any) {
	ResponseWithStatus(ctx, http.StatusOK, data)
}

func ResponseWithStatus(ctx *gin.Context, status int, data any) {
	ctx.JSON(status, gin.H{
		"code":    CodeSuccess,
		"message": "success",
		"data":    data,
	})
}

func ResponseErrorWithMsg(ctx *gin.Context, code ResCode, message string) {
	ctx.JSON(http.StatusOK, gin.H{
		"code":    code,
		"message": message,
	})
}
