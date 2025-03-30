package service

import (
	"context"
	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
)

// CustomerService интерфейс сервиса для работы с клиентами
type CustomerService interface {
	GetAll(ctx context.Context) ([]domain.Customer, error)
	GetByID(ctx context.Context, id string) (domain.Customer, error)
	Create(ctx context.Context, req domain.CustomerRequest) (domain.Customer, error)
	Update(ctx context.Context, id string, req domain.CustomerRequest) (domain.Customer, error)
	Delete(ctx context.Context, id string) error
}

type customerService struct {
	repo repository.CustomerRepository
	log  *logger.Logger
}

// NewCustomerService создает новый сервис для работы с клиентами
func NewCustomerService(repo repository.CustomerRepository, log *logger.Logger) CustomerService {
	return &customerService{
		repo: repo,
		log:  log,
	}
}

func (s *customerService) GetAll(ctx context.Context) ([]domain.Customer, error) {
	s.log.Debug("Getting all customers")
	return s.repo.GetAll(ctx)
}

func (s *customerService) GetByID(ctx context.Context, id string) (domain.Customer, error) {
	s.log.Debug("Getting customer by ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Customer{}, repository.ErrInvalidData
	}

	return s.repo.GetByID(ctx, uuidID)
}

func (s *customerService) Create(ctx context.Context, req domain.CustomerRequest) (domain.Customer, error) {
	s.log.Debug("Creating customer with email: %s", req.Email)

	customer := domain.Customer{
		ID:         uuid.New(),
		Email:      req.Email,
		Name:       req.Name,
		Phone:      req.Phone,
		ExternalID: req.ExternalID,
		Metadata:   req.Metadata,
	}

	return s.repo.Create(ctx, customer)
}

func (s *customerService) Update(ctx context.Context, id string, req domain.CustomerRequest) (domain.Customer, error) {
	s.log.Debug("Updating customer with ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Customer{}, repository.ErrInvalidData
	}

	existing, err := s.repo.GetByID(ctx, uuidID)
	if err != nil {
		return domain.Customer{}, err
	}

	// Обновляем поля
	existing.Email = req.Email
	existing.Name = req.Name
	existing.Phone = req.Phone

	if req.ExternalID != "" {
		existing.ExternalID = req.ExternalID
	}

	if req.Metadata != nil {
		existing.Metadata = req.Metadata
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		return domain.Customer{}, err
	}

	return existing, nil
}

func (s *customerService) Delete(ctx context.Context, id string) error {
	s.log.Debug("Deleting customer with ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return repository.ErrInvalidData
	}

	return s.repo.Delete(ctx, uuidID)
}
