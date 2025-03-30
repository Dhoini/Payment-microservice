package repository

import (
	"context"
	"sync"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
)

// PaymentRepository интерфейс для работы с платежами
type PaymentRepository interface {
	GetAll(ctx context.Context) ([]domain.Payment, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Payment, error)
	GetByCustomerID(ctx context.Context, customerID uuid.UUID) ([]domain.Payment, error)
	Create(ctx context.Context, payment domain.Payment) (domain.Payment, error)
	Update(ctx context.Context, payment domain.Payment) error
}

// InMemoryPaymentRepository реализация репозитория платежей в памяти
type InMemoryPaymentRepository struct {
	payments map[uuid.UUID]domain.Payment
	mutex    sync.RWMutex
	log      *logger.Logger
}

// NewInMemoryPaymentRepository создает новый репозиторий платежей в памяти
func NewInMemoryPaymentRepository(log *logger.Logger) *InMemoryPaymentRepository {
	return &InMemoryPaymentRepository{
		payments: make(map[uuid.UUID]domain.Payment),
		log:      log,
	}
}

// GetAll возвращает все платежи
func (r *InMemoryPaymentRepository) GetAll(ctx context.Context) ([]domain.Payment, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	payments := make([]domain.Payment, 0, len(r.payments))
	for _, payment := range r.payments {
		payments = append(payments, payment)
	}

	return payments, nil
}

// GetByID возвращает платеж по ID
func (r *InMemoryPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Payment, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	payment, exists := r.payments[id]
	if !exists {
		return domain.Payment{}, ErrNotFound
	}

	return payment, nil
}

// GetByCustomerID возвращает платежи по ID клиента
func (r *InMemoryPaymentRepository) GetByCustomerID(ctx context.Context, customerID uuid.UUID) ([]domain.Payment, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var payments []domain.Payment
	for _, payment := range r.payments {
		if payment.CustomerID == customerID {
			payments = append(payments, payment)
		}
	}

	return payments, nil
}

// Create создает новый платеж
func (r *InMemoryPaymentRepository) Create(ctx context.Context, payment domain.Payment) (domain.Payment, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	payment.CreatedAt = time.Now()
	payment.UpdatedAt = time.Now()

	r.payments[payment.ID] = payment

	return payment, nil
}

// Update обновляет существующий платеж
func (r *InMemoryPaymentRepository) Update(ctx context.Context, payment domain.Payment) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	existing, exists := r.payments[payment.ID]
	if !exists {
		return ErrNotFound
	}

	payment.CreatedAt = existing.CreatedAt
	payment.UpdatedAt = time.Now()

	r.payments[payment.ID] = payment

	return nil
}
