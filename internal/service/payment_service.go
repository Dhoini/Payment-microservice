package service

import (
	"context"
	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
)

// PaymentService интерфейс сервиса для работы с платежами
type PaymentService interface {
	GetAll(ctx context.Context) ([]domain.Payment, error)
	GetByID(ctx context.Context, id string) (domain.Payment, error)
	GetByCustomerID(ctx context.Context, customerID string) ([]domain.Payment, error)
	Create(ctx context.Context, req domain.PaymentRequest) (domain.Payment, error)
	UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) (domain.Payment, error)
}

type paymentService struct {
	repo         repository.PaymentRepository
	customerRepo repository.CustomerRepository
	log          *logger.Logger
}

// NewPaymentService создает новый сервис для работы с платежами
func NewPaymentService(repo repository.PaymentRepository, customerRepo repository.CustomerRepository, log *logger.Logger) PaymentService {
	return &paymentService{
		repo:         repo,
		customerRepo: customerRepo,
		log:          log,
	}
}

func (s *paymentService) GetAll(ctx context.Context) ([]domain.Payment, error) {
	s.log.Debug("Getting all payments")
	return s.repo.GetAll(ctx)
}

func (s *paymentService) GetByID(ctx context.Context, id string) (domain.Payment, error) {
	s.log.Debug("Getting payment by ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Payment{}, repository.ErrInvalidData
	}

	return s.repo.GetByID(ctx, uuidID)
}

func (s *paymentService) GetByCustomerID(ctx context.Context, customerID string) ([]domain.Payment, error) {
	s.log.Debug("Getting payments by customer ID: %s", customerID)

	uuidCustomerID, err := uuid.Parse(customerID)
	if err != nil {
		s.log.Warn("Invalid UUID format for customer ID: %s", customerID)
		return nil, repository.ErrInvalidData
	}

	// Проверяем существование клиента
	_, err = s.customerRepo.GetByID(ctx, uuidCustomerID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Customer not found: %s", customerID)
		} else {
			s.log.Error("Error fetching customer: %v", err)
		}
		return nil, err
	}

	return s.repo.GetByCustomerID(ctx, uuidCustomerID)
}

func (s *paymentService) Create(ctx context.Context, req domain.PaymentRequest) (domain.Payment, error) {
	s.log.Debug("Creating payment for customer: %s", req.CustomerID)

	// Парсим UUID из строки
	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		s.log.Warn("Invalid UUID format for customer ID: %s", req.CustomerID)
		return domain.Payment{}, repository.ErrInvalidData
	}

	// Проверяем существование клиента
	_, err = s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Customer not found: %s", req.CustomerID)
		} else {
			s.log.Error("Error fetching customer: %v", err)
		}
		return domain.Payment{}, err
	}

	var methodID uuid.UUID
	if req.MethodID != "" {
		methodID, err = uuid.Parse(req.MethodID)
		if err != nil {
			s.log.Warn("Invalid UUID format for method ID: %s", req.MethodID)
			return domain.Payment{}, repository.ErrInvalidData
		}
	}

	payment := domain.Payment{
		ID:          uuid.New(),
		CustomerID:  customerID,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: req.Description,
		Status:      domain.PaymentStatusPending,
		MethodID:    methodID,
		Metadata:    req.Metadata,
	}

	// В реальном сервисе здесь была бы интеграция с платежным шлюзом (Stripe и т.д.)
	// для создания реального платежа. Пока просто создаем платеж со статусом "pending"

	return s.repo.Create(ctx, payment)
}

func (s *paymentService) UpdateStatus(ctx context.Context, id string, status domain.PaymentStatus) (domain.Payment, error) {
	s.log.Debug("Updating payment status: %s -> %s", id, status)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Payment{}, repository.ErrInvalidData
	}

	payment, err := s.repo.GetByID(ctx, uuidID)
	if err != nil {
		return domain.Payment{}, err
	}

	payment.Status = status

	if err := s.repo.Update(ctx, payment); err != nil {
		return domain.Payment{}, err
	}

	return payment, nil
}
