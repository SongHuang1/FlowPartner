package response

const (
	// CodeOK 成功
	CodeOK = 0

	// 1000-1999 客户端错误
	CodeInvalidParam   = 1001
	CodeMissingParam   = 1002
	CodeNotImplemented = 1003
	CodeNameConflict   = 1004 // 名称冲突（如智能体名称重复）
	CodeNotFound       = 1005 // 资源不存在

	// 2000-2999 服务端错误
	CodeInternalError = 2001

	// 4000-4999 安全拦截
	CodeDangerousAction  = 4001
	CodePermissionDenied = 4002
	CodeUserRejected     = 4003

	// 5000-5999 API Key 锁定相关
	CodeUnlockRateLimited   = 5001
	CodeAPIKeyNotConfigured = 5002
	CodeWrongPassword       = 5003
)
