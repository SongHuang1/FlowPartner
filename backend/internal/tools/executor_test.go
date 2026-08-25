package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeExecutor(t *testing.T) (*ToolExecutor, string) {
	t.Helper()
	dir := t.TempDir()
	executor, err := NewToolExecutor(dir)
	if err != nil {
		t.Fatalf("NewToolExecutor: %v", err)
	}
	return executor, dir
}

func mustArgs(t *testing.T, args map[string]interface{}) string {
	t.Helper()
	data, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(data)
}

func TestExecute_UnknownTool(t *testing.T) {
	executor, _ := makeExecutor(t)
	result := executor.Execute(context.Background(), "s1", "nonexistent", "{}")
	if result.Success {
		t.Fatal("expected failure for unknown tool")
	}
	if result.ErrorCode != ErrToolNotFound {
		t.Errorf("expected error code %s, got %s", ErrToolNotFound, result.ErrorCode)
	}
	if !strings.Contains(result.Result, "未找到工具") {
		t.Errorf("expected Chinese error message, got %q", result.Result)
	}
}

func TestExecute_InvalidJSON(t *testing.T) {
	executor, _ := makeExecutor(t)
	result := executor.Execute(context.Background(), "s1", "read", `{not json`)
	if result.Success {
		t.Fatal("expected failure for invalid JSON")
	}
	if result.ErrorCode != ErrToolError {
		t.Errorf("expected error code %s, got %s", ErrToolError, result.ErrorCode)
	}
}

func TestExecute_ReadMissingPath(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"path": "nonexistent.txt"})
	result := executor.Execute(context.Background(), "s1", "read", args)
	if result.Success {
		t.Fatal("expected failure for missing path arg")
	}
}

func TestExecute_WriteMissingArgs(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"path": "test.txt"})
	result := executor.Execute(context.Background(), "s1", "write", args)
	if result.Success {
		t.Fatal("expected failure for missing content arg")
	}
}

func TestExecute_BashMissingCommand(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{})
	result := executor.Execute(context.Background(), "s1", "bash", args)
	if result.Success {
		t.Fatal("expected failure for missing command arg")
	}
}

func TestExecute_EditMissingArgs(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"path": "test.txt"})
	result := executor.Execute(context.Background(), "s1", "edit", args)
	if result.Success {
		t.Fatal("expected failure for missing args")
	}
}

// --- Read 工具测试 ---

func TestRead_ExistingFile(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	args := mustArgs(t, map[string]interface{}{"path": "test.txt"})
	result := executor.Execute(context.Background(), "s1", "read", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if result.Result != "hello world" {
		t.Errorf("got %q, want %q", result.Result, "hello world")
	}
}

func TestRead_NonexistentFile(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"path": "missing.txt"})
	result := executor.Execute(context.Background(), "s1", "read", args)

	if result.Success {
		t.Fatal("expected failure for nonexistent file")
	}
	if !strings.Contains(result.Result, "文件不存在") {
		t.Errorf("expected Chinese error, got %q", result.Result)
	}
}

func TestRead_Directory(t *testing.T) {
	executor, dir := makeExecutor(t)
	subDir := filepath.Join(dir, "subdir")
	os.Mkdir(subDir, 0755)

	args := mustArgs(t, map[string]interface{}{"path": "subdir"})
	result := executor.Execute(context.Background(), "s1", "read", args)

	if result.Success {
		t.Fatal("expected failure for directory")
	}
	if !strings.Contains(result.Result, "路径是目录而非文件") {
		t.Errorf("expected Chinese error, got %q", result.Result)
	}
}

func TestRead_LargeFile(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "large.bin")
	// 创建 >10MB 的文件
	data := make([]byte, 10*1024*1024+1)
	os.WriteFile(testFile, data, 0644)

	args := mustArgs(t, map[string]interface{}{"path": "large.bin"})
	result := executor.Execute(context.Background(), "s1", "read", args)

	if result.Success {
		t.Fatal("expected failure for large file")
	}
	if result.ErrorCode != ErrFileTooLarge {
		t.Errorf("expected error code %s, got %s", ErrFileTooLarge, result.ErrorCode)
	}
}

func TestRead_Truncation(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "long.txt")
	os.WriteFile(testFile, []byte(strings.Repeat("x", 15000)), 0644)

	args := mustArgs(t, map[string]interface{}{"path": "long.txt"})
	result := executor.Execute(context.Background(), "s1", "read", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if !strings.Contains(result.Result, "文件过长，已截断") {
		t.Errorf("expected truncation message, got %q", result.Result)
	}
}

func TestRead_Unicode(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "unicode.txt")
	os.WriteFile(testFile, []byte("中文测试 日本語 🎉"), 0644)

	args := mustArgs(t, map[string]interface{}{"path": "unicode.txt"})
	result := executor.Execute(context.Background(), "s1", "read", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if result.Result != "中文测试 日本語 🎉" {
		t.Errorf("got %q", result.Result)
	}
}

func TestRead_OutsideWorkspace(t *testing.T) {
	executor, _ := makeExecutor(t)
	parentDir := filepath.Dir(executor.guard.WorkingDir())
	outsideFile := filepath.Join(parentDir, "outside.txt")

	args := mustArgs(t, map[string]interface{}{"path": outsideFile})
	result := executor.Execute(context.Background(), "s1", "read", args)

	if result.Success {
		t.Fatal("expected failure for outside workspace")
	}
	if result.ErrorCode != ErrPathOutside {
		t.Errorf("expected error code %s, got %s", ErrPathOutside, result.ErrorCode)
	}
}

func TestRead_AbsolutePathInside(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	args := mustArgs(t, map[string]interface{}{"path": testFile})
	result := executor.Execute(context.Background(), "s1", "read", args)

	if !result.Success {
		t.Fatalf("expected success with absolute path inside workspace, got: %s", result.Result)
	}
}

// --- Write 工具测试 ---

func TestWrite_NewFile(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "output.txt")

	args := mustArgs(t, map[string]interface{}{"path": "output.txt", "content": "hello"})
	result := executor.Execute(context.Background(), "s1", "write", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if !strings.Contains(result.Result, "成功写入") {
		t.Errorf("expected success message, got %q", result.Result)
	}
	data, _ := os.ReadFile(testFile)
	if string(data) != "hello" {
		t.Errorf("file content: got %q, want %q", string(data), "hello")
	}
}

func TestWrite_CreatesParentDirs(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "sub", "dir", "file.txt")

	args := mustArgs(t, map[string]interface{}{"path": "sub/dir/file.txt", "content": "content"})
	result := executor.Execute(context.Background(), "s1", "write", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("expected file to be created with parent dirs")
	}
}

func TestWrite_OverwritesExisting(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "existing.txt")
	os.WriteFile(testFile, []byte("old"), 0644)

	args := mustArgs(t, map[string]interface{}{"path": "existing.txt", "content": "new"})
	result := executor.Execute(context.Background(), "s1", "write", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	data, _ := os.ReadFile(testFile)
	if string(data) != "new" {
		t.Errorf("file content: got %q, want %q", string(data), "new")
	}
}

func TestWrite_Unicode(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "unicode.txt")

	args := mustArgs(t, map[string]interface{}{"path": "unicode.txt", "content": "中文 🎉"})
	result := executor.Execute(context.Background(), "s1", "write", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	data, _ := os.ReadFile(testFile)
	if string(data) != "中文 🎉" {
		t.Errorf("file content: got %q", string(data))
	}
}

func TestWrite_EmptyContent(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "empty.txt")

	args := mustArgs(t, map[string]interface{}{"path": "empty.txt", "content": ""})
	result := executor.Execute(context.Background(), "s1", "write", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	data, _ := os.ReadFile(testFile)
	if len(data) != 0 {
		t.Errorf("expected empty file, got %d bytes", len(data))
	}
}

func TestWrite_OutsideWorkspace(t *testing.T) {
	executor, _ := makeExecutor(t)
	parentDir := filepath.Dir(executor.guard.WorkingDir())
	outsideFile := filepath.Join(parentDir, "outside.txt")

	args := mustArgs(t, map[string]interface{}{"path": outsideFile, "content": "nope"})
	result := executor.Execute(context.Background(), "s1", "write", args)

	if result.Success {
		t.Fatal("expected failure for outside workspace")
	}
	if result.ErrorCode != ErrPathOutside {
		t.Errorf("expected error code %s, got %s", ErrPathOutside, result.ErrorCode)
	}
}

// --- Bash 工具测试 ---

func TestBash_SimpleCommand(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"command": "echo hello"})
	result := executor.Execute(context.Background(), "s1", "bash", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if !strings.Contains(result.Result, "hello") {
		t.Errorf("expected output to contain 'hello', got %q", result.Result)
	}
}

func TestBash_FailingCommand(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"command": "exit 1"})
	result := executor.Execute(context.Background(), "s1", "bash", args)

	if result.Success {
		t.Fatal("expected failure for failing command")
	}
	if !strings.Contains(result.Result, "命令执行失败") {
		t.Errorf("expected Chinese error, got %q", result.Result)
	}
}

func TestBash_WorkingDirectory(t *testing.T) {
	executor, dir := makeExecutor(t)
	// 在工作目录内创建文件
	testFile := filepath.Join(dir, "bash_test.txt")
	os.WriteFile(testFile, []byte("found"), 0644)

	var cmd string
	if isWindows() {
		cmd = "type bash_test.txt"
	} else {
		cmd = "cat bash_test.txt"
	}
	args := mustArgs(t, map[string]interface{}{"command": cmd})
	result := executor.Execute(context.Background(), "s1", "bash", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if !strings.Contains(result.Result, "found") {
		t.Errorf("expected output to contain 'found', got %q", result.Result)
	}
}

func TestBash_Truncation(t *testing.T) {
	executor, _ := makeExecutor(t)
	// 生成超过 10000 字符的输出
	var cmd string
	if isWindows() {
		cmd = "for /L %i in (1,1,10001) do @echo x"
	} else {
		cmd = "seq 1 10001"
	}
	args := mustArgs(t, map[string]interface{}{"command": cmd})
	result := executor.Execute(context.Background(), "s1", "bash", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if !strings.Contains(result.Result, "输出过长，已截断") {
		t.Errorf("expected truncation message, got %q", result.Result[:100])
	}
}

func TestBash_EmptyOutput(t *testing.T) {
	executor, _ := makeExecutor(t)
	var cmd string
	if isWindows() {
		cmd = "rem no output"
	} else {
		cmd = "true"
	}
	args := mustArgs(t, map[string]interface{}{"command": cmd})
	result := executor.Execute(context.Background(), "s1", "bash", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if !strings.Contains(result.Result, "无输出") {
		t.Errorf("expected empty output message, got %q", result.Result)
	}
}

func TestBash_Timeout(t *testing.T) {
	if isWindows() {
		t.Skip("skipping timeout test on Windows (cmd /c timeout requires interactive input)")
	}
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"command": "sleep 60"})
	result := executor.Execute(context.Background(), "s1", "bash", args)

	if result.Success {
		t.Fatal("expected failure for timeout")
	}
	if !strings.Contains(result.Result, "超时") {
		t.Errorf("expected timeout message, got %q", result.Result)
	}
}

func TestBash_CommandWithDotDot(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"command": "cat ../../etc/passwd"})
	result := executor.Execute(context.Background(), "s1", "bash", args)

	if result.Success {
		t.Fatal("expected failure for command with ..")
	}
	if result.ErrorCode != ErrPathOutside {
		t.Errorf("expected error code %s, got %s", ErrPathOutside, result.ErrorCode)
	}
	if !strings.Contains(result.Result, "路径逃逸符号") {
		t.Errorf("expected Chinese error about .., got %q", result.Result)
	}
}

func TestBash_CommandWithAbsolutePath(t *testing.T) {
	executor, _ := makeExecutor(t)
	var cmd string
	if isWindows() {
		cmd = "type C:\\Windows\\System32\\drivers\\etc\\hosts"
	} else {
		cmd = "cat /etc/hosts"
	}
	args := mustArgs(t, map[string]interface{}{"command": cmd})
	result := executor.Execute(context.Background(), "s1", "bash", args)

	if result.Success {
		t.Fatal("expected failure for command with absolute path")
	}
	if result.ErrorCode != ErrPathOutside {
		t.Errorf("expected error code %s, got %s", ErrPathOutside, result.ErrorCode)
	}
	if !strings.Contains(result.Result, "绝对路径") {
		t.Errorf("expected Chinese error about absolute path, got %q", result.Result)
	}
}

func TestBash_CommandWithCdToOutside(t *testing.T) {
	executor, _ := makeExecutor(t)
	var cmd string
	if isWindows() {
		cmd = "cd C:\\Windows && dir"
	} else {
		cmd = "cd /etc && ls"
	}
	args := mustArgs(t, map[string]interface{}{"command": cmd})
	result := executor.Execute(context.Background(), "s1", "bash", args)

	if result.Success {
		t.Fatal("expected failure for cd to outside")
	}
	if result.ErrorCode != ErrPathOutside {
		t.Errorf("expected error code %s, got %s", ErrPathOutside, result.ErrorCode)
	}
	if !strings.Contains(result.Result, "切换到工作目录外") {
		t.Errorf("expected Chinese error about cd, got %q", result.Result)
	}
}

// --- Edit 工具测试 ---

func TestEdit_ExactMatchOnce(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "edit_test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	args := mustArgs(t, map[string]interface{}{
		"path":       "edit_test.txt",
		"old_string": "world",
		"new_string": "Go",
	})
	result := executor.Execute(context.Background(), "s1", "edit", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	if result.Result != "成功替换 1 处" {
		t.Errorf("got %q", result.Result)
	}
	data, _ := os.ReadFile(testFile)
	if string(data) != "hello Go" {
		t.Errorf("file content: got %q, want %q", string(data), "hello Go")
	}
}

func TestEdit_NoMatch(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "edit_test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	args := mustArgs(t, map[string]interface{}{
		"path":       "edit_test.txt",
		"old_string": "nonexistent",
		"new_string": "replacement",
	})
	result := executor.Execute(context.Background(), "s1", "edit", args)

	if result.Success {
		t.Fatal("expected failure for no match")
	}
	if !strings.Contains(result.Result, "未找到匹配内容") {
		t.Errorf("expected Chinese error, got %q", result.Result)
	}
	// 文件不应被修改
	data, _ := os.ReadFile(testFile)
	if string(data) != "hello world" {
		t.Errorf("file should not be modified, got %q", string(data))
	}
}

func TestEdit_MultipleMatches(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "edit_test.txt")
	os.WriteFile(testFile, []byte("abc abc abc"), 0644)

	args := mustArgs(t, map[string]interface{}{
		"path":       "edit_test.txt",
		"old_string": "abc",
		"new_string": "xyz",
	})
	result := executor.Execute(context.Background(), "s1", "edit", args)

	if result.Success {
		t.Fatal("expected failure for multiple matches")
	}
	if result.ErrorCode != ErrEditMatchCount {
		t.Errorf("expected error code %s, got %s", ErrEditMatchCount, result.ErrorCode)
	}
	if !strings.Contains(result.Result, "匹配数 3 大于 1") {
		t.Errorf("expected match count error, got %q", result.Result)
	}
	// 文件不应被修改
	data, _ := os.ReadFile(testFile)
	if string(data) != "abc abc abc" {
		t.Errorf("file should not be modified, got %q", string(data))
	}
}

func TestEdit_EmptyOldString(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "edit_test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	args := mustArgs(t, map[string]interface{}{
		"path":       "edit_test.txt",
		"old_string": "",
		"new_string": "replacement",
	})
	result := executor.Execute(context.Background(), "s1", "edit", args)

	if result.Success {
		t.Fatal("expected failure for empty old_string")
	}
	if !strings.Contains(result.Result, "old_string 不能为空") {
		t.Errorf("expected Chinese error, got %q", result.Result)
	}
}

func TestEdit_MultilineMatch(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "edit_test.txt")
	os.WriteFile(testFile, []byte("line1\nline2\nline3"), 0644)

	args := mustArgs(t, map[string]interface{}{
		"path":       "edit_test.txt",
		"old_string": "line1\nline2",
		"new_string": "new_line1\nnew_line2",
	})
	result := executor.Execute(context.Background(), "s1", "edit", args)

	if !result.Success {
		t.Fatalf("expected success, got: %s", result.Result)
	}
	data, _ := os.ReadFile(testFile)
	if string(data) != "new_line1\nnew_line2\nline3" {
		t.Errorf("file content: got %q", string(data))
	}
}

func TestEdit_NonexistentFile(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{
		"path":       "nonexistent.txt",
		"old_string": "a",
		"new_string": "b",
	})
	result := executor.Execute(context.Background(), "s1", "edit", args)

	if result.Success {
		t.Fatal("expected failure for nonexistent file")
	}
	if !strings.Contains(result.Result, "文件不存在") {
		t.Errorf("expected Chinese error, got %q", result.Result)
	}
}

func TestEdit_OutsideWorkspace(t *testing.T) {
	executor, _ := makeExecutor(t)
	parentDir := filepath.Dir(executor.guard.WorkingDir())
	outsideFile := filepath.Join(parentDir, "outside.txt")

	args := mustArgs(t, map[string]interface{}{
		"path":       outsideFile,
		"old_string": "a",
		"new_string": "b",
	})
	result := executor.Execute(context.Background(), "s1", "edit", args)

	if result.Success {
		t.Fatal("expected failure for outside workspace")
	}
	if result.ErrorCode != ErrPathOutside {
		t.Errorf("expected error code %s, got %s", ErrPathOutside, result.ErrorCode)
	}
}

func TestEdit_AbsolutePathInside(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "edit_test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	args := mustArgs(t, map[string]interface{}{
		"path":       testFile,
		"old_string": "hello",
		"new_string": "world",
	})
	result := executor.Execute(context.Background(), "s1", "edit", args)

	if !result.Success {
		t.Fatalf("expected success with absolute path, got: %s", result.Result)
	}
}
