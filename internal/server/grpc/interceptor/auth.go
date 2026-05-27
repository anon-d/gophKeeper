// Package interceptor содержит gRPC-интерсепторы.
package interceptor

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const userIDKey contextKey = "user_id"

// TokenParser — интерфейс для валидации JWT-токенов.
type TokenParser interface {
	ParseToken(token string) (userID string, err error)
}

// skipMethods — методы, не требующие авторизации.
var skipMethods = map[string]bool{
	"/api.AuthService/Register": true,
	"/api.AuthService/Login":    true,
}

// AuthInterceptor возвращает unary-интерсептор для проверки JWT.
func AuthInterceptor(parser TokenParser) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if skipMethods[info.FullMethod] {
			return handler(ctx, req)
		}
		ctx, err := authorize(ctx, parser)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamAuthInterceptor возвращает stream-интерсептор для проверки JWT.
func StreamAuthInterceptor(parser TokenParser) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if skipMethods[info.FullMethod] {
			return handler(srv, ss)
		}
		ctx, err := authorize(ss.Context(), parser)
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

func authorize(ctx context.Context, parser TokenParser) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization token")
	}

	token := strings.TrimPrefix(values[0], "Bearer ")
	userID, err := parser.ParseToken(token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	return context.WithValue(ctx, userIDKey, userID), nil
}

// UserIDFromContext извлекает userID из контекста.
func UserIDFromContext(ctx context.Context) (string, error) {
	v, ok := ctx.Value(userIDKey).(string)
	if !ok || v == "" {
		return "", status.Error(codes.Unauthenticated, "user_id not found in context")
	}
	return v, nil
}

// wrappedStream оборачивает ServerStream с подменённым контекстом.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
