package servers

import (
	"context"

	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

type AuthService interface {
	Register(ctx context.Context, username, password string) error
	Login(ctx context.Context, username, password string) (string, error)
}

type UserServer struct {
	pb.UnimplementedAuthServiceServer
}

func (u *UserServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return nil, nil
}

func (u *UserServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	return nil, nil
}
