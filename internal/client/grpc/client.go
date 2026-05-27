// Package grpc предоставляет gRPC-клиент для взаимодействия с сервером GophKeeper.
package grpc

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

// Client — gRPC-клиент GophKeeper.
type Client struct {
	auth    pb.AuthServiceClient
	secrets pb.SecretsServiceClient
	token   string
}

// New создаёт gRPC-клиент.
func New(conn grpc.ClientConnInterface) *Client {
	return &Client{
		auth:    pb.NewAuthServiceClient(conn),
		secrets: pb.NewSecretsServiceClient(conn),
	}
}

// withToken добавляет JWT-токен в контекст.
func (c *Client) withToken(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
}

// Register регистрирует нового пользователя.
func (c *Client) Register(ctx context.Context, username, passHash string) error {
	_, err := c.auth.Register(ctx, &pb.RegisterRequest{
		Username: username,
		Password: passHash,
	})
	return err
}

// Login аутентифицирует пользователя и сохраняет токен.
func (c *Client) Login(ctx context.Context, username, passHash string) error {
	resp, err := c.auth.Login(ctx, &pb.LoginRequest{
		Username: username,
		Password: passHash,
	})
	if err != nil {
		return err
	}
	c.token = resp.GetToken()
	return nil
}

// ListSecrets возвращает список секретов.
func (c *Client) ListSecrets(ctx context.Context) ([]*pb.Secret, error) {
	resp, err := c.secrets.ListSecrets(c.withToken(ctx), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.GetSecrets(), nil
}

// CreateSecret создаёт секрет (мелкие данные).
func (c *Client) CreateSecret(ctx context.Context, secret *pb.Secret) error {
	_, err := c.secrets.CreateSecret(c.withToken(ctx), &pb.CreateSecretRequest{Secret: secret})
	return err
}

// GetSecret возвращает секрет по ID.
func (c *Client) GetSecret(ctx context.Context, id string) (*pb.Secret, error) {
	resp, err := c.secrets.GetSecret(c.withToken(ctx), &pb.GetSecretRequest{SecretId: id})
	if err != nil {
		return nil, err
	}
	return resp.GetSecret(), nil
}

// UpdateSecret обновляет секрет. Возвращает новую версию.
func (c *Client) UpdateSecret(ctx context.Context, secret *pb.Secret) (int64, error) {
	resp, err := c.secrets.UpdateSecret(c.withToken(ctx), &pb.UpdateSecretRequest{Secret: secret})
	if err != nil {
		return 0, err
	}
	return resp.GetNewVersion(), nil
}

// DeleteSecret удаляет секрет по ID.
func (c *Client) DeleteSecret(ctx context.Context, id string) error {
	_, err := c.secrets.DeleteSecret(c.withToken(ctx), &pb.DeleteSecretRequest{SecretId: id})
	return err
}

const chunkSize = 64 * 1024 // 64 KB

// ProgressFunc — callback для отслеживания прогресса (sent байт, total байт).
type ProgressFunc func(sent, total int64)

// UploadFile загружает файл через client-side streaming.
// reader должен уже быть зашифрованным (AES-CTR).
func (c *Client) UploadFile(ctx context.Context, secret *pb.Secret, reader io.Reader, totalSize int64, onProgress ProgressFunc) error {
	stream, err := c.secrets.CreateSecretStream(c.withToken(ctx))
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	// Первый чанк — метаданные
	if err := stream.Send(&pb.CreateSecretChunkRequest{Meta: secret}); err != nil {
		return fmt.Errorf("send meta: %w", err)
	}

	// Остальные чанки — данные
	buf := make([]byte, chunkSize)
	var sent int64
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&pb.CreateSecretChunkRequest{
				Chunk: buf[:n],
			}); sendErr != nil {
				return fmt.Errorf("send chunk: %w", sendErr)
			}
			sent += int64(n)
			if onProgress != nil {
				onProgress(sent, totalSize)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
	}

	_, err = stream.CloseAndRecv()
	return err
}

// DownloadFile скачивает файл через server-side streaming.
// Возвращает метаданные секрета и пишет зашифрованные данные в writer.
func (c *Client) DownloadFile(ctx context.Context, id string, writer io.Writer, onProgress ProgressFunc) (*pb.Secret, error) {
	stream, err := c.secrets.GetSecretStream(c.withToken(ctx), &pb.GetSecretRequest{SecretId: id})
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}

	// Первый чанк — метаданные
	first, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("recv meta: %w", err)
	}
	meta := first.GetMeta()

	// Остальные чанки — данные
	var received int64
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("recv chunk: %w", err)
		}
		data := chunk.GetChunk()
		if _, err := writer.Write(data); err != nil {
			return nil, fmt.Errorf("write chunk: %w", err)
		}
		received += int64(len(data))
		if onProgress != nil {
			onProgress(received, 0) // total неизвестен
		}
	}

	return meta, nil
}
