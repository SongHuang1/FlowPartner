package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

func clearSettingsFile(t *testing.T) {
	t.Helper()
	dir, err := storage.DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	path := filepath.Join(dir, "settings.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove settings.json: %v", err)
	}
}

func TestSettingsHandler_Get_Defaults(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	if data["model"] != "gpt-4" {
		t.Errorf("expected default model 'gpt-4', got %v", data["model"])
	}
	if data["agent_id"] != "default" {
		t.Errorf("expected default agent_id 'default', got %v", data["agent_id"])
	}
}

func TestSettingsHandler_Put_And_Get(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	handler := &SettingsHandler{}

	// PUT 新设置（模型来自激活的模型配置）
	body := `{"model":"gpt-3.5","agent_id":"test-agent","context_window":4096,"working_directory":"/tmp/test","language":"en-US","model_configs":[{"id":"cfg-test","name":"测试","base_url":"https://api.example.com/v1","model_name":"gpt-3.5","encrypted_api_key":"enc-key","temperature":0.7,"timeout_secs":30}],"active_config_id":"cfg-test"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// GET 验证
	req = httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec = httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", rec.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	if data["model"] != "gpt-3.5" {
		t.Errorf("expected model 'gpt-3.5', got %v", data["model"])
	}
	if data["agent_id"] != "test-agent" {
		t.Errorf("expected agent_id 'test-agent', got %v", data["agent_id"])
	}
}

func TestSettingsHandler_Put_InvalidJSON(t *testing.T) {
	handler := &SettingsHandler{}
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSettingsHandler_Put_EmptyModel(t *testing.T) {
	handler := &SettingsHandler{}
	body := `{"model":"","context_window":4096}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSettingsHandler_Put_NegativeContextWindow(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":-1}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (fallback to default), got %d", rec.Code)
	}

	// GET 验证 context_window 回退为默认值 8192
	req = httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec = httptest.NewRecorder()
	handler.Get(rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	data := resp.Data.(map[string]interface{})
	if data["context_window"] != float64(8192) {
		t.Errorf("expected context_window fallback to 8192, got %v", data["context_window"])
	}
}

func TestSettingsHandler_ResponseFormat(t *testing.T) {
	handler := &SettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	var raw map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	required := []string{"code", "message", "data", "timestamp", "request_id"}
	for _, field := range required {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

// TestSettingsHandler_Put_ZeroContextWindow 验证 context_window=0 时回退为默认值
func TestSettingsHandler_Put_ZeroContextWindow(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":0}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (fallback to default) for context_window=0, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_LargeContextWindow 验证极大的 context_window 可以被保存
func TestSettingsHandler_Put_LargeContextWindow(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":999999,"language":"zh-CN"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for large context_window, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSettingsHandler_Put_SpecialCharsInModel 验证 model 字段包含特殊字符
func TestSettingsHandler_Put_SpecialCharsInModel(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	handler := &SettingsHandler{}
	body := `{"model":"gpt-4-turbo-preview#2024","context_window":4096,"language":"zh-CN"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for special chars in model, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSettingsHandler_Put_EmptyBody 验证空请求体被拒绝
func TestSettingsHandler_Put_EmptyBody(t *testing.T) {
	handler := &SettingsHandler{}
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(""))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_OverwriteExisting 验证 PUT 覆盖已有设置
func TestSettingsHandler_Put_OverwriteExisting(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	handler := &SettingsHandler{}

	// 第一次 PUT
	body1 := `{"model":"gpt-4","agent_id":"agent1","context_window":4096,"language":"en-US","model_configs":[{"id":"cfg-1","name":"配置1","base_url":"https://api.example.com/v1","model_name":"gpt-4","encrypted_api_key":"enc-key-1","temperature":0.7,"timeout_secs":30}],"active_config_id":"cfg-1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body1))
	rec := httptest.NewRecorder()
	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first PUT expected 200, got %d", rec.Code)
	}

	// 第二次 PUT（覆盖）
	body2 := `{"model":"gpt-3.5","agent_id":"agent2","context_window":2048,"language":"zh-CN","model_configs":[{"id":"cfg-2","name":"配置2","base_url":"https://api.example.com/v1","model_name":"gpt-3.5","encrypted_api_key":"enc-key-2","temperature":0.7,"timeout_secs":30}],"active_config_id":"cfg-2"}`
	req = httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body2))
	rec = httptest.NewRecorder()
	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("second PUT expected 200, got %d", rec.Code)
	}

	// GET 验证是第二次的值
	req = httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec = httptest.NewRecorder()
	handler.Get(rec, req)

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["model"] != "gpt-3.5" {
		t.Errorf("expected model 'gpt-3.5' after overwrite, got %v", data["model"])
	}
	if data["agent_id"] != "agent2" {
		t.Errorf("expected agent_id 'agent2' after overwrite, got %v", data["agent_id"])
	}
}

// TestSettingsHandler_Get_AfterCorruptedFile 验证文件损坏时返回默认值
func TestSettingsHandler_Get_AfterCorruptedFile(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()

	// 写入损坏的 JSON 到 settings.json
	dir, err := storage.DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	corruptPath := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(corruptPath, []byte("{invalid"), 0600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	// 测试结束后清理，避免污染后续测试
	defer func() {
		if err := os.Remove(corruptPath); err != nil && !os.IsNotExist(err) {
			t.Logf("cleanup failed: %v", err)
		}
	}()

	handler := &SettingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even with corrupted file, got %d", rec.Code)
	}

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	// 应该返回默认值
	if data["model"] != "gpt-4" {
		t.Errorf("expected default model 'gpt-4', got %v", data["model"])
	}
}

// TestDefaultSettings 验证 DefaultSettings 返回正确的默认值
func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	if s.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %q", s.Model)
	}
	if s.AgentID != "default" {
		t.Errorf("expected agent_id 'default', got %q", s.AgentID)
	}
	if s.ContextWindow != 8192 {
		t.Errorf("expected context_window 8192, got %d", s.ContextWindow)
	}
	if s.Language != "zh-CN" {
		t.Errorf("expected language 'zh-CN', got %q", s.Language)
	}
	if s.WorkingDirectory != "" {
		t.Errorf("expected empty working_directory, got %q", s.WorkingDirectory)
	}
	if s.TrashDir != "" {
		t.Errorf("expected empty trash_dir (not configured), got %q", s.TrashDir)
	}
}

func TestValidateTrashDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	base := t.TempDir()
	workDir := filepath.Join(base, "work")
	os.MkdirAll(workDir, 0755)

	valid := []string{
		"",                               // 未配置 = 合法
		filepath.Join(base, "trash"),     // 独立绝对目录
		filepath.Join(workDir, ".trash"), // 工作目录后代（推荐用法）
		filepath.Join(home, ".flowpartner_trash"), // 主目录后代（推荐用法）
	}
	for _, dir := range valid {
		if err := validateTrashDir(dir, workDir); err != nil {
			t.Errorf("expected valid trash_dir %q, got error: %v", dir, err)
		}
	}

	invalid := []string{
		"relative/path",
		workDir,            // 等于工作目录
		base,               // 工作目录祖先
		home,               // 用户主目录
		filepath.Dir(home), // 主目录祖先
	}
	for _, dir := range invalid {
		if err := validateTrashDir(dir, workDir); err == nil {
			t.Errorf("expected error for invalid trash_dir %q", dir)
		}
	}
}

// TestSettingsHandler_Put_TrashDir 验证 trash_dir 可保存与非法值被拒绝
func TestSettingsHandler_Put_TrashDir(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	clearSettingsFile(t)
	handler := &SettingsHandler{}

	base := t.TempDir()
	trashDir := filepath.Join(base, "trash")
	os.MkdirAll(filepath.Dir(trashDir), 0755)

	putSettings := func(t *testing.T, s map[string]interface{}) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.Put(rec, req)
		return rec
	}

	// 合法保存
	rec := putSettings(t, map[string]interface{}{
		"model": "gpt-4", "agent_id": "default", "context_window": 8192,
		"working_directory": base, "trash_dir": trashDir,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid trash_dir, got %d: %s", rec.Code, rec.Body.String())
	}

	// 空字符串（未配置）合法
	rec = putSettings(t, map[string]interface{}{
		"model": "gpt-4", "agent_id": "default", "context_window": 8192,
		"working_directory": base, "trash_dir": "",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty trash_dir, got %d: %s", rec.Code, rec.Body.String())
	}

	// 相对路径被拒绝
	rec = putSettings(t, map[string]interface{}{
		"model": "gpt-4", "agent_id": "default", "context_window": 8192,
		"trash_dir": "relative/trash",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for relative trash_dir, got %d: %s", rec.Code, rec.Body.String())
	}

	// 工作目录本身被拒绝
	rec = putSettings(t, map[string]interface{}{
		"model": "gpt-4", "agent_id": "default", "context_window": 8192,
		"working_directory": base, "trash_dir": base,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for working dir as trash_dir, got %d: %s", rec.Code, rec.Body.String())
	}
}
