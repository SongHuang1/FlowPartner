package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

func setupUnlockTest(t *testing.T) (*UnlockHandler, *keystore.KeyStore) {
	t.Helper()
	keystore.Reset()
	ks := keystore.Instance()
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	return &UnlockHandler{}, ks
}

func saveEncryptedKeyForTest(t *testing.T, apiKey, password string) {
	t.Helper()
	encrypted, err := flowcrypto.Encrypt(apiKey, []byte(password))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	if err := storage.WriteJSON("settings.json", settings); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}
}

func postUnlock(h *UnlockHandler, password string) *httptest.ResponseRecorder {
	reqBody, _ := json.Marshal(UnlockRequest{Password: password})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)
	return w
}

func decodeLockStatus(t *testing.T, body []byte) keystore.LockStatus {
	t.Helper()
	var resp response.Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	data, _ := json.Marshal(resp.Data)
	var status keystore.LockStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("unmarshal lock status failed: %v", err)
	}
	return status
}

// --- Unlock 基本流程 ---

func TestUnlock_Success(t *testing.T) {
	h, ks := setupUnlockTest(t)

	ks.SetAPIKeyConfigured(true)
	saveEncryptedKeyForTest(t, "sk-test-api-key", "TestPass123")

	w := postUnlock(h, "TestPass123")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodeOK {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestUnlock_WrongPassword(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	saveEncryptedKeyForTest(t, "sk-test", "CorrectPass123")

	w := postUnlock(h, "WrongPass123")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodeWrongPassword {
		t.Errorf("expected code %d, got %d", response.CodeWrongPassword, resp.Code)
	}
}

func TestUnlock_EmptyPassword(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	saveEncryptedKeyForTest(t, "test-key", "CorrectPass123")

	w := postUnlock(h, "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for empty password, got %d", w.Code)
	}
}

func TestUnlock_NoAPIKeyConfigured(t *testing.T) {
	h, _ := setupUnlockTest(t)

	w := postUnlock(h, "AnyPass123")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodeAPIKeyNotConfigured {
		t.Errorf("expected code %d, got %d", response.CodeAPIKeyNotConfigured, resp.Code)
	}
}

func TestUnlock_NoEncryptedKeyInSettings(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)

	if err := storage.WriteJSON("settings.json", DefaultSettings()); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	w := postUnlock(h, "AnyPass123")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when no encrypted key, got %d", w.Code)
	}
}

func TestUnlock_SetsKeyInStore(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)

	apiKey := "sk-unique-test-key-12345"
	saveEncryptedKeyForTest(t, apiKey, "TestPass123")

	w := postUnlock(h, "TestPass123")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	key, ok := ks.GetKey()
	if !ok {
		t.Fatal("KeyStore should have key after unlock")
	}
	if string(key) != apiKey {
		t.Errorf("KeyStore has wrong key: got %q, want %q", string(key), apiKey)
	}
}

func TestUnlock_WrongPassword_IncrementsCounter(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	saveEncryptedKeyForTest(t, "test-key", "CorrectPass123")

	for i := 0; i < 3; i++ {
		postUnlock(h, "WrongPass123")
	}

	status := ks.GetLockStatus()
	if status.FailedAttempts != 3 {
		t.Errorf("FailedAttempts should be 3, got %d", status.FailedAttempts)
	}
}

// --- Unlock 请求校验 ---

func TestUnlock_InvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"not json at all", "invalid json"},
		{"malformed json", "{password: unquoted}"},
		{"empty body", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := setupUnlockTest(t)

			req := httptest.NewRequest(http.MethodPost, "/api/unlock", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			h.Post(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d", tt.name, w.Code)
			}
		})
	}
}

// --- Lock ---

func TestLock_Success(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	ks.Unlock([]byte("sk-test-key"))

	req := httptest.NewRequest(http.MethodPost, "/api/lock", nil)
	w := httptest.NewRecorder()
	h.Lock(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if ks.IsUnlocked() {
		t.Fatal("should be locked after Lock()")
	}
}

func TestLock_WhenAlreadyLocked(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	ks.Lock()

	req := httptest.NewRequest(http.MethodPost, "/api/lock", nil)
	w := httptest.NewRecorder()
	h.Lock(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if ks.IsUnlocked() {
		t.Fatal("should remain locked")
	}
}

// --- Lock status ---

func TestStatus_Initial(t *testing.T) {
	h, _ := setupUnlockTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/lock_status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodeOK {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestStatus_AfterUnlock(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	ks.Unlock([]byte("sk-test-key"))

	req := httptest.NewRequest(http.MethodGet, "/api/lock_status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	status := decodeLockStatus(t, w.Body.Bytes())
	if status.Locked {
		t.Fatal("should not be locked after Unlock()")
	}
	if !status.HasAPIKey {
		t.Fatal("HasAPIKey should be true")
	}
}

func TestStatus_AfterLock(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	ks.Unlock([]byte("sk-test-key"))
	ks.Lock()

	req := httptest.NewRequest(http.MethodGet, "/api/lock_status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	status := decodeLockStatus(t, w.Body.Bytes())
	if !status.Locked {
		t.Fatal("should be locked")
	}
	if !status.HasAPIKey {
		t.Fatal("HasAPIKey should be true")
	}
}

// --- 速率限制（HTTP 层）---

func TestUnlock_RateLimit_ExactThreshold(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	saveEncryptedKeyForTest(t, "test-key", "CorrectPass123")

	// 前 5 次错误密码均返回 401（第 6 次请求起速率限制生效）
	for i := 0; i < 5; i++ {
		w := postUnlock(h, "WrongPass123")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, w.Code)
		}
	}

	// 第 6 次应被速率限制拦截（即使密码正确）
	w := postUnlock(h, "CorrectPass123")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after rate limit, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUnlock_RateLimit_BlocksCorrectPassword 验证速率限制期间即使密码正确也被拒绝。
// 过期后恢复的行为在 keystore 包测试中覆盖（handler 层无法操纵未导出的 lockedUntil）。
func TestUnlock_RateLimit_BlocksCorrectPassword(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))
	for i := 0; i < 5; i++ {
		ks.VerifyPassword([]byte("WrongPass123"), encrypted)
	}

	if status := ks.GetLockStatus(); !status.Locked {
		t.Fatal("should be locked after 5 failed attempts")
	}

	saveEncryptedKeyForTest(t, "test-key", "CorrectPass123")

	w := postUnlock(h, "CorrectPass123")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 during rate limit, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnlock_RateLimitResponseFormat(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))
	for i := 0; i < 5; i++ {
		ks.VerifyPassword([]byte("WrongPass123"), encrypted)
	}

	w := postUnlock(h, "AnyPass123")

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodeUnlockRateLimited {
		t.Errorf("expected code %d, got %d", response.CodeUnlockRateLimited, resp.Code)
	}
}

// --- 响应格式与并发 ---

func TestUnlock_ResponseFormat(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	saveEncryptedKeyForTest(t, "test-key", "TestPass123")

	w := postUnlock(h, "TestPass123")

	var raw map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &raw)

	required := []string{"code", "message", "data", "timestamp", "request_id"}
	for _, field := range required {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

func TestUnlock_ConcurrentUnlock(t *testing.T) {
	h, ks := setupUnlockTest(t)
	ks.SetAPIKeyConfigured(true)
	saveEncryptedKeyForTest(t, "test-key", "TestPass123")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			postUnlock(h, "TestPass123")
		}()
	}
	wg.Wait()
}
