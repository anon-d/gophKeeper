package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/anon-d/gophKeeper/internal/server/domain"
)

// --- Mocks ---

type mockSecretRepo struct {
	secrets map[string]*domain.Secret
}

func newMockSecretRepo() *mockSecretRepo {
	return &mockSecretRepo{secrets: make(map[string]*domain.Secret)}
}

func (m *mockSecretRepo) CreateSecret(ctx context.Context, secret *domain.Secret) error {
	m.secrets[secret.ID] = secret
	return nil
}

func (m *mockSecretRepo) GetSecret(ctx context.Context, id, userID string) (*domain.Secret, error) {
	s, ok := m.secrets[id]
	if !ok || s.UserID != userID {
		return nil, errors.New("not found")
	}
	return s, nil
}

func (m *mockSecretRepo) UpdateSecret(ctx context.Context, secret *domain.Secret) (int, error) {
	existing, ok := m.secrets[secret.ID]
	if !ok || existing.UserID != secret.UserID {
		return 0, errors.New("not found")
	}
	if existing.Version != secret.Version {
		return 0, errors.New("version conflict")
	}
	secret.Version++
	m.secrets[secret.ID] = secret
	return secret.Version, nil
}

func (m *mockSecretRepo) ListSecrets(ctx context.Context, userID string) ([]*domain.Secret, error) {
	var result []*domain.Secret
	for _, s := range m.secrets {
		if s.UserID == userID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSecretRepo) DeleteSecret(ctx context.Context, id, userID string) error {
	s, ok := m.secrets[id]
	if !ok || s.UserID != userID {
		return errors.New("not found")
	}
	delete(m.secrets, id)
	return nil
}

type mockStorage struct {
	objects map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{objects: make(map[string][]byte)}
}

func (m *mockStorage) Put(ctx context.Context, key string, reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.objects[key] = data
	return nil
}

func (m *mockStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	data, ok := m.objects[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockStorage) Delete(ctx context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

// --- Tests ---

func TestCreateAndGetSecret(t *testing.T) {
	svc := NewSecretService(newMockSecretRepo(), newMockStorage())
	ctx := context.Background()

	secret := &domain.Secret{
		ID:      "s1",
		UserID:  "u1",
		Type:    domain.Credentials,
		Title:   "VK",
		Payload: []byte(`{"login":"user","password":"pass"}`),
	}

	if err := svc.CreateSecret(ctx, secret); err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	got, err := svc.GetSecret(ctx, "s1", "u1")
	if err != nil {
		t.Fatalf("GetSecret: %v", err)
	}
	if got.Title != "VK" {
		t.Fatalf("title: got %q, want %q", got.Title, "VK")
	}
}

func TestGetSecretWrongUser(t *testing.T) {
	svc := NewSecretService(newMockSecretRepo(), newMockStorage())
	ctx := context.Background()

	_ = svc.CreateSecret(ctx, &domain.Secret{ID: "s1", UserID: "u1", Title: "test"})

	_, err := svc.GetSecret(ctx, "s1", "u2")
	if err == nil {
		t.Fatal("expected error for wrong user")
	}
}

func TestListSecrets(t *testing.T) {
	svc := NewSecretService(newMockSecretRepo(), newMockStorage())
	ctx := context.Background()

	_ = svc.CreateSecret(ctx, &domain.Secret{ID: "s1", UserID: "u1", Title: "first"})
	_ = svc.CreateSecret(ctx, &domain.Secret{ID: "s2", UserID: "u1", Title: "second"})
	_ = svc.CreateSecret(ctx, &domain.Secret{ID: "s3", UserID: "u2", Title: "other"})

	list, err := svc.ListSecrets(ctx, "u1")
	if err != nil {
		t.Fatalf("ListSecrets: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(list))
	}
}

func TestDeleteSecret(t *testing.T) {
	repo := newMockSecretRepo()
	storage := newMockStorage()
	svc := NewSecretService(repo, storage)
	ctx := context.Background()

	_ = svc.CreateSecret(ctx, &domain.Secret{ID: "s1", UserID: "u1", Title: "test"})

	if err := svc.DeleteSecret(ctx, "s1", "u1"); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}

	_, err := svc.GetSecret(ctx, "s1", "u1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestDeleteSecretWithMinIO(t *testing.T) {
	repo := newMockSecretRepo()
	storage := newMockStorage()
	svc := NewSecretService(repo, storage)
	ctx := context.Background()

	_ = svc.CreateSecret(ctx, &domain.Secret{ID: "s1", UserID: "u1", Ref: "u1/s1"})
	storage.objects["u1/s1"] = []byte("binary data")

	_ = svc.DeleteSecret(ctx, "s1", "u1")

	if _, ok := storage.objects["u1/s1"]; ok {
		t.Fatal("expected MinIO object to be deleted")
	}
}

func TestCreateSecretStream(t *testing.T) {
	repo := newMockSecretRepo()
	storage := newMockStorage()
	svc := NewSecretService(repo, storage)
	ctx := context.Background()

	secret := &domain.Secret{ID: "s1", UserID: "u1", Type: domain.Binary, Title: "file.bin"}
	data := []byte("binary file content here")

	err := svc.CreateSecretStream(ctx, secret, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("CreateSecretStream: %v", err)
	}

	// Проверяем что ссылка сохранена в БД
	saved := repo.secrets["s1"]
	if saved.Ref == "" {
		t.Fatal("expected Ref to be set")
	}
	if len(saved.Payload) != 0 {
		t.Fatal("expected empty payload in DB for stream secret")
	}

	// Проверяем что данные в MinIO
	if !bytes.Equal(storage.objects[saved.Ref], data) {
		t.Fatal("MinIO data mismatch")
	}
}

func TestGetSecretPayload(t *testing.T) {
	repo := newMockSecretRepo()
	storage := newMockStorage()
	svc := NewSecretService(repo, storage)
	ctx := context.Background()

	_ = svc.CreateSecret(ctx, &domain.Secret{ID: "s1", UserID: "u1", Ref: "u1/s1"})
	storage.objects["u1/s1"] = []byte("file data")

	secret, reader, err := svc.GetSecretPayload(ctx, "s1", "u1")
	if err != nil {
		t.Fatalf("GetSecretPayload: %v", err)
	}
	defer reader.Close()

	if secret.ID != "s1" {
		t.Fatal("wrong secret")
	}

	data, _ := io.ReadAll(reader)
	if !bytes.Equal(data, []byte("file data")) {
		t.Fatalf("got %q, want %q", data, "file data")
	}
}

func TestUpdateSecret(t *testing.T) {
	repo := newMockSecretRepo()
	svc := NewSecretService(repo, newMockStorage())
	ctx := context.Background()

	_ = svc.CreateSecret(ctx, &domain.Secret{ID: "s1", UserID: "u1", Title: "old", Version: 1})

	newVer, err := svc.UpdateSecret(ctx, &domain.Secret{
		ID: "s1", UserID: "u1", Title: "new", Version: 1,
	})
	if err != nil {
		t.Fatalf("UpdateSecret: %v", err)
	}
	if newVer != 2 {
		t.Fatalf("expected version 2, got %d", newVer)
	}
}

func TestUpdateSecretVersionConflict(t *testing.T) {
	repo := newMockSecretRepo()
	svc := NewSecretService(repo, newMockStorage())
	ctx := context.Background()

	_ = svc.CreateSecret(ctx, &domain.Secret{ID: "s1", UserID: "u1", Title: "v1", Version: 1})

	// Первое обновление — OK
	_, _ = svc.UpdateSecret(ctx, &domain.Secret{ID: "s1", UserID: "u1", Title: "v2", Version: 1})

	// Второе обновление со старой версией — конфликт
	_, err := svc.UpdateSecret(ctx, &domain.Secret{ID: "s1", UserID: "u1", Title: "v3", Version: 1})
	if err == nil {
		t.Fatal("expected version conflict error")
	}
}
