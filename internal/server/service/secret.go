package service

import (
	"context"
	"fmt"
	"io"

	"github.com/anon-d/gophKeeper/internal/server/domain"
	minioRepo "github.com/anon-d/gophKeeper/internal/server/repository/minio"
)

// SecretRepo — интерфейс репозитория секретов (PostgreSQL).
type SecretRepo interface {
	CreateSecret(ctx context.Context, secret *domain.Secret) error
	GetSecret(ctx context.Context, id, userID string) (*domain.Secret, error)
	UpdateSecret(ctx context.Context, secret *domain.Secret) (int, error)
	ListSecrets(ctx context.Context, userID string) ([]*domain.Secret, error)
	DeleteSecret(ctx context.Context, id, userID string) error
}

// ObjectStorage — интерфейс хранилища бинарных данных (MinIO).
type ObjectStorage interface {
	Put(ctx context.Context, key string, reader io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// SecretService — сервис для работы с секретами.
type SecretService struct {
	repo    SecretRepo
	storage ObjectStorage
}

// NewSecretService создаёт новый сервис секретов.
func NewSecretService(repo SecretRepo, storage ObjectStorage) *SecretService {
	return &SecretService{
		repo:    repo,
		storage: storage,
	}
}

// ListSecrets возвращает список секретов пользователя (без payload).
func (s *SecretService) ListSecrets(ctx context.Context, userID string) ([]*domain.Secret, error) {
	return s.repo.ListSecrets(ctx, userID)
}

// CreateSecret сохраняет мелкий секрет (payload в postgres).
func (s *SecretService) CreateSecret(ctx context.Context, secret *domain.Secret) error {
	return s.repo.CreateSecret(ctx, secret)
}

// CreateSecretStream сохраняет бинарный секрет: payload стримится в MinIO, ссылка — в postgres.
func (s *SecretService) CreateSecretStream(ctx context.Context, secret *domain.Secret, reader io.Reader) error {
	key := minioRepo.ObjectKey(secret.UserID, secret.ID)

	if err := s.storage.Put(ctx, key, reader); err != nil {
		return fmt.Errorf("upload to storage: %w", err)
	}

	secret.Ref = key
	secret.Payload = nil // payload в MinIO, не в postgres

	if err := s.repo.CreateSecret(ctx, secret); err != nil {
		// Откат: удаляем объект из MinIO если запись в БД не прошла
		_ = s.storage.Delete(ctx, key)
		return fmt.Errorf("save secret: %w", err)
	}
	return nil
}

// GetSecret возвращает секрет с payload из postgres (для мелких данных).
func (s *SecretService) GetSecret(ctx context.Context, id, userID string) (*domain.Secret, error) {
	return s.repo.GetSecret(ctx, id, userID)
}

// GetSecretPayload возвращает метаданные + reader для payload из MinIO (для бинарных данных).
func (s *SecretService) GetSecretPayload(ctx context.Context, id, userID string) (*domain.Secret, io.ReadCloser, error) {
	secret, err := s.repo.GetSecret(ctx, id, userID)
	if err != nil {
		return nil, nil, err
	}
	if secret.Ref == "" {
		return nil, nil, fmt.Errorf("secret %s has no binary payload", id)
	}

	reader, err := s.storage.Get(ctx, secret.Ref)
	if err != nil {
		return nil, nil, fmt.Errorf("get from storage: %w", err)
	}
	return secret, reader, nil
}

// UpdateSecret обновляет секрет с оптимистичной блокировкой по версии.
func (s *SecretService) UpdateSecret(ctx context.Context, secret *domain.Secret) (int, error) {
	newVersion, err := s.repo.UpdateSecret(ctx, secret)
	if err != nil {
		return 0, fmt.Errorf("update secret: %w", err)
	}
	return newVersion, nil
}

// DeleteSecret удаляет секрет из postgres и MinIO (если есть).
func (s *SecretService) DeleteSecret(ctx context.Context, id, userID string) error {
	secret, err := s.repo.GetSecret(ctx, id, userID)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteSecret(ctx, id, userID); err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}

	// Удаляем из MinIO если был бинарный payload
	if secret.Ref != "" {
		_ = s.storage.Delete(ctx, secret.Ref)
	}
	return nil
}
