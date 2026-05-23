package crypto

import (
	"bytes"
	"io"
	"testing"
)

func TestStreamEncryptDecrypt(t *testing.T) {
	password := "stream-master-pass"
	original := []byte("This is a large file content that we want to encrypt in a streaming fashion для теста")

	meta, err := NewStreamMeta()
	if err != nil {
		t.Fatalf("NewStreamMeta: %v", err)
	}
	meta.Filename = "test.bin"

	// Шифрование
	encReader, err := NewEncryptReader(password, meta, bytes.NewReader(original))
	if err != nil {
		t.Fatalf("NewEncryptReader: %v", err)
	}
	encrypted, err := io.ReadAll(encReader)
	if err != nil {
		t.Fatalf("read encrypted: %v", err)
	}

	if bytes.Equal(encrypted, original) {
		t.Fatal("encrypted should differ from original")
	}
	if len(encrypted) != len(original) {
		t.Fatalf("CTR should preserve length: got %d, want %d", len(encrypted), len(original))
	}

	// Дешифрование
	decReader, err := NewDecryptReader(password, meta, bytes.NewReader(encrypted))
	if err != nil {
		t.Fatalf("NewDecryptReader: %v", err)
	}
	decrypted, err := io.ReadAll(decReader)
	if err != nil {
		t.Fatalf("read decrypted: %v", err)
	}

	if !bytes.Equal(decrypted, original) {
		t.Fatalf("decrypted != original")
	}
}

func TestStreamDecryptWrongPassword(t *testing.T) {
	meta, _ := NewStreamMeta()
	original := []byte("secret data")

	encReader, _ := NewEncryptReader("correct", meta, bytes.NewReader(original))
	encrypted, _ := io.ReadAll(encReader)

	decReader, _ := NewDecryptReader("wrong", meta, bytes.NewReader(encrypted))
	decrypted, _ := io.ReadAll(decReader)

	// CTR не даёт ошибку — просто мусор на выходе
	if bytes.Equal(decrypted, original) {
		t.Fatal("wrong password should produce different output")
	}
}

func TestStreamMetaEncodeDecode(t *testing.T) {
	meta, err := NewStreamMeta()
	if err != nil {
		t.Fatalf("NewStreamMeta: %v", err)
	}
	meta.Filename = "movie.mov"

	encoded, err := meta.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := DecodeStreamMeta(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if !bytes.Equal(decoded.Salt, meta.Salt) {
		t.Fatal("salt mismatch")
	}
	if !bytes.Equal(decoded.IV, meta.IV) {
		t.Fatal("IV mismatch")
	}
	if decoded.Filename != "movie.mov" {
		t.Fatalf("filename: got %q, want %q", decoded.Filename, "movie.mov")
	}
}

func TestStreamChunked(t *testing.T) {
	password := "chunked-test"
	meta, _ := NewStreamMeta()

	// Большие данные — проверяем что чанками работает
	original := make([]byte, 256*1024) // 256 KB
	for i := range original {
		original[i] = byte(i % 251)
	}

	// Шифрование чанками (как при стриме)
	encReader, _ := NewEncryptReader(password, meta, bytes.NewReader(original))
	var encBuf bytes.Buffer
	buf := make([]byte, 64*1024) // 64KB чанки
	for {
		n, err := encReader.Read(buf)
		if n > 0 {
			encBuf.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read chunk: %v", err)
		}
	}

	// Дешифрование
	decReader, _ := NewDecryptReader(password, meta, &encBuf)
	decrypted, _ := io.ReadAll(decReader)

	if !bytes.Equal(decrypted, original) {
		t.Fatal("chunked encrypt/decrypt failed")
	}
}
