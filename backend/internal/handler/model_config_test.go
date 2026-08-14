package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

func setupTestStorage(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	storage.SetDataDirForTest(dir)
	keystore.Reset()
}

func TestModelConfig_Create(t *testing.T) {
	setupTestStorage(t)

	body := `{"name":"Test Config","base_url":"https://api.openai.com/v1","model_name":"gpt-4","temperature":0.7,"response_format":"text","timeout_secs":30}`
	req := httptest.NewRequest(http.MethodPost, "/api/model_configs", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()

	h := &ModelConfigHandler{}
	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp["data"].(map[string]interface{})
	if data["name"] != "Test Config" {
		t.Errorf("name: got %v, want Test Config", data["name"])
	}
	if data["id"] == "" {
		t.Error("expected non-empty id")
	}
}

func TestModelConfig_CreateValidationError(t *testing.T) {
	setupTestStorage(t)

	tests := []struct {
		name   string
		body   string
		errMsg string
	}{
		{"empty name", `{"name":"","base_url":"https://api.openai.com/v1","model_name":"gpt-4"}`, "name cannot be empty"},
		{"empty model_name", `{"name":"Test","base_url":"https://api.openai.com/v1","model_name":""}`, "model_name cannot be empty"},
		{"internal url", `{"name":"Test","base_url":"http://localhost:8080","model_name":"gpt-4"}`, "internal/private"},
		{"bad temperature", `{"name":"Test","base_url":"https://api.openai.com/v1","model_name":"gpt-4","temperature":5}`, "temperature"},
		{"invalid response_format", `{"name":"Test","base_url":"https://api.openai.com/v1","model_name":"gpt-4","response_format":"xml"}`, "response_format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/model_configs", bytes.NewReader([]byte(tt.body)))
			w := httptest.NewRecorder()

			h := &ModelConfigHandler{}
			h.Create(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
			if !strings.Contains(w.Body.String(), tt.errMsg) {
				t.Errorf("expected error containing %q, got %s", tt.errMsg, w.Body.String())
			}
		})
	}
}

func TestModelConfig_Delete(t *testing.T) {
	setupTestStorage(t)

	body := `{"name":"To Delete","base_url":"https://api.openai.com/v1","model_name":"gpt-4"}`
	req := httptest.NewRequest(http.MethodPost, "/api/model_configs", bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	h := &ModelConfigHandler{}
	h.Create(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp["data"].(map[string]interface{})
	id := data["id"].(string)

	req = httptest.NewRequest(http.MethodDelete, "/api/model_configs/"+id, nil)
	w = httptest.NewRecorder()
	h.Delete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestModelConfig_ActivateRateLimited(t *testing.T) {
	dir := t.TempDir()
	storage.SetDataDirForTest(dir)
	keystore.Reset()
	ks := keystore.Instance()

	for i := 0; i < 5; i++ {
		ks.RecordFailedAttempt()
	}

	status := ks.GetLockStatus()
	if !status.Locked {
		t.Fatal("expected KeyStore to be locked after 5 failed attempts")
	}
	if status.FailedAttempts != 5 {
		t.Fatalf("expected 5 failed attempts, got %d", status.FailedAttempts)
	}
}

func TestSettings_Migration(t *testing.T) {
	setupTestStorage(t)

	oldSettings := `{
		"model": "gpt-4",
		"base_url": "https://api.openai.com/v1",
		"model_name": "gpt-4",
		"encrypted_api_key": "old-encrypted-key",
		"temperature": 0.5
	}`
	storage.WriteJSON("settings.json", json.RawMessage(oldSettings))

	settings := LoadSettings()

	if len(settings.ModelConfigs) != 1 {
		t.Fatalf("expected 1 migrated config, got %d", len(settings.ModelConfigs))
	}

	cfg := settings.ModelConfigs[0]
	if cfg.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("base_url: got %q", cfg.BaseURL)
	}
	if cfg.ModelName != "gpt-4" {
		t.Errorf("model_name: got %q", cfg.ModelName)
	}
	if cfg.Temperature != 0.5 {
		t.Errorf("temperature: got %f", cfg.Temperature)
	}
	if settings.ActiveConfigID != cfg.ID {
		t.Errorf("active_config_id: got %q, want %q", settings.ActiveConfigID, cfg.ID)
	}
}

func TestSettings_MigrationSkippedWhenNewFormat(t *testing.T) {
	setupTestStorage(t)

	newSettings := `{
		"model_configs": [
			{"id": "custom-1", "name": "Custom", "base_url": "https://api.deepseek.com/v1", "model_name": "deepseek-chat", "temperature": 0.3}
		],
		"active_config_id": "custom-1"
	}`
	storage.WriteJSON("settings.json", json.RawMessage(newSettings))

	settings := LoadSettings()

	if len(settings.ModelConfigs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(settings.ModelConfigs))
	}
	if settings.ModelConfigs[0].ID != "custom-1" {
		t.Errorf("config id: got %q, want custom-1", settings.ModelConfigs[0].ID)
	}
}

func TestSettings_DeriveFlatFields(t *testing.T) {
	setupTestStorage(t)

	settings := `{
		"model_configs": [
			{"id": "cfg-1", "name": "Active", "base_url": "https://api.openai.com/v1", "model_name": "gpt-4o", "encrypted_api_key": "enc-key-1"},
			{"id": "cfg-2", "name": "Inactive", "base_url": "https://api.deepseek.com/v1", "model_name": "deepseek-chat", "encrypted_api_key": "enc-key-2"}
		],
		"active_config_id": "cfg-1"
	}`
	storage.WriteJSON("settings.json", json.RawMessage(settings))

	s := LoadSettings()

	if s.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("derived base_url: got %q", s.BaseURL)
	}
	if s.ModelName != "gpt-4o" {
		t.Errorf("derived model_name: got %q", s.ModelName)
	}
	if s.EncryptedAPIKey != "enc-key-1" {
		t.Errorf("derived encrypted_api_key: got %q", s.EncryptedAPIKey)
	}
}

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://api.openai.com/v1", false},
		{"valid http", "http://localhost:11434/api", true},
		{"no scheme", "api.openai.com", true},
		{"internal ip", "http://192.168.1.1/api", true},
		{"empty", "", false},
		{"localhost", "http://localhost:8080", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBaseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBaseURL(%q): err=%v, wantErr=%v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestMergeModelConfigs(t *testing.T) {
	existing := []ModelConfig{
		{ID: "a", Name: "Config A", BaseURL: "https://a.com"},
		{ID: "b", Name: "Config B", BaseURL: "https://b.com"},
	}

	incoming := []ModelConfig{
		{ID: "b", Name: "Config B Updated", BaseURL: "https://b-new.com"},
		{ID: "c", Name: "Config C", BaseURL: "https://c.com"},
	}

	result := mergeModelConfigs(existing, incoming)

	if len(result) != 3 {
		t.Fatalf("expected 3 configs, got %d", len(result))
	}

	configMap := make(map[string]ModelConfig)
	for _, cfg := range result {
		configMap[cfg.ID] = cfg
	}

	if configMap["a"].Name != "Config A" {
		t.Errorf("config a should be unchanged")
	}
	if configMap["b"].Name != "Config B Updated" {
		t.Errorf("config b should be updated, got %s", configMap["b"].Name)
	}
	if configMap["c"].Name != "Config C" {
		t.Errorf("config c should be added")
	}
}

func TestSettings_PutMerge(t *testing.T) {
	setupTestStorage(t)

	initial := `{
		"model_configs": [
			{"id": "existing-1", "name": "Existing", "base_url": "https://api.openai.com/v1", "model_name": "gpt-4"}
		],
		"active_config_id": "existing-1"
	}`
	storage.WriteJSON("settings.json", json.RawMessage(initial))

	putBody := `{
		"model": "gpt-4",
		"context_window": 8192,
		"language": "zh-CN",
		"model_configs": [
			{"id": "existing-1", "name": "Updated", "base_url": "https://api.deepseek.com/v1", "model_name": "deepseek-chat"},
			{"id": "new-1", "name": "New", "base_url": "https://api.agnes-ai.cn/v1", "model_name": "agnes-2.0-flash"}
		]
	}`

	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader([]byte(putBody)))
	w := httptest.NewRecorder()
	h := &SettingsHandler{}
	h.Put(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := resp["data"].(map[string]interface{})
	configs, _ := data["model_configs"].([]interface{})

	if len(configs) != 2 {
		t.Fatalf("expected 2 configs after merge, got %d", len(configs))
	}
}


