package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Reason 快照触发原因。
type Reason string

const (
	ReasonDebounce   Reason = "debounce"   // 变更静默 60s
	ReasonTicker     Reason = "ticker"     // 15min 周期兜底
	ReasonLock       Reason = "lock"       // 系统锁屏 flush
	ReasonManual     Reason = "manual"     // 用户手动触发
	ReasonPreRestore Reason = "prerestore" // 还原前的自动预快照
)

// SkippedFile 记录被跳过的文件及其原因。
type SkippedFile struct {
	Path   string `json:"path"`             // 相对工作区根的路径
	Reason string `json:"reason"`           // secret | too_large | read_error | symlink_restore_failed
	Detail string `json:"detail,omitempty"` // 补充说明（大小、错误信息等）
}

// SymlinkEntry 记录快照捕获到的符号链接本身（不跟随目标）。
type SymlinkEntry struct {
	Path   string `json:"path"`   // 相对工作区根的路径
	Target string `json:"target"` // 链接目标（原始字符串）
}

// Manifest 快照清单。manifest.json 必须最后写入并携带 Complete 标记。
type Manifest struct {
	SnapshotID              string         `json:"snapshot_id"`
	ProjectID               string         `json:"project_id"`
	Reason                  Reason         `json:"reason"`
	CreatedAt               time.Time      `json:"created_at"`
	WorkspaceRoot           string         `json:"workspace_root"`            // 快照时的原始绝对根
	WorkspaceRootNormalized string         `json:"workspace_root_normalized"` // 规范化后的根（还原校验用）
	FileCount               int            `json:"file_count"`
	TotalSizeBytes          int64          `json:"total_size_bytes"`
	Complete                bool           `json:"complete"`
	SkippedFiles            []SkippedFile  `json:"skipped_files"`
	Symlinks                []SymlinkEntry `json:"symlinks"`
}

// NormalizeRoot 规范化工作区根路径，用于 project_id 哈希与还原校验。
// 规则：Abs + Clean + 统一分隔符 + Windows 大小写折叠（整串小写），保持路径稳定。
func NormalizeRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("解析工作区根失败: %w", err)
	}
	clean := filepath.Clean(abs)
	if runtime.GOOS == "windows" {
		// Windows 大小写不敏感：统一小写，避免盘符大小写变化导致 project_id 漂移
		clean = strings.ToLower(clean)
		// 统一分隔符：Clean 不转换正斜杠，手动归一
		clean = strings.ReplaceAll(clean, "/", string(filepath.Separator))
	}
	return clean, nil
}

// ProjectID 由规范化后的工作区根做 SHA256，取前 16 位十六进制。
func ProjectID(normalizedRoot string) string {
	sum := sha256.Sum256([]byte(normalizedRoot))
	return hex.EncodeToString(sum[:])[:16]
}

func NewSnapshotID(now time.Time, dir string) (string, error) {
	base := now.UTC().Format("20060102-150405")
	candidate := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("检查快照目录失败: %w", err)
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
}

// LoadManifest 读取并解析某个快照目录下的 manifest.json。
func LoadManifest(snapshotDir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(snapshotDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("读取 manifest 失败: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败: %w", err)
	}
	return &m, nil
}

// IsComplete 判断快照是否完成（manifest 存在且带 complete 标记）。
func IsComplete(snapshotDir string) bool {
	m, err := LoadManifest(snapshotDir)
	return err == nil && m.Complete
}
