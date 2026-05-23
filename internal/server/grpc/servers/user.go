package servers

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

// AuthService — интерфейс сервиса аутентификации.
type AuthService interface {
	Register(ctx context.Context, username, password string) error
	Login(ctx context.Context, username, password string) (string, error)
}

// UserServer — gRPC-сервер аутентификации.
type UserServer struct {
	pb.UnimplementedAuthServiceServer
	authService AuthService
}

// NewUserServer создаёт новый UserServer.
func NewUserServer(authService AuthService) *UserServer {
	return &UserServer{authService: authService}
}

// Register регистрирует нового пользователя.
func (u *UserServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req.GetUsername() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password are required")
	}

	if err := u.authService.Register(ctx, req.GetUsername(), req.GetPassword()); err != nil {
		return nil, status.Errorf(codes.Internal, "register: %v", err)
	}
	return &pb.RegisterResponse{Status: "success"}, nil
}

// Login аутентифицирует пользователя и возвращает JWT-токен.
func (u *UserServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	token, err := u.authService.Login(ctx, req.GetUsername(), req.GetPassword())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	return &pb.LoginResponse{Token: token}, nil
}
