package handlers

import (
	"context"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/internal/service"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	customerpb "github.com/Dhoini/Payment-microservice/proto/customer/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CustomerHandler обработчик для gRPC сервиса клиентов
type CustomerHandler struct {
	customerpb.UnimplementedCustomerServiceServer
	service service.CustomerService
	log     *logger.Logger
}

// NewCustomerHandler создает новый обработчик клиентов
func NewCustomerHandler(service service.CustomerService, log *logger.Logger) *CustomerHandler {
	return &CustomerHandler{
		service: service,
		log:     log,
	}
}

// CreateCustomer создает нового клиента
func (h *CustomerHandler) CreateCustomer(ctx context.Context, req *customerpb.CreateCustomerRequest) (*customerpb.CustomerResponse, error) {
	h.log.Debug("gRPC CreateCustomer request received")

	// Преобразуем запрос gRPC в доменную модель
	customerReq := domain.CustomerRequest{
		Email:      req.Email,
		Name:       req.Name,
		Phone:      req.Phone,
		ExternalID: req.ExternalId,
		Metadata:   req.Metadata,
	}

	// Создаем клиента
	customer, err := h.service.Create(ctx, customerReq)
	if err != nil {
		if err == repository.ErrDuplicate {
			return nil, status.Errorf(codes.AlreadyExists, "customer with this email already exists")
		}
		h.log.Error("Failed to create customer: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create customer: %v", err)
	}

	// Преобразуем доменную модель в ответ gRPC
	return h.toCustomerResponse(customer), nil
}

// GetCustomer возвращает информацию о клиенте по ID
func (h *CustomerHandler) GetCustomer(ctx context.Context, req *customerpb.GetCustomerRequest) (*customerpb.CustomerResponse, error) {
	h.log.Debug("gRPC GetCustomer request received for ID: %s", req.Id)

	// Получаем клиента
	customer, err := h.service.GetByID(ctx, req.Id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "customer not found: %s", req.Id)
		}
		if err == repository.ErrInvalidData {
			return nil, status.Errorf(codes.InvalidArgument, "invalid customer ID: %s", req.Id)
		}
		h.log.Error("Failed to get customer: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get customer: %v", err)
	}

	// Преобразуем доменную модель в ответ gRPC
	return h.toCustomerResponse(customer), nil
}

// ListCustomers возвращает список клиентов
func (h *CustomerHandler) ListCustomers(ctx context.Context, req *customerpb.ListCustomersRequest) (*customerpb.ListCustomersResponse, error) {
	h.log.Debug("gRPC ListCustomers request received")

	// Получаем всех клиентов
	customers, err := h.service.GetAll(ctx)
	if err != nil {
		h.log.Error("Failed to list customers: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to list customers: %v", err)
	}

	// В реальном проекте здесь должны быть применены фильтры и пагинация
	// на основе параметров запроса (limit, offset, search)

	// Преобразуем доменные модели в ответы gRPC
	pbCustomers := make([]*customerpb.CustomerResponse, len(customers))
	for i, customer := range customers {
		pbCustomers[i] = h.toCustomerResponse(customer)
	}

	return &customerpb.ListCustomersResponse{
		Customers: pbCustomers,
		Total:     int32(len(customers)),
	}, nil
}

// UpdateCustomer обновляет информацию о клиенте
func (h *CustomerHandler) UpdateCustomer(ctx context.Context, req *customerpb.UpdateCustomerRequest) (*customerpb.CustomerResponse, error) {
	h.log.Debug("gRPC UpdateCustomer request received for ID: %s", req.Id)

	// Преобразуем запрос gRPC в доменную модель
	customerReq := domain.CustomerRequest{
		Email:      req.Email,
		Name:       req.Name,
		Phone:      req.Phone,
		ExternalID: req.ExternalId,
		Metadata:   req.Metadata,
	}

	// Обновляем клиента
	customer, err := h.service.Update(ctx, req.Id, customerReq)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "customer not found: %s", req.Id)
		}
		if err == repository.ErrInvalidData {
			return nil, status.Errorf(codes.InvalidArgument, "invalid customer ID or data: %s", req.Id)
		}
		if err == repository.ErrDuplicate {
			return nil, status.Errorf(codes.AlreadyExists, "customer with this email already exists")
		}
		h.log.Error("Failed to update customer: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to update customer: %v", err)
	}

	// Преобразуем доменную модель в ответ gRPC
	return h.toCustomerResponse(customer), nil
}

// DeleteCustomer удаляет клиента
func (h *CustomerHandler) DeleteCustomer(ctx context.Context, req *customerpb.DeleteCustomerRequest) (*customerpb.DeleteCustomerResponse, error) {
	h.log.Debug("gRPC DeleteCustomer request received for ID: %s", req.Id)

	// Удаляем клиента
	err := h.service.Delete(ctx, req.Id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Errorf(codes.NotFound, "customer not found: %s", req.Id)
		}
		if err == repository.ErrInvalidData {
			return nil, status.Errorf(codes.InvalidArgument, "invalid customer ID: %s", req.Id)
		}
		h.log.Error("Failed to delete customer: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to delete customer: %v", err)
	}

	return &customerpb.DeleteCustomerResponse{
		Success: true,
	}, nil
}

// Вспомогательные методы для преобразования между доменными моделями и protobuf

// toCustomerResponse преобразует доменную модель клиента в ответ gRPC
func (h *CustomerHandler) toCustomerResponse(customer domain.Customer) *customerpb.CustomerResponse {
	return &customerpb.CustomerResponse{
		Id:         customer.ID.String(),
		Email:      customer.Email,
		Name:       customer.Name,
		Phone:      customer.Phone,
		ExternalId: customer.ExternalID,
		Metadata:   customer.Metadata,
		CreatedAt:  timestamppb.New(customer.CreatedAt),
		UpdatedAt:  timestamppb.New(customer.UpdatedAt),
	}
}
