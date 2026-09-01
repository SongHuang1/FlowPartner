package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// executeEdit 在文件中搜索 old_string 并替换为 new_string。
// 精确匹配要求：old_string 必须恰好出现 1 次。
func (e *ToolExecutor) executeEdit(ctx context.Context, args map[string]interface{}) ToolResult {
	path, ok := getStringArg(args, "path")
	if !ok {
		return ToolResult{Success: false, Result: "缺少参数: path", ErrorCode: ErrToolError}
	}
	oldString, ok := getStringArg(args, "old_string")
	if !ok {
		return ToolResult{Success: false, Result: "缺少参数: old_string", ErrorCode: ErrToolError}
	}
	newString, ok := getStringArg(args, "new_string")
	if !ok {
		return ToolResult{Success: false, Result: "缺少参数: new_string", ErrorCode: ErrToolError}
	}

	// old_string 不能为空
	if oldString == "" {
		return ToolResult{Success: false, Result: "old_string 不能为空", ErrorCode: ErrToolError}
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

	// 读取文件
	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolResult{Success: false, Result: fmt.Sprintf("文件不存在: %s", path), ErrorCode: ErrToolError}
		}
		return ToolResult{Success: false, Result: fmt.Sprintf("读取文件失败: %v", err), ErrorCode: ErrToolError}
	}

	content := string(data)

	// 统计匹配次数
	matchCount := strings.Count(content, oldString)

	switch {
	case matchCount == 0:
		return ToolResult{Success: false, Result: "未找到匹配内容", ErrorCode: ErrToolError}
	case matchCount > 1:
		return ToolResult{
			Success:   false,
			Result:    fmt.Sprintf("匹配数 %d 大于 1，请提供更精确的匹配内容", matchCount),
			ErrorCode: ErrEditMatchCount,
		}
	}

	// 恰好匹配 1 次，执行替换
	newContent := strings.Replace(content, oldString, newString, 1)

	if err := os.WriteFile(resolved, []byte(newContent), 0o644); err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("写入文件失败: %v", err), ErrorCode: ErrToolError}
	}

	return ToolResult{Success: true, Result: "成功替换 1 处"}
}
