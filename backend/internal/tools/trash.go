package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// trashSeq 进程内自增序号，保证同一秒内并发删除同名文件不冲突（时间戳仅秒级精度）。
var trashSeq uint64

// executeTrash 将目标文件/目录移入回收站（F5-F11）。
// 回收站目录为预授权安全目的地（F10）；源路径若在工作区外需先经审批（CheckPath 处理）。
func (e *ToolExecutor) executeTrash(ctx context.Context, args map[string]interface{}) ToolResult {
	paths, missing := trashPathsFromArgs(args)
	if missing {
		return ToolResult{Success: false, Result: "缺少参数: path 或 paths", ErrorCode: ErrToolError}
	}

	trashDir := e.trashDir
	if trashDir == "" {
		return ToolResult{
			Success:   false,
			Result:    "回收站目录未配置。请在设置 → 智能体中指定回收站目录后重试。",
			ErrorCode: ErrTrashNotConfigured,
		}
	}
	if len(paths) == 0 {
		return ToolResult{Success: false, Result: "缺少参数: path 或 paths 不能为空", ErrorCode: ErrToolError}
	}

	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return ToolResult{Success: false, Result: fmt.Sprintf("创建回收站目录失败: %v", err), ErrorCode: ErrToolError}
	}

	var moved []string
	var failed []string
	for _, p := range paths {
		if !IsApproved(ctx) {
			if err := e.guard.Validate(p); err != nil {
				failed = append(failed, fmt.Sprintf("%s（%v）", p, err))
				continue
			}
		}
		resolved, err := e.guard.Resolve(p)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s（%v）", p, err))
			continue
		}
		if _, err := os.Lstat(resolved); err != nil {
			failed = append(failed, fmt.Sprintf("%s（文件或目录不存在）", p))
			continue
		}
		destName, err := moveIntoTrash(resolved, trashDir)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s（%v）", p, err))
			continue
		}
		moved = append(moved, destName)
	}

	if len(failed) > 0 {
		return ToolResult{
			Success:   false,
			Result:    fmt.Sprintf("部分条目未成功移入回收站。已成功: %s；失败: %s", strings.Join(moved, ", "), strings.Join(failed, "; ")),
			ErrorCode: ErrToolError,
		}
	}
	return ToolResult{Success: true, Result: fmt.Sprintf("已移入回收站 %d 个条目: %s", len(moved), strings.Join(moved, ", "))}
}

func trashPathsFromArgs(args map[string]interface{}) ([]string, bool) {
	if p, ok := getStringArg(args, "path"); ok && p != "" {
		return []string{p}, false
	}
	if ps, ok := getStringSliceArg(args, "paths"); ok && len(ps) > 0 {
		return ps, false
	}
	return nil, true
}

func moveIntoTrash(src, trashDir string) (string, error) {
	name := uniqueTrashName(trashDir, filepath.Base(src))
	dest := filepath.Join(trashDir, name)
	if err := renameOrCopy(src, dest); err != nil {
		return "", err
	}
	writeTrashMeta(trashDir, name, src)
	return name, nil
}

func uniqueTrashName(dir, base string) string {
	ts := time.Now().UTC().Format("20060102T150405000000Z")
	seq := atomic.AddUint64(&trashSeq, 1)
	for n := uint64(0); ; n++ {
		name := fmt.Sprintf("%s__%d__%s", ts, seq+n, base)
		if _, err := os.Lstat(filepath.Join(dir, name)); os.IsNotExist(err) {
			return name
		}
	}
}

func renameOrCopy(src, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		return nil
	}

	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("无法访问源路径: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.Symlink(link, dest); err != nil {
			return err
		}
		if err := os.Remove(src); err != nil {
			os.RemoveAll(dest)
			return fmt.Errorf("复制后删除源失败: %w", err)
		}
		return nil
	}

	if info.IsDir() {
		if err := copyDir(src, dest); err != nil {
			os.RemoveAll(dest)
			return err
		}
	} else {
		if err := copyFile(src, dest); err != nil {
			os.RemoveAll(dest)
			return err
		}
	}

	if err := os.RemoveAll(src); err != nil {
		os.RemoveAll(dest)
		return fmt.Errorf("复制后删除源失败: %w", err)
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("复制文件失败: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("关闭目标文件失败: %w", closeErr)
	}
	if info, err := os.Stat(src); err == nil {
		_ = os.Chmod(dest, info.Mode())
	}
	return nil
}

func copyDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sp := filepath.Join(src, entry.Name())
		dp := filepath.Join(dest, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := copyDir(sp, dp); err != nil {
				return err
			}
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(sp)
			if err != nil {
				return err
			}
			if err := os.Symlink(link, dp); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(sp, dp); err != nil {
			return err
		}
	}
	return nil
}

func writeTrashMeta(trashDir, name, originalPath string) {
	meta := struct {
		OriginalPath string    `json:"original_path"`
		DeletedAt    time.Time `json:"deleted_at"`
	}{
		OriginalPath: originalPath,
		DeletedAt:    time.Now().UTC(),
	}
	data, err := json.Marshal(meta)
	if err != nil {
		log.Printf("[trash] 写入元数据失败: %v", err)
		return
	}
	metaPath := filepath.Join(trashDir, name+".meta.json")
	if err := os.WriteFile(metaPath, data, 0600); err != nil {
		log.Printf("[trash] 写入元数据失败: %v", err)
	}
}
