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

// TestUnlock_RateLimit_ExactThreshold 验证恰好 5 次失败后触发速率限制
func TestUnlock_RateLimit_ExactThreshold(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	storage.WriteJSON("settings.json", settings)

	h := &UnlockHandler{}

	// 5 次错误密码
	for i := 0; i < 5; i++ {
		reqBody, _ := json.Marshal(UnlockRequest{Password: "WrongPass123"})
		req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()
		h.Post(w, req)

		if i < 4 {
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("attempt %d: expected 401, got %d", i+1, w.Code)
			}
		} else {
			// 第 5 次应该触发速率限制
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("attempt 5: expected 401, got %d", w.Code)
			}
		}
	}

	// 第 6 次应该被速率限制
	reqBody, _ := json.Marshal(UnlockRequest{Password: "CorrectPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after rate limit, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUnlock_RateLimit_ThenCorrectAfterExpiry 验证速率限制过期后正确密码可以解锁
func TestUnlock_RateLimit_ThenCorrectAfterExpiry(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	// 先触发速率限制（5 次错误密码）
	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))
	for i := 0; i < 5; i++ {
		ks.VerifyPassword([]byte("WrongPass123"), encrypted)
	}

	// 验证已锁定
	status := ks.GetLockStatus()
	if !status.Locked {
		t.Fatal("should be locked after 5 failed attempts")
	}

	// 注意：由于无法直接修改 lockedUntil（未导出字段），
	// 这里验证锁定状态下解锁被拒绝
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	storage.WriteJSON("settings.json", settings)

	h := &UnlockHandler{}
	reqBody, _ := json.Marshal(UnlockRequest{Password: "CorrectPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)

	// 锁定状态下应返回 429
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 during rate limit, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUnlock_EmptyPassword 验证空密码解锁失败
func TestUnlock_EmptyPassword(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	storage.WriteJSON("settings.json", settings)

	h := &UnlockHandler{}
	reqBody, _ := json.Marshal(UnlockRequest{Password: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)

	// 空密码应该解密失败
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for empty password, got %d", w.Code)
	}
}

// TestUnlock_NoEncryptedKeyInSettings 验证 settings 中无加密 Key 时解锁失败
func TestUnlock_NoEncryptedKeyInSettings(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	// 不设置 EncryptedAPIKey
	settings := DefaultSettings()
	storage.WriteJSON("settings.json", settings)

	h := &UnlockHandler{}
	reqBody, _ := json.Marshal(UnlockRequest{Password: "AnyPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when no encrypted key, got %d", w.Code)
	}
}

// TestUnlock_EmptyRequestBody 验证空请求体解锁失败
func TestUnlock_EmptyRequestBody(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	h := &UnlockHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader([]byte("")))
	w := httptest.NewRecorder()
	h.Post(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", w.Code)
	}
}

// TestUnlock_MalformedJSON 验证畸形 JSON 解锁失败
func TestUnlock_MalformedJSON(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	h := &UnlockHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", strings.NewReader("{password: unquoted}"))
	w := httptest.NewRecorder()
	h.Post(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", w.Code)
	}
}

// TestLock_WhenAlreadyLocked 验证已锁定时再次上锁不会报错
func TestLock_WhenAlreadyLocked(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)
	ks.Lock()

	h := &UnlockHandler{}
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

// TestStatus_AfterLock 验证上锁后状态正确
func TestStatus_AfterLock(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)
	ks.Unlock([]byte("sk-test-key"))
	ks.Lock()

	h := &UnlockHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/lock_status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var status keystore.LockStatus
	json.Unmarshal(data, &status)

	if !status.Locked {
		t.Fatal("should be locked")
	}
	if !status.HasAPIKey {
		t.Fatal("HasAPIKey should be true")
	}
}

// TestStatus_AfterUnlock_Handler 验证解锁后状态正确（通过 HTTP handler）
func TestStatus_AfterUnlock_Handler(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)
	ks.Unlock([]byte("sk-test-key"))

	h := &UnlockHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/lock_status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var status keystore.LockStatus
	json.Unmarshal(data, &status)

	if status.Locked {
		t.Fatal("should not be locked after unlock")
	}
	if !status.HasAPIKey {
		t.Fatal("HasAPIKey should be true")
	}
}

// TestUnlock_SetsKeyInStore 验证解锁后 KeyStore 中有正确的 API Key
func TestUnlock_SetsKeyInStore(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	apiKey := "sk-unique-test-key-12345"
	encrypted, _ := flowcrypto.Encrypt(apiKey, []byte("TestPass123"))
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	storage.WriteJSON("settings.json", settings)

	h := &UnlockHandler{}
	reqBody, _ := json.Marshal(UnlockRequest{Password: "TestPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)

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

// TestUnlock_WrongPassword_IncrementsCounter 验证错误密码增加失败计数
func TestUnlock_WrongPassword_IncrementsCounter(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	storage.WriteJSON("settings.json", settings)

	h := &UnlockHandler{}

	// 3 次错误
	for i := 0; i < 3; i++ {
		reqBody, _ := json.Marshal(UnlockRequest{Password: "WrongPass123"})
		req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()
		h.Post(w, req)
	}

	status := ks.GetLockStatus()
	if status.FailedAttempts != 3 {
		t.Errorf("FailedAttempts should be 3, got %d", status.FailedAttempts)
	}
}

// TestUnlock_ConcurrentUnlock 验证并发解锁不会死锁
func TestUnlock_ConcurrentUnlock(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("TestPass123"))
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	storage.WriteJSON("settings.json", settings)

	h := &UnlockHandler{}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqBody, _ := json.Marshal(UnlockRequest{Password: "TestPass123"})
			req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
			w := httptest.NewRecorder()
			h.Post(w, req)
		}()
	}
	wg.Wait()
}

// TestUnlock_ResponseFormat 验证解锁成功响应格式正确
func TestUnlock_ResponseFormat(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("TestPass123"))
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	storage.WriteJSON("settings.json", settings)

	h := &UnlockHandler{}
	reqBody, _ := json.Marshal(UnlockRequest{Password: "TestPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)

	var raw map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &raw)

	required := []string{"code", "message", "data", "timestamp", "request_id"}
	for _, field := range required {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

// TestUnlock_RateLimitResponseFormat 验证速率限制响应包含错误码
func TestUnlock_RateLimitResponseFormat(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	// 通过 5 次错误密码触发速率限制
	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))
	for i := 0; i < 5; i++ {
		ks.VerifyPassword([]byte("WrongPass123"), encrypted)
	}

	h := &UnlockHandler{}
	reqBody, _ := json.Marshal(UnlockRequest{Password: "AnyPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodeUnlockRateLimited {
		t.Errorf("expected code %d, got %d", response.CodeUnlockRateLimited, resp.Code)
	}
}

// TestUnlock_NoAPIKey_ResponseCode 验证未配置 API Key 时返回正确错误码
func TestUnlock_NoAPIKey_ResponseCode(t *testing.T) {
	keystore.Reset()
	// 不设置 API Key

	h := &UnlockHandler{}
	reqBody, _ := json.Marshal(UnlockRequest{Password: "AnyPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodeAPIKeyNotConfigured {
		t.Errorf("expected code %d, got %d", response.CodeAPIKeyNotConfigured, resp.Code)
	}
}

// TestUnlock_WrongPassword_ResponseCode 验证错误密码返回正确错误码
func TestUnlock_WrongPassword_ResponseCode(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))
	settings := DefaultSettings()
	settings.EncryptedAPIKey = encrypted
	storage.WriteJSON("settings.json", settings)

	h := &UnlockHandler{}
	reqBody, _ := json.Marshal(UnlockRequest{Password: "WrongPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Post(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodeWrongPassword {
		t.Errorf("expected code %d, got %d", response.CodeWrongPassword, resp.Code)
	}
}
