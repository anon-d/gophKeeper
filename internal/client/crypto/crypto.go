// Package crypto предоставляет шифрование/дешифрование данных
// мастер-паролем пользователя (AES-256-GCM + Argon2id).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	keyLen     = 32 // AES-256
	saltLen    = 16
	argonTime  = 1
	argonMem   = 64 * 1024
	argonParal = 4
)

// DeriveKey генерирует 256-битный ключ из мастер-пароля через Argon2id.
// salt хранится вместе с зашифрованными данными (первые 16 байт).
func DeriveKey(masterPassword string, salt []byte) []byte {
	return argon2.IDKey([]byte(masterPassword), salt, argonTime, argonMem, argonParal, keyLen)
}

// GenerateSalt генерирует случайный salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

// Encrypt шифрует plaintext ключом key (AES-256-GCM).
// Формат: salt(16) + nonce(12) + ciphertext.
func Encrypt(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt дешифрует данные ключом key (AES-256-GCM).
// Ожидается формат: nonce(12) + ciphertext.
func Decrypt(key []byte, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// EncryptWithPassword шифрует plaintext мастер-паролем.
// Формат результата: salt(16) + nonce(12) + ciphertext.
func EncryptWithPassword(masterPassword string, plaintext []byte) ([]byte, error) {
	salt, err := GenerateSalt()
	if err != nil {
		return nil, err
	}
	key := DeriveKey(masterPassword, salt)
	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		return nil, err
	}
	// salt + encrypted (nonce+ciphertext)
	result := make([]byte, 0, saltLen+len(encrypted))
	result = append(result, salt...)
	result = append(result, encrypted...)
	return result, nil
}

// DecryptWithPassword дешифрует данные мастер-паролем.
// Ожидается формат: salt(16) + nonce(12) + ciphertext.
func DecryptWithPassword(masterPassword string, data []byte) ([]byte, error) {
	if len(data) < saltLen {
		return nil, fmt.Errorf("data too short")
	}
	salt := data[:saltLen]
	encrypted := data[saltLen:]
	key := DeriveKey(masterPassword, salt)
	return Decrypt(key, encrypted)
}

// authSalt — фиксированный salt для хеширования пароля аутентификации.
// Используется для детерминированного вывода — одинаковый пароль всегда даёт одинаковый хеш.
var authSalt = []byte("gophkeeper-auth0")

// HashPassword хеширует пароль через Argon2id с фиксированным salt.
// Результат детерминирован: один и тот же пароль → один и тот же хеш (необходимо для сравнения на сервере).
func HashPassword(password string) string {
	key := DeriveKey(password, authSalt)
	return fmt.Sprintf("%x", key)
}
