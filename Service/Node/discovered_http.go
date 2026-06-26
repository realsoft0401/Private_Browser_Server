package Node

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"private_browser_server/Pkg/HttpResponse"
	DiscoveryService "private_browser_server/Service/Discovery"
)

func ListDiscovered(ctx *gin.Context) {
	HttpResponse.ResponseWithStatus(ctx, http.StatusOK, gin.H{
		"items": DiscoveryService.List(),
		"total": len(DiscoveryService.List()),
	})
}
