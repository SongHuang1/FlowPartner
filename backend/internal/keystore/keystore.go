package keystore

import (
	"fmt"
	"sync"
	"time"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
)

const (
	maxFailedAttempts = 5
	lockoutDuration   = 30 * time.Second
)

type LockStatus struct {
	Locked         bool      `json:"locked"`
	LockedUntil    time.Time `json:"locked_until,omitempty"`
	FailedAttempts int       `json:"failed_attempts"`
	HasAPIKey      bool      `json:"has_api_key"`
}

type KeyStore struct {
	mu             sync.RWMutex
	apiKey         []byte
	unlocked       bool
	failedAttempts int
	lockedUntil    time.Time
	hasAPIKey      bool
}

var (
	instance *KeyStore
	once     sync.Once
)

func Instance() *KeyStore {
	once.Do(func() {
		instance = &KeyStore{}
	})
	return instance
}

func Reset() {
	once = sync.Once{}
	instance = nil
}

func (ks *KeyStore) SetAPIKeyConfigured(configured bool) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.hasAPIKey = configured
}

func (ks *KeyStore) VerifyPassword(password []byte, encryptedKey string) bool {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if time.Now().Before(ks.lockedUntil) {
		return false
	}

	_, err := flowcrypto.Decrypt(encryptedKey, password)
	if err != nil {
		ks.recordFailedAttempt()
		return false
	}

	ks.failedAttempts = 0
	return true
}

// RecordFailedAttempt 记录一次密码错误（不执行解密，用于已解密失败后的计数）
func (ks *KeyStore) RecordFailedAttempt() {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.recordFailedAttempt()
}

// recordFailedAttempt 内部方法（调用方需持有写锁）
func (ks *KeyStore) recordFailedAttempt() {
	ks.failedAttempts++
	if ks.failedAttempts >= maxFailedAttempts {
		ks.lockedUntil = time.Now().Add(lockoutDuration)
	}
}

func (ks *KeyStore) Unlock(apiKey []byte) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if time.Now().Before(ks.lockedUntil) {
		return fmt.Errorf("too many failed attempts, try again in %v", time.Until(ks.lockedUntil))
	}

	if len(apiKey) == 0 {
		return fmt.Errorf("API Key not configured")
	}

	ks.apiKey = make([]byte, len(apiKey))
	copy(ks.apiKey, apiKey)
	ks.unlocked = true
	ks.failedAttempts = 0
	return nil
}

func (ks *KeyStore) Lock() {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if ks.apiKey != nil {
		for i := range ks.apiKey {
			ks.apiKey[i] = 0
		}
		ks.apiKey = nil
	}
	ks.unlocked = false
}

// SwitchKey 原子切换：在单次写锁内零化旧 Key → 存入新 Key → 保持解锁，避免 TOCTOU 竞态
func (ks *KeyStore) SwitchKey(newKey []byte) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.apiKey != nil {
		for i := range ks.apiKey {
			ks.apiKey[i] = 0
		}
	}
	ks.apiKey = make([]byte, len(newKey))
	copy(ks.apiKey, newKey)
	ks.unlocked = true
	ks.failedAttempts = 0
}

var (
	ErrRateLimited = fmt.Errorf("too many failed attempts")
	ErrWrongPassword = fmt.Errorf("wrong password")
)

// TryActivate 原子操作：检查锁定状态 → 解密 → 切换 Key
// 在单次写锁内完成检查和切换，避免 TOCTOU 竞态
func (ks *KeyStore) TryActivate(encryptedKey string, password []byte) ([]byte, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if time.Now().Before(ks.lockedUntil) {
		return nil, ErrRateLimited
	}

	apiKey, err := flowcrypto.Decrypt(encryptedKey, password)
	if err != nil {
		ks.failedAttempts++
		if ks.failedAttempts >= 5 {
			ks.lockedUntil = time.Now().Add(30 * time.Second)
		}
		return nil, ErrWrongPassword
	}

	if ks.apiKey != nil {
		for i := range ks.apiKey {
			ks.apiKey[i] = 0
		}
	}
	ks.apiKey = []byte(apiKey)
	ks.unlocked = true
	ks.failedAttempts = 0
	return []byte(apiKey), nil
}

func (ks *KeyStore) GetKey() ([]byte, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	if !ks.unlocked || ks.apiKey == nil {
		return nil, false
	}
	keyCopy := make([]byte, len(ks.apiKey))
	copy(keyCopy, ks.apiKey)
	return keyCopy, true
}

func (ks *KeyStore) IsUnlocked() bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.unlocked
}

func (ks *KeyStore) GetLockStatus() LockStatus {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return LockStatus{
		Locked:         !ks.unlocked,
		LockedUntil:    ks.lockedUntil,
		FailedAttempts: ks.failedAttempts,
		HasAPIKey:      ks.hasAPIKey,
	}
}

func (ks *KeyStore) LockedUntil() time.Time {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.lockedUntil
}
