package payment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/services"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
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
		log:            log,
	}
}

func (s *PaymentServer) CreateSubscription(ctx context.Context, req *CreateSubscriptionRequest) (*CreateSubscriptionResponse, error) {
	s.log.Infow("gRPC CreateSubscription request received", "userID", req.UserId, "planID", req.PlanId, "idempotencyKey", req.IdempotencyKey)

	// !!!  Получение UserID из контекста (после авторизации) !!!
	// userID, ok := ctx.Value(ContextUserIDKey).(string)
	// if !ok {
	//  s.log.Errorw("UserID not found in context", "method", "CreateSubscription")
	//  return nil, status.Errorf(codes.Unauthenticated, "UserID not found in context")
	// }

	input := services.CreateSubscriptionInput{
		UserID:         req.UserId, //  !!!  Заменить на userID из контекста !!!
		PlanID:         req.PlanId,
		UserEmail:      fmt.Sprintf("%s@example.com", req.UserId), //  !!!  Временная заглушка !!!
		IdempotencyKey: req.IdempotencyKey,
	}

	output, err := s.paymentService.CreateSubscription(ctx, input)
	if err != nil {
		s.log.Errorw("Failed to create subscription via service", "error", err, "userID", req.UserId)
		return nil, mapErrorToGRPCStatus(err)
	}

	s.log.Infow("Subscription created successfully via gRPC", "subscriptionID", output.Subscription.SubscriptionID)

	return &CreateSubscriptionResponse{
		SubscriptionId: output.Subscription.SubscriptionID,
		ClientSecret:   output.ClientSecret,
		CreatedAt:      timestamppb.New(output.Subscription.CreatedAt),
	}, nil
}

func (s *PaymentServer) CancelSubscription(ctx context.Context, req *CancelSubscriptionRequest) (*CancelSubscriptionResponse, error) {
	s.log.Infow("gRPC CancelSubscription request received", "userID", req.UserId, "subscriptionID", req.SubscriptionId, "idempotencyKey", req.IdempotencyKey)

	// !!!  Получение UserID из контекста (после авторизации) !!!
	// userID, ok := ctx.Value(ContextUserIDKey).(string)
	// if !ok {
	//  s.log.Errorw("UserID not found in context", "method", "CancelSubscription")
	//  return nil, status.Errorf(codes.Unauthenticated, "UserID not found in context")
	// }

	err := s.paymentService.CancelSubscription(ctx, req.UserId, req.SubscriptionId, req.IdempotencyKey) //  !!!  Заменить на userID из контекста !!!
	if err != nil {
		s.log.Errorw("Failed to cancel subscription via service", "error", err, "subscriptionID", req.SubscriptionId)
		return nil, mapErrorToGRPCStatus(err)
	}

	s.log.Infow("Subscription canceled successfully via gRPC", "subscriptionID", req.SubscriptionId)

	return &CancelSubscriptionResponse{
		Success:    true,
		CanceledAt: timestamppb.New(time.Now()), //  !!!  Можно брать из сервиса, если возвращает время отмены !!!
	}, nil
}

func (s *PaymentServer) GetSubscription(ctx context.Context, req *GetSubscriptionRequest) (*GetSubscriptionResponse, error) {
	s.log.Infow("gRPC GetSubscription request received", "userID", req.UserId, "subscriptionID", req.SubscriptionId)

	// !!!  Получение UserID из контекста (после авторизации) !!!
	// userID, ok := ctx.Value(ContextUserIDKey).(string)
	// if !ok {
	//  s.log.Errorw("UserID not found in context", "method", "GetSubscription")
	//  return nil, status.Errorf(codes.Unauthenticated, "UserID not found in context")
	// }

	subscription, err := s.paymentService.GetSubscriptionByID(ctx, req.UserId, req.SubscriptionId) //  !!!  Заменить на userID из контекста !!!
	if err != nil {
		s.log.Warnw("Failed to get subscription via service", "error", err, "subscriptionID", req.SubscriptionId)
		return nil, mapErrorToGRPCStatus(err)
	}

	s.log.Infow("Subscription retrieved successfully via gRPC", "subscriptionID", subscription.SubscriptionID)

	var canceledAt *timestamppb.Timestamp
	if subscription.CanceledAt != nil {
		canceledAt = timestamppb.New(*subscription.CanceledAt)
	}

	return &GetSubscriptionResponse{
		SubscriptionId: subscription.SubscriptionID,
		PlanId:         subscription.PlanID,
		CreatedAt:      timestamppb.New(subscription.CreatedAt),
		CanceledAt:     canceledAt,
	}, nil
}

func mapErrorToGRPCStatus(err error) error {
	switch {
	case errors.Is(err, services.ErrSubscriptionNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, services.ErrPaymentFailed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, services.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, services.ErrInternalServer): //  !!!  Добавлено !!!
		return status.Error(codes.Internal, err.Error())
	default:
		return status.Error(codes.Internal, "Internal server error")
	}
}
