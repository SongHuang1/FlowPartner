package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeTrashExecutor 创建带 trashDir 的执行器，返回执行器与工作目录。
func makeTrashExecutor(t *testing.T, trashDir string) (*ToolExecutor, string) {
	t.Helper()
	dir := t.TempDir()
	executor, err := NewToolExecutor(dir, WithTrashDir(trashDir))
	if err != nil {
		t.Fatalf("NewToolExecutor: %v", err)
	}
	return executor, dir
}

func TestTrash_MovesFileToTrash(t *testing.T) {
	executor, dir := makeTrashExecutor(t, t.TempDir())
	trashDir := executor.trashDir

	src := filepath.Join(dir, "old.log")
	os.WriteFile(src, []byte("content"), 0644)

	args := mustArgs(t, map[string]interface{}{"path": "old.log"})
	result := executor.Execute(context.Background(), "s1", "trash", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	// 源文件已不在工作区
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source file should be moved away, err=%v", err)
	}
	// 回收站内有带时间戳前缀的条目
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		t.Fatalf("read trash dir: %v", err)
	}
	var found string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "old.log") && strings.Contains(e.Name(), "__") {
			found = e.Name()
			break
		}
	}
	if found == "" {
		t.Fatalf("expected trashed entry with timestamp prefix in %v", entries)
	}
	// 内容保留
	data, _ := os.ReadFile(filepath.Join(trashDir, found))
	if string(data) != "content" {
		t.Errorf("trashed content mismatch: got %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(trashDir, found+".meta.json")); err != nil {
		t.Errorf("expected meta file to be written: %v", err)
	}
}

func TestTrash_NotConfigured(t *testing.T) {
	executor, dir := makeExecutor(t)
	src := filepath.Join(dir, "keep.txt")
	os.WriteFile(src, []byte("data"), 0644)

	args := mustArgs(t, map[string]interface{}{"path": "keep.txt"})
	result := executor.Execute(context.Background(), "s1", "trash", args)

	if result.Success {
		t.Fatal("expected failure when trash_dir not configured")
	}
	if result.ErrorCode != ErrTrashNotConfigured {
		t.Errorf("expected error code %s, got %s", ErrTrashNotConfigured, result.ErrorCode)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("file must not be moved when trash_dir not configured, err=%v", err)
	}
}

func TestTrash_SourceNotFound(t *testing.T) {
	executor, _ := makeTrashExecutor(t, t.TempDir())
	args := mustArgs(t, map[string]interface{}{"path": "missing.txt"})
	result := executor.Execute(context.Background(), "s1", "trash", args)
	if result.Success {
		t.Fatal("expected failure for missing source")
	}
	trashDir := executor.trashDir
	entries, _ := os.ReadDir(trashDir)
	if len(entries) != 0 {
		t.Errorf("expected no trash entries for missing source, got %d", len(entries))
	}
}

func TestTrash_MovesDirectory(t *testing.T) {
	executor, dir := makeTrashExecutor(t, t.TempDir())
	trashDir := executor.trashDir

	srcDir := filepath.Join(dir, "subdir")
	os.MkdirAll(filepath.Join(srcDir, "nested", "empty"), 0755)
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(srcDir, "nested", "b.txt"), []byte("b"), 0644)

	args := mustArgs(t, map[string]interface{}{"path": "subdir"})
	result := executor.Execute(context.Background(), "s1", "trash", args)
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}

	entries, _ := os.ReadDir(trashDir)
	if len(entries) != 2 { // 1 条目 + 1 meta 文件
		t.Fatalf("expected 1 trashed entry (+1 meta), got %d", len(entries))
	}
	var dest string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".meta.json") {
			continue
		}
		dest = filepath.Join(trashDir, e.Name())
	}
	if dest == "" {
		t.Fatal("expected a trashed directory entry")
	}
	if _, err := os.Stat(filepath.Join(dest, "nested", "b.txt")); err != nil {
		t.Errorf("recursive content missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "nested", "empty")); err != nil {
		t.Errorf("empty dir not preserved: %v", err)
	}
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Errorf("source dir should be moved, err=%v", err)
	}
}

func TestTrash_SameNameNoOverwrite(t *testing.T) {
	executor, dir := makeTrashExecutor(t, t.TempDir())
	trashDir := executor.trashDir

	for i := 0; i < 3; i++ {
		sub := filepath.Join(dir, "batch")
		os.MkdirAll(sub, 0755)
		os.WriteFile(filepath.Join(sub, "same.log"), []byte(string(rune('a'+i))), 0644)

		args := mustArgs(t, map[string]interface{}{"path": filepath.Join("batch", "same.log")})
		result := executor.Execute(context.Background(), "s1", "trash", args)
		if !result.Success {
			t.Fatalf("iteration %d: expected success, got: %s", i, result.Result)
		}
	}

	entries, _ := os.ReadDir(trashDir)
	names := make(map[string]bool)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".meta.json") {
			continue
		}
		if names[e.Name()] {
			t.Errorf("duplicate trash entry name: %s", e.Name())
		}
		names[e.Name()] = true
	}
	if len(names) != 3 {
		t.Errorf("expected 3 distinct entries, got %d", len(names))
	}
}

func TestTrash_BatchAllInsideWorkspace(t *testing.T) {
	executor, dir := makeTrashExecutor(t, t.TempDir())
	trashDir := executor.trashDir

	os.WriteFile(filepath.Join(dir, "a.log"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.log"), []byte("b"), 0644)

	args := mustArgs(t, map[string]interface{}{"paths": []string{"a.log", "b.log"}})
	result := executor.Execute(context.Background(), "s1", "trash", args)
	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	entries, _ := os.ReadDir(trashDir)
	if len(entries) != 4 { // 2 entries + 2 meta files
		t.Errorf("expected 4 entries (2 items + 2 meta), got %d", len(entries))
	}
}

func TestTrash_BatchMixedOutsideWorkspaceRejectedByCheckPath(t *testing.T) {
	executor, dir := makeTrashExecutor(t, t.TempDir())
	outside := filepath.Join(filepath.Dir(dir), "outside.log")
	os.WriteFile(outside, []byte("x"), 0644)
	defer os.Remove(outside)

	args := mustArgs(t, map[string]interface{}{"paths": []string{"a.log", outside}})
	_, _, _, err := executor.CheckPath("trash", args)
	if err == nil {
		t.Fatal("expected error for mixed workspace paths")
	}
}

func TestTrash_CheckPath_OutsideNeedsPermission(t *testing.T) {
	executor, _ := makeTrashExecutor(t, t.TempDir())
	outside := filepath.Join(filepath.Dir(executor.guard.WorkingDir()), "outside.txt")

	args := mustArgs(t, map[string]interface{}{"path": outside})
	needs, raw, resolved, err := executor.CheckPath("trash", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needs {
		t.Error("expected needsPermission=true for outside path")
	}
	if raw != outside || resolved == "" {
		t.Errorf("expected raw=%s resolved non-empty, got raw=%s resolved=%s", outside, raw, resolved)
	}
}

func TestTrash_CheckPath_InsideNoPermission(t *testing.T) {
	executor, _ := makeTrashExecutor(t, t.TempDir())
	args := mustArgs(t, map[string]interface{}{"path": "inside.txt"})
	needs, _, _, err := executor.CheckPath("trash", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needs {
		t.Error("expected no permission needed for inside path")
	}
}

func TestUniqueTrashName_NoColons(t *testing.T) {
	name := uniqueTrashName(t.TempDir(), "file.txt")
	for _, bad := range []string{":", "\\", "/"} {
		if strings.Contains(name, bad) {
			t.Errorf("trash name %q must not contain %q (N7)", name, bad)
		}
	}
}
