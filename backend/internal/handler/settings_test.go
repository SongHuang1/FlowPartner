package handler

import (
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
	handler := &SettingsHandler{}

	// PUT 新设置
	body := `{"model":"gpt-3.5","agent_id":"test-agent","context_window":4096,"working_directory":"/tmp/test","language":"en-US"}`
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
	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":-1}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
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

// TestSettingsHandler_Put_ZeroContextWindow 验证 context_window=0 被拒绝
func TestSettingsHandler_Put_ZeroContextWindow(t *testing.T) {
	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":0}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for context_window=0, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_LargeContextWindow 验证极大的 context_window 可以被保存
func TestSettingsHandler_Put_LargeContextWindow(t *testing.T) {
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
	handler := &SettingsHandler{}

	// 第一次 PUT
	body1 := `{"model":"gpt-4","agent_id":"agent1","context_window":4096,"language":"en-US"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body1))
	rec := httptest.NewRecorder()
	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first PUT expected 200, got %d", rec.Code)
	}

	// 第二次 PUT（覆盖）
	body2 := `{"model":"gpt-3.5","agent_id":"agent2","context_window":2048,"language":"zh-CN"}`
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
}
