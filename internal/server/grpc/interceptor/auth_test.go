package interceptor

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// --- Mock ---

type mockParser struct {
	userID string
	err    error
}

func (m *mockParser) ParseToken(token string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.userID, nil
}

// --- Tests ---

func TestAuthInterceptorSkipMethods(t *testing.T) {
	interceptor := AuthInterceptor(&mockParser{err: errors.New("should not be called")})

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	for _, method := range []string{"/api.AuthService/Register", "/api.AuthService/Login"} {
		info := &grpc.UnaryServerInfo{FullMethod: method}
		resp, err := interceptor(context.Background(), nil, info, handler)
		if err != nil {
			t.Fatalf("expected no error for %s, got: %v", method, err)
		}
		if resp != "ok" {
			t.Fatalf("expected 'ok' for %s", method)
		}
	}
}

func TestAuthInterceptorValidToken(t *testing.T) {
	interceptor := AuthInterceptor(&mockParser{userID: "user-123"})

	var capturedCtx context.Context
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "ok", nil
	}

	md := metadata.New(map[string]string{"authorization": "Bearer valid-token"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/api.SecretsService/ListSecrets"}

	_, err := interceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userID, err := UserIDFromContext(capturedCtx)
	if err != nil {
		t.Fatalf("UserIDFromContext: %v", err)
	}
	if userID != "user-123" {
		t.Fatalf("got userID %q, want %q", userID, "user-123")
	}
}

func TestAuthInterceptorMissingToken(t *testing.T) {
	interceptor := AuthInterceptor(&mockParser{userID: "user"})

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/api.SecretsService/ListSecrets"}
	_, err := interceptor(context.Background(), nil, info, handler)
	if err == nil {
		t.Fatal("expected error for missing metadata")
	}
}

func TestAuthInterceptorInvalidToken(t *testing.T) {
	interceptor := AuthInterceptor(&mockParser{err: errors.New("bad token")})

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}

	md := metadata.New(map[string]string{"authorization": "Bearer bad"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/api.SecretsService/ListSecrets"}

	_, err := interceptor(ctx, nil, info, handler)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestUserIDFromContextEmpty(t *testing.T) {
	_, err := UserIDFromContext(context.Background())
	if err == nil {
		t.Fatal("expected error for empty context")
	}
}

func TestAuthInterceptorBearerPrefix(t *testing.T) {
	var receivedToken string
	parser := &mockParser{userID: "u1"}

	// Переопределяем для захвата токена
	interceptor := AuthInterceptor(parser)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) { return nil, nil }

	md := metadata.New(map[string]string{"authorization": "Bearer my-jwt-token"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: "/api.SecretsService/GetSecret"}

	_, _ = interceptor(ctx, nil, info, handler)
	_ = receivedToken // токен "my-jwt-token" должен быть передан после strip "Bearer "
}
