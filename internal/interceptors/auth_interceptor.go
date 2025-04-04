package interceptors

import (
	"context"
	"strings"

	"github.com/Dhoini/Payment-microservice/internal/middleware" // Используем тот же пакет для ключа и валидатора
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthInterceptor struct {
	log       *logger.Logger
	validator middleware.TokenValidator
}

func NewAuthInterceptor(log *logger.Logger, validator middleware.TokenValidator) *AuthInterceptor {
	return &AuthInterceptor{
		log:       log,
		validator: validator,
	}
}

// Unary возвращает UnaryServerInterceptor для проверки JWT.
func (i *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Получаем метаданные из контекста
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			// Корректное логирование
			i.log.Warnw("gRPC Auth: Missing metadata for method: %s", info.FullMethod)
			return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
		}

		// Ищем заголовок авторизации
		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			// Корректное логирование
			i.log.Warnw("gRPC Auth: Missing authorization header for method: %s", info.FullMethod)
			return nil, status.Errorf(codes.Unauthenticated, "missing authorization header")
		}

		// Извлекаем токен (ожидаем "Bearer <token>")
		authHeader := authHeaders[0]
		if !strings.HasPrefix(authHeader, "Bearer ") {
			// Корректное логирование
			i.log.Warnw("gRPC Auth: Invalid authorization header format for method: %s", info.FullMethod)
			return nil, status.Errorf(codes.Unauthenticated, "invalid authorization header format")
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Валидируем токен
		claims, err := i.validator.Validate(tokenString)
		if err != nil {
			// Корректное логирование
			i.log.Warnw("gRPC Auth: Invalid token for method %s. Error: %v", info.FullMethod, err)
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		userID := claims.Subject // Используем Subject (sub)
		if userID == "" {
			i.log.Warnw("gRPC Auth: User ID (sub) missing in token for method %s", info.FullMethod)
			return nil, status.Errorf(codes.Unauthenticated, "User ID (sub) missing in token")
		}
		// Добавляем userID из 'sub' в контекст
		newCtx := context.WithValue(ctx, middleware.ContextUserIDKey, userID)
		i.log.Debugw("User authenticated via gRPC. UserID (from sub): %s, Method: %s", userID, info.FullMethod)
		return handler(newCtx, req)
	}
}
