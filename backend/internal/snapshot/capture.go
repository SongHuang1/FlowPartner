package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

func Capture(ctx context.Context, workingDir, snapshotDir, projectID string, reason Reason, includeSecrets bool) (*Manifest, error) {
	normalized, err := NormalizeRoot(workingDir)
	if err != nil {
		return nil, fmt.Errorf("规范化工作区根失败: %w", err)
	}
	if normalizedProjectID := ProjectID(normalized); projectID != normalizedProjectID {
		return nil, fmt.Errorf("项目标识不匹配: 传入 %s，计算 %s", projectID, normalizedProjectID)
	}

	projectDir := filepath.Join(snapshotDir, projectID)
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建项目快照目录失败: %w", err)
	}

	snapshotID, err := NewSnapshotID(time.Now(), projectDir)
	if err != nil {
		return nil, err
	}
	destRoot := filepath.Join(projectDir, snapshotID)

	excluder := NewExcluder(includeSecrets, filepath.Join(projectDir, snapshotID))

	// 目标目录先创建；若中途失败，保留为未完成文件夹（无 manifest），
	// ListSnapshots 与 Restore 一律排除。
	if err := os.MkdirAll(destRoot, 0o700); err != nil {
		return nil, fmt.Errorf("创建快照目录失败: %w", err)
	}

	absRoot, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("解析工作区根失败: %w", err)
	}

	m := &Manifest{
		SnapshotID:              snapshotID,
		ProjectID:               projectID,
		Reason:                  reason,
		CreatedAt:               time.Now().UTC(),
		WorkspaceRoot:           filepath.Clean(absRoot),
		WorkspaceRootNormalized: normalized,
		SkippedFiles:            []SkippedFile{},
		Symlinks:                []SymlinkEntry{},
	}

	if err := walkAndCopy(ctx, workingDir, destRoot, excluder, m); err != nil {
		return nil, err
	}

	if m.FileCount == 0 && len(m.Symlinks) == 0 {
		os.RemoveAll(destRoot)
		return nil, nil
	}

	m.Complete = true
	if err := writeManifest(destRoot, m); err != nil {
		return nil, fmt.Errorf("写入 manifest 失败（快照视为未完成）: %w", err)
	}

	log.Printf("[snapshot] 完成 snapshot_id=%s reason=%s files=%d bytes=%d skipped=%d",
		snapshotID, reason, m.FileCount, m.TotalSizeBytes, len(m.SkippedFiles))
	return m, nil
}

func walkAndCopy(ctx context.Context, workingDir, destRoot string, ex *Excluder, m *Manifest) error {
	return filepath.WalkDir(workingDir, func(path string, d os.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			// 无法读取某子目录 → 跳过并记录，不中断
			rel, relErr := filepath.Rel(workingDir, path)
			if relErr == nil && rel != "." {
				m.SkippedFiles = append(m.SkippedFiles, SkippedFile{Path: rel, Reason: "read_error", Detail: "无法读取目录: " + walkErr.Error()})
			}
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == workingDir {
			return nil
		}

		rel, err := filepath.Rel(workingDir, path)
		if err != nil {
			return nil
		}
		dest := filepath.Join(destRoot, rel)

		info, err := os.Lstat(path)
		if err != nil {
			m.SkippedFiles = append(m.SkippedFiles, SkippedFile{Path: rel, Reason: "read_error", Detail: err.Error()})
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				m.SkippedFiles = append(m.SkippedFiles, SkippedFile{Path: rel, Reason: "read_error", Detail: "无法读取符号链接: " + err.Error()})
				return nil
			}
			m.Symlinks = append(m.Symlinks, SymlinkEntry{Path: rel, Target: target})
			return nil
		case info.IsDir():
			if ex.IsExcludedDir(path) {
				m.SkippedFiles = append(m.SkippedFiles, SkippedFile{Path: rel, Reason: "excluded_dir", Detail: "目录被排除规则跳过"})
				return filepath.SkipDir
			}
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return fmt.Errorf("创建目标目录失败（快照未完成）: %w", err)
			}
			return nil
		case info.Mode().IsRegular():
			if ex.IsExcludedDir(filepath.Dir(path)) {
				return nil
			}
			if skip, reason, detail := ex.ShouldSkipFile(path, info.Size()); skip {
				m.SkippedFiles = append(m.SkippedFiles, SkippedFile{Path: rel, Reason: reason, Detail: detail})
				return nil
			}
			if err := copyFile(path, dest, info.Mode()); err != nil {
				// 读取/复制源失败 vs 写入目标失败：
				// 通过 stat 源来区分——源仍存在且可读，说明是目标写入失败。
				if src, statErr := os.Stat(path); statErr != nil || !src.Mode().IsRegular() {
					m.SkippedFiles = append(m.SkippedFiles, SkippedFile{Path: rel, Reason: "read_error", Detail: err.Error()})
					return nil
				}
				return fmt.Errorf("写入快照文件失败（快照未完成）: %w", err)
			}
			m.FileCount++
			m.TotalSizeBytes += info.Size()
		}
		return nil
	})
}

// copyFile 复制文件内容并保持权限位。
func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".fp_tmp_*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode.Perm()); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

// writeManifest 原子写 manifest.json（先写临时文件再 rename）。
func writeManifest(destRoot string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(destRoot, ".manifest_tmp_*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(destRoot, "manifest.json"))
}
