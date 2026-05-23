package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := DeriveKey("test-password", []byte("1234567890123456"))
	plaintext := []byte("hello world секрет")

	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Fatal("encrypted should differ from plaintext")
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecryptWithPassword(t *testing.T) {
	password := "my-master-password"
	plaintext := []byte(`{"login":"user","password":"pass123"}`)

	encrypted, err := EncryptWithPassword(password, plaintext)
	if err != nil {
		t.Fatalf("EncryptWithPassword: %v", err)
	}

	// Минимальный размер: salt(16) + nonce(12) + ciphertext(>=len(plaintext))
	if len(encrypted) < 28+len(plaintext) {
		t.Fatalf("encrypted too short: %d bytes", len(encrypted))
	}

	decrypted, err := DecryptWithPassword(password, encrypted)
	if err != nil {
		t.Fatalf("DecryptWithPassword: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWithWrongPassword(t *testing.T) {
	encrypted, _ := EncryptWithPassword("correct", []byte("secret"))
	_, err := DecryptWithPassword("wrong", encrypted)
	if err == nil {
		t.Fatal("expected error with wrong password")
	}
}

func TestDecryptTooShort(t *testing.T) {
	_, err := DecryptWithPassword("pass", []byte("short"))
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	salt := []byte("1234567890123456")
	k1 := DeriveKey("password", salt)
	k2 := DeriveKey("password", salt)
	if !bytes.Equal(k1, k2) {
		t.Fatal("same password+salt should produce same key")
	}
}

func TestDeriveKeyDifferentSalt(t *testing.T) {
	k1 := DeriveKey("password", []byte("salt1234567890ab"))
	k2 := DeriveKey("password", []byte("salt1234567890cd"))
	if bytes.Equal(k1, k2) {
		t.Fatal("different salts should produce different keys")
	}
}
