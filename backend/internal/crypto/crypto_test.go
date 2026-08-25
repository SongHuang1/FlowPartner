package crypto

import (
	"bytes"
	"encoding/base64"
	"runtime"
	"strings"
	"testing"
)

// --- RoundTrip 基本行为 ---

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "sk-test-api-key-12345"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted == "" {
		t.Fatal("encrypted output is empty")
	}

	if encrypted == plaintext {
		t.Fatal("encrypted output equals plaintext")
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted text mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_EmptyPassword(t *testing.T) {
	password := []byte("")
	plaintext := "sk-test-api-key"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt with empty password failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt with empty password failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted text mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_EmptyPlaintext(t *testing.T) {
	password := []byte("TestPass123")

	encrypted, err := Encrypt("", password)
	if err != nil {
		t.Fatalf("Encrypt empty string failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

func TestEncryptDecrypt_LongPlaintext(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"10KB", 10000},
		{"100KB", 100000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password := []byte("TestPass123")
			plaintext := strings.Repeat("a", tt.size)

			encrypted, err := Encrypt(plaintext, password)
			if err != nil {
				t.Fatalf("Encrypt long text failed: %v", err)
			}

			decrypted, err := Decrypt(encrypted, password)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != plaintext {
				t.Errorf("decrypted long text mismatch (size %d)", tt.size)
			}
		})
	}
}

func TestEncryptDecrypt_UnicodePlaintext(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "API密钥-🔐-日本語テスト-العربية"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt unicode text failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt unicode text failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted unicode mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_SpecialCharsInPlaintext(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "line1\nline2\ttab\r\n\"quoted\"\\backslash\x00null"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt special chars failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt special chars failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("decrypted special chars mismatch: got %q, want %q", decrypted, plaintext)
	}
}

// --- 加密输出格式与随机性 ---

func TestEncrypt_OutputFormat(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "test-data"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Fatalf("encrypted output is not valid base64: %v", err)
	}

	// 最小长度: salt(saltLen) + iv(ivLen) + tag(16)
	if len(decoded) < saltLen+ivLen+16 {
		t.Errorf("encrypted data too short: got %d bytes, want at least %d", len(decoded), saltLen+ivLen+16)
	}

	assertNotAllZero(t, "salt", decoded[:saltLen])
	assertNotAllZero(t, "IV", decoded[saltLen:saltLen+ivLen])
}

// TestEncrypt_UniqueIV 验证相同明文和密码两次加密产生不同密文（随机 Salt/IV），且都能解密回原文。
func TestEncrypt_UniqueIV(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "sk-test-api-key-12345"

	encrypted1, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("First encrypt failed: %v", err)
	}

	encrypted2, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Second encrypt failed: %v", err)
	}

	if encrypted1 == encrypted2 {
		t.Fatal("two encryptions with same plaintext and password should produce different ciphertexts")
	}

	dec1, _ := Decrypt(encrypted1, password)
	dec2, _ := Decrypt(encrypted2, password)
	if dec1 != plaintext || dec2 != plaintext {
		t.Fatal("both ciphertexts should decrypt to original plaintext")
	}
}

func TestEncrypt_UniqueSalt(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "same-plaintext"

	enc1, _ := Encrypt(plaintext, password)
	enc2, _ := Encrypt(plaintext, password)

	dec1, _ := base64.StdEncoding.DecodeString(enc1)
	dec2, _ := base64.StdEncoding.DecodeString(enc2)

	salt1 := dec1[:saltLen]
	salt2 := dec2[:saltLen]

	if bytes.Equal(salt1, salt2) {
		t.Error("two encryptions should use different salts")
	}
}

// --- 解密失败路径 ---

func TestDecrypt_WrongPassword(t *testing.T) {
	password := []byte("CorrectPass123")
	plaintext := "sk-secret-key"

	encrypted, _ := Encrypt(plaintext, password)

	_, err := Decrypt(encrypted, []byte("WrongPass123"))
	if err == nil {
		t.Fatal("Decrypt with wrong password should fail")
	}

	// 错误信息不应包含敏感细节
	if strings.Contains(err.Error(), plaintext) {
		t.Error("error message should not contain the plaintext")
	}
}

// TestDecrypt_TamperedData 验证 Salt/IV/密文 任一位置被篡改后解密都失败。
func TestDecrypt_TamperedData(t *testing.T) {
	tests := []struct {
		name      string
		flipIndex func(dataLen int) int
	}{
		{"salt", func(int) int { return 0 }},
		{"IV", func(int) int { return saltLen }},
		{"ciphertext", func(dataLen int) int { return dataLen - 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password := []byte("TestPass123")
			plaintext := "sk-secret-api-key"

			encrypted, err := Encrypt(plaintext, password)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}
			decoded, err := base64.StdEncoding.DecodeString(encrypted)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			idx := tt.flipIndex(len(decoded))
			decoded[idx] ^= 0x01

			corrupted := base64.StdEncoding.EncodeToString(decoded)
			if _, err := Decrypt(corrupted, password); err == nil {
				t.Fatalf("Decrypt with tampered %s should fail", tt.name)
			}
		})
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	password := []byte("TestPass123")
	shortData := base64.StdEncoding.EncodeToString([]byte("short"))

	_, err := Decrypt(shortData, password)
	if err == nil {
		t.Fatal("Decrypt with too-short data should fail")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	password := []byte("TestPass123")

	_, err := Decrypt("!!!invalid-base64!!!", password)
	if err == nil {
		t.Fatal("Decrypt with invalid base64 should fail")
	}
}

// --- ZeroBytes ---

func TestZeroBytes(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	ZeroBytes(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("byte %d not zeroed: got %d", i, b)
		}
	}
}

func TestZeroBytes_KeepAlive(t *testing.T) {
	data := []byte("sensitive-data-here")
	ZeroBytes(data)
	runtime.KeepAlive(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("byte %d not zeroed after KeepAlive: got %d", i, b)
		}
	}
}

func TestZeroBytes_NilSlice(t *testing.T) {
	ZeroBytes(nil)
}

func TestZeroBytes_EmptySlice(t *testing.T) {
	ZeroBytes([]byte{})
}

// --- deriveKey ---

func TestDeriveKey_Deterministic(t *testing.T) {
	password := []byte("TestPass123")
	salt := make([]byte, saltLen)
	for i := range salt {
		salt[i] = byte(i)
	}

	key1 := deriveKey(password, salt)
	key2 := deriveKey(password, salt)

	if len(key1) != keyLen {
		t.Errorf("derived key length: got %d, want %d", len(key1), keyLen)
	}

	if !bytes.Equal(key1, key2) {
		t.Error("deriveKey should be deterministic for same password and salt")
	}
}

func TestDeriveKey_DifferentSalts(t *testing.T) {
	password := []byte("TestPass123")
	salt1 := make([]byte, saltLen)
	salt2 := make([]byte, saltLen)
	salt2[0] = 1

	key1 := deriveKey(password, salt1)
	key2 := deriveKey(password, salt2)

	if bytes.Equal(key1, key2) {
		t.Error("different salts should derive different keys")
	}
}

// --- 辅助函数 ---

func assertNotAllZero(t *testing.T, name string, data []byte) {
	t.Helper()
	for _, b := range data {
		if b != 0 {
			return
		}
	}
	t.Errorf("%s should not be all zeros", name)
}
