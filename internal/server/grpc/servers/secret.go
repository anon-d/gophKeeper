package servers

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/anon-d/gophKeeper/internal/server/domain"
	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

type SecretsService interface {
	ListSecrets(ctx context.Context) ([]*domain.Secret, error)
	CreateSecret(ctx context.Context, secret *domain.Secret) error
	GetSecret(ctx context.Context, id string) (*domain.Secret, error)
	GetSecretPayload(ctx context.Context, id string) (*domain.Secret, io.ReadCloser, error)
	CreateSecretStream(ctx context.Context, secret *domain.Secret, reader io.Reader) error
	DeleteSecret(ctx context.Context, id string) error
}

type SecretServer struct {
	pb.UnimplementedSecretsServiceServer
	secretsService SecretsService
}

func (s *SecretServer) ListSecrets(ctx context.Context, req *emptypb.Empty) (*pb.ListSecretResponse, error) {
	secrets, err := s.secretsService.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}
	secretsPB := make([]*pb.Secret, 0, len(secrets))
	for i := range secrets {
		secretsPB = append(secretsPB, secretConverterToPB(secrets[i]))
	}
	return &pb.ListSecretResponse{Secrets: secretsPB}, nil
}

func (s *SecretServer) CreateSecret(ctx context.Context, req *pb.CreateSecretRequest) (*pb.CreateSecretResponse, error) {
	err := s.secretsService.CreateSecret(ctx, secretConverterToDomain(req.GetSecret()))
	if err != nil {
		return &pb.CreateSecretResponse{Status: "error"}, err
	}
	return &pb.CreateSecretResponse{Status: "success"}, nil
}

// CreateSecretStream — client-side streaming.
// Клиент отправляет поток чанков, сервер собирает их и сохраняет.
//
// Протокол:
//   1-й чанк: meta заполнен (метаданные секрета), chunk пустой
//   2..N чанки: meta = nil, chunk содержит часть payload
//   Клиент завершает поток (io.EOF) — сервер сохраняет и отвечает
func (s *SecretServer) CreateSecretStream(stream pb.SecretsService_CreateSecretStreamServer) error {
	// 1. Читаем первый чанк — в нём метаданные
	first, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive metadata chunk: %w", err)
	}
	if first.GetMeta() == nil {
		return fmt.Errorf("first chunk must contain metadata")
	}

	secret := secretConverterToDomain(first.GetMeta())

	// 2. Создаём pipe: всё что пишем в writer — читается из reader
	pr, pw := io.Pipe()

	// 3. В отдельной горутине читаем остальные чанки из стрима и пишем в pipe
	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			if _, err := pw.Write(chunk.GetChunk()); err != nil {
				return
			}
		}
	}()

	// 4. Сервис читает из reader и сохраняет данные
	if err := s.secretsService.CreateSecretStream(stream.Context(), secret, pr); err != nil {
		return err
	}

	// Проверяем, не было ли ошибки в горутине чтения
	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	default:
	}

	return stream.SendAndClose(&pb.CreateSecretResponse{Status: "success"})
}

func (s *SecretServer) GetSecret(ctx context.Context, req *pb.GetSecretRequest) (*pb.GetSecretResponse, error) {
	secret, err := s.secretsService.GetSecret(ctx, req.GetSecretId())
	if err != nil {
		return nil, err
	}
	return &pb.GetSecretResponse{Secret: secretConverterToPB(secret)}, nil
}

// GetSecretStream — server-side streaming.
// Сервер отправляет данные клиенту чанками.
//
// Протокол:
//   1-й чанк: meta заполнен (метаданные секрета), chunk пустой
//   2..N чанки: meta = nil, chunk содержит часть payload
func (s *SecretServer) GetSecretStream(req *pb.GetSecretRequest, stream pb.SecretsService_GetSecretStreamServer) error {
	const chunkSize = 64 * 1024 // 64 KB

	// 1. Получаем метаданные и reader для payload
	secret, reader, err := s.secretsService.GetSecretPayload(stream.Context(), req.GetSecretId())
	if err != nil {
		return err
	}
	defer reader.Close()

	// 2. Первый чанк — только метаданные, без данных
	if err := stream.Send(&pb.GetSecretChunkResponse{
		Meta: secretConverterToPB(secret),
	}); err != nil {
		return err
	}

	// 3. Остальные чанки — только данные
	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&pb.GetSecretChunkResponse{
				Chunk: buf[:n],
			}); sendErr != nil {
				return sendErr
			}
		}
		if err == io.EOF {
			return nil // стрим завершён, gRPC закроет его автоматически
		}
		if err != nil {
			return fmt.Errorf("failed to read payload: %w", err)
		}
	}
}

func (s *SecretServer) DeleteSecret(ctx context.Context, req *pb.DeleteSecretRequest) (*pb.DeleteSecretResponse, error) {
	err := s.secretsService.DeleteSecret(ctx, req.GetSecretId())
	if err != nil {
		return &pb.DeleteSecretResponse{Status: "error"}, err
	}
	return &pb.DeleteSecretResponse{Status: "success"}, nil
}

func secretConverterToPB(secret *domain.Secret) *pb.Secret {
	return nil
}

func secretConverterToDomain(secret *pb.Secret) *domain.Secret {
	return nil
}
