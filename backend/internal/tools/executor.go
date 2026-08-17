package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

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
