package grpc

import (
	"log/slog"
	"net"

	"google.golang.org/grpc"

	"github.com/anon-d/gophKeeper/internal/server/grpc/interceptor"
	"github.com/anon-d/gophKeeper/internal/server/grpc/servers"
	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

// GRPCServer — gRPC-сервер приложения.
type GRPCServer struct {
	server *grpc.Server
	logger *slog.Logger
	addr   string
}

// NewGRPCServer создаёт gRPC-сервер с инжектированными зависимостями.
func NewGRPCServer(
	logger *slog.Logger,
	addr string,
	authService servers.AuthService,
	secretsService servers.SecretsService,
	tokenParser interceptor.TokenParser,
) *GRPCServer {
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(interceptor.AuthInterceptor(tokenParser)),
		grpc.StreamInterceptor(interceptor.StreamAuthInterceptor(tokenParser)),
	)

	pb.RegisterAuthServiceServer(srv, servers.NewUserServer(authService))
	pb.RegisterSecretsServiceServer(srv, servers.NewSecretServer(secretsService))

	return &GRPCServer{
		server: srv,
		logger: logger,
		addr:   addr,
	}
}

// Run запускает gRPC-сервер.
func (g *GRPCServer) Run() error {
	l, err := net.Listen("tcp", g.addr)
	if err != nil {
		return err
	}
	g.logger.Info("gRPC server started", "addr", g.addr)
	return g.server.Serve(l)
}

// Stop останавливает gRPC-сервер.
func (g *GRPCServer) Stop() {
	g.server.GracefulStop()
}
