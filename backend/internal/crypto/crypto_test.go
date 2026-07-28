package crypto

import (
	"bytes"
	"encoding/base64"
	"runtime"
	"testing"
)

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

func TestDecrypt_WrongPassword(t *testing.T) {
	password := []byte("CorrectPass123")
	wrongPassword := []byte("WrongPass123")
	plaintext := "sk-test-api-key-12345"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, wrongPassword)
	if err == nil {
		t.Fatal("Decrypt with wrong password should fail")
	}
}

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

func TestDecrypt_CorruptedData(t *testing.T) {
	password := []byte("TestPass123")
	plaintext := "sk-test-api-key-12345"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	for i := range decoded {
		decoded[i] ^= 0xFF
	}

	corrupted := base64.StdEncoding.EncodeToString(decoded)
	_, err = Decrypt(corrupted, password)
	if err == nil {
		t.Fatal("Decrypt with corrupted data should fail")
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
	password := []byte("TestPass123")
	plaintext := bytes.Repeat([]byte("a"), 10000)

	encrypted, err := Encrypt(string(plaintext), password)
	if err != nil {
		t.Fatalf("Encrypt long text failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != string(plaintext) {
		t.Error("decrypted long text mismatch")
	}
}
