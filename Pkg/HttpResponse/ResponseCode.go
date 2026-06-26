package HttpResponse

type ResCode int64

const (
	CodeSuccess       ResCode = 1000
	CodeInvalidParams ResCode = 1002
	CodeConflict      ResCode = 1003
	CodeNotFound      ResCode = 1004
	CodeRemoteError   ResCode = 1005
	CodeUnauthorized  ResCode = 1006
	CodeInternalError ResCode = 1500
)
