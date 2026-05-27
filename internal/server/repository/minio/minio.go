// Package minio предоставляет репозиторий для хранения бинарных данных в MinIO.
package minio

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

const bucket = "secrets"

// Repo — репозиторий для работы с MinIO.
type Repo struct {
	client *minio.Client
}

// New создаёт новый MinIO-репозиторий и проверяет наличие бакета.
func New(ctx context.Context, client *minio.Client) (*Repo, error) {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}
	return &Repo{client: client}, nil
}

// ObjectKey формирует ключ объекта: secrets/{userID}/{secretID}.
func ObjectKey(userID, secretID string) string {
	return fmt.Sprintf("%s/%s", userID, secretID)
}

// Put загружает данные из reader в MinIO. Размер неизвестен — MinIO сам разбивает на части.
func (r *Repo) Put(ctx context.Context, key string, reader io.Reader) error {
	_, err := r.client.PutObject(ctx, bucket, key, reader, -1, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

// Get возвращает reader для чтения данных из MinIO.
func (r *Repo) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := r.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	return obj, nil
}

// Delete удаляет объект из MinIO.
func (r *Repo) Delete(ctx context.Context, key string) error {
	if err := r.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object %s: %w", key, err)
	}
	return nil
}
