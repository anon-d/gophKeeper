package servers

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/anon-d/gophKeeper/internal/server/domain"
	"github.com/anon-d/gophKeeper/internal/server/grpc/interceptor"
	pb "github.com/anon-d/gophKeeper/pkg/proto/api"
)

// SecretsService — интерфейс сервиса секретов.
type SecretsService interface {
	ListSecrets(ctx context.Context, userID string) ([]*domain.Secret, error)
	CreateSecret(ctx context.Context, secret *domain.Secret) error
	GetSecret(ctx context.Context, id, userID string) (*domain.Secret, error)
	GetSecretPayload(ctx context.Context, id, userID string) (*domain.Secret, io.ReadCloser, error)
	CreateSecretStream(ctx context.Context, secret *domain.Secret, reader io.Reader) error
	UpdateSecret(ctx context.Context, secret *domain.Secret) (int, error)
	DeleteSecret(ctx context.Context, id, userID string) error
}

// SecretServer — gRPC-сервер секретов.
type SecretServer struct {
	pb.UnimplementedSecretsServiceServer
	secretsService SecretsService
}

// NewSecretServer создаёт новый SecretServer.
func NewSecretServer(secretsService SecretsService) *SecretServer {
	return &SecretServer{secretsService: secretsService}
}

func (s *SecretServer) ListSecrets(ctx context.Context, req *emptypb.Empty) (*pb.ListSecretResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	secrets, err := s.secretsService.ListSecrets(ctx, userID)
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
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	secret := secretConverterToDomain(req.GetSecret())
	secret.UserID = userID
	err = s.secretsService.CreateSecret(ctx, secret)
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

	userID, err := interceptor.UserIDFromContext(stream.Context())
	if err != nil {
		return err
	}
	secret := secretConverterToDomain(first.GetMeta())
	secret.UserID = userID

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
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	secret, err := s.secretsService.GetSecret(ctx, req.GetSecretId(), userID)
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

	userID, err := interceptor.UserIDFromContext(stream.Context())
	if err != nil {
		return err
	}

	// 1. Получаем метаданные и reader для payload
	secret, reader, err := s.secretsService.GetSecretPayload(stream.Context(), req.GetSecretId(), userID)
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

// UpdateSecret обновляет секрет с оптимистичной блокировкой.
func (s *SecretServer) UpdateSecret(ctx context.Context, req *pb.UpdateSecretRequest) (*pb.UpdateSecretResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	secret := secretConverterToDomain(req.GetSecret())
	secret.UserID = userID

	newVersion, err := s.secretsService.UpdateSecret(ctx, secret)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "конфликт версий: %v", err)
	}
	return &pb.UpdateSecretResponse{NewVersion: int64(newVersion)}, nil
}

func (s *SecretServer) DeleteSecret(ctx context.Context, req *pb.DeleteSecretRequest) (*pb.DeleteSecretResponse, error) {
	userID, err := interceptor.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = s.secretsService.DeleteSecret(ctx, req.GetSecretId(), userID)
	if err != nil {
		return &pb.DeleteSecretResponse{Status: "error"}, err
	}
	return &pb.DeleteSecretResponse{Status: "success"}, nil
}

func secretConverterToPB(secret *domain.Secret) *pb.Secret {
	return &pb.Secret{
		Id:        secret.ID,
		Type:      pb.SecretType(secret.Type),
		Title:     secret.Title,
		Payload:   secret.Payload,
		Metadata:  secret.Metadata,
		Version:   int64(secret.Version),
		CreatedAt: timestamppb.New(secret.CreatedAt),
		UpdatedAt: timestamppb.New(secret.UpdatedAt),
	}
}

func secretConverterToDomain(secret *pb.Secret) *domain.Secret {
	return &domain.Secret{
		ID:       secret.GetId(),
		Type:     domain.SecretType(secret.GetType()),
		Title:    secret.GetTitle(),
		Payload:  secret.GetPayload(),
		Metadata: secret.GetMetadata(),
		Version:  int(secret.GetVersion()),
	}
}
