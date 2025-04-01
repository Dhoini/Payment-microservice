package payment

import (
	"context"
	"errors"
	"fmt" // Добавлено для заглушки email
	"time"

	"github.com/Dhoini/Payment-microservice/internal/services"
	"github.com/Dhoini/Payment-microservice/pkg/logger" // <-- Используем ваш логгер
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PaymentServer реализует gRPC сервер для PaymentService.
type PaymentServer struct {
	paymentService *services.PaymentService
	log            *logger.Logger // <-- Используем ваш логгер
	// ВАЖНО: в сгенерированном коде (*_grpc.pb.go) сервер должен встраивать
	// UnimplementedPaymentServiceServer для прямой совместимости.
	// Убедитесь, что эта строка есть в сгенерированном файле:
	// type PaymentServiceServer interface { ... mustEmbedUnimplementedPaymentServiceServer() }
	UnimplementedPaymentServiceServer // <--- Убедитесь, что эта строка есть/добавлена
}

// NewPaymentServer создает новый экземпляр PaymentServer.
func NewPaymentServer(paymentService *services.PaymentService, log *logger.Logger) *PaymentServer { // <-- Используем ваш логгер
	return &PaymentServer{
		paymentService: paymentService,
		log:            log, // Можно добавить .Named("PaymentGRPCServer"), если логгер поддерживает
	}
}

// CreateSubscription создает подписку через gRPC.
func (s *PaymentServer) CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*CreateSubscriptionResponse, error) {
	s.log.Infow("gRPC CreateSubscription request received", "userID", req.UserId, "planID", req.PlanId)

	// Создаем входную структуру для сервиса
	// !!! ВНИМАНИЕ: Email пользователя не приходит в gRPC запросе. Откуда его взять?
	// Варианты: 1) Добавить в proto-файл, 2) Запросить у User-сервиса, 3) Заглушка
	userEmail := fmt.Sprintf("%s@example.com", req.UserId) // !!! ЗАГЛУШКА !!!
	s.log.Warnw("Using placeholder email for Stripe customer creation", "userID", req.UserId, "email", userEmail)

	input := services.CreateSubscriptionInput{
		UserID:         req.UserId,
		PlanID:         req.PlanId,
		UserEmail:      userEmail, // Используем заглушку или полученный email
		IdempotencyKey: req.IdempotencyKey,
	}

	// Вызываем метод сервиса
	output, err := s.paymentService.CreateSubscription(ctx, input)
	if err != nil {
		s.log.Errorw("Failed to create subscription via service", "error", err, "userID", req.UserId)
		// Преобразуем ошибку сервиса в gRPC статус
		return nil, mapErrorToGRPCStatus(err)
	}

	s.log.Infow("Subscription created successfully via gRPC", "subscriptionID", output.Subscription.SubscriptionID)

	// Преобразуем результат сервиса в gRPC ответ
	return &CreateSubscriptionResponse{
		SubscriptionId: output.Subscription.SubscriptionID,
		ClientSecret:   output.ClientSecret,
		// Используем время из сохраненной сущности
		CreatedAt: timestamppb.New(output.Subscription.CreatedAt),
	}, nil
}

// CancelSubscription отменяет подписку через gRPC.
func (s *PaymentServer) CancelSubscription(ctx context.Context, req *CancelSubscriptionRequest) (*CancelSubscriptionResponse, error) {
	s.log.Infow("gRPC CancelSubscription request received", "userID", req.UserId, "subscriptionID", req.SubscriptionId)

	// Вызываем метод сервиса
	err := s.paymentService.CancelSubscription(ctx, req.UserId, req.SubscriptionId, req.IdempotencyKey)
	if err != nil {
		s.log.Errorw("Failed to cancel subscription via service", "error", err, "subscriptionID", req.SubscriptionId)
		return nil, mapErrorToGRPCStatus(err)
	}

	s.log.Infow("Subscription canceled successfully via gRPC", "subscriptionID", req.SubscriptionId)

	// Возвращаем успешный gRPC ответ
	return &CancelSubscriptionResponse{
		Success:    true,
		CanceledAt: timestamppb.New(time.Now()), // Можно брать время из сервиса, если оно там возвращается
	}, nil
}

// GetSubscription получает детали подписки через gRPC.
func (s *PaymentServer) GetSubscription(ctx context.Context, req *GetSubscriptionRequest) (*GetSubscriptionResponse, error) {
	s.log.Infow("gRPC GetSubscription request received", "userID", req.UserId, "subscriptionID", req.SubscriptionId)

	// Вызываем метод сервиса
	subscription, err := s.paymentService.GetSubscriptionByID(ctx, req.UserId, req.SubscriptionId)
	if err != nil {
		s.log.Warnw("Failed to get subscription via service", "error", err, "subscriptionID", req.SubscriptionId)
		return nil, mapErrorToGRPCStatus(err)
	}

	s.log.Infow("Subscription retrieved successfully via gRPC", "subscriptionID", subscription.SubscriptionID)

	// Преобразуем результат сервиса в gRPC ответ
	var canceledAt *timestamppb.Timestamp
	if subscription.CanceledAt != nil { // Проверяем, что время отмены не nil
		canceledAt = timestamppb.New(*subscription.CanceledAt)
	}

	return &GetSubscriptionResponse{
		SubscriptionId: subscription.SubscriptionID,
		PlanId:         subscription.PlanID,
		// Status: // В gRPC ответе нет статуса, можно добавить в proto если нужно
		CreatedAt:  timestamppb.New(subscription.CreatedAt),
		CanceledAt: canceledAt,
		// StripeCustomerId: // Нет в gRPC ответе
	}, nil
}

// mapErrorToGRPCStatus преобразует ошибки из слоя сервиса в статусы gRPC.
func mapErrorToGRPCStatus(err error) error {
	switch {
	case errors.Is(err, services.ErrSubscriptionNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, services.ErrPaymentFailed): // Пример другой ошибки
		// Возможно, стоит использовать InvalidArgument или FailedPrecondition
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, services.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	// TODO: Добавить обработку других специфических ошибок сервиса
	default:
		// Общая ошибка сервера для необработанных случаев
		return status.Error(codes.Internal, "Internal server error")
	}
}
