package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

func setupUnlockTest() (*UnlockHandler, *keystore.KeyStore) {
	keystore.Reset()
	ks := keystore.Instance()
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

func TestUnlock_Success(t *testing.T) {
	_, ks := setupUnlockTest()

	password := "TestPass123"
	apiKey := "sk-test-api-key"

	ks.SetAPIKeyConfigured(true)
	saveEncryptedKeyForTest(t, apiKey, password)

	reqBody, _ := json.Marshal(UnlockRequest{Password: password})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	h := &UnlockHandler{}
	h.Post(w, req)

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
	_, ks := setupUnlockTest()
	ks.SetAPIKeyConfigured(true)
	saveEncryptedKeyForTest(t, "sk-test", "CorrectPass123")

	reqBody, _ := json.Marshal(UnlockRequest{Password: "WrongPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	h := &UnlockHandler{}
	h.Post(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUnlock_NoAPIKeyConfigured(t *testing.T) {
	_, _ = setupUnlockTest()

	reqBody, _ := json.Marshal(UnlockRequest{Password: "AnyPass123"})
	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	h := &UnlockHandler{}
	h.Post(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUnlock_InvalidJSON(t *testing.T) {
	_, _ = setupUnlockTest()

	req := httptest.NewRequest(http.MethodPost, "/api/unlock", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	h := &UnlockHandler{}
	h.Post(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLock_Success(t *testing.T) {
	_, ks := setupUnlockTest()
	ks.SetAPIKeyConfigured(true)
	ks.Unlock([]byte("sk-test-key"))

	req := httptest.NewRequest(http.MethodPost, "/api/lock", nil)
	w := httptest.NewRecorder()

	h := &UnlockHandler{}
	h.Lock(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if ks.IsUnlocked() {
		t.Fatal("should be locked after Lock()")
	}
}

func TestStatus_Initial(t *testing.T) {
	_, _ = setupUnlockTest()

	req := httptest.NewRequest(http.MethodGet, "/api/lock_status", nil)
	w := httptest.NewRecorder()

	h := &UnlockHandler{}
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
	_, ks := setupUnlockTest()
	ks.SetAPIKeyConfigured(true)
	ks.Unlock([]byte("sk-test-key"))

	req := httptest.NewRequest(http.MethodGet, "/api/lock_status", nil)
	w := httptest.NewRecorder()

	h := &UnlockHandler{}
	h.Status(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var status keystore.LockStatus
	json.Unmarshal(data, &status)

	if status.Locked {
		t.Fatal("should not be locked after Unlock()")
	}
	if !status.HasAPIKey {
		t.Fatal("HasAPIKey should be true")
	}
}

func TestUnlock_RateLimit(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))
	_ = encrypted

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ks.VerifyPassword([]byte("WrongPass123"), encrypted)
		}()
	}
	wg.Wait()

	status := ks.GetLockStatus()
	if !status.Locked {
		t.Fatal("should be locked after 5 failed attempts")
	}
}
