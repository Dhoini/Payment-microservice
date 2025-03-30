package service

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
)

// WebhookService интерфейс сервиса для работы с вебхуками
type WebhookService interface {
	// ProcessWebhook обрабатывает вебхук-событие
	ProcessWebhook(ctx context.Context, req domain.WebhookEventRequest) error

	// GetWebhookEvents возвращает список вебхук-событий
	GetWebhookEvents(ctx context.Context, limit, offset int) ([]domain.WebhookEvent, error)

	// GetWebhookEventByID возвращает вебхук-событие по ID
	GetWebhookEventByID(ctx context.Context, id string) (domain.WebhookEvent, error)

	// RetryWebhookEvent повторно обрабатывает вебхук-событие
	RetryWebhookEvent(ctx context.Context, id string) error
}

// WebhookEventRepository интерфейс репозитория для работы с вебхук-событиями
type WebhookEventRepository interface {
	// CreateEvent создает новое вебхук-событие
	CreateEvent(ctx context.Context, event domain.WebhookEvent) (domain.WebhookEvent, error)

	// GetEvents возвращает список вебхук-событий
	GetEvents(ctx context.Context, limit, offset int) ([]domain.WebhookEvent, error)

	// GetEventByID возвращает вебхук-событие по ID
	GetEventByID(ctx context.Context, id uuid.UUID) (domain.WebhookEvent, error)

	// UpdateEvent обновляет вебхук-событие
	UpdateEvent(ctx context.Context, event domain.WebhookEvent) error
}

// InMemoryWebhookEventRepository реализация репозитория вебхук-событий в памяти
type InMemoryWebhookEventRepository struct {
	events map[uuid.UUID]domain.WebhookEvent
	mutex  sync.RWMutex
	log    *logger.Logger
}

// NewInMemoryWebhookEventRepository создает новый репозиторий вебхук-событий в памяти
func NewInMemoryWebhookEventRepository(log *logger.Logger) *InMemoryWebhookEventRepository {
	return &InMemoryWebhookEventRepository{
		events: make(map[uuid.UUID]domain.WebhookEvent),
		log:    log,
	}
}

// CreateEvent создает новое вебхук-событие
func (r *InMemoryWebhookEventRepository) CreateEvent(ctx context.Context, event domain.WebhookEvent) (domain.WebhookEvent, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	event.CreatedAt = time.Now()
	event.UpdatedAt = time.Now()

	r.events[event.ID] = event

	return event, nil
}

// GetEvents возвращает список вебхук-событий
func (r *InMemoryWebhookEventRepository) GetEvents(ctx context.Context, limit, offset int) ([]domain.WebhookEvent, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	events := make([]domain.WebhookEvent, 0, len(r.events))
	for _, event := range r.events {
		events = append(events, event)
	}

	// Сортируем события по времени создания (новые в начале)
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})

	// Применяем пагинацию
	if offset >= len(events) {
		return []domain.WebhookEvent{}, nil
	}

	end := offset + limit
	if end > len(events) {
		end = len(events)
	}

	return events[offset:end], nil
}

// GetEventByID возвращает вебхук-событие по ID
func (r *InMemoryWebhookEventRepository) GetEventByID(ctx context.Context, id uuid.UUID) (domain.WebhookEvent, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	event, exists := r.events[id]
	if !exists {
		return domain.WebhookEvent{}, repository.ErrNotFound
	}

	return event, nil
}

// UpdateEvent обновляет вебхук-событие
func (r *InMemoryWebhookEventRepository) UpdateEvent(ctx context.Context, event domain.WebhookEvent) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, exists := r.events[event.ID]
	if !exists {
		return repository.ErrNotFound
	}

	event.UpdatedAt = time.Now()
	r.events[event.ID] = event

	return nil
}

// webhookService реализация сервиса для работы с вебхуками
type webhookService struct {
	repo        WebhookEventRepository
	paymentSvc  PaymentService
	customerSvc CustomerService
	log         *logger.Logger
}

// NewWebhookService создает новый сервис для работы с вебхуками
func NewWebhookService(
	repo WebhookEventRepository,
	paymentSvc PaymentService,
	customerSvc CustomerService,
	log *logger.Logger,
) WebhookService {
	return &webhookService{
		repo:        repo,
		paymentSvc:  paymentSvc,
		customerSvc: customerSvc,
		log:         log,
	}
}

// ProcessWebhook обрабатывает вебхук-событие
func (s *webhookService) ProcessWebhook(ctx context.Context, req domain.WebhookEventRequest) error {
	// Создаем новое вебхук-событие
	event := domain.WebhookEvent{
		ID:           uuid.New(),
		ExternalID:   req.ExternalID,
		Type:         req.Type,
		Status:       domain.WebhookEventStatusPending,
		Payload:      req.Payload,
		ResourceID:   req.ResourceID,
		Provider:     req.Provider,
		AttemptCount: 0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Сохраняем событие
	event, err := s.repo.CreateEvent(ctx, event)
	if err != nil {
		s.log.Error("Failed to save webhook event: %v", err)
		return err
	}

	// Обрабатываем событие асинхронно
	go func() {
		// Создаем новый контекст для асинхронной обработки
		asyncCtx := context.Background()

		// Обрабатываем событие
		err := s.processEvent(asyncCtx, event)
		if err != nil {
			s.log.Error("Failed to process webhook event: %v", err)
			// Обновляем статус события
			event.Status = domain.WebhookEventStatusFailed
			event.ErrorMessage = err.Error()
		} else {
			// Обновляем статус события
			event.Status = domain.WebhookEventStatusProcessed
			processedAt := time.Now()
			event.ProcessedAt = &processedAt
		}

		// Обновляем событие
		event.AttemptCount++
		lastAttempt := time.Now()
		event.LastAttempt = &lastAttempt

		err = s.repo.UpdateEvent(asyncCtx, event)
		if err != nil {
			s.log.Error("Failed to update webhook event: %v", err)
		}
	}()

	return nil
}

// processEvent обрабатывает конкретное вебхук-событие
func (s *webhookService) processEvent(ctx context.Context, event domain.WebhookEvent) error {
	// Обрабатываем событие в зависимости от типа
	switch event.Type {
	case domain.WebhookEventTypePaymentCompleted:
		return s.processPaymentCompletedEvent(ctx, event)
	case domain.WebhookEventTypePaymentFailed:
		return s.processPaymentFailedEvent(ctx, event)
	case domain.WebhookEventTypePaymentRefunded:
		return s.processPaymentRefundedEvent(ctx, event)
	case domain.WebhookEventTypeCustomerCreated:
		return s.processCustomerCreatedEvent(ctx, event)
	case domain.WebhookEventTypeCustomerUpdated:
		return s.processCustomerUpdatedEvent(ctx, event)
	case domain.WebhookEventTypeCustomerDeleted:
		return s.processCustomerDeletedEvent(ctx, event)
	default:
		s.log.Warn("Unknown webhook event type: %s", event.Type)
		return nil // Не считаем неизвестный тип ошибкой
	}
}

// processPaymentCompletedEvent обрабатывает событие успешного платежа
func (s *webhookService) processPaymentCompletedEvent(ctx context.Context, event domain.WebhookEvent) error {
	// Получаем ResourceID (ID платежа)
	paymentID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		return err
	}

	// Обновляем статус платежа
	_, err = s.paymentSvc.UpdateStatus(ctx, paymentID.String(), domain.PaymentStatusCompleted)
	return err
}

// processPaymentFailedEvent обрабатывает событие неудачного платежа
func (s *webhookService) processPaymentFailedEvent(ctx context.Context, event domain.WebhookEvent) error {
	// Получаем ResourceID (ID платежа)
	paymentID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		return err
	}

	// Обновляем статус платежа
	_, err = s.paymentSvc.UpdateStatus(ctx, paymentID.String(), domain.PaymentStatusFailed)
	return err
}

// processPaymentRefundedEvent обрабатывает событие возврата платежа
func (s *webhookService) processPaymentRefundedEvent(ctx context.Context, event domain.WebhookEvent) error {
	// Получаем ResourceID (ID платежа)
	paymentID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		return err
	}

	// Обновляем статус платежа
	_, err = s.paymentSvc.UpdateStatus(ctx, paymentID.String(), domain.PaymentStatusRefunded)
	return err
}

// processCustomerCreatedEvent обрабатывает событие создания клиента
func (s *webhookService) processCustomerCreatedEvent(ctx context.Context, event domain.WebhookEvent) error {
	// В данном случае обработка не требуется, так как клиент создается в нашей системе
	// и только потом в Stripe
	return nil
}

// processCustomerUpdatedEvent обрабатывает событие обновления клиента
func (s *webhookService) processCustomerUpdatedEvent(ctx context.Context, event domain.WebhookEvent) error {
	// В зависимости от бизнес-логики, возможно, требуется синхронизация данных
	// между нашей системой и Stripe
	return nil
}

// processCustomerDeletedEvent обрабатывает событие удаления клиента
func (s *webhookService) processCustomerDeletedEvent(ctx context.Context, event domain.WebhookEvent) error {
	// Получаем ResourceID (ID клиента)
	customerID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		return err
	}

	// Получаем клиента
	customer, err := s.customerSvc.GetByID(ctx, customerID.String())
	if err != nil {
		return err
	}

	// Помечаем клиента как удаленный в метаданных
	if customer.Metadata == nil {
		customer.Metadata = make(map[string]string)
	}
	customer.Metadata["stripe_deleted"] = "true"

	// Создаем запрос на обновление
	updateRequest := domain.CustomerRequest{
		Email:      customer.Email,
		Name:       customer.Name,
		Phone:      customer.Phone,
		ExternalID: customer.ExternalID,
		Metadata:   customer.Metadata,
	}

	// Обновляем клиента
	_, err = s.customerSvc.Update(ctx, customerID.String(), updateRequest)
	return err
}

// GetWebhookEvents возвращает список вебхук-событий
func (s *webhookService) GetWebhookEvents(ctx context.Context, limit, offset int) ([]domain.WebhookEvent, error) {
	return s.repo.GetEvents(ctx, limit, offset)
}

// GetWebhookEventByID возвращает вебхук-событие по ID
func (s *webhookService) GetWebhookEventByID(ctx context.Context, id string) (domain.WebhookEvent, error) {
	uuid, err := uuid.Parse(id)
	if err != nil {
		return domain.WebhookEvent{}, repository.ErrInvalidData
	}

	return s.repo.GetEventByID(ctx, uuid)
}

// RetryWebhookEvent повторно обрабатывает вебхук-событие
func (s *webhookService) RetryWebhookEvent(ctx context.Context, id string) error {
	uuid, err := uuid.Parse(id)
	if err != nil {
		return repository.ErrInvalidData
	}

	// Получаем событие
	event, err := s.repo.GetEventByID(ctx, uuid)
	if err != nil {
		return err
	}

	// Проверяем, что событие не в обработке
	if event.Status == domain.WebhookEventStatusPending {
		return fmt.Errorf("webhook event is already being processed")
	}

	// Сбрасываем статус и запускаем обработку
	event.Status = domain.WebhookEventStatusPending
	event.ErrorMessage = ""

	err = s.repo.UpdateEvent(ctx, event)
	if err != nil {
		return err
	}

	// Обрабатываем событие
	return s.processEvent(ctx, event)
}
