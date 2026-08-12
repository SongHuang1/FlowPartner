package handler

import (
	"os"
	"testing"

	"github.com/songhuang/flowpartner/backend/internal/storage"
)

// TestMain 将数据目录隔离到临时目录，避免与其他测试二进制并行写入 ~/.flowpartner 冲突（Windows 文件锁）
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "flowpartner-handler-test-*")
	if err != nil {
		panic(err)
	}
	storage.SetDataDirForTest(tmpDir)
	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}
