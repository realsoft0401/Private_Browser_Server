package HttpResponse

type ResCode int64

const (
	CodeSuccess        ResCode = 1000
	CodeInvalidParams  ResCode = 1001
	CodeNotFound       ResCode = 1002
	CodeConflict       ResCode = 1003
	CodeRemoteError    ResCode = 1004
	CodeServerBusy     ResCode = 1005
	CodeUnauthorized   ResCode = 1006
	CodeForbidden      ResCode = 1007
	CodeNotImplemented ResCode = 1099
)

// codeMsgMap 是统一响应码到默认中文文案的映射。
//
// Server 对外是商业化入口，错误信息必须保留可排障语义；后续新增响应码时应同步补这里。
var codeMsgMap = map[ResCode]string{
	CodeSuccess:        "success",
	CodeInvalidParams:  "请求参数错误",
	CodeNotFound:       "数据不存在",
	CodeConflict:       "数据状态冲突",
	CodeRemoteError:    "Edge API 或外部依赖调用失败",
	CodeServerBusy:     "服务繁忙",
	CodeUnauthorized:   "未登录或 token 无效",
	CodeForbidden:      "没有操作权限",
	CodeNotImplemented: "能力已规划，尚未实现",
}

func (c ResCode) Msg() string {
	if msg, ok := codeMsgMap[c]; ok {
		return msg
	}
	return codeMsgMap[CodeServerBusy]
}
