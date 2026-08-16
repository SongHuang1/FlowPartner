package handler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/songhuang/flowpartner/backend/internal/storage"
)

// packageTestRoot 包级测试数据根目录，在整个测试进程期间持续存在
// （与 t.TempDir 不同，后者会在单个测试结束后删除，破坏后续依赖共享数据目录的测试）
var packageTestRoot string

// newPersistentTestDir 在包级根目录下创建独立的子目录，测试进程结束前不会被删除
func newPersistentTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(packageTestRoot, "case-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	return filepath.Clean(dir)
}

// TestMain 将数据目录隔离到临时目录，避免与其他测试二进制并行写入 ~/.flowpartner 冲突（Windows 文件锁）
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "flowpartner-handler-test-*")
	if err != nil {
		panic(err)
	}
	packageTestRoot = tmpDir
	storage.SetDataDirForTest(tmpDir)
	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}
