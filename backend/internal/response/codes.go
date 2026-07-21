package response

const (
	// CodeOK 成功
	CodeOK = 0

	// 1000-1999 客户端错误
	CodeInvalidParam  = 1001
	CodeMissingParam   = 1002
	CodeNotImplemented = 1003

	// 2000-2999 服务端错误
	CodeInternalError = 2001

	// 4000-4999 安全拦截（后续步骤使用）
	CodeDangerousAction  = 4001
	CodePermissionDenied = 4002
	CodeUserRejected     = 4003
)
