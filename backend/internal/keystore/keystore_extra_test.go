package keystore

import (
	"sync"
	"testing"
	"time"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
)

// TestKeyStore_RateLimit_Exactly5Attempts 验证恰好 5 次失败后触发速率限制
func TestKeyStore_RateLimit_Exactly5Attempts(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance
	ks.SetAPIKeyConfigured(true)

	password := []byte("WrongPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))

	// 4 次失败不应触发速率限制（LockedUntil 应为零值）
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
	if status.LockedUntil.IsZero() {
		t.Fatal("rate limit should be active after 5 failed attempts")
	}
	if status.FailedAttempts != 5 {
		t.Errorf("FailedAttempts should be 5, got %d", status.FailedAttempts)
	}
}

// TestKeyStore_RateLimit_ResetOnCorrectPassword 验证正确密码后失败计数重置
func TestKeyStore_RateLimit_ResetOnCorrectPassword(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

	correctPassword := []byte("CorrectPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", correctPassword)

	// 4 次失败
	for i := 0; i < 4; i++ {
		ks.VerifyPassword([]byte("WrongPass123"), encrypted)
	}

	status := ks.GetLockStatus()
	if status.FailedAttempts != 4 {
		t.Errorf("FailedAttempts should be 4, got %d", status.FailedAttempts)
	}

	// 1 次成功
	ok := ks.VerifyPassword(correctPassword, encrypted)
	if !ok {
		t.Fatal("correct password should succeed")
	}

	status = ks.GetLockStatus()
	if status.FailedAttempts != 0 {
		t.Errorf("FailedAttempts should be 0 after correct password, got %d", status.FailedAttempts)
	}
}

// TestKeyStore_RateLimit_LockedUntilDuration 验证锁定时间为 30 秒
func TestKeyStore_RateLimit_LockedUntilDuration(t *testing.T) {
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
	expectedUnlock := time.Now().Add(30 * time.Second)

	// 允许 1 秒误差
	diff := status.LockedUntil.Sub(expectedUnlock)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("LockedUntil should be ~30s from now, got %v (expected ~%v)", status.LockedUntil, expectedUnlock)
	}
}

// TestKeyStore_RateLimit_UnlockBlocked 验证速率限制期间 Unlock 被阻止
func TestKeyStore_RateLimit_UnlockBlocked(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance
	ks.SetAPIKeyConfigured(true)

	// 手动设置锁定状态
	ks.lockedUntil = time.Now().Add(30 * time.Second)

	err := ks.Unlock([]byte("sk-test-key"))
	if err == nil {
		t.Fatal("Unlock should fail during rate limit period")
	}
}

// TestKeyStore_VerifyPassword_BlockedDuringRateLimit 验证速率限制期间 VerifyPassword 返回 false
func TestKeyStore_VerifyPassword_BlockedDuringRateLimit(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

	// 手动设置锁定状态
	ks.lockedUntil = time.Now().Add(30 * time.Second)

	correctPassword := []byte("CorrectPass123")
	encrypted, _ := flowcrypto.Encrypt("test-key", correctPassword)

	// 即使密码正确，在锁定期间也应返回 false
	ok := ks.VerifyPassword(correctPassword, encrypted)
	if ok {
		t.Fatal("VerifyPassword should return false during rate limit period")
	}
}

// TestKeyStore_ConcurrentUnlockLock 验证并发 Unlock/Lock 不会死锁
func TestKeyStore_ConcurrentUnlockLock(t *testing.T) {
	Reset()
	ks := Instance()

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

	// 只要不死锁就算通过，最终状态不确定
}

// TestKeyStore_ConcurrentVerifyPassword 验证并发 VerifyPassword 不会竞态
func TestKeyStore_ConcurrentVerifyPassword(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

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

// TestKeyStore_GetKey_NilAfterLock 验证 Lock 后 GetKey 返回 nil, false
func TestKeyStore_GetKey_NilAfterLock(t *testing.T) {
	Reset()
	ks := Instance()

	ks.Unlock([]byte("sk-test-key"))
	key, ok := ks.GetKey()
	if !ok || key == nil {
		t.Fatal("GetKey should return valid key when unlocked")
	}

	ks.Lock()
	key, ok = ks.GetKey()
	if ok || key != nil {
		t.Fatal("GetKey should return nil, false after Lock()")
	}
}

// TestKeyStore_IsUnlocked_InitialState 验证初始状态为锁定
func TestKeyStore_IsUnlocked_InitialState(t *testing.T) {
	Reset()
	ks := Instance()

	if ks.IsUnlocked() {
		t.Fatal("initial state should be locked")
	}
}

// TestKeyStore_GetLockStatus_InitialState 验证初始锁定状态
func TestKeyStore_GetLockStatus_InitialState(t *testing.T) {
	Reset()
	ks := Instance()

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

// TestKeyStore_Unlock_ZeroLengthKey 验证 Unlock 空字节切片失败
func TestKeyStore_Unlock_ZeroLengthKey(t *testing.T) {
	Reset()
	ks := Instance()

	err := ks.Unlock([]byte{})
	if err == nil {
		t.Fatal("Unlock with empty byte slice should fail")
	}
}

// TestKeyStore_Unlock_NilKey 验证 Unlock nil 失败
func TestKeyStore_Unlock_NilKey(t *testing.T) {
	Reset()
	ks := Instance()

	err := ks.Unlock(nil)
	if err == nil {
		t.Fatal("Unlock with nil should fail")
	}
}

// TestKeyStore_Lock_Idempotent 验证多次 Lock 不会 panic
func TestKeyStore_Lock_Idempotent(t *testing.T) {
	Reset()
	ks := Instance()

	ks.Lock()
	ks.Lock()
	ks.Lock()

	if ks.IsUnlocked() {
		t.Fatal("should remain locked after multiple Lock() calls")
	}
}

// TestKeyStore_SetAPIKeyConfigured_Idempotent 验证重复设置 hasAPIKey 不会异常
func TestKeyStore_SetAPIKeyConfigured_Idempotent(t *testing.T) {
	Reset()
	ks := Instance()

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

// TestKeyStore_RateLimit_6Attempts 验证 6 次失败后仍然锁定（超过阈值不会重置）
// 注意：第 6 次调用被速率限制阻止（lockedUntil 已设置），FailedAttempts 保持为 5
func TestKeyStore_RateLimit_6Attempts(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance
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
	// 第 6 次调用被速率限制阻止，FailedAttempts 保持为 5
	if status.FailedAttempts != 5 {
		t.Errorf("FailedAttempts should be 5 (6th attempt blocked by rate limit), got %d", status.FailedAttempts)
	}
}

// TestKeyStore_RateLimit_ExpireAndRetry 验证速率限制过期后可以重试
func TestKeyStore_RateLimit_ExpireAndRetry(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance
	ks.SetAPIKeyConfigured(true)

	// 设置已过期的锁定
	ks.lockedUntil = time.Now().Add(-1 * time.Second)

	// 现在应该可以解锁
	err := ks.Unlock([]byte("sk-test-key"))
	if err != nil {
		t.Fatalf("Unlock should succeed after rate limit expires: %v", err)
	}

	if !ks.IsUnlocked() {
		t.Fatal("should be unlocked after rate limit expires")
	}
}

// TestKeyStore_GetKey_ModifyCopyDoesNotAffectOriginal 验证修改 GetKey 返回的副本不影响原始密钥
func TestKeyStore_GetKey_ModifyCopyDoesNotAffectOriginal(t *testing.T) {
	Reset()
	ks := Instance()

	originalKey := []byte("sk-original-key-12345")
	ks.Unlock(originalKey)

	copy1, _ := ks.GetKey()
	copy2, _ := ks.GetKey()

	// 修改第一个副本
	for i := range copy1 {
		copy1[i] = 0
	}

	// 第二个副本不应受影响
	copy2Str := string(copy2)
	if copy2Str != "sk-original-key-12345" {
		t.Errorf("modifying copy1 should not affect copy2, got %q", copy2Str)
	}

	// 内部密钥也不应受影响
	internalKey, _ := ks.GetKey()
	if string(internalKey) != "sk-original-key-12345" {
		t.Error("modifying copy should not affect internal key")
	}
}

// TestKeyStore_RecordFailedAttempt_Increment 验证 RecordFailedAttempt 递增失败计数
func TestKeyStore_RecordFailedAttempt_Increment(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

	// 初始状态
	status := ks.GetLockStatus()
	if status.FailedAttempts != 0 {
		t.Fatalf("initial FailedAttempts should be 0, got %d", status.FailedAttempts)
	}

	// 调用 RecordFailedAttempt
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

// TestKeyStore_RecordFailedAttempt_TriggersRateLimit 验证 RecordFailedAttempt 5 次后触发速率限制
func TestKeyStore_RecordFailedAttempt_TriggersRateLimit(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

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

	// 验证锁定时间约为 30 秒后
	expectedUnlock := time.Now().Add(30 * time.Second)
	diff := status.LockedUntil.Sub(expectedUnlock)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("LockedUntil should be ~30s from now, got %v (expected ~%v)", status.LockedUntil, expectedUnlock)
	}
}

// TestKeyStore_RecordFailedAttempt_BlocksVerifyPassword 验证速率限制期间 VerifyPassword 被阻止
func TestKeyStore_RecordFailedAttempt_BlocksVerifyPassword(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

	encrypted, _ := flowcrypto.Encrypt("test-key", []byte("CorrectPass123"))

	// 触发速率限制
	for i := 0; i < 5; i++ {
		ks.RecordFailedAttempt()
	}

	// 即使密码正确，在锁定期间也应返回 false
	ok := ks.VerifyPassword([]byte("CorrectPass123"), encrypted)
	if ok {
		t.Fatal("VerifyPassword should return false during rate limit period")
	}
}

// TestKeyStore_RecordFailedAttempt_UnlockBlocked 验证速率限制期间 Unlock 被阻止
func TestKeyStore_RecordFailedAttempt_UnlockBlocked(t *testing.T) {
	once = sync.Once{}
	instance = &KeyStore{}
	ks := instance

	// 触发速率限制
	for i := 0; i < 5; i++ {
		ks.RecordFailedAttempt()
	}

	err := ks.Unlock([]byte("sk-test-key"))
	if err == nil {
		t.Fatal("Unlock should fail during rate limit period")
	}
}
