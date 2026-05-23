// Package postgres предоставляет реализацию репозитория на основе PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/anon-d/gophKeeper/internal/server/domain"
	db "github.com/anon-d/gophKeeper/internal/server/repository/postgres/gen"
)

// Repo — репозиторий для работы с PostgreSQL.
type Repo struct {
	q    *db.Queries
	pool *pgxpool.Pool
}

// New создаёт новый экземпляр Repo.
func New(pool *pgxpool.Pool) *Repo {
	return &Repo{
		q:    db.New(pool),
		pool: pool,
	}
}

// CreateUser сохраняет нового пользователя.
func (r *Repo) CreateUser(ctx context.Context, user *domain.User) error {
	return r.q.CreateUser(ctx, db.CreateUserParams{
		ID:       uuidToPgtype(user.ID),
		Username: user.Username,
		Password: user.PassHash,
	})
}

// GetUserByUsername возвращает пользователя по логину.
func (r *Repo) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	row, err := r.q.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &domain.User{
		ID:       uuidToString(row.ID),
		Username: row.Username,
		PassHash: row.Password,
	}, nil
}

// CreateSecret сохраняет секрет в БД.
func (r *Repo) CreateSecret(ctx context.Context, secret *domain.Secret) error {
	return r.q.CreateSecret(ctx, db.CreateSecretParams{
		ID:       uuidToPgtype(secret.ID),
		UserID:   uuidToPgtype(secret.UserID),
		Type:     int16(secret.Type),
		Title:    secret.Title,
		Metadata: pgtype.Text{String: secret.Metadata, Valid: secret.Metadata != ""},
		Payload:  secret.Payload,
		Ref:      pgtype.Text{String: secret.Ref, Valid: secret.Ref != ""},
	})
}

// GetSecret возвращает секрет по ID с проверкой владельца.
func (r *Repo) GetSecret(ctx context.Context, id, userID string) (*domain.Secret, error) {
	row, err := r.q.GetSecretById(ctx, db.GetSecretByIdParams{
		ID:     uuidToPgtype(id),
		UserID: uuidToPgtype(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("get secret: %w", err)
	}
	return rowToSecret(row), nil
}

// ListSecrets возвращает список секретов пользователя (без payload).
func (r *Repo) ListSecrets(ctx context.Context, userID string) ([]*domain.Secret, error) {
	rows, err := r.q.ListSecrets(ctx, uuidToPgtype(userID))
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	secrets := make([]*domain.Secret, 0, len(rows))
	for _, row := range rows {
		secrets = append(secrets, &domain.Secret{
			ID:        uuidToString(row.ID),
			Type:      domain.SecretType(row.Type),
			Title:     row.Title,
			Version:   int(row.Version),
			CreatedAt: row.CreatedAt.Time,
			UpdatedAt: row.UpdatedAt.Time,
		})
	}
	return secrets, nil
}

// UpdateSecret обновляет секрет с оптимистичной блокировкой. Возвращает новую версию.
func (r *Repo) UpdateSecret(ctx context.Context, secret *domain.Secret) (int, error) {
	newVersion, err := r.q.UpdateSecret(ctx, db.UpdateSecretParams{
		ID:       uuidToPgtype(secret.ID),
		UserID:   uuidToPgtype(secret.UserID),
		Title:    secret.Title,
		Metadata: pgtype.Text{String: secret.Metadata, Valid: secret.Metadata != ""},
		Payload:  secret.Payload,
		Ref:      pgtype.Text{String: secret.Ref, Valid: secret.Ref != ""},
		Version:  int32(secret.Version),
	})
	if err != nil {
		return 0, fmt.Errorf("update secret: %w", err)
	}
	return int(newVersion), nil
}

// DeleteSecret удаляет секрет с проверкой владельца.
func (r *Repo) DeleteSecret(ctx context.Context, id, userID string) error {
	return r.q.DeleteSecret(ctx, db.DeleteSecretParams{
		ID:     uuidToPgtype(id),
		UserID: uuidToPgtype(userID),
	})
}

func rowToSecret(row db.GetSecretByIdRow) *domain.Secret {
	return &domain.Secret{
		ID:        uuidToString(row.ID),
		Type:      domain.SecretType(row.Type),
		Title:     row.Title,
		Metadata:  row.Metadata.String,
		Payload:   row.Payload,
		Ref:       row.Ref.String,
		Version:   int(row.Version),
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

func uuidToPgtype(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
