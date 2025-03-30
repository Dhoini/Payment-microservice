package handlers

import (
	"context"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/internal/service"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	paymentpb "github.com/Dhoini/Payment-microservice/proto/payment/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PaymentHandler обработчик для gRPC сервиса платежей
type PaymentHandler struct {
	paymentpb.UnimplementedPaymentServiceServer
	service service.PaymentService
	log     *logger.Logger
}

// NewPaymentHandler создает новый обработчик платежей
func NewPaymentHandler(service service.PaymentService, log *logger.Logger) *PaymentHandler {
	return &PaymentHandler{
		service: service,
		log:     log,
	}
}

// CreatePayment создает новый платеж
func (h *PaymentHandler) CreatePayment(ctx context.Context, req *paymentpb.CreatePaymentRequest) (*paymentpb.PaymentResponse, error) {
	h.log.Debug("gRPC CreatePayment request received")

	// Преобразуем запрос gRPC в доменную модель
	paymentReq := domain.PaymentRequest{
		CustomerID:  req.CustomerId,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: req.Description,
		MethodID:    req.MethodId,
		Metadata:    req.Metadata,
	}

	// Создаем платеж
	payment, err := h.service.Create(ctx, paymentReq)
	if err != nil {
		h.log.Error("Failed to create payment: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create payment: %v", err)
	}

	// Преобразуем доменную модель в ответ gRPC
	return h.toPaymentResponse(payment), nil
}

// GetPayment возвращает информацию о платеже по ID
func (h *PaymentHandler) GetPayment(ctx context.Context, req *paymentpb.GetPaymentRequest) (*paymentpb.PaymentResponse, error) {
	h.log.Debug("gRPC GetPayment request received for ID: %s", req.Id)

	// Получаем платеж
	payment, err := h.service.GetByID(ctx, req.Id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "payment not found: %s", req.Id)
		}
		if err == repository.ErrInvalidData {
			return nil, status.Errorf(codes.InvalidArgument, "invalid payment ID: %s", req.Id)
		}
		h.log.Error("Failed to get payment: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get payment: %v", err)
	}

	// Преобразуем доменную модель в ответ gRPC
	return h.toPaymentResponse(payment), nil
}

// ListPayments возвращает список платежей
func (h *PaymentHandler) ListPayments(ctx context.Context, req *paymentpb.ListPaymentsRequest) (*paymentpb.ListPaymentsResponse, error) {
	h.log.Debug("gRPC ListPayments request received")

	// Получаем все платежи
	payments, err := h.service.GetAll(ctx)
	if err != nil {
		h.log.Error("Failed to list payments: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to list payments: %v", err)
	}

	// В реальном проекте здесь должны быть применены фильтры и пагинация
	// на основе параметров запроса (limit, offset, status, dates)

	// Преобразуем доменные модели в ответы gRPC
	pbPayments := make([]*paymentpb.PaymentResponse, len(payments))
	for i, payment := range payments {
		pbPayments[i] = h.toPaymentResponse(payment)
	}

	return &paymentpb.ListPaymentsResponse{
		Payments: pbPayments,
		Total:    int32(len(payments)),
	}, nil
}

// UpdatePaymentStatus обновляет статус платежа
func (h *PaymentHandler) UpdatePaymentStatus(ctx context.Context, req *paymentpb.UpdatePaymentStatusRequest) (*paymentpb.PaymentResponse, error) {
	h.log.Debug("gRPC UpdatePaymentStatus request received for ID: %s", req.Id)

	// Преобразуем статус из gRPC в доменную модель
	status := h.fromPaymentStatusPb(req.Status)

	// Обновляем статус платежа
	payment, err := h.service.UpdateStatus(ctx, req.Id, status)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "payment not found: %s", req.Id)
		}
		if err == repository.ErrInvalidData {
			return nil, status.Errorf(codes.InvalidArgument, "invalid payment ID: %s", req.Id)
		}
		h.log.Error("Failed to update payment status: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to update payment status: %v", err)
	}

	// Преобразуем доменную модель в ответ gRPC
	return h.toPaymentResponse(payment), nil
}

// ListPaymentsByCustomer возвращает список платежей для клиента
func (h *PaymentHandler) ListPaymentsByCustomer(ctx context.Context, req *paymentpb.ListPaymentsByCustomerRequest) (*paymentpb.ListPaymentsResponse, error) {
	h.log.Debug("gRPC ListPaymentsByCustomer request received for customer ID: %s", req.CustomerId)

	// Получаем платежи для клиента
	payments, err := h.service.GetByCustomerID(ctx, req.CustomerId)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "customer not found: %s", req.CustomerId)
		}
		if err == repository.ErrInvalidData {
			return nil, status.Errorf(codes.InvalidArgument, "invalid customer ID: %s", req.CustomerId)
		}
		h.log.Error("Failed to list payments for customer: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to list payments for customer: %v", err)
	}

	// В реальном проекте здесь должны быть применены фильтры и пагинация
	// на основе параметров запроса (limit, offset, status)

	// Преобразуем доменные модели в ответы gRPC
	pbPayments := make([]*paymentpb.PaymentResponse, len(payments))
	for i, payment := range payments {
		pbPayments[i] = h.toPaymentResponse(payment)
	}

	return &paymentpb.ListPaymentsResponse{
		Payments: pbPayments,
		Total:    int32(len(payments)),
	}, nil
}

// RefundPayment создает возврат платежа
func (h *PaymentHandler) RefundPayment(ctx context.Context, req *paymentpb.RefundPaymentRequest) (*paymentpb.PaymentResponse, error) {
	h.log.Debug("gRPC RefundPayment request received for payment ID: %s", req.PaymentId)

	// Получаем платеж
	payment, err := h.service.GetByID(ctx, req.PaymentId)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "payment not found: %s", req.PaymentId)
		}
		if err == repository.ErrInvalidData {
			return nil, status.Errorf(codes.InvalidArgument, "invalid payment ID: %s", req.PaymentId)
		}
		h.log.Error("Failed to get payment for refund: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get payment for refund: %v", err)
	}

	// Проверяем, что платеж можно вернуть
	if payment.Status != domain.PaymentStatusCompleted {
		return nil, status.Errorf(codes.FailedPrecondition, "payment is not in a refundable state: %s", payment.Status)
	}

	// Обновляем статус платежа на REFUNDED
	payment, err = h.service.UpdateStatus(ctx, req.PaymentId, domain.PaymentStatusRefunded)
	if err != nil {
		h.log.Error("Failed to refund payment: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to refund payment: %v", err)
	}

	// В реальном проекте здесь должна быть интеграция с платежной системой
	// для создания реального возврата и сохранения информации о нем

	// Преобразуем доменную модель в ответ gRPC
	return h.toPaymentResponse(payment), nil
}

// Вспомогательные методы для преобразования между доменными моделями и protobuf

// toPaymentResponse преобразует доменную модель платежа в ответ gRPC
func (h *PaymentHandler) toPaymentResponse(payment domain.Payment) *paymentpb.PaymentResponse {
	return &paymentpb.PaymentResponse{
		Id:            payment.ID.String(),
		CustomerId:    payment.CustomerID.String(),
		Amount:        payment.Amount,
		Currency:      payment.Currency,
		Description:   payment.Description,
		Status:        h.toPaymentStatusPb(payment.Status),
		MethodId:      payment.MethodID.String(),
		MethodType:    payment.MethodType,
		TransactionId: payment.TransactionID,
		ReceiptUrl:    payment.ReceiptURL,
		ErrorMessage:  payment.ErrorMessage,
		Metadata:      payment.Metadata,
		CreatedAt:     timestamppb.New(payment.CreatedAt),
		UpdatedAt:     timestamppb.New(payment.UpdatedAt),
	}
}

// toPaymentStatusPb преобразует доменный статус платежа в статус gRPC
func (h *PaymentHandler) toPaymentStatusPb(status domain.PaymentStatus) paymentpb.PaymentStatus {
	switch status {
	case domain.PaymentStatusPending:
		return paymentpb.PaymentStatus_PAYMENT_STATUS_PENDING
	case domain.PaymentStatusCompleted:
		return paymentpb.PaymentStatus_PAYMENT_STATUS_COMPLETED
	case domain.PaymentStatusFailed:
		return paymentpb.PaymentStatus_PAYMENT_STATUS_FAILED
	case domain.PaymentStatusRefunded:
		return paymentpb.PaymentStatus_PAYMENT_STATUS_REFUNDED
	default:
		return paymentpb.PaymentStatus_PAYMENT_STATUS_UNSPECIFIED
	}
}

// fromPaymentStatusPb преобразует статус gRPC в доменный статус платежа
func (h *PaymentHandler) fromPaymentStatusPb(status paymentpb.PaymentStatus) domain.PaymentStatus {
	switch status {
	case paymentpb.PaymentStatus_PAYMENT_STATUS_PENDING:
		return domain.PaymentStatusPending
	case paymentpb.PaymentStatus_PAYMENT_STATUS_COMPLETED:
		return domain.PaymentStatusCompleted
	case paymentpb.PaymentStatus_PAYMENT_STATUS_FAILED:
		return domain.PaymentStatusFailed
	case paymentpb.PaymentStatus_PAYMENT_STATUS_REFUNDED:
		return domain.PaymentStatusRefunded
	default:
		return domain.PaymentStatusPending
	}
}
