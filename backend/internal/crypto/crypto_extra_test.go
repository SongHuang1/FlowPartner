package crypto

import (
	"encoding/base64"
	"strings"
	"testing"
)

// TestEncryptDecrypt_EmptyPassword 验证空密码加密/解密仍然有效（虽然业务层会阻止空密码）
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

// TestEncryptDecrypt_UnicodePlaintext 验证 Unicode 字符（中文、emoji）加密解密正确
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

// TestEncryptDecrypt_SpecialCharsInPlaintext 验证特殊字符（换行、null字节、引号）加密解密正确
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

// TestEncrypt_OutputFormat 验证加密输出格式为 Base64(Salt+IV+Ciphertext+Tag)
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

	// 最小长度: salt(16) + iv(12) + tag(16) = 44 bytes
	if len(decoded) < saltLen+ivLen+16 {
		t.Errorf("encrypted data too short: got %d bytes, want at least %d", len(decoded), saltLen+ivLen+16)
	}

	// 验证 salt 不为全零（随机生成）
	salt := decoded[:saltLen]
	allZero := true
	for _, b := range salt {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("salt should not be all zeros")
	}

	// 验证 IV 不为全零（随机生成）
	iv := decoded[saltLen : saltLen+ivLen]
	allZero = true
	for _, b := range iv {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("IV should not be all zeros")
	}
}

// TestEncrypt_UniqueSalt 验证两次加密使用不同 Salt
func TestEncrypt_UniqueSalt(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "same-plaintext"

	enc1, _ := Encrypt(plaintext, password)
	enc2, _ := Encrypt(plaintext, password)

	dec1, _ := base64.StdEncoding.DecodeString(enc1)
	dec2, _ := base64.StdEncoding.DecodeString(enc2)

	salt1 := dec1[:saltLen]
	salt2 := dec2[:saltLen]

	same := true
	for i := range salt1 {
		if salt1[i] != salt2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("two encryptions should use different salts")
	}
}

// TestEncrypt_UniqueIV_DifferentPlaintext 验证两次加密使用不同 IV
func TestEncrypt_UniqueIV_DifferentPlaintext(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "same-plaintext"

	enc1, _ := Encrypt(plaintext, password)
	enc2, _ := Encrypt(plaintext, password)

	dec1, _ := base64.StdEncoding.DecodeString(enc1)
	dec2, _ := base64.StdEncoding.DecodeString(enc2)

	iv1 := dec1[saltLen : saltLen+ivLen]
	iv2 := dec2[saltLen : saltLen+ivLen]

	same := true
	for i := range iv1 {
		if iv1[i] != iv2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("two encryptions should use different IVs")
	}
}

// TestDecrypt_WrongPassword_DifferentError 验证错误密码返回通用错误信息（不泄露细节）
func TestDecrypt_WrongPassword_DifferentError(t *testing.T) {
	password := []byte("CorrectPass123")
	plaintext := "sk-secret-key"

	encrypted, _ := Encrypt(plaintext, password)

	_, err := Decrypt(encrypted, []byte("WrongPass123"))
	if err == nil {
		t.Fatal("Decrypt with wrong password should fail")
	}

	// 错误信息不应包含敏感细节
	errMsg := err.Error()
	if strings.Contains(errMsg, plaintext) {
		t.Error("error message should not contain the plaintext")
	}
}

// TestDecrypt_TamperedCiphertext 验证密文被篡改后解密失败（GCM 认证标签校验）
func TestDecrypt_TamperedCiphertext(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "sk-secret-api-key"

	encrypted, _ := Encrypt(plaintext, password)
	decoded, _ := base64.StdEncoding.DecodeString(encrypted)

	// 篡改密文部分（跳过 salt+iv，修改最后一个字节）
	decoded[len(decoded)-1] ^= 0x01

	corrupted := base64.StdEncoding.EncodeToString(decoded)
	_, err := Decrypt(corrupted, password)
	if err == nil {
		t.Fatal("Decrypt with tampered ciphertext should fail")
	}
}

// TestDecrypt_TamperedIV 验证 IV 被篡改后解密失败
func TestDecrypt_TamperedIV(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "sk-secret-api-key"

	encrypted, _ := Encrypt(plaintext, password)
	decoded, _ := base64.StdEncoding.DecodeString(encrypted)

	// 篡改 IV 部分的第一个字节
	decoded[saltLen] ^= 0xFF

	corrupted := base64.StdEncoding.EncodeToString(decoded)
	_, err := Decrypt(corrupted, password)
	if err == nil {
		t.Fatal("Decrypt with tampered IV should fail")
	}
}

// TestDecrypt_TamperedSalt 验证 Salt 被篡改后解密失败（派生密钥不同）
func TestDecrypt_TamperedSalt(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "sk-secret-api-key"

	encrypted, _ := Encrypt(plaintext, password)
	decoded, _ := base64.StdEncoding.DecodeString(encrypted)

	// 篡改 Salt 部分的第一个字节
	decoded[0] ^= 0xFF

	corrupted := base64.StdEncoding.EncodeToString(decoded)
	_, err := Decrypt(corrupted, password)
	if err == nil {
		t.Fatal("Decrypt with tampered salt should fail")
	}
}

// TestZeroBytes_NilSlice 验证 ZeroBytes 处理 nil 切片不 panic
func TestZeroBytes_NilSlice(t *testing.T) {
	// 不应 panic
	ZeroBytes(nil)
}

// TestZeroBytes_EmptySlice 验证 ZeroBytes 处理空切片不 panic
func TestZeroBytes_EmptySlice(t *testing.T) {
	// 不应 panic
	ZeroBytes([]byte{})
}

// TestEncryptDecrypt_MaxLengthPlaintext 验证超长明文加密解密正确
func TestEncryptDecrypt_MaxLengthPlaintext(t *testing.T) {
	password := []byte("TestPass123")
	// 模拟一个超长的 API Key（虽然实际不会这么长）
	plaintext := strings.Repeat("x", 100000)

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt long plaintext failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt long plaintext failed: %v", err)
	}

	if decrypted != plaintext {
		t.Error("decrypted long plaintext mismatch")
	}
}

// TestDeriveKey_Deterministic 验证相同密码和盐派生相同密钥
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

	for i := range key1 {
		if key1[i] != key2[i] {
			t.Error("deriveKey should be deterministic for same password and salt")
			break
		}
	}
}

// TestDeriveKey_DifferentSalts 验证不同盐派生不同密钥
func TestDeriveKey_DifferentSalts(t *testing.T) {
	password := []byte("TestPass123")
	salt1 := make([]byte, saltLen)
	salt2 := make([]byte, saltLen)
	salt2[0] = 1

	key1 := deriveKey(password, salt1)
	key2 := deriveKey(password, salt2)

	same := true
	for i := range key1 {
		if key1[i] != key2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different salts should derive different keys")
	}
}
