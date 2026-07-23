package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

func setupChatTest() (*ChatHandler, *keystore.KeyStore) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)
	ks.Unlock([]byte("sk-test-api-key"))
	return &ChatHandler{}, ks
}

func TestChat_Unlocked(t *testing.T) {
	_, _ = setupChatTest()

	reqBody, _ := json.Marshal(ChatRequest{Content: "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(string(reqBody)))
	w := httptest.NewRecorder()

	h := &ChatHandler{}
	h.Post(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (agent unavailable), got %d: %s", w.Code, w.Body.String())
	}
}

func TestChat_Locked(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)
	ks.Lock()

	reqBody, _ := json.Marshal(ChatRequest{Content: "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(string(reqBody)))
	w := httptest.NewRecorder()

	h := &ChatHandler{}
	h.Post(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodePermissionDenied {
		t.Errorf("expected code %d, got %d", response.CodePermissionDenied, resp.Code)
	}
}

func TestChat_InvalidJSON(t *testing.T) {
	_, _ = setupChatTest()

	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	h := &ChatHandler{}
	h.Post(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestChat_LoadConversationHistory_Empty(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	convPath := filepath.Join(dir, "conversations.json")
	os.Remove(convPath)

	h := &ChatHandler{}
	history := h.loadConversationHistory()

	if history == nil {
		t.Fatal("loadConversationHistory should return non-nil slice")
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d messages", len(history))
	}
}

func TestChat_GetAgentPort_Default(t *testing.T) {
	keystore.Reset()

	h := &ChatHandler{}
	port := h.getAgentPort()

	if port != 8989 {
		t.Errorf("expected default port 8989, got %d", port)
	}
}

func TestSanitizeError_BearerToken(t *testing.T) {
	err := fmt.Errorf("request failed with Authorization: Bearer sk-secret-key")
	sanitized := sanitizeError(err)

	if strings.Contains(sanitized, "sk-secret-key") {
		t.Error("sanitized error should not contain the API key")
	}
	if sanitized == err.Error() {
		t.Error("sanitized error should be different from original")
	}
}

func TestSanitizeError_APIKey(t *testing.T) {
	err := fmt.Errorf("api_key=sk-1234567890abcdef")
	sanitized := sanitizeError(err)

	if strings.Contains(sanitized, "sk-1234567890abcdef") {
		t.Error("sanitized error should not contain the API key")
	}
}

func TestSanitizeError_NoSensitiveData(t *testing.T) {
	err := fmt.Errorf("connection refused")
	sanitized := sanitizeError(err)

	if sanitized != "connection refused" {
		t.Errorf("expected 'connection refused', got %q", sanitized)
	}
}

func TestSanitizeError_OpenAIKeyFormat(t *testing.T) {
	err := fmt.Errorf("authentication failed for sk-abcdefghijklmnopqrstuvwxyz123456")
	sanitized := sanitizeError(err)

	if strings.Contains(sanitized, "sk-abc") {
		t.Error("sanitized error should not contain OpenAI key format")
	}
}
