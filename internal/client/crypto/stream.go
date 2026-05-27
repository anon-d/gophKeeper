package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
)

// StreamMeta хранит параметры потокового шифрования в metadata секрета.
type StreamMeta struct {
	Salt     []byte `json:"salt"`
	IV       []byte `json:"iv"`
	Filename string `json:"filename,omitempty"` // оригинальное имя файла с расширением
}

// NewStreamMeta генерирует новые параметры шифрования.
func NewStreamMeta() (*StreamMeta, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("generate iv: %w", err)
	}
	return &StreamMeta{Salt: salt, IV: iv}, nil
}

// Encode сериализует метаданные в JSON.
func (sm *StreamMeta) Encode() (string, error) {
	data, err := json.Marshal(sm)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DecodeStreamMeta десериализует метаданные из JSON.
func DecodeStreamMeta(data string) (*StreamMeta, error) {
	var sm StreamMeta
	if err := json.Unmarshal([]byte(data), &sm); err != nil {
		return nil, err
	}
	return &sm, nil
}

// NewEncryptReader оборачивает reader потоковым AES-CTR шифрованием.
func NewEncryptReader(masterPassword string, meta *StreamMeta, r io.Reader) (io.Reader, error) {
	stream, err := newCTRStream(masterPassword, meta)
	if err != nil {
		return nil, err
	}
	return &cipher.StreamReader{S: stream, R: r}, nil
}

// NewDecryptReader оборачивает reader потоковым AES-CTR дешифрованием.
// AES-CTR симметричен — шифрование и дешифрование используют одну операцию.
func NewDecryptReader(masterPassword string, meta *StreamMeta, r io.Reader) (io.Reader, error) {
	return NewEncryptReader(masterPassword, meta, r)
}

func newCTRStream(masterPassword string, meta *StreamMeta) (cipher.Stream, error) {
	key := DeriveKey(masterPassword, meta.Salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	return cipher.NewCTR(block, meta.IV), nil
}
