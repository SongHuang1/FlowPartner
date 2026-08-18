package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// approvalContextKey 是 context key 类型，用于标记该次执行已通过越权审批。
type approvalContextKey struct{}

// WithApproval 在 context 中标记本次执行已通过越权审批，跳过工具内的路径校验。
func WithApproval(ctx context.Context) context.Context {
	return context.WithValue(ctx, approvalContextKey{}, true)
}

// IsApproved 检查 context 中是否携带越权审批标记。
func IsApproved(ctx context.Context) bool {
	v, _ := ctx.Value(approvalContextKey{}).(bool)
	return v
}

// ToolExecutor 工具执行器，持有 PathGuard，按工具名分发执行。
type ToolExecutor struct {
	guard *PathGuard
}

// NewToolExecutor 创建工具执行器。workingDir 必须是已解析的绝对路径。
func NewToolExecutor(workingDir string) (*ToolExecutor, error) {
	guard, err := NewPathGuard(workingDir)
	if err != nil {
		return nil, err
	}
	return &ToolExecutor{guard: guard}, nil
}

// Execute 执行指定工具，返回 ToolResult。
func (e *ToolExecutor) Execute(ctx context.Context, sessionID, toolName, argsJSON string) ToolResult {
	log.Printf("[ToolExecutor] Session: %s, Tool: %s, Args length: %d", sessionID, toolName, len(argsJSON))

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ToolResult{
			Success:   false,
			Result:    fmt.Sprintf("参数格式错误（非 JSON）: %v", err),
			ErrorCode: ErrToolError,
		}
	}

	switch toolName {
	case "read":
		return e.executeRead(ctx, args)
	case "write":
		return e.executeWrite(ctx, args)
	case "bash":
		return e.executeBash(ctx, args)
	case "edit":
		return e.executeEdit(ctx, args)
	default:
		return ToolResult{
			Success:   false,
			Result:    fmt.Sprintf("未找到工具: %s", toolName),
			ErrorCode: ErrToolNotFound,
		}
	}
}

func (e *ToolExecutor) CheckPath(toolName string, argsJSON string) (needsPermission bool, rawPath string, resolvedPath string, err error) {
	if toolName == "bash" {
		return false, "", "", nil
	}
	if toolName != "read" && toolName != "write" && toolName != "edit" {
		return false, "", "", nil
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return false, "", "", fmt.Errorf("参数格式错误: %w", err)
	}

	path, ok := getStringArg(args, "path")
	if !ok {
		return false, "", "", fmt.Errorf("缺少参数: path")
	}

	resolved, resolveErr := e.guard.Resolve(path)
	if resolveErr != nil {
		return false, "", "", fmt.Errorf("解析路径失败: %w", resolveErr)
	}

	validateErr := e.guard.Validate(path)
	if validateErr != nil {
		return true, path, resolved, nil
	}

	return false, "", "", nil
}

// getStringArg 从参数 map 中安全提取字符串值。
func getStringArg(args map[string]interface{}, key string) (string, bool) {
	val, ok := args[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	if !ok {
		return "", false
	}
	return str, true
}
