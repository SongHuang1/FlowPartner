package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
