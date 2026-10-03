package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/storage"
)

func validPurgeEntry(entry string) bool {
	if entry == "" || entry == "." || entry == ".." {
		return false
	}
	if strings.ContainsAny(entry, `/\`) {
		return false
	}
	return true
}

func (e *ToolExecutor) executePurge(ctx context.Context, args map[string]interface{}) ToolResult {
	if !IsApproved(ctx) {
		return ToolResult{
			Success:   false,
			Result:    "purge 为不可逆操作，必须通过用户显式审批后才能执行",
			ErrorCode: ErrToolError,
		}
	}

	entry, _ := getStringArg(args, "entry")
	trashDir := e.trashDir
	if trashDir == "" {
		return ToolResult{
			Success:   false,
			Result:    "回收站目录未配置，无法执行 purge",
			ErrorCode: ErrTrashNotConfigured,
		}
	}
	if entry != "" && !validPurgeEntry(entry) {
		return ToolResult{
			Success:   false,
			Result:    "purge 的 entry 必须是单一文件名分量，不能包含路径分隔符或 '..'",
			ErrorCode: ErrToolError,
		}
	}

	target := filepath.Join(trashDir, entry)

	guard, err := NewPathGuard(trashDir)
	if err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("回收站目录不可用: %v", err), ErrorCode: ErrToolError}
	}
	if err := guard.Validate(target); err != nil {
		return ToolResult{
			Success:   false,
			Result:    fmt.Sprintf("purge 目标超出回收站目录范围，已拒绝执行: %v", err),
			ErrorCode: ErrToolError,
		}
	}

	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			msg := "回收站为空，无可删除条目"
			if entry != "" {
				msg = fmt.Sprintf("回收站中不存在条目: %s", entry)
			}
			return ToolResult{Success: false, Result: msg, ErrorCode: ErrToolError}
		}
		return ToolResult{Success: false, Result: fmt.Sprintf("无法访问回收站条目: %v", err), ErrorCode: ErrToolError}
	}

	count := countPurgeEntries(target, info)
	if err := os.RemoveAll(target); err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("永久删除失败: %v", err), ErrorCode: ErrToolError}
	}

	logPurgeAudit(ctx, target, count)
	return ToolResult{Success: true, Result: fmt.Sprintf("已永久删除 %d 个条目", count)}
}

func countPurgeEntries(target string, info os.FileInfo) int {
	if !info.IsDir() {
		return 1
	}
	count := 0
	_ = filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		count++
		return nil
	})
	return count
}

func logPurgeAudit(ctx context.Context, target string, count int) {
	entry := struct {
		Time       string `json:"time"`
		Session    string `json:"session"`
		ApprovalID string `json:"approval_id"`
		Target     string `json:"target"`
		Count      int    `json:"count"`
	}{
		Time:       time.Now().UTC().Format(time.RFC3339),
		Session:    SessionIDFrom(ctx),
		ApprovalID: ApprovalIDFrom(ctx),
		Target:     target,
		Count:      count,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[purge] 审计日志序列化失败: %v", err)
		return
	}
	dir, err := storage.DataDir()
	if err != nil {
		log.Printf("[purge] 无法获取数据目录写入审计日志: %v", err)
		return
	}
	path := filepath.Join(dir, "trash_audit.log")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Printf("[purge] 无法写入审计日志: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		log.Printf("[purge] 写入审计日志失败: %v", err)
	}
}
