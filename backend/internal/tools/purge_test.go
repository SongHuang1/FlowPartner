package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/songhuang/flowpartner/backend/internal/storage"
)

// setupTrashFixture 创建带回收站的执行器，并放入若干条目。
func setupTrashFixture(t *testing.T) (*ToolExecutor, string, string) {
	t.Helper()
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()

	trashDir := t.TempDir()
	executor, _ := makeTrashExecutor(t, trashDir)

	entry1 := filepath.Join(trashDir, "20260101T000000000001Z__1__a.log")
	entry2 := filepath.Join(trashDir, "20260101T000000000002Z__1__b.log")
	os.WriteFile(entry1, []byte("a"), 0644)
	os.WriteFile(entry2, []byte("b"), 0644)
	os.WriteFile(entry1+".meta.json", []byte(`{"original_path":"/x/a.log"}`), 0600)
	return executor, trashDir, entry1
}

func TestPurge_RequiresApproval(t *testing.T) {
	executor, trashDir, _ := setupTrashFixture(t)

	args := mustArgs(t, map[string]interface{}{})
	result := executor.Execute(context.Background(), "s1", "purge", args)

	if result.Success {
		t.Fatal("expected failure without approval")
	}
	// 零删除
	entries, _ := os.ReadDir(trashDir)
	if len(entries) != 3 {
		t.Errorf("expected no deletion without approval, got %d entries", len(entries))
	}
}

func TestPurge_ApprovedClearsAll(t *testing.T) {
	executor, trashDir, _ := setupTrashFixture(t)
	ctx := WithApproval(WithSessionID(WithApprovalID(context.Background(), "approval-1"), "sess-1"))

	args := mustArgs(t, map[string]interface{}{})
	result := executor.Execute(ctx, "s1", "purge", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	entries, _ := os.ReadDir(trashDir)
	if len(entries) != 0 {
		t.Errorf("expected trash dir emptied, got %d entries", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(mustDataDir(t), "trash_audit.log"))
	if err != nil {
		t.Fatalf("expected audit log file: %v", err)
	}
	if !strings.Contains(string(data), "approval-1") || !strings.Contains(string(data), "s1") {
		t.Errorf("audit log missing approval_id/session: %s", string(data))
	}
}

func TestPurge_ApprovedDeletesSingleEntry(t *testing.T) {
	executor, trashDir, entry1 := setupTrashFixture(t)
	ctx := WithApproval(context.Background())

	args := mustArgs(t, map[string]interface{}{"entry": filepath.Base(entry1)})
	result := executor.Execute(ctx, "s1", "purge", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if _, err := os.Stat(entry1); !os.IsNotExist(err) {
		t.Errorf("entry should be deleted, err=%v", err)
	}
	remaining, _ := os.ReadDir(trashDir)
	if len(remaining) != 2 { // b.log + a.log.meta.json
		t.Errorf("expected 2 remaining entries, got %d", len(remaining))
	}
}

func TestPurge_RejectsPathTraversal(t *testing.T) {
	executor, trashDir, _ := setupTrashFixture(t)
	ctx := WithApproval(context.Background())

	for _, entry := range []string{"..", "../x", "a/b", `a\b`, ".", "/abs", "a/.."} {
		args := mustArgs(t, map[string]interface{}{"entry": entry})
		result := executor.Execute(ctx, "s1", "purge", args)
		if result.Success {
			t.Errorf("entry %q should be rejected", entry)
		}
	}
	entries, _ := os.ReadDir(trashDir)
	if len(entries) != 3 {
		t.Errorf("expected nothing deleted after traversal attempts, got %d entries", len(entries))
	}
}

func TestPurge_NotConfigured(t *testing.T) {
	executor, _ := makeExecutor(t)
	ctx := WithApproval(context.Background())
	args := mustArgs(t, map[string]interface{}{})
	result := executor.Execute(ctx, "s1", "purge", args)
	if result.Success {
		t.Fatal("expected failure when trash_dir not configured")
	}
	if result.ErrorCode != ErrTrashNotConfigured {
		t.Errorf("expected %s, got %s", ErrTrashNotConfigured, result.ErrorCode)
	}
}

func TestPurge_CheckPathAlwaysNeedsPermission(t *testing.T) {
	executor, trashDir, _ := setupTrashFixture(t)

	args := mustArgs(t, map[string]interface{}{})
	needs, raw, resolved, err := executor.CheckPath("purge", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needs {
		t.Error("expected needsPermission=true for purge")
	}
	if raw != trashDir || resolved != trashDir {
		t.Errorf("expected raw/resolved = %s, got %s/%s", trashDir, raw, resolved)
	}
}

func TestPurge_CheckPath_RejectsBadEntry(t *testing.T) {
	executor, _, _ := setupTrashFixture(t)
	args := mustArgs(t, map[string]interface{}{"entry": "../../etc/passwd"})
	_, _, _, err := executor.CheckPath("purge", args)
	if err == nil {
		t.Fatal("expected error for path traversal entry")
	}
}

func mustDataDir(t *testing.T) string {
	t.Helper()
	dir, err := storage.DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	return dir
}
