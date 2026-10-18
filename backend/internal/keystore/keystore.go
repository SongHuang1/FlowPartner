package keystore

import (
	"fmt"
	"sync"
	"time"

	flowcrypto "github.com/SongHuang1/FlowPartner/backend/internal/crypto"
)

const (
	maxFailedAttempts = 5
	lockoutDuration   = 30 * time.Second
)

type LockStatus struct {
	Locked         bool      `json:"locked"`
	LockedUntil    time.Time `json:"locked_until,omitzero"`
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

// SwitchKey atomically switches to a new key in a single write lock:
// zero old key → store new key → stay unlocked. Prevents TOCTOU races.
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
	ErrRateLimited   = fmt.Errorf("too many failed attempts")
	ErrWrongPassword = fmt.Errorf("wrong password")
)

// TryActivate verifies the password and loads the API key atomically.
// Supports both initial load and key switching under a single write lock.
func (ks *KeyStore) TryActivate(encryptedKey string, password []byte) ([]byte, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if time.Now().Before(ks.lockedUntil) {
		return nil, ErrRateLimited
	}

	apiKey, err := flowcrypto.Decrypt(encryptedKey, password)
	if err != nil {
		ks.failedAttempts++
		if ks.failedAttempts >= maxFailedAttempts {
			ks.lockedUntil = time.Now().Add(lockoutDuration)
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
