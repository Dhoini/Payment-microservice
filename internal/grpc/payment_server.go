package payment

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dhoini/Payment-microservice/internal/middleware" // Для ключа контекста
	"github.com/Dhoini/Payment-microservice/internal/models"
	"github.com/Dhoini/Payment-microservice/internal/services"
	"github.com/Dhoini/Payment-microservice/pkg/logger" // Ваш логгер

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PaymentServer struct {
	paymentService *services.PaymentService
	log            *logger.Logger
	UnimplementedPaymentServiceServer
}

func NewPaymentServer(paymentService *services.PaymentService, log *logger.Logger) *PaymentServer {
	return &PaymentServer{
		paymentService: paymentService,
		log:            log, // Используем переданный логгер
	}
}

// CreateSubscription обрабатывает gRPC запрос на создание подписки.
func (s *PaymentServer) CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*CreateSubscriptionResponse, error) {
	// Получаем UserID из контекста
	userIDValue, ok := ctx.Value(middleware.ContextUserIDKey).(string)
	if !ok {
		s.log.Errorw("UserID not found in gRPC context. Method: CreateSubscription")
		return nil, status.Errorf(codes.Unauthenticated, "UserID not found in context")
	}

	s.log.Infow("gRPC CreateSubscription request received. UserID: %s, PlanID: %s, Email: %s, IdempotencyKey: %s",
		userIDValue, req.PlanId, req.UserEmail, req.IdempotencyKey)

	// Валидация входных данных
	if req.PlanId == "" {
		s.log.Warnw("Missing plan_id in CreateSubscription request. UserID: %s", userIDValue)
		return nil, status.Errorf(codes.InvalidArgument, "plan_id is required")
	}
	if req.UserEmail == "" {
		s.log.Warnw("Missing user_email in CreateSubscription request. UserID: %s", userIDValue)
		return nil, status.Errorf(codes.InvalidArgument, "user_email is required")
	}

	// Подготовка ввода для сервиса
	input := services.CreateSubscriptionInput{
		UserID:         userIDValue,
		PlanID:         req.PlanId,
		UserEmail:      req.UserEmail,
		IdempotencyKey: req.IdempotencyKey,
	}

	// Вызов сервисного слоя
	output, err := s.paymentService.CreateSubscription(ctx, input)
	if err != nil {
		s.log.Errorw("Service failed to create subscription. UserID: %s, Error: %v", userIDValue, err)
		return nil, mapErrorToGRPCStatus(err, s.log) // Передаем логгер в mapError
	}

	s.log.Infow("Subscription created successfully via gRPC. UserID: %s, SubscriptionID: %s, Status: %s",
		userIDValue, output.Subscription.SubscriptionID, output.Subscription.Status)

	// Формирование успешного gRPC ответа
	return &CreateSubscriptionResponse{
		SubscriptionId: output.Subscription.SubscriptionID,
		ClientSecret:   output.ClientSecret,
		CreatedAt:      timestamppb.New(output.Subscription.CreatedAt),
		Status:         output.Subscription.Status,
	}, nil
}

// CancelSubscription обрабатывает gRPC запрос на отмену подписки.
func (s *PaymentServer) CancelSubscription(ctx context.Context, req *CancelSubscriptionRequest) (*CancelSubscriptionResponse, error) {
	userIDValue, ok := ctx.Value(middleware.ContextUserIDKey).(string)
	if !ok {
		s.log.Errorw("UserID not found in gRPC context. Method: CancelSubscription")
		return nil, status.Errorf(codes.Unauthenticated, "UserID not found in context")
	}

	s.log.Infow("gRPC CancelSubscription request received. UserID: %s, SubscriptionID: %s, IdempotencyKey: %s",
		userIDValue, req.SubscriptionId, req.IdempotencyKey)

	// Валидация
	if req.SubscriptionId == "" {
		s.log.Warnw("Missing subscription_id in CancelSubscription request. UserID: %s", userIDValue)
		return nil, status.Errorf(codes.InvalidArgument, "subscription_id is required")
	}

	// Вызов сервиса
	err := s.paymentService.CancelSubscription(ctx, userIDValue, req.SubscriptionId, req.IdempotencyKey)
	if err != nil {
		s.log.Errorw("Service failed to cancel subscription. UserID: %s, SubscriptionID: %s, Error: %v",
			userIDValue, req.SubscriptionId, err)
		return nil, mapErrorToGRPCStatus(err, s.log)
	}

	s.log.Infow("Subscription cancellation initiated successfully via gRPC. UserID: %s, SubscriptionID: %s",
		userIDValue, req.SubscriptionId)

	return &CancelSubscriptionResponse{
		Success:    true,
		CanceledAt: timestamppb.Now(),
	}, nil
}

// GetSubscription обрабатывает gRPC запрос на получение информации о подписке.
func (s *PaymentServer) GetSubscription(ctx context.Context, req *GetSubscriptionRequest) (*GetSubscriptionResponse, error) {
	userIDValue, ok := ctx.Value(middleware.ContextUserIDKey).(string)
	if !ok {
		s.log.Errorw("UserID not found in gRPC context. Method: GetSubscription")
		return nil, status.Errorf(codes.Unauthenticated, "UserID not found in context")
	}

	s.log.Infow("gRPC GetSubscription request received. UserID: %s, SubscriptionID: %s",
		userIDValue, req.SubscriptionId)

	// Валидация
	if req.SubscriptionId == "" {
		s.log.Warnw("Missing subscription_id in GetSubscription request. UserID: %s", userIDValue)
		return nil, status.Errorf(codes.InvalidArgument, "subscription_id is required")
	}

	// Вызов сервиса
	subscription, err := s.paymentService.GetSubscriptionByID(ctx, userIDValue, req.SubscriptionId)
	if err != nil {
		s.log.Warnw("Service failed to get subscription. UserID: %s, SubscriptionID: %s, Error: %v",
			userIDValue, req.SubscriptionId, err)
		return nil, mapErrorToGRPCStatus(err, s.log)
	}

	s.log.Infow("Subscription retrieved successfully via gRPC. UserID: %s, SubscriptionID: %s",
		userIDValue, subscription.SubscriptionID)

	// Формирование ответа
	grpcSub := mapModelToProtoSubscription(subscription)
	return &GetSubscriptionResponse{
		Subscription: grpcSub,
	}, nil
}

// mapErrorToGRPCStatus преобразует ошибки сервисного слоя в статус gRPC.
func mapErrorToGRPCStatus(err error, log *logger.Logger) error { // Принимает логгер
	switch {
	case errors.Is(err, services.ErrSubscriptionNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, services.ErrPaymentFailed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, services.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, services.ErrStripeClient):
		return status.Error(codes.Internal, fmt.Sprintf("Payment provider error: %v", err))
	case errors.Is(err, services.ErrInternalServer):
		return status.Error(codes.Internal, err.Error())
	default:
		// Используем переданный логгер
		log.Errorw("Unknown error occurred in service layer. Error: %v", err)
		return status.Error(codes.Internal, "Internal server error")
	}
}

// mapModelToProtoSubscription (без изменений)
func mapModelToProtoSubscription(sub *models.Subscription) *Subscription {
	if sub == nil {
		return nil
	}
	grpcSub := &Subscription{
		SubscriptionId:   sub.SubscriptionID,
		UserId:           sub.UserID,
		PlanId:           sub.PlanID,
		Status:           sub.Status,
		StripeCustomerId: sub.StripeCustomerID,
		CreatedAt:        timestamppb.New(sub.CreatedAt),
		UpdatedAt:        timestamppb.New(sub.UpdatedAt),
	}
	if sub.ExpiresAt != nil {
		grpcSub.ExpiresAt = timestamppb.New(*sub.ExpiresAt)
	}
	if sub.CanceledAt != nil {
		grpcSub.CanceledAt = timestamppb.New(*sub.CanceledAt)
	}
	return grpcSub
}
