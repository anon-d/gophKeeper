package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anon-d/gophKeeper/internal/server/domain"
)

// --- Mock ---

type mockUserRepo struct {
	users map[string]*domain.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[string]*domain.User)}
}

func (m *mockUserRepo) CreateUser(ctx context.Context, user *domain.User) error {
	if _, exists := m.users[user.Username]; exists {
		return errors.New("user exists")
	}
	m.users[user.Username] = user
	return nil
}

func (m *mockUserRepo) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	u, ok := m.users[username]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

// --- Tests ---

func TestRegister(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewAuthService(repo, "test-secret", time.Hour)

	err := svc.Register(context.Background(), "alice", "hash123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, ok := repo.users["alice"]; !ok {
		t.Fatal("user not saved")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewAuthService(repo, "test-secret", time.Hour)

	_ = svc.Register(context.Background(), "alice", "hash123")
	err := svc.Register(context.Background(), "alice", "hash456")
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
}

func TestLoginSuccess(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewAuthService(repo, "test-secret", time.Hour)

	_ = svc.Register(context.Background(), "bob", "passhash")

	token, err := svc.Login(context.Background(), "bob", "passhash")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewAuthService(repo, "test-secret", time.Hour)

	_ = svc.Register(context.Background(), "bob", "correct")

	_, err := svc.Login(context.Background(), "bob", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUnknownUser(t *testing.T) {
	repo := newMockUserRepo()
	svc := NewAuthService(repo, "test-secret", time.Hour)

	_, err := svc.Login(context.Background(), "unknown", "pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestParseToken(t *testing.T) {
	svc := NewAuthService(newMockUserRepo(), "jwt-secret-key", time.Hour)

	_ = svc.Register(context.Background(), "carol", "hash")
	token, _ := svc.Login(context.Background(), "carol", "hash")

	userID, err := svc.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if userID == "" {
		t.Fatal("expected non-empty userID")
	}
}

func TestParseTokenInvalid(t *testing.T) {
	svc := NewAuthService(newMockUserRepo(), "secret", time.Hour)

	_, err := svc.ParseToken("invalid.token.here")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestParseTokenWrongSecret(t *testing.T) {
	svc1 := NewAuthService(newMockUserRepo(), "secret1", time.Hour)
	svc2 := NewAuthService(newMockUserRepo(), "secret2", time.Hour)

	repo := newMockUserRepo()
	svc1.repo = repo
	_ = svc1.Register(context.Background(), "dave", "hash")
	token, _ := svc1.Login(context.Background(), "dave", "hash")

	_, err := svc2.ParseToken(token)
	if err == nil {
		t.Fatal("expected error with wrong secret")
	}
}
