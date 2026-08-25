package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPathGuard_ResolveRelativePath(t *testing.T) {
	dir := t.TempDir()
	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	resolved, err := g.Resolve("subdir/file.txt")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	expected := filepath.Join(dir, "subdir", "file.txt")
	if resolved != expected {
		t.Errorf("got %q, want %q", resolved, expected)
	}
}

func TestPathGuard_ResolveAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	absPath := filepath.Join(dir, "file.txt")
	resolved, err := g.Resolve(absPath)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved != filepath.Clean(absPath) {
		t.Errorf("got %q, want %q", resolved, filepath.Clean(absPath))
	}
}

func TestPathGuard_ResolveEmptyPath(t *testing.T) {
	g, err := NewPathGuard(t.TempDir())
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	_, err = g.Resolve("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestPathGuard_ValidateInsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	// 创建测试文件
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	if err := g.Validate(filepath.Join(dir, "test.txt")); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestPathGuard_ValidateRelativePathInside(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	if err := g.Validate("test.txt"); err != nil {
		t.Errorf("expected no error for relative path inside workspace, got %v", err)
	}
}

func TestPathGuard_ValidateDotDotEscape(t *testing.T) {
	dir := t.TempDir()
	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	err = g.Validate("../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for dot-dot escape")
	}
	if !contains(err.Error(), "超出工作目录范围") {
		t.Errorf("expected Chinese error message, got %q", err.Error())
	}
}

func TestPathGuard_ValidateOutsideWorkspace(t *testing.T) {
	dir := t.TempDir()
	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	// 使用明确在工作目录外的路径
	parentDir := filepath.Dir(dir)
	outsidePath := filepath.Join(parentDir, "outside.txt")
	err = g.Validate(outsidePath)
	if err == nil {
		t.Fatal("expected error for path outside workspace")
	}
}

func TestPathGuard_ValidateWorkDirItself(t *testing.T) {
	dir := t.TempDir()
	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	// 工作目录本身应该允许
	if err := g.Validate(dir); err != nil {
		t.Errorf("expected no error for workspace root, got %v", err)
	}
}

func TestPathGuard_ValidateNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	// 不存在的文件，词法检查通过即可（写入时会创建）
	if err := g.Validate("new_file.txt"); err != nil {
		t.Errorf("expected no error for nonexistent file inside workspace, got %v", err)
	}
}

func TestPathGuard_ValidateSymlinkInside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows (requires elevated privileges)")
	}

	dir := t.TempDir()
	// 创建指向工作目录内文件的符号链接
	target := filepath.Join(dir, "real.txt")
	os.WriteFile(target, []byte("hello"), 0644)
	link := filepath.Join(dir, "link.txt")
	os.Symlink(target, link)

	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	if err := g.Validate(link); err != nil {
		t.Errorf("expected no error for symlink pointing inside workspace, got %v", err)
	}
}

func TestPathGuard_ValidateSymlinkOutside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows (requires elevated privileges)")
	}

	dir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	os.WriteFile(outsideFile, []byte("secret"), 0644)

	// 创建指向工作目录外文件的符号链接
	link := filepath.Join(dir, "sneaky_link")
	os.Symlink(outsideFile, link)

	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	err = g.Validate(link)
	if err == nil {
		t.Fatal("expected error for symlink pointing outside workspace")
	}
}

func TestPathGuard_EmptyWorkingDir(t *testing.T) {
	_, err := NewPathGuard("")
	if err == nil {
		t.Fatal("expected error for empty working directory")
	}
}

func TestPathGuard_NonexistentWorkingDir(t *testing.T) {
	_, err := NewPathGuard("/nonexistent/path/12345")
	if err == nil {
		t.Fatal("expected error for nonexistent working directory")
	}
}

func TestPathGuard_WorkDirIsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping symlink test on Windows (requires elevated privileges)")
	}

	realDir := t.TempDir()
	linkDir := filepath.Join(t.TempDir(), "link_to_work")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	// 必须真实存在：若目标不存在，Validate 第二层会因 IsNotExist 短路，
	// 穿透符号链接的真实落点检查就不会发生。
	if err := os.WriteFile(filepath.Join(realDir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	g, err := NewPathGuard(linkDir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	// Resolve 的契约是词法解析：返回基于传入工作目录的绝对路径，不解析符号链接。
	resolved, err := g.Resolve("file.txt")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	wantLexical := filepath.Join(linkDir, "file.txt")
	if resolved != wantLexical {
		t.Errorf("got %q, want lexical path %q", resolved, wantLexical)
	}

	// 安全校验必须穿透符号链接：Validate 第二层用 EvalSymlinks 对比
	// workingDirReal（realDir），真实落点在工作区内则放行。
	if err := g.Validate("file.txt"); err != nil {
		t.Errorf("Validate through workdir symlink should pass: %v", err)
	}
}

func TestPathGuard_WindowsCaseInsensitive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping case-insensitive test on non-Windows")
	}

	dir := t.TempDir()
	g, err := NewPathGuard(dir)
	if err != nil {
		t.Fatalf("NewPathGuard: %v", err)
	}

	// Windows 下大小写不同应该视为同一路径
	upperPath := filepath.Join(dir, "FILE.TXT")
	lowerPath := filepath.Join(dir, "file.txt")

	// 两个路径应该都被判定为在工作目录内
	if !g.pathInside(upperPath, g.workingDir) {
		t.Error("expected upper case path to be inside workspace on Windows")
	}
	if !g.pathInside(lowerPath, g.workingDir) {
		t.Error("expected lower case path to be inside workspace on Windows")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
