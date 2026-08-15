package keystore

import (
	"sync"
	"testing"
	"time"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
)

func TestKeyStore_Singleton(t *testing.T) {
	Reset()
	ks1 := Instance()
	ks2 := Instance()
	if ks1 != ks2 {
		t.Fatal("Instance() should return the same pointer")
	}
}

func TestKeyStore_Reset(t *testing.T) {
	Reset()
	ks1 := Instance()
	Reset()
	ks2 := Instance()
	if ks1 == ks2 {
		t.Fatal("Reset() should create a new instance")
	}
}

func TestKeyStore_UnlockAndLock(t *testing.T) {
	Reset()
	ks := Instance()

	apiKey := []byte("sk-test-api-key")
	err := ks.Unlock(apiKey)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	if !ks.IsUnlocked() {
		t.Fatal("should be unlocked after Unlock()")
	}

	key, ok := ks.GetKey()
	if !ok {
		t.Fatal("GetKey should return true when unlocked")
	}
	if string(key) != string(apiKey) {
		t.Errorf("GetKey returned wrong key: got %q, want %q", key, apiKey)
	}

	ks.Lock()
	if ks.IsUnlocked() {
		t.Fatal("should be locked after Lock()")
	}

	_, ok = ks.GetKey()
	if ok {
		t.Fatal("GetKey should return false when locked")
	}
}

func TestKeyStore_UnlockEmptyKey(t *testing.T) {
	Reset()
	ks := Instance()

	err := ks.Unlock([]byte{})
	if err == nil {
		t.Fatal("Unlock with empty key should fail")
	}
}

func TestKeyStore_GetKeyReturnsCopy(t *testing.T) {
	Reset()
	ks := Instance()

	apiKey := []byte("sk-test-api-key")
	ks.Unlock(apiKey)

	key1, _ := ks.GetKey()
	key2, _ := ks.GetKey()

	key1[0] = 'X'
	if key2[0] == 'X' {
		t.Fatal("GetKey should return a copy, not a reference")
	}
}

func TestKeyStore_LockClearsMemory(t *testing.T) {
	Reset()
	ks := Instance()

	apiKey := []byte("sk-sensitive-api-key")
	ks.Unlock(apiKey)
	ks.Lock()

	key, ok := ks.GetKey()
	if ok {
		t.Fatal("GetKey should return false after Lock()")
	}
	if key != nil {
		t.Fatal("key should be nil after Lock()")
	}
}

func TestKeyStore_RateLimit(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance
	ks.SetAPIKeyConfigured(true)

	password := []byte("WrongPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))

	for i := 0; i < 5; i++ {
		ks.VerifyPassword(password, encrypted)
	}

	status := ks.GetLockStatus()
	if !status.Locked {
		t.Fatal("should be locked after 5 failed attempts")
	}
	if status.LockedUntil.IsZero() {
		t.Fatal("LockedUntil should be set")
	}
	if time.Now().After(status.LockedUntil) {
		t.Fatal("LockedUntil should be in the future")
	}
	if status.FailedAttempts != 5 {
		t.Errorf("FailedAttempts should be 5, got %d", status.FailedAttempts)
	}
}

func TestKeyStore_VerifyPasswordCorrect(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

	password := []byte("CorrectPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", password)

	ok := ks.VerifyPassword(password, encrypted)
	if !ok {
		t.Fatal("VerifyPassword with correct password should return true")
	}

	status := ks.GetLockStatus()
	if status.FailedAttempts != 0 {
		t.Errorf("FailedAttempts should be 0 after correct password, got %d", status.FailedAttempts)
	}
}

func TestKeyStore_VerifyPasswordWrong(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))

	ok := ks.VerifyPassword([]byte("WrongPass123"), encrypted)
	if ok {
		t.Fatal("VerifyPassword with wrong password should return false")
	}

	status := ks.GetLockStatus()
	if status.FailedAttempts != 1 {
		t.Errorf("FailedAttempts should be 1, got %d", status.FailedAttempts)
	}
}

func TestKeyStore_GetLockStatus_HasAPIKey(t *testing.T) {
	Reset()
	ks := Instance()

	ks.SetAPIKeyConfigured(true)
	status := ks.GetLockStatus()
	if !status.HasAPIKey {
		t.Fatal("HasAPIKey should be true after SetAPIKeyConfigured(true)")
	}

	ks.SetAPIKeyConfigured(false)
	status = ks.GetLockStatus()
	if status.HasAPIKey {
		t.Fatal("HasAPIKey should be false after SetAPIKeyConfigured(false)")
	}
}

func TestKeyStore_ConcurrentAccess(t *testing.T) {
	Reset()
	ks := Instance()

	ks.Unlock([]byte("sk-test-key"))

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			ks.IsUnlocked()
			ks.GetKey()
			ks.GetLockStatus()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestKeyStore_RateLimitExpired(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

	ks.lockedUntil = time.Now().Add(-1 * time.Second)

	apiKey := []byte("sk-test-key")
	err := ks.Unlock(apiKey)
	if err != nil {
		t.Fatalf("Unlock should succeed after rate limit expires: %v", err)
	}
}

func TestKeyStore_SwitchKey(t *testing.T) {
	Reset()
	ks := Instance()

	oldKey := []byte("sk-old-key")
	if err := ks.Unlock(oldKey); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	newKey := []byte("sk-new-key")
	ks.SwitchKey(newKey)

	if !ks.IsUnlocked() {
		t.Fatal("SwitchKey should keep the keystore unlocked")
	}

	key, ok := ks.GetKey()
	if !ok {
		t.Fatal("GetKey should return true after SwitchKey")
	}
	if string(key) != string(newKey) {
		t.Errorf("GetKey returned %q, want %q", key, newKey)
	}

	if status := ks.GetLockStatus(); status.FailedAttempts != 0 {
		t.Errorf("SwitchKey should reset failed attempts, got %d", status.FailedAttempts)
	}
}

func TestKeyStore_TryActivate_Success(t *testing.T) {
	Reset()
	ks := Instance()

	encrypted, err := flowcrypto.Encrypt("sk-activate-key", []byte("CorrectPass123"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	key, err := ks.TryActivate(encrypted, []byte("CorrectPass123"))
	if err != nil {
		t.Fatalf("TryActivate with correct password should succeed: %v", err)
	}
	if string(key) != "sk-activate-key" {
		t.Errorf("TryActivate returned key %q, want %q", key, "sk-activate-key")
	}

	if !ks.IsUnlocked() {
		t.Fatal("keystore should be unlocked after TryActivate")
	}

	status := ks.GetLockStatus()
	if status.FailedAttempts != 0 {
		t.Errorf("FailedAttempts should be 0, got %d", status.FailedAttempts)
	}
}

func TestKeyStore_TryActivate_WrongPassword(t *testing.T) {
	Reset()
	ks := Instance()

	encrypted, err := flowcrypto.Encrypt("test-key", []byte("sk-activate-key"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	key, err := ks.TryActivate(encrypted, []byte("WrongPass123"))
	if err != ErrWrongPassword {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}
	if key != nil {
		t.Fatalf("expected nil key on wrong password, got %q", key)
	}

	if ks.IsUnlocked() {
		t.Fatal("keystore should stay locked after wrong password")
	}

	status := ks.GetLockStatus()
	if status.FailedAttempts != 1 {
		t.Errorf("FailedAttempts should be 1, got %d", status.FailedAttempts)
	}
}

func TestKeyStore_TryActivate_RateLimited(t *testing.T) {
	Reset()
	ks := Instance()

	encrypted, err := flowcrypto.Encrypt("test-key", []byte("sk-activate-key"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		if _, err := ks.TryActivate(encrypted, []byte("WrongPass123")); err != ErrWrongPassword {
			t.Fatalf("attempt %d: expected ErrWrongPassword, got %v", i+1, err)
		}
	}

	_, err = ks.TryActivate(encrypted, []byte("CorrectPass123"))
	if err != ErrRateLimited {
		t.Fatalf("expected ErrRateLimited after 5 failed attempts, got %v", err)
	}

	if ks.LockedUntil().IsZero() {
		t.Fatal("LockedUntil should be set after rate limiting")
	}
	if time.Now().After(ks.LockedUntil()) {
		t.Fatal("LockedUntil should be in the future")
	}
}

func TestKeyStore_TryActivate_SwitchZeroesOldKey(t *testing.T) {
	Reset()
	ks := Instance()

	encrypted, err := flowcrypto.Encrypt("sk-new-after-switch", []byte("CorrectPass123"))
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if err := ks.Unlock([]byte("sk-old-key")); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	key, err := ks.TryActivate(encrypted, []byte("CorrectPass123"))
	if err != nil {
		t.Fatalf("TryActivate failed: %v", err)
	}
	if string(key) != "sk-new-after-switch" {
		t.Errorf("TryActivate returned %q, want %q", key, "sk-new-after-switch")
	}
	if got, _ := ks.GetKey(); string(got) != "sk-new-after-switch" {
		t.Errorf("GetKey returned %q, want new key", got)
	}
}

func TestKeyStore_LockedUntil(t *testing.T) {
	Reset()
	ks := Instance()

	if !ks.LockedUntil().IsZero() {
		t.Fatal("LockedUntil should be zero initially")
	}

	future := time.Now().Add(30 * time.Second)
	ks.lockedUntil = future
	if !ks.LockedUntil().Equal(future) {
		t.Errorf("LockedUntil returned %v, want %v", ks.LockedUntil(), future)
	}
}
