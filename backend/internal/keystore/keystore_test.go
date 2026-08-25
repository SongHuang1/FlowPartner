package keystore

import (
	"sync"
	"testing"
	"time"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
)

// newTestKeyStore 返回一个全新的 KeyStore（等价于进程启动时的单例初始状态）。
func newTestKeyStore() *KeyStore {
	Reset()
	return Instance()
}

// --- 生命周期 ---

func TestKeyStore_Singleton(t *testing.T) {
	newTestKeyStore()
	ks1 := Instance()
	ks2 := Instance()
	if ks1 != ks2 {
		t.Fatal("Instance() should return the same pointer")
	}
}

func TestKeyStore_Reset(t *testing.T) {
	newTestKeyStore()
	ks1 := Instance()
	Reset()
	ks2 := Instance()
	if ks1 == ks2 {
		t.Fatal("Reset() should create a new instance")
	}
}

// --- 初始状态 ---

func TestKeyStore_IsUnlocked_InitialState(t *testing.T) {
	ks := newTestKeyStore()

	if ks.IsUnlocked() {
		t.Fatal("initial state should be locked")
	}
}

func TestKeyStore_GetLockStatus_InitialState(t *testing.T) {
	ks := newTestKeyStore()

	status := ks.GetLockStatus()
	if !status.Locked {
		t.Fatal("initial state should be locked")
	}
	if status.HasAPIKey {
		t.Fatal("initial state should not have API key")
	}
	if status.FailedAttempts != 0 {
		t.Fatal("initial FailedAttempts should be 0")
	}
}

// --- Unlock / Lock / GetKey ---

func TestKeyStore_UnlockAndLock(t *testing.T) {
	ks := newTestKeyStore()

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

func TestKeyStore_Unlock_EmptyOrNilKey(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
	}{
		{"nil key", nil},
		{"empty byte slice", []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ks := newTestKeyStore()
			if err := ks.Unlock(tt.key); err == nil {
				t.Fatalf("Unlock with %s should fail", tt.name)
			}
		})
	}
}

func TestKeyStore_LockClearsMemory(t *testing.T) {
	ks := newTestKeyStore()

	originalKey := []byte("sk-original-key-12345")
	if err := ks.Unlock(originalKey); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	copy1, ok := ks.GetKey()
	if !ok || copy1 == nil {
		t.Fatal("GetKey should return valid key when unlocked")
	}

	ks.Lock()
	key, ok := ks.GetKey()
	if ok || key != nil {
		t.Fatal("GetKey should return nil, false after Lock()")
	}
	if len(copy1) != len(originalKey) {
		t.Fatalf("copy length changed: got %d, want %d", len(copy1), len(originalKey))
	}
}

func TestKeyStore_Lock_Idempotent(t *testing.T) {
	ks := newTestKeyStore()

	ks.Lock()
	ks.Lock()
	ks.Lock()

	if ks.IsUnlocked() {
		t.Fatal("should remain locked after multiple Lock() calls")
	}
}

func TestKeyStore_GetKey_ModifyCopyDoesNotAffectOriginal(t *testing.T) {
	ks := newTestKeyStore()

	originalKey := []byte("sk-original-key-12345")
	ks.Unlock(originalKey)

	copy1, _ := ks.GetKey()
	copy2, _ := ks.GetKey()

	for i := range copy1 {
		copy1[i] = 0
	}

	if string(copy2) != string(originalKey) {
		t.Errorf("modifying copy1 should not affect copy2, got %q", copy2)
	}

	internalKey, _ := ks.GetKey()
	if string(internalKey) != string(originalKey) {
		t.Errorf("modifying copy should not affect internal key, got %q", internalKey)
	}
}

// --- 速率限制 ---

func TestKeyStore_RateLimit_Exactly5Attempts(t *testing.T) {
	ks := newTestKeyStore()
	ks.SetAPIKeyConfigured(true)

	password := []byte("WrongPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))

	// 4 次失败不应触发速率限制
	for i := 0; i < 4; i++ {
		ks.VerifyPassword(password, encrypted)
	}
	status := ks.GetLockStatus()
	if !status.LockedUntil.IsZero() {
		t.Fatal("rate limit should not be active after 4 failed attempts")
	}

	// 第 5 次失败触发速率限制
	ks.VerifyPassword(password, encrypted)
	status = ks.GetLockStatus()
	if !status.Locked {
		t.Fatal("should be locked after 5 failed attempts")
	}
	if status.LockedUntil.IsZero() {
		t.Fatal("LockedUntil should be set after 5 failed attempts")
	}
	if time.Now().After(status.LockedUntil) {
		t.Fatal("LockedUntil should be in the future")
	}
	if status.FailedAttempts != 5 {
		t.Errorf("FailedAttempts should be 5, got %d", status.FailedAttempts)
	}
}

func TestKeyStore_RateLimit_ResetOnCorrectPassword(t *testing.T) {
	ks := newTestKeyStore()

	correctPassword := []byte("CorrectPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", correctPassword)

	for i := 0; i < 4; i++ {
		ks.VerifyPassword([]byte("WrongPass123"), encrypted)
	}

	status := ks.GetLockStatus()
	if status.FailedAttempts != 4 {
		t.Errorf("FailedAttempts should be 4, got %d", status.FailedAttempts)
	}

	ok := ks.VerifyPassword(correctPassword, encrypted)
	if !ok {
		t.Fatal("correct password should succeed")
	}

	status = ks.GetLockStatus()
	if status.FailedAttempts != 0 {
		t.Errorf("FailedAttempts should be 0 after correct password, got %d", status.FailedAttempts)
	}
}

func TestKeyStore_RateLimit_LockedUntilDuration(t *testing.T) {
	ks := newTestKeyStore()
	ks.SetAPIKeyConfigured(true)

	password := []byte("WrongPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))

	for i := 0; i < 5; i++ {
		ks.VerifyPassword(password, encrypted)
	}

	status := ks.GetLockStatus()
	expectedUnlock := time.Now().Add(30 * time.Second)

	diff := status.LockedUntil.Sub(expectedUnlock)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("LockedUntil should be ~30s from now, got %v (expected ~%v)", status.LockedUntil, expectedUnlock)
	}
}

// TestKeyStore_RateLimit_6Attempts 验证超过阈值后第 6 次调用被速率限制阻止，计数保持为 5。
func TestKeyStore_RateLimit_6Attempts(t *testing.T) {
	ks := newTestKeyStore()
	ks.SetAPIKeyConfigured(true)

	password := []byte("WrongPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))

	for i := 0; i < 6; i++ {
		ks.VerifyPassword(password, encrypted)
	}

	status := ks.GetLockStatus()
	if status.LockedUntil.IsZero() {
		t.Fatal("rate limit should still be active after 6 failed attempts")
	}
	if status.FailedAttempts != 5 {
		t.Errorf("FailedAttempts should be 5 (6th attempt blocked by rate limit), got %d", status.FailedAttempts)
	}
}

func TestKeyStore_RateLimit_ExpireAndRetry(t *testing.T) {
	ks := newTestKeyStore()
	ks.SetAPIKeyConfigured(true)

	ks.lockedUntil = time.Now().Add(-1 * time.Second)

	err := ks.Unlock([]byte("sk-test-key"))
	if err != nil {
		t.Fatalf("Unlock should succeed after rate limit expires: %v", err)
	}
	if !ks.IsUnlocked() {
		t.Fatal("should be unlocked after rate limit expires")
	}
}

func TestKeyStore_RecordFailedAttempt_Increment(t *testing.T) {
	ks := newTestKeyStore()

	status := ks.GetLockStatus()
	if status.FailedAttempts != 0 {
		t.Fatalf("initial FailedAttempts should be 0, got %d", status.FailedAttempts)
	}

	ks.RecordFailedAttempt()
	status = ks.GetLockStatus()
	if status.FailedAttempts != 1 {
		t.Errorf("after 1 attempt: FailedAttempts should be 1, got %d", status.FailedAttempts)
	}

	ks.RecordFailedAttempt()
	status = ks.GetLockStatus()
	if status.FailedAttempts != 2 {
		t.Errorf("after 2 attempts: FailedAttempts should be 2, got %d", status.FailedAttempts)
	}
}

func TestKeyStore_RecordFailedAttempt_TriggersRateLimit(t *testing.T) {
	ks := newTestKeyStore()

	// 4 次不触发
	for i := 0; i < 4; i++ {
		ks.RecordFailedAttempt()
	}
	status := ks.GetLockStatus()
	if !status.LockedUntil.IsZero() {
		t.Fatal("rate limit should not be active after 4 attempts")
	}

	// 第 5 次触发
	ks.RecordFailedAttempt()
	status = ks.GetLockStatus()
	if status.LockedUntil.IsZero() {
		t.Fatal("rate limit should be active after 5 attempts")
	}

	expectedUnlock := time.Now().Add(30 * time.Second)
	diff := status.LockedUntil.Sub(expectedUnlock)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("LockedUntil should be ~30s from now, got %v (expected ~%v)", status.LockedUntil, expectedUnlock)
	}
}

func TestKeyStore_RecordFailedAttempt_BlocksVerifyPassword(t *testing.T) {
	ks := newTestKeyStore()

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))

	for i := 0; i < 5; i++ {
		ks.RecordFailedAttempt()
	}

	ok := ks.VerifyPassword([]byte("CorrectPass123"), encrypted)
	if ok {
		t.Fatal("VerifyPassword should return false during rate limit period")
	}
}

func TestKeyStore_RecordFailedAttempt_UnlockBlocked(t *testing.T) {
	ks := newTestKeyStore()

	for i := 0; i < 5; i++ {
		ks.RecordFailedAttempt()
	}

	err := ks.Unlock([]byte("sk-test-key"))
	if err == nil {
		t.Fatal("Unlock should fail during rate limit period")
	}
}

func TestKeyStore_VerifyPassword_BlockedDuringRateLimit_ExpiredRecovers(t *testing.T) {
	ks := newTestKeyStore()

	correctPassword := []byte("CorrectPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", correctPassword)

	ks.lockedUntil = time.Now().Add(-1 * time.Second)

	ok := ks.VerifyPassword(correctPassword, encrypted)
	if !ok {
		t.Fatal("VerifyPassword should succeed once rate limit expired")
	}
}

// --- VerifyPassword 基本行为 ---

func TestKeyStore_VerifyPasswordCorrect(t *testing.T) {
	ks := newTestKeyStore()

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
	ks := newTestKeyStore()

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

// --- 锁状态标志 ---

func TestKeyStore_GetLockStatus_HasAPIKey(t *testing.T) {
	ks := newTestKeyStore()

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

func TestKeyStore_SetAPIKeyConfigured_Idempotent(t *testing.T) {
	ks := newTestKeyStore()

	ks.SetAPIKeyConfigured(true)
	ks.SetAPIKeyConfigured(true)
	status := ks.GetLockStatus()
	if !status.HasAPIKey {
		t.Fatal("HasAPIKey should be true")
	}

	ks.SetAPIKeyConfigured(false)
	ks.SetAPIKeyConfigured(false)
	status = ks.GetLockStatus()
	if status.HasAPIKey {
		t.Fatal("HasAPIKey should be false")
	}
}

// --- 并发 ---

func TestKeyStore_ConcurrentAccess(t *testing.T) {
	ks := newTestKeyStore()

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

// TestKeyStore_ConcurrentUnlockLock 验证并发 Unlock/Lock 不会死锁。
func TestKeyStore_ConcurrentUnlockLock(t *testing.T) {
	ks := newTestKeyStore()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			ks.Unlock([]byte("sk-test-key"))
		}()
		go func() {
			defer wg.Done()
			ks.Lock()
		}()
	}
	wg.Wait()
}

func TestKeyStore_ConcurrentVerifyPassword(t *testing.T) {
	ks := newTestKeyStore()

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				ks.VerifyPassword([]byte("WrongPass123"), encrypted)
			} else {
				ks.VerifyPassword([]byte("CorrectPass123"), encrypted)
			}
		}(i)
	}
	wg.Wait()
}

// --- SwitchKey / TryActivate ---

func TestKeyStore_SwitchKey(t *testing.T) {
	ks := newTestKeyStore()

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
	ks := newTestKeyStore()

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
	ks := newTestKeyStore()

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
	ks := newTestKeyStore()

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
	ks := newTestKeyStore()

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

// --- LockedUntil 访问器 ---

func TestKeyStore_LockedUntil(t *testing.T) {
	ks := newTestKeyStore()

	if !ks.LockedUntil().IsZero() {
		t.Fatal("LockedUntil should be zero initially")
	}

	future := time.Now().Add(30 * time.Second)
	ks.lockedUntil = future
	if !ks.LockedUntil().Equal(future) {
		t.Errorf("LockedUntil returned %v, want %v", ks.LockedUntil(), future)
	}
}
