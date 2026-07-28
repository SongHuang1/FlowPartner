package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"runtime"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen  = 16
	ivLen    = 12
	keyLen   = 32

	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
)

func deriveKey(password []byte, salt []byte) []byte {
	return argon2.IDKey(password, salt, argonTime, argonMemory, argonThreads, keyLen)
}

func Encrypt(plaintext string, password []byte) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	iv := make([]byte, ivLen)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	key := deriveKey(password, salt)
	defer ZeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	ciphertext := gcm.Seal(nil, iv, []byte(plaintext), nil)

	result := make([]byte, 0, saltLen+ivLen+len(ciphertext))
	result = append(result, salt...)
	result = append(result, iv...)
	result = append(result, ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

func Decrypt(encoded string, password []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(data) < saltLen+ivLen+16 {
		return "", fmt.Errorf("encrypted data too short")
	}

	salt := data[:saltLen]
	iv := data[saltLen : saltLen+ivLen]
	ciphertext := data[saltLen+ivLen:]

	key := deriveKey(password, salt)
	defer ZeroBytes(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: wrong password or corrupted data")
	}

	return string(plaintext), nil
}

func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
