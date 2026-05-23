// Package service содержит бизнес-логику сервера.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/anon-d/gophKeeper/internal/server/domain"
)

var (
	ErrUserExists       = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// UserRepo — интерфейс репозитория пользователей.
type UserRepo interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)
}

// AuthService — сервис аутентификации.
type AuthService struct {
	repo      UserRepo
	jwtSecret []byte
	tokenTTL  time.Duration
}

// NewAuthService создаёт новый сервис аутентификации.
func NewAuthService(repo UserRepo, jwtSecret string, tokenTTL time.Duration) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  tokenTTL,
	}
}

// Register регистрирует нового пользователя.
// Клиент отправляет username + хеш пароля. Сервер сохраняет как есть.
func (s *AuthService) Register(ctx context.Context, username, passHash string) error {
	user := &domain.User{
		ID:       uuid.New().String(),
		Username: username,
		PassHash: passHash,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("register: %w", err)
	}
	return nil
}

// Login проверяет учётные данные и возвращает JWT-токен.
// Клиент отправляет username + хеш пароля. Сервер сравнивает с БД.
func (s *AuthService) Login(ctx context.Context, username, passHash string) (string, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return "", ErrInvalidCredentials
	}

	// Прямое сравнение хешей — клиент хеширует, сервер только хранит
	if user.PassHash != passHash {
		return "", ErrInvalidCredentials
	}

	token, err := s.generateToken(user.ID)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return token, nil
}

func (s *AuthService) generateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(s.tokenTTL).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ParseToken валидирует JWT и возвращает userID.
func (s *AuthService) ParseToken(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", fmt.Errorf("user_id not found in token")
	}
	return userID, nil
}
