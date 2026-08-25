package snapshot

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// 保留策略常量
const (
	retentionDays   = 30
	maxStorageBytes = int64(5) * 1024 * 1024 * 1024 // 5GB
)

func Cleanup(projectDir string, now time.Time) (deleted []string, err error) {
	snapshots, err := listSnapshotDirs(projectDir)
	if err != nil {
		return nil, err
	}

	// 1. 未完成快照：全部删除（不进入保留期判断）。
	for _, s := range snapshots {
		if !s.complete {
			log.Printf("[snapshot] 清理未完成快照: %s", s.id)
			if err := os.RemoveAll(s.path); err != nil {
				return deleted, fmt.Errorf("删除未完成快照 %s 失败: %w", s.id, err)
			}
			deleted = append(deleted, s.id)
		}
	}
	completed := make([]snapshotInfo, 0, len(snapshots))
	for _, s := range snapshots {
		if s.complete {
			completed = append(completed, s)
		}
	}
	if len(completed) == 0 {
		return deleted, nil
	}
	sort.Slice(completed, func(i, j int) bool { return completed[i].created.Before(completed[j].created) })

	// 2. 时间保留：删除超过 30 天的快照。
	cutoff := now.Add(-retentionDays * 24 * time.Hour)
	for _, s := range completed {
		if s.created.Before(cutoff) {
			log.Printf("[snapshot] 清理过期快照: %s (created=%s)", s.id, s.created.Format(time.RFC3339))
			if err := os.RemoveAll(s.path); err != nil {
				return deleted, fmt.Errorf("删除过期快照 %s 失败: %w", s.id, err)
			}
			deleted = append(deleted, s.id)
		}
	}

	// 3. 容量保留：总占用 > 5GB 时从旧到新删除。
	remaining := make([]snapshotInfo, 0, len(completed))
	for _, s := range completed {
		if !containsStr(deleted, s.id) {
			remaining = append(remaining, s)
		}
	}
	total := int64(0)
	for _, s := range remaining {
		total += s.size
	}
	for total > maxStorageBytes && len(remaining) > 0 {
		oldest := remaining[0]
		if oldest.size > maxStorageBytes && len(remaining) == 1 {
			// 单个快照超容：无可删快照，记警告并停止，避免死循环。
			log.Printf("[snapshot] 警告: 单个快照 %s (%s) 超过容量上限，无法通过清理满足要求",
				oldest.id, formatSize(oldest.size))
			break
		}
		log.Printf("[snapshot] 容量清理: 删除快照 %s (总占用 %s > 5GB)", oldest.id, formatSize(total))
		if err := os.RemoveAll(oldest.path); err != nil {
			return deleted, fmt.Errorf("容量清理删除快照 %s 失败: %w", oldest.id, err)
		}
		deleted = append(deleted, oldest.id)
		total -= oldest.size
		remaining = remaining[1:]
	}
	return deleted, nil
}

type snapshotInfo struct {
	id       string
	path     string
	created  time.Time
	size     int64
	complete bool
}

// listSnapshotDirs 列出项目目录下的全部快照文件夹并读取大小与 manifest 状态。
func listSnapshotDirs(projectDir string) ([]snapshotInfo, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取快照目录失败: %w", err)
	}
	var result []snapshotInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(projectDir, e.Name())
		m, loadErr := LoadManifest(path)
		created := time.Now()
		if loadErr == nil && !m.CreatedAt.IsZero() {
			created = m.CreatedAt
		} else if fi, statErr := os.Stat(path); statErr == nil {
			created = fi.ModTime()
		}
		size, sizeErr := dirSize(path)
		if sizeErr != nil {
			log.Printf("[snapshot] 计算快照 %s 大小失败: %v", e.Name(), sizeErr)
		}
		result = append(result, snapshotInfo{
			id:       e.Name(),
			path:     path,
			created:  created,
			size:     size,
			complete: loadErr == nil && m.Complete,
		})
	}
	return result, nil
}

// dirSize 递归计算目录占用字节数。
func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过不可读文件，不中断统计
		}
		if d.Type().IsRegular() {
			if info, statErr := d.Info(); statErr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total, err
}

func containsStr(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
