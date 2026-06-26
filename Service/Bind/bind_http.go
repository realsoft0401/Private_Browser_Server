package Bind

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	BindModel "private_browser_server/Models/Bind"
	"private_browser_server/Pkg/HttpResponse"
	"private_browser_server/Settings"
)

func BindClient(ctx *gin.Context) {
	var request BindModel.BindRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "bind request body 非法")
		return
	}
	request.NodeServerBaseURL = resolveNodeServerBaseURL(ctx, request.ClientIP)
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 20*time.Second)
	defer cancel()
	result, err := NewService().BindByAccountAndClientIP(requestCtx, request)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, result)
}

// resolveNodeServerBaseURL 负责给 bind/push 链路补齐 Node 当前可访问地址。
//
// 设计来源：
// - 最新链路已经明确：Client 启动时不知道 Node，只有 Node 发现并 bind 之后，才由 Node 把自己的控制地址写回 Client；
// - 因此这里不能再让 Client 自己猜 `127.0.0.1:3400`，而必须由 Node 在当前请求上下文里显式给出；
// - 当前先以本次 HTTP 请求看到的 Host 为准，保证局域网直连联调成立；后续如果前面挂统一网关，再在这里收口真实外显地址策略。
func resolveNodeServerBaseURL(ctx *gin.Context, clientIP string) string {
	scheme := "http"
	if ctx.Request.TLS != nil {
		scheme = "https"
	}
	port := Settings.Conf.ServerConfig.Port
	if port <= 0 {
		port = 3400
	}
	if localIP := detectOutboundLocalIP(strings.TrimSpace(clientIP)); localIP != "" {
		return fmt.Sprintf("%s://%s:%d", scheme, localIP, port)
	}
	if localIP := detectFirstLANIPv4(); localIP != "" {
		return fmt.Sprintf("%s://%s:%d", scheme, localIP, port)
	}
	host := strings.TrimSpace(ctx.Request.Host)
	if host == "" {
		return ""
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func detectOutboundLocalIP(targetIP string) string {
	if net.ParseIP(targetIP) == nil {
		return ""
	}
	conn, err := net.Dial("udp4", net.JoinHostPort(targetIP, "80"))
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return ""
	}
	ip := addr.IP.To4()
	if ip == nil || ip.IsLoopback() {
		return ""
	}
	return ip.String()
}

func detectFirstLANIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, addrErr := iface.Addrs()
		if addrErr != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			ip = ip.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

func UnbindClient(ctx *gin.Context) {
	clientID := ctx.Param("clientId")
	var request BindModel.UnbindRequest
	if err := ctx.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeInvalidParams, "unbind request body 非法")
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 20*time.Second)
	defer cancel()
	result, err := NewService().UnbindClient(requestCtx, clientID, request)
	if err != nil {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeRemoteError, err.Error())
		return
	}
	HttpResponse.ResponseSuccess(ctx, result)
}
