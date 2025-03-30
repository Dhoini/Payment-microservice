package repository

import (
	"context"
	"sync"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
)

// CustomerRepository интерфейс для работы с клиентами
type CustomerRepository interface {
	GetAll(ctx context.Context) ([]domain.Customer, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Customer, error)
	Create(ctx context.Context, customer domain.Customer) (domain.Customer, error)
	Update(ctx context.Context, customer domain.Customer) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// InMemoryCustomerRepository реализация репозитория в памяти
type InMemoryCustomerRepository struct {
	customers map[uuid.UUID]domain.Customer
	mutex     sync.RWMutex
	log       *logger.Logger
}

// NewInMemoryCustomerRepository создает новый репозиторий клиентов в памяти
func NewInMemoryCustomerRepository(log *logger.Logger) *InMemoryCustomerRepository {
	return &InMemoryCustomerRepository{
		customers: make(map[uuid.UUID]domain.Customer),
		log:       log,
	}
}

// GetAll возвращает всех клиентов
func (r *InMemoryCustomerRepository) GetAll(ctx context.Context) ([]domain.Customer, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	customers := make([]domain.Customer, 0, len(r.customers))
	for _, customer := range r.customers {
		customers = append(customers, customer)
	}

	return customers, nil
}

// GetByID возвращает клиента по ID
func (r *InMemoryCustomerRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Customer, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	customer, exists := r.customers[id]
	if !exists {
		return domain.Customer{}, ErrNotFound
	}

	return customer, nil
}

// Create создает нового клиента
func (r *InMemoryCustomerRepository) Create(ctx context.Context, customer domain.Customer) (domain.Customer, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Проверка на уникальность email
	for _, c := range r.customers {
		if c.Email == customer.Email {
			return domain.Customer{}, ErrDuplicate
		}
	}

	customer.CreatedAt = time.Now()
	customer.UpdatedAt = time.Now()

	r.customers[customer.ID] = customer

	return customer, nil
}

// Update обновляет существующего клиента
func (r *InMemoryCustomerRepository) Update(ctx context.Context, customer domain.Customer) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	existing, exists := r.customers[customer.ID]
	if !exists {
		return ErrNotFound
	}

	// Проверка на уникальность email
	for id, c := range r.customers {
		if c.Email == customer.Email && id != customer.ID {
			return ErrDuplicate
		}
	}

	customer.CreatedAt = existing.CreatedAt
	customer.UpdatedAt = time.Now()

	r.customers[customer.ID] = customer

	return nil
}

// Delete удаляет клиента
func (r *InMemoryCustomerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.customers[id]; !exists {
		return ErrNotFound
	}

	delete(r.customers, id)

	return nil
}
