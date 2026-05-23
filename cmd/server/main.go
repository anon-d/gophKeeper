package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	grpcServer "github.com/anon-d/gophKeeper/internal/server/grpc"
	minioRepo "github.com/anon-d/gophKeeper/internal/server/repository/minio"
	pgRepo "github.com/anon-d/gophKeeper/internal/server/repository/postgres"
	"github.com/anon-d/gophKeeper/internal/server/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- PostgreSQL ---
	dbDSN := envOrDefault("DATABASE_DSN", "postgres://keeper:keeper@localhost:5432/keeper?sslmode=disable")
	pool, err := pgxpool.New(ctx, dbDSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	pgRepository := pgRepo.New(pool)

	// --- MinIO ---
	minioEndpoint := envOrDefault("MINIO_ENDPOINT", "localhost:9000")
	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(envOrDefault("MINIO_ACCESS_KEY", "minioadmin"), envOrDefault("MINIO_SECRET_KEY", "minioadmin"), ""),
		Secure: false,
	})
	if err != nil {
		logger.Error("failed to connect to minio", "error", err)
		os.Exit(1)
	}
	minioRepository, err := minioRepo.New(ctx, minioClient)
	if err != nil {
		logger.Error("failed to init minio repo", "error", err)
		os.Exit(1)
	}

	// --- Services ---
	jwtSecret := envOrDefault("JWT_SECRET", "super-secret-key")
	authSvc := service.NewAuthService(pgRepository, jwtSecret, 24*time.Hour)
	secretSvc := service.NewSecretService(pgRepository, minioRepository)

	// --- gRPC ---
	addr := envOrDefault("GRPC_ADDR", ":44044")
	srv := grpcServer.NewGRPCServer(logger, addr, authSvc, secretSvc, authSvc)

	// --- Graceful shutdown ---
	go func() {
		if err := srv.Run(); err != nil {
			logger.Error("grpc server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down...")
	srv.Stop()
	logger.Info("server stopped")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
