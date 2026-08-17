package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PathGuard 校验目标路径是否在工作目录范围内。
// 所有工具（包括只读的 read）的目标路径都必须通过此校验。
type PathGuard struct {
	workingDir     string // 工作目录绝对路径（已 Clean）
	workingDirReal string // 工作目录真实路径（EvalSymlinks 后）
	caseSensitive  bool   // 路径比较是否大小写敏感
}

// NewPathGuard 创建 PathGuard。workingDir 必须是已解析的绝对路径。
// 若 workingDir 为空，调用方应回退到用户主目录后再传入。
func NewPathGuard(workingDir string) (*PathGuard, error) {
	if workingDir == "" {
		return nil, fmt.Errorf("工作目录不能为空")
	}

	absDir, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("解析工作目录失败: %w", err)
	}
	cleanDir := filepath.Clean(absDir)

	realDir, err := filepath.EvalSymlinks(cleanDir)
	if err != nil {
		// 工作目录不存在或无法解析符号链接——先检查目录是否存在，
		// 若不存在则用 Clean 路径作为基准（新目录会在写入时创建）
		if _, statErr := os.Stat(cleanDir); statErr != nil {
			return nil, fmt.Errorf("工作目录不存在且无法解析: %w", err)
		}
		// 存在但 EvalSymlinks 失败（权限等）——拒绝
		return nil, fmt.Errorf("无法校验工作目录路径安全性: %w", err)
	}

	return &PathGuard{
		workingDir:     cleanDir,
		workingDirReal: realDir,
		caseSensitive:  runtime.GOOS != "windows",
	}, nil
}

// WorkingDir 返回工作目录的 Clean 绝对路径。
func (g *PathGuard) WorkingDir() string {
	return g.workingDir
}

// Resolve 将目标路径解析为绝对路径。相对路径相对于工作目录解析。
// 返回清理后的绝对路径，不进行安全性校验（需配合 Validate 使用）。
func (g *PathGuard) Resolve(targetPath string) (string, error) {
	if targetPath == "" {
		return "", fmt.Errorf("路径不能为空")
	}

	// 如果是相对路径，相对于工作目录解析
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(g.workingDir, targetPath)
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("解析路径失败: %w", err)
	}

	return filepath.Clean(absPath), nil
}

// Validate 校验目标路径是否在工作目录范围内。
// 执行双层检查：词法检查（Clean 后不逃逸）+ 真实路径检查（EvalSymlinks 后不逃逸）。
func (g *PathGuard) Validate(targetPath string) error {
	resolved, err := g.Resolve(targetPath)
	if err != nil {
		return err
	}

	// 第一层：词法检查——Clean 后的路径必须在工作目录下
	if !g.pathInside(resolved, g.workingDir) {
		return fmt.Errorf("操作被拒绝：目标路径 %q 超出工作目录范围", targetPath)
	}

	// 第二层：真实路径检查——如果目标路径存在，解析符号链接后仍必须在工作目录内
	realPath, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在——词法检查通过即可（写入时会创建）
			return nil
		}
		return fmt.Errorf("无法校验路径安全性: %w", err)
	}

	if !g.pathInside(realPath, g.workingDirReal) {
		return fmt.Errorf("操作被拒绝：目标路径 %q（解析符号链接后）超出工作目录范围", targetPath)
	}

	return nil
}

// ValidateBashWorkDir 校验 bash 工具的工作目录（即工作目录本身）。
func (g *PathGuard) ValidateBashWorkDir() error {
	// bash 工具的工作目录就是工作目录本身，无需额外校验
	return nil
}

// pathInside 检查 child 是否在 parent 目录下（含自身）。
// Windows 下大小写不敏感比较。
func (g *PathGuard) pathInside(child, parent string) bool {
	if g.caseSensitive {
		return strings.HasPrefix(child, parent+string(filepath.Separator)) || child == parent
	}
	// Windows: 大小写不敏感
	sep := string(filepath.Separator)
	// 确保 parent 以分隔符结尾（除非是根目录）
	parentWithSep := parent
	if !strings.HasSuffix(parent, sep) {
		parentWithSep = parent + sep
	}
	// 使用 strings.EqualFold 逐字符比较
	if strings.EqualFold(child, parent) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(child), strings.ToLower(parentWithSep))
}
