package Env

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	envDao "private_browser_server/Dao/Env"
	EdgeAPI "private_browser_server/EdgeClient"
	"private_browser_server/Pkg/HttpResponse"
	NodeService "private_browser_server/Service/Node"
)

type serviceError struct {
	code HttpResponse.ResCode
	msg  string
}

func (e *serviceError) Error() string { return e.msg }

func invalidError(msg string) error {
	return &serviceError{code: HttpResponse.CodeInvalidParams, msg: msg}
}

func conflictError(msg string) error { return &serviceError{code: HttpResponse.CodeConflict, msg: msg} }

func notFoundError(msg string) error { return &serviceError{code: HttpResponse.CodeNotFound, msg: msg} }

func remoteError(msg string) error {
	return &serviceError{code: HttpResponse.CodeRemoteError, msg: msg}
}

func internalError(msg string) error {
	return &serviceError{code: HttpResponse.CodeServerBusy, msg: msg}
}

func writeServiceError(ctx *gin.Context, err error) {
	var serviceErr *serviceError
	if errors.As(err, &serviceErr) {
		HttpResponse.ResponseErrorWithMsg(ctx, serviceErr.code, serviceErr.msg)
		return
	}
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, err.Error())
}

func mapDaoError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, envDao.ErrEnvNotFound) {
		return notFoundError("环境包不存在或不属于当前主账号")
	}
	if strings.Contains(err.Error(), "constraint failed") || strings.Contains(err.Error(), "UNIQUE") {
		return conflictError("中心环境包索引已存在，不能重复写入相同 envId")
	}
	return internalError("中心环境包索引写入或查询失败: " + err.Error())
}

func mapEdgeError(err error) error {
	return mapEdgeActionError("创建环境包", err)
}

func mapEdgeActionError(action string, err error) error {
	var edgeErr *EdgeAPI.EdgeError
	if errors.As(err, &edgeErr) {
		return remoteError("调用 Edge " + action + "失败: " + edgeErr.Error())
	}
	return remoteError("调用 Edge " + action + "失败: " + err.Error())
}

func respondClientNotReady(ctx *gin.Context, err error) {
	var readyErr *NodeService.BusinessReadyError
	if errors.As(err, &readyErr) {
		HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeConflict, readyErr.Error())
		return
	}
	HttpResponse.ResponseErrorWithMsg(ctx, HttpResponse.CodeServerBusy, "检查 Client 业务前置条件失败: "+err.Error())
}
