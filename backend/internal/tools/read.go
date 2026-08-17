package tools

import (
	"context"
	"fmt"
	"os"
	"unicode/utf8"
)

const (
	maxReadFileSize  = 10 * 1024 * 1024 // 10MB
	maxReadCharCount = 10000
)

// executeRead 读取文件内容（UTF-8），超过 10000 字符截断。
func (e *ToolExecutor) executeRead(_ context.Context, args map[string]interface{}) ToolResult {
	path, ok := getStringArg(args, "path")
	if !ok {
		return ToolResult{Success: false, Result: "缺少参数: path", ErrorCode: ErrToolError}
	}

	// 路径校验
	if err := e.guard.Validate(path); err != nil {
		return ToolResult{Success: false, Result: err.Error(), ErrorCode: ErrPathOutside}
	}

	resolved, err := e.guard.Resolve(path)
	if err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("解析路径失败: %v", err), ErrorCode: ErrToolError}
	}

	// 检查文件是否存在
	info, err := os.Stat(resolved)
	if os.IsNotExist(err) {
		return ToolResult{Success: false, Result: fmt.Sprintf("文件不存在: %s", path), ErrorCode: ErrToolError}
	}
	if err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("无法访问文件: %v", err), ErrorCode: ErrToolError}
	}

	// 必须是文件，不能是目录
	if info.IsDir() {
		return ToolResult{Success: false, Result: fmt.Sprintf("路径是目录而非文件: %s", path), ErrorCode: ErrToolError}
	}

	// 文件大小检查（>10MB 拒绝）
	if info.Size() > maxReadFileSize {
		return ToolResult{
			Success:   false,
			Result:    fmt.Sprintf("文件过大（%d 字节），超过 10MB 限制，无法读取", info.Size()),
			ErrorCode: ErrFileTooLarge,
		}
	}

	// 读取文件
	data, err := os.ReadFile(resolved)
	if err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("读取文件失败: %v", err), ErrorCode: ErrToolError}
	}

	// UTF-8 解码检查
	if !utf8.Valid(data) {
		return ToolResult{Success: false, Result: "文件内容不是有效的 UTF-8 编码", ErrorCode: ErrToolError}
	}

	content := string(data)

	// 超过 10000 字符截断
	if len(content) > maxReadCharCount {
		content = content[:maxReadCharCount] + "\n... [文件过长，已截断]"
	}

	return ToolResult{Success: true, Result: content}
}
