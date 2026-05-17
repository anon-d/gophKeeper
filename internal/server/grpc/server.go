package grpc

import (
	"log/slog"
	"net"

	"google.golang.org/grpc"

	"github.com/anon-d/gophKeeper/internal/server/grpc/servers"
	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

type GRPCServer struct {
	server *grpc.Server
	logger *slog.Logger
}

func NewGRPCServer(logger *slog.Logger) (*GRPCServer, error) {
	srv := grpc.NewServer()
	pb.RegisterAuthServiceServer(srv, &servers.UserServer{})
	pb.RegisterSecretsServiceServer(srv, &servers.SecretServer{})

	return &GRPCServer{
		server: srv,
		logger: logger,
	}, nil
}

func (g *GRPCServer) Run() error {
	l, err := net.Listen("tcp", "44044")
	if err != nil {
		return err
	}
	if err := g.server.Serve(l); err != nil {
		return err
	}
	return nil
}
