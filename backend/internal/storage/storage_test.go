package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDataDir_CreatesDirectory(t *testing.T) {
	ResetDataDirCache()

	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir() error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat data dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("data dir is not a directory")
	}
}

func TestDataDir_CachesResult(t *testing.T) {
	ResetDataDirCache()

	dir1, err := DataDir()
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}

	dir2, err := DataDir()
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if dir1 != dir2 {
		t.Errorf("cached result differs: %q vs %q", dir1, dir2)
	}
}

func TestValidateFilename_Empty(t *testing.T) {
	if err := validateFilename(""); err != ErrInvalidFilename {
		t.Errorf("expected ErrInvalidFilename for empty, got %v", err)
	}
}

func TestValidateFilename_PathSeparator(t *testing.T) {
	invalid := []string{"foo/bar", `foo\bar`, "../etc/passwd", "..\\windows"}
	for _, name := range invalid {
		if err := validateFilename(name); err != ErrInvalidFilename {
			t.Errorf("expected ErrInvalidFilename for %q, got %v", name, err)
		}
	}
}

func TestValidateFilename_ValidNames(t *testing.T) {
	valid := []string{"settings.json", "conversations.json", "data-backup_v2.json"}
	for _, name := range valid {
		if err := validateFilename(name); err != nil {
			t.Errorf("expected no error for %q, got %v", name, err)
		}
	}
}

func TestWriteJSON_And_ReadJSON(t *testing.T) {
	ResetDataDirCache()

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	src := testData{Name: "test", Value: 42}
	if err := WriteJSON("test_rw.json", src); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var dest testData
	if err := ReadJSON("test_rw.json", &dest); err != nil {
		t.Fatalf("ReadJSON error: %v", err)
	}

	if dest.Name != src.Name || dest.Value != src.Value {
		t.Errorf("data mismatch: got %+v, want %+v", dest, src)
	}
}

func TestReadJSON_NotFound(t *testing.T) {
	ResetDataDirCache()

	var dest map[string]interface{}
	err := ReadJSON("nonexistent.json", &dest)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestReadJSON_ParseError(t *testing.T) {
	ResetDataDirCache()

	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir error: %v", err)
	}
	corruptPath := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corruptPath, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	var dest map[string]interface{}
	err = ReadJSON("corrupt.json", &dest)
	if err == nil {
		t.Error("expected parse error, got nil")
	}
	if err == ErrNotFound {
		t.Error("parse error should not be ErrNotFound")
	}
}

func TestWriteJSON_InvalidFilename(t *testing.T) {
	err := WriteJSON("../etc/passwd", "data")
	if err != ErrInvalidFilename {
		t.Errorf("expected ErrInvalidFilename, got %v", err)
	}
}

func TestReadJSON_InvalidFilename(t *testing.T) {
	var dest interface{}
	err := ReadJSON("../../etc/passwd", &dest)
	if err != ErrInvalidFilename {
		t.Errorf("expected ErrInvalidFilename, got %v", err)
	}
}

func TestWriteJSON_FilePermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permissions not applicable on Windows")
	}

	ResetDataDirCache()

	if err := WriteJSON("test_perm.json", "hello"); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	dir, _ := DataDir()
	info, err := os.Stat(filepath.Join(dir, "test_perm.json"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected permission 0600, got %o", perm)
	}
}

func TestWriteJSON_NoTmpFileLeftBehind(t *testing.T) {
	ResetDataDirCache()

	if err := WriteJSON("test_tmp.json", "data"); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	dir, _ := DataDir()
	tmpPath := filepath.Join(dir, "test_tmp.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temporary file should not exist after successful write")
	}
}

// TestWriteJSON_EmptyFilename 验证空文件名被拒绝
func TestWriteJSON_EmptyFilename(t *testing.T) {
	ResetDataDirCache()
	err := WriteJSON("", "data")
	if err != ErrInvalidFilename {
		t.Errorf("expected ErrInvalidFilename for empty filename, got %v", err)
	}
}

// TestReadJSON_EmptyFilename 验证空文件名被拒绝
func TestReadJSON_EmptyFilename(t *testing.T) {
	ResetDataDirCache()
	var dest interface{}
	err := ReadJSON("", &dest)
	if err != ErrInvalidFilename {
		t.Errorf("expected ErrInvalidFilename for empty filename, got %v", err)
	}
}

// TestWriteJSON_ComplexNestedData 验证复杂嵌套数据结构读写
func TestWriteJSON_ComplexNestedData(t *testing.T) {
	ResetDataDirCache()

	type Inner struct {
		Value []string `json:"value"`
	}
	type Outer struct {
		Name   string            `json:"name"`
		Nested Inner             `json:"nested"`
		Map    map[string]int    `json:"map"`
	}

	src := Outer{
		Name:   "test",
		Nested: Inner{Value: []string{"a", "b", "c"}},
		Map:    map[string]int{"x": 1, "y": 2},
	}

	if err := WriteJSON("test_complex.json", src); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var dest Outer
	if err := ReadJSON("test_complex.json", &dest); err != nil {
		t.Fatalf("ReadJSON error: %v", err)
	}

	if dest.Name != src.Name {
		t.Errorf("name mismatch: got %q, want %q", dest.Name, src.Name)
	}
	if len(dest.Nested.Value) != 3 {
		t.Errorf("nested value length: got %d, want 3", len(dest.Nested.Value))
	}
	if dest.Map["x"] != 1 || dest.Map["y"] != 2 {
		t.Errorf("map mismatch: got %v", dest.Map)
	}
}

// TestValidateFilename_DotDot 验证 .. 路径遍历被阻止
func TestValidateFilename_DotDot(t *testing.T) {
	invalidNames := []string{
		"..",
		"foo/../bar",
		"foo\\..\\bar",
		"..\\bar",
		"../bar",
	}
	for _, name := range invalidNames {
		if err := validateFilename(name); err != ErrInvalidFilename {
			t.Errorf("expected ErrInvalidFilename for %q, got %v", name, err)
		}
	}
}

// TestValidateFilename_SingleDot 验证单点文件名是合法的
func TestValidateFilename_SingleDot(t *testing.T) {
	if err := validateFilename("."); err != nil {
		t.Errorf("expected no error for %q, got %v", ".", err)
	}
}

// TestWriteJSON_ReadAfterWrite 验证写入后能正确读取（基本读写一致性）
// 注意：此测试验证基本的读写一致性，不涉及 crash 场景下的原子性验证
func TestWriteJSON_ReadAfterWrite(t *testing.T) {
	ResetDataDirCache()

	type Data struct {
		Value string `json:"value"`
	}

	// 先写入一个有效值
	src := Data{Value: "original"}
	if err := WriteJSON("test_atomic.json", src); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	// 读取验证
	var dest Data
	if err := ReadJSON("test_atomic.json", &dest); err != nil {
		t.Fatalf("ReadJSON error: %v", err)
	}
	if dest.Value != "original" {
		t.Errorf("expected 'original', got %q", dest.Value)
	}
}

// TestReadJSON_WrongDestinationType 验证读取到不兼容类型时的行为
func TestReadJSON_WrongDestinationType(t *testing.T) {
	ResetDataDirCache()

	// 写入一个字符串值
	WriteJSON("test_type.json", "just a string")

	// 尝试读取到结构体（应该返回解析错误）
	type Struct struct {
		Field string `json:"field"`
	}
	var dest Struct
	err := ReadJSON("test_type.json", &dest)
	if err == nil {
		t.Error("expected error when reading string into struct, got nil")
	}
}

func TestValidSessionID(t *testing.T) {
	valid := []string{"sess_123_abc", "sess-1", "SESS_ABC", "a1_b2-c3"}
	for _, id := range valid {
		if !ValidSessionID(id) {
			t.Errorf("expected %q to be valid", id)
		}
	}
	invalid := []string{"", "../etc/passwd", "a/b", `a\b`, "a.b", "sess id", "sess_中文", strings.Repeat("a", 129)}
	for _, id := range invalid {
		if ValidSessionID(id) {
			t.Errorf("expected %q to be invalid", id)
		}
	}
}

func TestHistoryDir_CreatesDirectory(t *testing.T) {
	ResetDataDirCache()
	dir, err := HistoryDir()
	if err != nil {
		t.Fatalf("HistoryDir error: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat history dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("history dir is not a directory")
	}
}

func TestReadHistory_NotFound(t *testing.T) {
	ResetDataDirCache()
	_, err := ReadHistory("sess_nonexistent_1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestReadHistory_InvalidSessionID(t *testing.T) {
	ResetDataDirCache()
	_, err := ReadHistory("../etc/passwd")
	if err != ErrInvalidFilename {
		t.Errorf("expected ErrInvalidFilename, got %v", err)
	}
}

func TestReadHistory_ParsesJSONL(t *testing.T) {
	ResetDataDirCache()
	historyDir, err := HistoryDir()
	if err != nil {
		t.Fatalf("HistoryDir error: %v", err)
	}
	content := `[{"role":"user","content":"q1"},{"role":"assistant","content":"a1"}]
[{"role":"user","content":"q2"},{"role":"assistant","content":"a2"}]
`
	if err := os.WriteFile(filepath.Join(historyDir, "sess_jsonl_1.json"), []byte(content), 0600); err != nil {
		t.Fatalf("write history file: %v", err)
	}

	msgs, err := ReadHistory("sess_jsonl_1")
	if err != nil {
		t.Fatalf("ReadHistory error: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "q1" {
		t.Errorf("msgs[0] mismatch: %+v", msgs[0])
	}
	if msgs[3].Role != "assistant" || msgs[3].Content != "a2" {
		t.Errorf("msgs[3] mismatch: %+v", msgs[3])
	}
}

func TestReadHistory_SkipsMalformedLines(t *testing.T) {
	ResetDataDirCache()
	historyDir, err := HistoryDir()
	if err != nil {
		t.Fatalf("HistoryDir error: %v", err)
	}
	content := `[{"role":"user","content":"ok"},{"role":"assistant","content":"fine"}]
{broken json
[{"role":"user","content":"after"}]
`
	if err := os.WriteFile(filepath.Join(historyDir, "sess_broken_1.json"), []byte(content), 0600); err != nil {
		t.Fatalf("write history file: %v", err)
	}

	msgs, err := ReadHistory("sess_broken_1")
	if err != nil {
		t.Fatalf("ReadHistory error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (malformed line skipped), got %d", len(msgs))
	}
}

func TestListHistory_SortsByUpdatedAt(t *testing.T) {
	ResetDataDirCache()
	historyDir, err := HistoryDir()
	if err != nil {
		t.Fatalf("HistoryDir error: %v", err)
	}
	// 写入两个会话文件，并调整 mtime 保证排序可验证
	old := `[{"role":"user","content":"old question"},{"role":"assistant","content":"old answer"}]` + "\n"
	new := `[{"role":"user","content":"new question"},{"role":"assistant","content":"new answer"}]` + "\n"
	oldPath := filepath.Join(historyDir, "sess_old_1.json")
	newPath := filepath.Join(historyDir, "sess_new_1.json")
	if err := os.WriteFile(oldPath, []byte(old), 0600); err != nil {
		t.Fatalf("write old history: %v", err)
	}
	if err := os.WriteFile(newPath, []byte(new), 0600); err != nil {
		t.Fatalf("write new history: %v", err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes old: %v", err)
	}

	entries, err := ListHistory()
	if err != nil {
		t.Fatalf("ListHistory error: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 entries, got %d", len(entries))
	}
	// 最新的应排在前面
	if entries[0].SessionID != "sess_new_1" {
		t.Errorf("expected newest session first, got %q", entries[0].SessionID)
	}
	if entries[0].MessageCount != 2 {
		t.Errorf("expected message count 2, got %d", entries[0].MessageCount)
	}
	if entries[0].Title != "new question" {
		t.Errorf("expected title 'new question', got %q", entries[0].Title)
	}
}
