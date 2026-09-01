package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// executeWrite 写入文件，自动创建父目录。
func (e *ToolExecutor) executeWrite(ctx context.Context, args map[string]interface{}) ToolResult {
	path, ok := getStringArg(args, "path")
	if !ok {
		return ToolResult{Success: false, Result: "缺少参数: path", ErrorCode: ErrToolError}
	}
	content, ok := getStringArg(args, "content")
	if !ok {
		return ToolResult{Success: false, Result: "缺少参数: content", ErrorCode: ErrToolError}
	}

	// 路径校验（已通过越权审批时跳过）
	if !IsApproved(ctx) {
		if err := e.guard.Validate(path); err != nil {
			return ToolResult{Success: false, Result: err.Error(), ErrorCode: ErrPathOutside}
		}
	}

	resolved, err := e.guard.Resolve(path)
	if err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("解析路径失败: %v", err), ErrorCode: ErrToolError}
	}

	// 自动创建父目录
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("创建父目录失败: %v", err), ErrorCode: ErrToolError}
	}

	// 写入文件
	if err := os.WriteFile(resolved, []byte(content), 0o644); err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("写入文件失败: %v", err), ErrorCode: ErrToolError}
	}

	return ToolResult{Success: true, Result: fmt.Sprintf("成功写入 %d 个字符", len(content))}
}
