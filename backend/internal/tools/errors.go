package tools

// 错误码常量，供前端和日志区分错误类型。
const (
	ErrToolNotFound        = "TOOL_NOT_FOUND"
	ErrPathOutside         = "PATH_OUTSIDE_WORKSPACE"
	ErrFileTooLarge        = "FILE_TOO_LARGE"
	ErrToolError           = "TOOL_ERROR"
	ErrEditMatchCount      = "EDIT_MATCH_COUNT_ERROR"
	ErrDeletionBlocked     = "TOOL_DELETION_BLOCKED"
	ErrTrashNotConfigured  = "TOOL_TRASH_NOT_CONFIGURED"
)

// ToolResult 工具执行结果。
type ToolResult struct {
	Success   bool   `json:"success"`
	Result    string `json:"result"`
	ErrorCode string `json:"error_code"`
}
