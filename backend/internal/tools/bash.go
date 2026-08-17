package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	bashTimeout       = 30 * time.Second
	maxBashCharCount  = 10000
)

// executeBash 在工作目录内执行 bash 命令，超时 30 秒。
func (e *ToolExecutor) executeBash(ctx context.Context, args map[string]interface{}) ToolResult {
	command, ok := getStringArg(args, "command")
	if !ok {
		return ToolResult{Success: false, Result: "缺少参数: command", ErrorCode: ErrToolError}
	}

	// bash 工具的工作目录限制
	if err := e.guard.ValidateBashWorkDir(); err != nil {
		return ToolResult{Success: false, Result: err.Error(), ErrorCode: ErrPathOutside}
	}

	// 检查命令是否试图访问工作目录外的路径
	if err := e.validateBashCommand(command); err != nil {
		return ToolResult{Success: false, Result: err.Error(), ErrorCode: ErrPathOutside}
	}

	// 带超时的 context
	ctx, cancel := context.WithTimeout(ctx, bashTimeout)
	defer cancel()

	// 在 Windows 上使用 cmd /c，在其他平台使用 sh -c
	cmd := buildCommand(ctx, command)
	cmd.Dir = e.guard.WorkingDir()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// 合并 stdout 和 stderr
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	// 超时检查
	if ctx.Err() == context.DeadlineExceeded {
		return ToolResult{
			Success:   false,
			Result:    "命令执行超时（30 秒）",
			ErrorCode: ErrToolError,
		}
	}

	// 命令执行失败
	if err != nil {
		return ToolResult{
			Success:   false,
			Result:    fmt.Sprintf("命令执行失败: %v\n%s", err, output),
			ErrorCode: ErrToolError,
		}
	}

	// UTF-8 解码检查
	if !utf8.ValidString(output) {
		return ToolResult{
			Success:   false,
			Result:    "命令输出不是有效的 UTF-8 编码",
			ErrorCode: ErrToolError,
		}
	}

	// 超过 10000 字符截断
	if len(output) > maxBashCharCount {
		output = output[:maxBashCharCount] + "\n... [输出过长，已截断]"
	}

	if output == "" {
		output = "(命令执行成功，无输出)"
	}

	return ToolResult{Success: true, Result: output}
}

// validateBashCommand 检查 bash 命令是否试图访问工作目录外的路径。
func (e *ToolExecutor) validateBashCommand(command string) error {
	// 简单检查：命令中不能包含 ".."（防止路径逃逸）
	if strings.Contains(command, "..") {
		return fmt.Errorf("操作被拒绝：命令中包含路径逃逸符号 '..'")
	}

	// 分割命令为参数（简单按空格分割，不处理引号内的空格）
	parts := strings.Fields(command)

	// 检查 cd 命令是否切换到工作目录外
	// 简单启发式：如果命令包含 "cd " 后跟绝对路径或 ".."
	if strings.Contains(command, "cd ") {
		for i, part := range parts {
			if part == "cd" && i+1 < len(parts) {
				target := parts[i+1]
				if strings.HasPrefix(target, "..") {
					return fmt.Errorf("操作被拒绝：命令试图切换到工作目录外")
				}
				if isAbsolutePath(target) {
					return fmt.Errorf("操作被拒绝：命令试图切换到工作目录外")
				}
			}
		}
	}

	// 检查其他参数是否包含绝对路径
	for _, part := range parts {
		if isAbsolutePath(part) {
			return fmt.Errorf("操作被拒绝：命令中包含绝对路径")
		}
	}

	return nil
}

// isAbsolutePath 检查路径是否为绝对路径（Unix 或 Windows）。
// 注意：Windows 命令行选项（如 /L、/c）以 / 开头但不是路径，需要排除。
func isAbsolutePath(path string) bool {
	// Windows 绝对路径：C:\ 或 C:/（盘符 + 冒号 + 斜杠）
	if len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	// UNC 路径：\\server
	if strings.HasPrefix(path, "\\\\") {
		return true
	}
	// Unix 绝对路径：/ 开头，但排除 Windows 命令行选项（/ + 单个字母，如 /L、/c）
	if strings.HasPrefix(path, "/") {
		remainder := path[1:]
		// 如果 / 后面紧跟单个字母就结束了（或后跟空格/空白），这是 Windows 选项，不是路径
		if len(remainder) == 1 && remainder[0] >= 'A' && remainder[0] <= 'Z' {
			return false
		}
		if len(remainder) == 1 && remainder[0] >= 'a' && remainder[0] <= 'z' {
			return false
		}
		return true
	}
	return false
}

// buildCommand 根据平台构建命令执行器。
func buildCommand(ctx context.Context, command string) *exec.Cmd {
	if isWindows() {
		return exec.CommandContext(ctx, "cmd", "/c", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func isWindows() bool {
	return exec.Command("cmd", "/c", "echo").Run() == nil
}
