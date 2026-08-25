package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
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

type sessionIDKey struct{}

// WithSessionID 在 context 中记录当前会话 ID（用于审计日志）。
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// SessionIDFrom 从 context 中读取会话 ID。
func SessionIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(sessionIDKey{}).(string)
	return v
}

type approvalIDKey struct{}

// WithApprovalID 在 context 中记录本次审批的 approval_id（用于审计日志）。
func WithApprovalID(ctx context.Context, approvalID string) context.Context {
	return context.WithValue(ctx, approvalIDKey{}, approvalID)
}

// ApprovalIDFrom 从 context 中读取 approval_id。
func ApprovalIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(approvalIDKey{}).(string)
	return v
}

// ToolExecutor 工具执行器，持有 PathGuard，按工具名分发执行。
type ToolExecutor struct {
	guard    *PathGuard
	trashDir string
}

// ToolExecutorOption 是创建 ToolExecutor 的可选配置。
type ToolExecutorOption func(*ToolExecutor)

// WithTrashDir 设置回收站目录（trash/purge 工具使用）。
func WithTrashDir(dir string) ToolExecutorOption {
	return func(e *ToolExecutor) {
		e.trashDir = dir
	}
}

// NewToolExecutor 创建工具执行器。workingDir 必须是已解析的绝对路径。
func NewToolExecutor(workingDir string, opts ...ToolExecutorOption) (*ToolExecutor, error) {
	guard, err := NewPathGuard(workingDir)
	if err != nil {
		return nil, err
	}
	e := &ToolExecutor{guard: guard}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

// Execute 执行指定工具，返回 ToolResult。
func (e *ToolExecutor) Execute(ctx context.Context, sessionID, toolName, argsJSON string) ToolResult {
	log.Printf("[ToolExecutor] Session: %s, Tool: %s, Args length: %d", sessionID, toolName, len(argsJSON))
	ctx = WithSessionID(ctx, sessionID)

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
	case "trash":
		return e.executeTrash(ctx, args)
	case "purge":
		return e.executePurge(ctx, args)
	default:
		return ToolResult{
			Success:   false,
			Result:    fmt.Sprintf("未找到工具: %s", toolName),
			ErrorCode: ErrToolNotFound,
		}
	}
}

func (e *ToolExecutor) CheckPath(toolName string, argsJSON string) (needsPermission bool, rawPath string, resolvedPath string, err error) {
	switch toolName {
	case "bash":
		return false, "", "", nil
	case "read", "write", "edit":
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return false, "", "", fmt.Errorf("参数格式错误: %w", err)
		}
		path, ok := getStringArg(args, "path")
		if !ok {
			return false, "", "", fmt.Errorf("缺少参数: path")
		}
		return e.checkSinglePath(path)
	case "trash":
		return e.checkTrashPath(argsJSON)
	case "purge":
		return e.checkPurgePath(argsJSON)
	default:
		return false, "", "", nil
	}
}

// checkSinglePath 校验单个源路径是否越权，供 read/write/edit/trash 单路径模式复用。
func (e *ToolExecutor) checkSinglePath(path string) (bool, string, string, error) {
	resolved, resolveErr := e.guard.Resolve(path)
	if resolveErr != nil {
		return false, "", "", fmt.Errorf("解析路径失败: %w", resolveErr)
	}
	if validateErr := e.guard.Validate(path); validateErr != nil {
		return true, path, resolved, nil
	}
	return false, "", "", nil
}

// checkTrashPath 校验 trash 工具参数：单路径模式越权走用户审批；
// paths 数组模式要求所有源路径均在工作区内，不支持审批。

func (e *ToolExecutor) checkTrashPath(argsJSON string) (bool, string, string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return false, "", "", fmt.Errorf("参数格式错误: %w", err)
	}
	if path, ok := getStringArg(args, "path"); ok {
		return e.checkSinglePath(path)
	}
	if paths, ok := getStringSliceArg(args, "paths"); ok {
		if len(paths) == 0 {
			return false, "", "", fmt.Errorf("缺少参数: paths 不能为空")
		}
		for _, p := range paths {
			if err := e.guard.Validate(p); err != nil {
				return false, "", "", fmt.Errorf("paths 数组模式要求所有源路径均在工作目录内；路径 %q 超出工作区，请改用单路径 trash 调用并分别审批", p)
			}
		}
		return false, "", "", nil
	}
	return false, "", "", fmt.Errorf("缺少参数: path 或 paths")
}

func (e *ToolExecutor) checkPurgePath(argsJSON string) (bool, string, string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return false, "", "", fmt.Errorf("参数格式错误: %w", err)
	}
	entry, _ := getStringArg(args, "entry")
	trashDir := e.trashDir
	if trashDir == "" {
		return false, "", "", fmt.Errorf("回收站目录未配置，无法执行 purge")
	}
	if entry != "" && !validPurgeEntry(entry) {
		return false, "", "", fmt.Errorf("purge 的 entry 必须是单一文件名分量，不能包含路径分隔符或 '..'")
	}
	target := filepath.Join(trashDir, entry)
	guard, err := NewPathGuard(trashDir)
	if err != nil {
		return false, "", "", fmt.Errorf("回收站目录不可用: %w", err)
	}
	if err := guard.Validate(target); err != nil {
		return false, "", "", fmt.Errorf("purge 目标超出回收站目录范围: %w", err)
	}
	return true, target, target, nil
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

// getStringSliceArg 从参数 map 中安全提取字符串数组值。
func getStringSliceArg(args map[string]interface{}, key string) ([]string, bool) {
	val, ok := args[key]
	if !ok {
		return nil, false
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		result = append(result, s)
	}
	return result, true
}
