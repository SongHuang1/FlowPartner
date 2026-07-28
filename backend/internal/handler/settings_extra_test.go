package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

// TestSettingsHandler_Put_BaseURL 验证 Base URL 字段可以被保存
func TestSettingsHandler_Put_BaseURL(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","base_url":"https://api.openai.com/v1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["base_url"] != "https://api.openai.com/v1" {
		t.Errorf("expected base_url 'https://api.openai.com/v1', got %v", data["base_url"])
	}
}

// TestSettingsHandler_Put_ModelName 验证 ModelName 字段可以被保存
func TestSettingsHandler_Put_ModelName(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","model_name":"gpt-4-turbo"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["model_name"] != "gpt-4-turbo" {
		t.Errorf("expected model_name 'gpt-4-turbo', got %v", data["model_name"])
	}
}

// TestSettingsHandler_Put_SystemPrompt 验证 SystemPrompt 字段可以被保存
func TestSettingsHandler_Put_SystemPrompt(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","system_prompt":"你是一个专业的编程助手。"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["system_prompt"] != "你是一个专业的编程助手。" {
		t.Errorf("expected system_prompt '你是一个专业的编程助手。', got %v", data["system_prompt"])
	}
}

// TestSettingsHandler_Put_Temperature 验证 Temperature 字段可以被保存
func TestSettingsHandler_Put_Temperature(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","temperature":1.5}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["temperature"] != 1.5 {
		t.Errorf("expected temperature 1.5, got %v", data["temperature"])
	}
}

// TestSettingsHandler_Put_TemperatureBoundaryZero 验证 Temperature=0.0 可以被保存（边界值）
func TestSettingsHandler_Put_TemperatureBoundaryZero(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","temperature":0.0}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for temperature=0.0, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSettingsHandler_Put_TemperatureBoundaryMax 验证 Temperature=2.0 可以被保存（边界值）
func TestSettingsHandler_Put_TemperatureBoundaryMax(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","temperature":2.0}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for temperature=2.0, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSettingsHandler_Put_TemperatureTooHigh 验证 Temperature > 2.0 被拒绝
func TestSettingsHandler_Put_TemperatureTooHigh(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","temperature":2.1}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for temperature=2.1, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_TemperatureNegative 验证 Temperature < 0 被拒绝
func TestSettingsHandler_Put_TemperatureNegative(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","temperature":-0.1}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for temperature=-0.1, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_CloseBehaviorMinimize 验证 close_behavior="minimize" 可以被保存
func TestSettingsHandler_Put_CloseBehaviorMinimize(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","close_behavior":"minimize"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSettingsHandler_Put_CloseBehaviorQuit 验证 close_behavior="quit" 可以被保存
func TestSettingsHandler_Put_CloseBehaviorQuit(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","close_behavior":"quit"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSettingsHandler_Put_CloseBehaviorAsk 验证 close_behavior="ask" 可以被保存
func TestSettingsHandler_Put_CloseBehaviorAsk(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","close_behavior":"ask"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSettingsHandler_Put_CloseBehaviorInvalid 验证无效的 close_behavior 被拒绝
func TestSettingsHandler_Put_CloseBehaviorInvalid(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","close_behavior":"invalid_value"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid close_behavior, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_CloseRemembered 验证 close_remembered 字段可以被保存
func TestSettingsHandler_Put_CloseRemembered(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","close_remembered":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["close_remembered"] != true {
		t.Errorf("expected close_remembered true, got %v", data["close_remembered"])
	}
}

// TestSettingsHandler_Put_WindowState 验证窗口状态字段可以被保存
func TestSettingsHandler_Put_WindowState(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","window_x":200,"window_y":150,"window_width":1400,"window_height":900}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["window_x"] != float64(200) {
		t.Errorf("expected window_x 200, got %v", data["window_x"])
	}
	if data["window_width"] != float64(1400) {
		t.Errorf("expected window_width 1400, got %v", data["window_width"])
	}
}

// TestSettingsHandler_Put_SidebarState 验证侧边栏状态字段可以被保存
func TestSettingsHandler_Put_SidebarState(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","sidebar_visible":false,"sidebar_view":"settings"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["sidebar_visible"] != false {
		t.Errorf("expected sidebar_visible false, got %v", data["sidebar_visible"])
	}
	if data["sidebar_view"] != "settings" {
		t.Errorf("expected sidebar_view 'settings', got %v", data["sidebar_view"])
	}
}

// TestSettingsHandler_Put_BaseURLNotHTTP 验证非 HTTP/HTTPS 的 Base URL 被拒绝
func TestSettingsHandler_Put_BaseURLNotHTTP(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","base_url":"ftp://example.com"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-HTTP base_url, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_BaseURLLoopback 验证 loopback 地址被拒绝（SSRF 防护）
func TestSettingsHandler_Put_BaseURLLoopback(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","base_url":"http://127.0.0.1:8080/v1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for loopback URL, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_BaseURLLocalhost 验证 localhost 被拒绝（SSRF 防护）
func TestSettingsHandler_Put_BaseURLLocalhost(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","base_url":"http://localhost:8080/v1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for localhost URL, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_BaseURLPrivateIP 验证私有 IP 被拒绝（SSRF 防护）
func TestSettingsHandler_Put_BaseURLPrivateIP(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","base_url":"http://192.168.1.1:8080/v1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for private IP URL, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_BaseURLMetadataService 验证云服务元数据地址被拒绝（SSRF 防护）
func TestSettingsHandler_Put_BaseURLMetadataService(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","base_url":"http://169.254.169.254/latest/meta-data"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for metadata service URL, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_APIKeyWithPassword 验证 API Key + 密码可以加密保存
func TestSettingsHandler_Put_APIKeyWithPassword(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"sk-test-key-12345","password":"TestPass123"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 验证 KeyStore 已解锁
	ks := keystore.Instance()
	if !ks.IsUnlocked() {
		t.Fatal("KeyStore should be unlocked after setting API key")
	}

	key, ok := ks.GetKey()
	if !ok {
		t.Fatal("GetKey should return true after setting API key")
	}
	if string(key) != "sk-test-key-12345" {
		t.Errorf("GetKey returned wrong key: got %q", string(key))
	}

	// 验证文件中的 API Key 是加密的
	dir, _ := storage.DataDir()
	data, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	var saved Settings
	json.Unmarshal(data, &saved)

	if saved.EncryptedAPIKey == "" {
		t.Fatal("EncryptedAPIKey should not be empty")
	}
	if strings.Contains(string(data), "sk-test-key-12345") {
		t.Error("plaintext API key should not appear in saved file")
	}
}

// TestSettingsHandler_Put_APIKeyWithoutPassword 验证 API Key 无密码时被拒绝
func TestSettingsHandler_Put_APIKeyWithoutPassword(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"sk-test-key-12345"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for API key without password, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_APIKeyWeakPassword 验证 API Key + 弱密码被拒绝
func TestSettingsHandler_Put_APIKeyWeakPassword(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"sk-test-key-12345","password":"weak"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for weak password, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_APIKeyEmptyPassword 验证 API Key + 空密码被拒绝
func TestSettingsHandler_Put_APIKeyEmptyPassword(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"sk-test-key-12345","password":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty password, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_EmptyLanguage 验证空 language 被拒绝
func TestSettingsHandler_Put_EmptyLanguage(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty language, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_WhitespaceModel 验证空白 model 被拒绝
func TestSettingsHandler_Put_WhitespaceModel(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"   ","context_window":4096,"language":"zh-CN"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for whitespace-only model, got %d", rec.Code)
	}
}

// TestSettingsHandler_Get_AllNewFields 验证 GET 返回所有新字段的默认值
func TestSettingsHandler_Get_AllNewFields(t *testing.T) {
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
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	// 验证新字段默认值
	if data["base_url"] != "https://api.openai.com/v1" {
		t.Errorf("expected default base_url, got %v", data["base_url"])
	}
	if data["model_name"] != "gpt-4" {
		t.Errorf("expected default model_name 'gpt-4', got %v", data["model_name"])
	}
	if data["system_prompt"] != "你是一个有帮助的 AI 助手。" {
		t.Errorf("expected default system_prompt, got %v", data["system_prompt"])
	}
	if data["temperature"] != 0.7 {
		t.Errorf("expected default temperature 0.7, got %v", data["temperature"])
	}
	if data["close_behavior"] != "ask" {
		t.Errorf("expected default close_behavior 'ask', got %v", data["close_behavior"])
	}
	if data["close_remembered"] != false {
		t.Errorf("expected default close_remembered false, got %v", data["close_remembered"])
	}
	if data["window_x"] != float64(100) {
		t.Errorf("expected default window_x 100, got %v", data["window_x"])
	}
	if data["window_width"] != float64(1200) {
		t.Errorf("expected default window_width 1200, got %v", data["window_width"])
	}
	if data["sidebar_visible"] != true {
		t.Errorf("expected default sidebar_visible true, got %v", data["sidebar_visible"])
	}
	if data["sidebar_view"] != "conversation" {
		t.Errorf("expected default sidebar_view 'conversation', got %v", data["sidebar_view"])
	}
}

// TestSettingsHandler_Put_AllNewFields 验证 PUT 所有新字段
func TestSettingsHandler_Put_AllNewFields(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{
		"model": "gpt-4",
		"agent_id": "test-agent",
		"context_window": 8192,
		"working_directory": "/tmp",
		"language": "en-US",
		"base_url": "https://api.deepseek.com/v1",
		"model_name": "deepseek-chat",
		"system_prompt": "You are a helpful assistant.",
		"temperature": 0.5,
		"close_behavior": "minimize",
		"close_remembered": true,
		"window_x": 50,
		"window_y": 60,
		"window_width": 1024,
		"window_height": 768,
		"sidebar_visible": false,
		"sidebar_view": "settings"
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// GET 验证
	req = httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec = httptest.NewRecorder()
	handler.Get(rec, req)

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["base_url"] != "https://api.deepseek.com/v1" {
		t.Errorf("expected base_url, got %v", data["base_url"])
	}
	if data["model_name"] != "deepseek-chat" {
		t.Errorf("expected model_name, got %v", data["model_name"])
	}
	if data["temperature"] != 0.5 {
		t.Errorf("expected temperature 0.5, got %v", data["temperature"])
	}
	if data["close_behavior"] != "minimize" {
		t.Errorf("expected close_behavior 'minimize', got %v", data["close_behavior"])
	}
}

// TestSettingsHandler_Put_PreservesAPIKey 验证 PUT 不修改 API Key 时保留已有加密 Key
func TestSettingsHandler_Put_PreservesAPIKey(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	// 先设置 API Key
	encrypted, _ := flowcrypto.Encrypt("sk-existing-key", []byte("TestPass123"))
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	storage.WriteJSON("settings.json", settings)

	// PUT 只修改 model
	handler := &SettingsHandler{}
	body := `{"model":"gpt-3.5","context_window":4096,"language":"zh-CN"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 验证加密 Key 被保留
	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	if data["encrypted_api_key"] != encrypted {
		t.Errorf("encrypted_api_key should be preserved when not explicitly set")
	}
}

// TestIsStrongPassword 验证密码强度校验函数
func TestIsStrongPassword(t *testing.T) {
	tests := []struct {
		password string
		expected bool
	}{
		{"Abcdefg1", true},       // 最小有效长度
		{"Abcdefgh1", true},      // 9 位有效密码
		{"ABCDEFGH1", false},     // 无小写
		{"abcdefgh1", false},     // 无大写
		{"Abcdefgh", false},      // 无数字
		{"Ab1", false},           // 太短
		{"", false},              // 空密码
		{"A1b", false},           // 太短但满足其他条件
		{"Abcdefg1!", true},      // 含特殊字符也有效
		{"12345678", false},      // 只有数字
		{"Password123", true},    // 常见有效密码
		{"aB1aB1aB1", true},      // 重复模式但满足条件
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			result := isStrongPassword(tt.password)
			if result != tt.expected {
				t.Errorf("isStrongPassword(%q) = %v, want %v", tt.password, result, tt.expected)
			}
		})
	}
}

// TestIsInternalURL 验证 SSRF 防护函数
func TestIsInternalURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"http://127.0.0.1:8080/v1", true},
		{"http://localhost:8080/v1", true},
		{"http://192.168.1.1:8080/v1", true},
		{"http://10.0.0.1:8080/v1", true},
		{"http://172.16.0.1:8080/v1", true},
		{"http://169.254.169.254/latest", true},
		{"http://metadata.google.internal/", true},
		{"https://api.openai.com/v1", false},
		{"https://api.deepseek.com/v1", false},
		{"https://example.com/api", false},
		{"not-a-url", true}, // 解析失败视为不安全
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := isInternalURL(tt.url)
			if result != tt.expected {
				t.Errorf("isInternalURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

// TestContainsString 验证 containsString 辅助函数
func TestContainsString(t *testing.T) {
	tests := []struct {
		slice    []string
		item     string
		expected bool
	}{
		{[]string{"minimize", "quit", "ask"}, "minimize", true},
		{[]string{"minimize", "quit", "ask"}, "quit", true},
		{[]string{"minimize", "quit", "ask"}, "ask", true},
		{[]string{"minimize", "quit", "ask"}, "invalid", false},
		{[]string{}, "anything", false},
		{[]string{"only"}, "only", true},
		{[]string{"only"}, "other", false},
	}

	for _, tt := range tests {
		result := containsString(tt.slice, tt.item)
		if result != tt.expected {
			t.Errorf("containsString(%v, %q) = %v, want %v", tt.slice, tt.item, result, tt.expected)
		}
	}
}

// TestSettingsHandler_Put_LinkLocalAddress 验证链路本地地址被拒绝（SSRF 防护）
func TestSettingsHandler_Put_LinkLocalAddress(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","base_url":"http://169.254.1.1:8080/v1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for link-local address, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_LoopbackIPv6 验证 IPv6 loopback 被拒绝（SSRF 防护）
func TestSettingsHandler_Put_LoopbackIPv6(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","base_url":"http://[::1]:8080/v1"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for IPv6 loopback, got %d", rec.Code)
	}
}

// TestSettingsHandler_Put_SaveAPIKey_FullFlow 验证 API Key 保存完整流程
// 流程：前端发送 api_key + 密码 → 后端加密存储 → KeyStore 解锁 → 可解密验证
func TestSettingsHandler_Put_SaveAPIKey_FullFlow(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)
	keystore.Reset()

	handler := &SettingsHandler{}
	apiKey := "sk-test-full-flow-key-12345"
	password := "StrongPass1"

	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"` + apiKey + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})

	// 验证 encrypted_api_key 已设置且不为空
	encrypted, ok := data["encrypted_api_key"].(string)
	if !ok || encrypted == "" {
		t.Fatal("encrypted_api_key should be set and non-empty")
	}

	// 验证 KeyStore 已解锁
	ks := keystore.Instance()
	if !ks.IsUnlocked() {
		t.Fatal("KeyStore should be unlocked after saving API Key")
	}

	// 验证 API Key 可从 KeyStore 获取
	keyBytes, found := ks.GetKey()
	if !found {
		t.Fatal("API Key should be retrievable from KeyStore")
	}
	if string(keyBytes) != apiKey {
		t.Errorf("retrieved API Key mismatch: got %q, want %q", string(keyBytes), apiKey)
	}

	// 验证加密数据可解密
	decrypted, err := flowcrypto.Decrypt(encrypted, []byte(password))
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if decrypted != apiKey {
		t.Errorf("decrypted API Key mismatch: got %q, want %q", decrypted, apiKey)
	}

	// 验证 LockStatus 显示已解锁
	status := ks.GetLockStatus()
	if status.Locked {
		t.Error("LockStatus.Locked should be false after saving API Key")
	}
	if !status.HasAPIKey {
		t.Error("LockStatus.HasAPIKey should be true after saving API Key")
	}
}

// TestSettingsHandler_Put_SaveAPIKey_PreservesExistingKey 验证不发送 api_key 时保留已有密钥
func TestSettingsHandler_Put_SaveAPIKey_PreservesExistingKey(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)
	keystore.Reset()

	handler := &SettingsHandler{}

	// 先保存 API Key
	body1 := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"sk-original-key","password":"StrongPass1"}`
	req1 := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body1))
	rec1 := httptest.NewRecorder()
	handler.Put(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("first PUT failed: %d", rec1.Code)
	}

	// 获取第一次保存的 encrypted_api_key
	var resp1 response.Response
	json.Unmarshal(rec1.Body.Bytes(), &resp1)
	encrypted1 := resp1.Data.(map[string]interface{})["encrypted_api_key"].(string)

	// 第二次 PUT 不发送 api_key（只更新 model）
	body2 := `{"model":"gpt-3.5","context_window":4096,"language":"zh-CN"}`
	req2 := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body2))
	rec2 := httptest.NewRecorder()
	handler.Put(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("second PUT failed: %d: %s", rec2.Code, rec2.Body.String())
	}

	// 验证 encrypted_api_key 被保留
	var resp2 response.Response
	json.Unmarshal(rec2.Body.Bytes(), &resp2)
	encrypted2 := resp2.Data.(map[string]interface{})["encrypted_api_key"].(string)

	if encrypted2 != encrypted1 {
		t.Errorf("encrypted_api_key should be preserved: got %q, want %q", encrypted2, encrypted1)
	}
}

// TestSettingsHandler_Put_SaveAPIKey_RequiresPassword 验证保存 API Key 时强制要求密码
func TestSettingsHandler_Put_SaveAPIKey_RequiresPassword(t *testing.T) {
	storage.ResetDataDirCache()
	clearSettingsFile(t)

	handler := &SettingsHandler{}
	body := `{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"sk-test-key"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Put(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
