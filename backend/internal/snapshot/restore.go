package snapshot

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RestoreOptions 还原参数。
type RestoreOptions struct {
	WorkingDir     string // 当前工作区根（还原目标）
	SnapshotDir    string // 储存目录
	ProjectID      string // 目标项目标识（当前工作区根的哈希）
	SnapshotID     string // 要还原的快照 id
	DeleteExtras   bool   // 是否删除多余文件（§2.7）
	IncludeSecrets bool   // 当前快照排除规则是否包含敏感文件（用于保护判定）
}

// RestoreResult 还原结果。
type RestoreResult struct {
	PreSnapshotID   string        `json:"pre_snapshot_id"`
	RestoredFiles   int           `json:"restored_files"`
	DeletedFiles    []string      `json:"deleted_files"`
	SymlinkFailures []SkippedFile `json:"symlink_failures"`
}

func Restore(ctx context.Context, opts RestoreOptions) (*RestoreResult, error) {
	snapshotPath := filepath.Join(opts.SnapshotDir, opts.ProjectID, opts.SnapshotID)
	m, err := LoadManifest(snapshotPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("该快照未完成（缺少清单文件），禁止还原")
		}
		return nil, fmt.Errorf("无法读取快照：%v", err)
	}
	if !m.Complete {
		return nil, fmt.Errorf("该快照未完成（缺少完成标记），禁止还原")
	}

	currentNormalized, err := NormalizeRoot(opts.WorkingDir)
	if err != nil {
		return nil, fmt.Errorf("规范化当前工作区根失败: %w", err)
	}
	if currentNormalized != m.WorkspaceRootNormalized {
		return nil, fmt.Errorf("该快照来自不同项目路径，禁止还原")
	}

	// 1. 预快照当前状态（保证可逆）。预快照归属当前项目标识。
	currentProjectID := ProjectID(currentNormalized)
	pre, err := Capture(ctx, opts.WorkingDir, opts.SnapshotDir, currentProjectID, ReasonPreRestore, opts.IncludeSecrets)
	if err != nil {
		return nil, fmt.Errorf("还原前预快照失败，已中止还原: %w", err)
	}
	result := &RestoreResult{DeletedFiles: []string{}}
	if pre != nil {
		result.PreSnapshotID = pre.SnapshotID
		log.Printf("[snapshot] 还原前预快照完成: %s", result.PreSnapshotID)
	}

	// 2. 写回快照文件（覆盖 + 新增）。
	ex := NewExcluder(opts.IncludeSecrets, "")
	restored, err := writeBack(ctx, snapshotPath, opts.WorkingDir)
	if err != nil {
		return nil, fmt.Errorf("还原写回中途失败，项目处于中间态；请用最近一次'还原前'快照 %s 恢复: %w", result.PreSnapshotID, err)
	}
	result.RestoredFiles = restored

	// 3. deleteExtras（排除/跳过文件永不删除）。
	if opts.DeleteExtras {
		deleted, err := deleteExtras(ctx, snapshotPath, opts.WorkingDir, ex)
		if err != nil {
			return nil, fmt.Errorf("删除多余文件中途失败；请用最近一次'还原前'快照 %s 恢复: %w", result.PreSnapshotID, err)
		}
		result.DeletedFiles = deleted
	}

	// 4. 重建符号链接（失败跳过并警告，不令还原失败）。
	for _, link := range m.Symlinks {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := rebuildSymlink(opts.WorkingDir, link); err != nil {
			result.SymlinkFailures = append(result.SymlinkFailures, SkippedFile{
				Path:   link.Path,
				Reason: "symlink_restore_failed",
				Detail: err.Error(),
			})
			log.Printf("[snapshot] 重建符号链接失败: %s → %s: %v", link.Path, link.Target, err)
		}
	}

	log.Printf("[snapshot] 还原完成 snapshot=%s restored=%d deleted=%d symlink_failed=%d",
		opts.SnapshotID, result.RestoredFiles, len(result.DeletedFiles), len(result.SymlinkFailures))
	return result, nil
}

func writeBack(ctx context.Context, snapshotPath, workingDir string) (int, error) {
	count := 0
	err := filepath.WalkDir(snapshotPath, func(path string, d os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == snapshotPath {
			return nil
		}
		if d.Name() == "manifest.json" && filepath.Dir(path) == snapshotPath {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(snapshotPath, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(workingDir, rel)
		if d.IsDir() {
			return ensureRealDir(dest)
		}
		if d.Type().IsRegular() {
			if err := ensureRealParent(dest); err != nil {
				return err
			}
			if err := copyFile(path, dest, 0o755); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

func ensureRealParent(dest string) error {
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	return ensureNoSymlinkComponents(dest)
}

// ensureRealDir 确保目录存在且不含符号链接组件。
func ensureRealDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return ensureNoSymlinkComponents(dir)
}

// ensureNoSymlinkComponents 从根向叶子逐级检查 dir 的每个路径组件；
// 若某级是符号链接则移除，防止写入路径经由符号链接逃逸到工作区根之外。
func ensureNoSymlinkComponents(dir string) error {
	current := filepath.VolumeName(dir)
	if current == "" {
		current = string(filepath.Separator)
	}
	rest := strings.TrimPrefix(dir, current)
	rest = strings.TrimPrefix(rest, string(filepath.Separator))
	if rest == "" {
		return nil
	}
	for _, comp := range strings.Split(rest, string(filepath.Separator)) {
		current = filepath.Join(current, comp)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil // 剩余组件将由后续创建，无需检查
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(current); err != nil {
				return fmt.Errorf("无法移除路径上的符号链接 %s: %w", current, err)
			}
			return nil // 已移除，后续组件将被真实创建
		}
	}
	return nil
}

// deleteExtras 删除"当前存在、未被排除、但快照内不存在"的文件。
// 被排除目录/密钥/超大文件永不删除（§2.7、§3.5、§4.10）。
func deleteExtras(ctx context.Context, snapshotPath, workingDir string, ex *Excluder) ([]string, error) {
	// 先收集快照内存在的相对路径集合。
	snapshotFiles := map[string]bool{}
	err := filepath.WalkDir(snapshotPath, func(path string, d os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == snapshotPath {
			return nil
		}
		if d.Name() == "manifest.json" && filepath.Dir(path) == snapshotPath {
			return nil
		}
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(snapshotPath, path)
		if err == nil && rel != "." {
			snapshotFiles[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var toDelete []string
	err = filepath.WalkDir(workingDir, func(path string, d os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == workingDir {
			return nil
		}
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(workingDir, path)
		if err != nil {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if ex.IsExcludedDir(path) {
				return filepath.SkipDir // 排除目录整体受保护
			}
			return nil
		}
		// 排除判定：密钥/超大永不删除（即使快照内没有）。
		if skip, reason, _ := ex.ShouldSkipFile(path, info.Size()); skip {
			log.Printf("[snapshot] deleteExtras 保护: %s (%s)", rel, reason)
			return nil
		}
		if snapshotFiles[filepath.ToSlash(rel)] {
			return nil // 快照中存在，保留
		}
		toDelete = append(toDelete, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 从最深路径开始删除（先子后父）。
	sort.Slice(toDelete, func(i, j int) bool { return len(toDelete[i]) > len(toDelete[j]) })
	deleted := make([]string, 0, len(toDelete))
	for _, p := range toDelete {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		if err := os.Remove(p); err != nil {
			return deleted, fmt.Errorf("删除多余文件 %s 失败: %w", p, err)
		}
		rel, _ := filepath.Rel(workingDir, p)
		deleted = append(deleted, filepath.ToSlash(rel))
	}
	return deleted, nil
}

func rebuildSymlink(workingDir string, link SymlinkEntry) error {
	dest := filepath.Join(workingDir, filepath.FromSlash(link.Path))
	if err := ensureRealParent(dest); err != nil {
		return err
	}
	// 目标已存在（文件/目录/旧链接）时先移除。
	if _, err := os.Lstat(dest); err == nil {
		if err := os.Remove(dest); err != nil {
			return err
		}
	}
	return os.Symlink(link.Target, dest)
}

// ProtectedEntry 受保护文件条目（还原确认框展示：被排除/跳过的文件）。
type ProtectedEntry struct {
	Path   string `json:"path"`
	Type   string `json:"type"` // secret | too_large | excluded_dir
	Detail string `json:"detail,omitempty"`
}

// ProtectedFiles 计算当前工作区中受保护（永不因 deleteExtras 被删除）的文件与目录。
func ProtectedFiles(workingDir string, includeSecrets bool) ([]ProtectedEntry, error) {
	ex := NewExcluder(includeSecrets, "")
	var entries []ProtectedEntry
	err := filepath.WalkDir(workingDir, func(path string, d os.DirEntry, walkErr error) error {
		if path == workingDir {
			return nil
		}
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(workingDir, path)
		if err != nil {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if ex.IsExcludedDir(path) {
				entries = append(entries, ProtectedEntry{Path: filepath.ToSlash(rel), Type: "excluded_dir"})
				return filepath.SkipDir
			}
			return nil
		}
		if info.Mode().IsRegular() {
			if skip, reason, detail := ex.ShouldSkipFile(path, info.Size()); skip {
				entries = append(entries, ProtectedEntry{Path: filepath.ToSlash(rel), Type: reason, Detail: detail})
			}
		}
		return nil
	})
	return entries, err
}
