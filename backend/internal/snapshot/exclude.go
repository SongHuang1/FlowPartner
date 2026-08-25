package snapshot

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// excludedDirNames 默认排除的目录名（任意层级命中即跳过，不递归进入）。
var excludedDirNames = map[string]bool{
	".git":         true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".venv":        true,
	"venv":         true,
	"vendor":       true,
	"__pycache__":  true,
}

// secretPatterns 默认排除的密钥文件模式（basename 匹配）。
// 用户显式开启"包含敏感文件"开关后不再应用。
var secretPatterns = []string{
	".env*",
	"*.key",
	"*.pem",
	"*.p12",
	"credentials.json",
	"id_rsa*",
	"id_ed25519*",
	"*.pfx",
	"*.keystore",
}

// maxFileSizeBytes 单个文件超过该大小则跳过（默认 100MB）。
const maxFileSizeBytes = 100 * 1024 * 1024

// Excluder 封装快照排除规则（目录名、密钥模式、超大文件、储存目录自身）。
type Excluder struct {
	includeSecrets bool
	snapshotRoot   string
}

// NewExcluder 创建排除器。snapshotRoot 允许为空（不额外排除）。
func NewExcluder(includeSecrets bool, snapshotRoot string) *Excluder {
	return &Excluder{includeSecrets: includeSecrets, snapshotRoot: snapshotRoot}
}

// IsExcludedDir 判断目录是否命中排除规则（按 basename）。
func (e *Excluder) IsExcludedDir(dir string) bool {
	if excludedDirNames[filepath.Base(dir)] {
		return true
	}
	if e.snapshotRoot != "" && pathEqualFold(dir, e.snapshotRoot) {
		return true
	}
	return false
}

// ShouldSkipFile 判断文件是否应被跳过；返回跳过原因与详情。
// reason 取值：secret（密钥）、too_large（超大）；排除目录由 walkAndCopy 直接标记为 excluded_dir，不经此判定。
func (e *Excluder) ShouldSkipFile(path string, size int64) (skipped bool, reason, detail string) {
	if !e.includeSecrets && isSecretFile(path) {
		return true, "secret", "敏感文件，默认不纳入快照（可在设置中开启包含）"
	}
	if size > maxFileSizeBytes {
		return true, "too_large", formatSize(size) + " 超过 100MB 上限"
	}
	return false, "", ""
}

// isSecretFile 按 basename 匹配密钥模式（Windows 大小写不敏感）。
func isSecretFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	for _, p := range secretPatterns {
		if matchPattern(name, p) {
			return true
		}
	}
	return false
}

// matchPattern 简单通配：* 匹配任意字符（含空）。模式已统一小写。
func matchPattern(name, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		mid := pattern[1 : len(pattern)-1]
		return len(mid) > 0 && strings.Contains(name, mid)
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, pattern[:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(name, pattern[1:])
	}
	return name == pattern
}

// pathEqualFold 大小写不敏感（Windows）的路径相等比较。
func pathEqualFold(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return strconv.FormatInt(size, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	v := float64(size) / float64(div)
	s := strconv.FormatFloat(v, 'f', 1, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return s + " " + []string{"KB", "MB", "GB"}[exp]
}
