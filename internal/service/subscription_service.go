package service

import (
	"context"
	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
	"time"
)

// SubscriptionService интерфейс сервиса для работы с подписками
type SubscriptionService interface {
	// Методы для управления подписками
	Create(ctx context.Context, req domain.SubscriptionRequest) (domain.Subscription, error)
	GetByID(ctx context.Context, id string) (domain.Subscription, error)
	GetAll(ctx context.Context) ([]domain.Subscription, error)
	GetByCustomerID(ctx context.Context, customerID string) ([]domain.Subscription, error)
	Cancel(ctx context.Context, id string, cancelAtPeriodEnd bool) (domain.Subscription, error)
	Pause(ctx context.Context, id string) (domain.Subscription, error)
	Resume(ctx context.Context, id string) (domain.Subscription, error)

	// Методы для управления планами подписок
	CreatePlan(ctx context.Context, req domain.SubscriptionPlanRequest) (domain.SubscriptionPlan, error)
	GetPlanByID(ctx context.Context, id string) (domain.SubscriptionPlan, error)
	GetAllPlans(ctx context.Context) ([]domain.SubscriptionPlan, error)
	UpdatePlan(ctx context.Context, id string, req domain.SubscriptionPlanRequest) (domain.SubscriptionPlan, error)
	DeletePlan(ctx context.Context, id string) error
}

// SubscriptionRepository интерфейс репозитория для работы с подписками
type SubscriptionRepository interface {
	// Методы для подписок
	GetAll(ctx context.Context) ([]domain.Subscription, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Subscription, error)
	GetByCustomerID(ctx context.Context, customerID uuid.UUID) ([]domain.Subscription, error)
	Create(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error)
	Update(ctx context.Context, subscription domain.Subscription) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Методы для планов подписок
	GetAllPlans(ctx context.Context) ([]domain.SubscriptionPlan, error)
	GetPlanByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPlan, error)
	CreatePlan(ctx context.Context, plan domain.SubscriptionPlan) (domain.SubscriptionPlan, error)
	UpdatePlan(ctx context.Context, plan domain.SubscriptionPlan) error
	DeletePlan(ctx context.Context, id uuid.UUID) error
}

// SubscriptionPlanRepository интерфейс репозитория для работы с планами подписок
type SubscriptionPlanRepository interface {
	GetAll(ctx context.Context) ([]domain.SubscriptionPlan, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPlan, error)
	Create(ctx context.Context, plan domain.SubscriptionPlan) (domain.SubscriptionPlan, error)
	Update(ctx context.Context, plan domain.SubscriptionPlan) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type subscriptionService struct {
	subscriptionRepo SubscriptionRepository
	customerRepo     repository.CustomerRepository
	paymentRepo      repository.PaymentRepository
	log              *logger.Logger
}

// NewSubscriptionService создает новый сервис для работы с подписками
func NewSubscriptionService(
	subscriptionRepo SubscriptionRepository,
	customerRepo repository.CustomerRepository,
	paymentRepo repository.PaymentRepository,
	log *logger.Logger,
) SubscriptionService {
	return &subscriptionService{
		subscriptionRepo: subscriptionRepo,
		customerRepo:     customerRepo,
		paymentRepo:      paymentRepo,
		log:              log,
	}
}

// Create создает новую подписку
func (s *subscriptionService) Create(ctx context.Context, req domain.SubscriptionRequest) (domain.Subscription, error) {
	s.log.Debug("Creating subscription for customer: %s, plan: %s", req.CustomerID, req.PlanID)

	// Проверяем существование клиента
	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		s.log.Warn("Invalid UUID format for customer ID: %s", req.CustomerID)
		return domain.Subscription{}, repository.ErrInvalidData
	}

	_, err = s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Customer not found: %s", req.CustomerID)
		} else {
			s.log.Error("Error fetching customer: %v", err)
		}
		return domain.Subscription{}, err
	}

	// Проверяем существование плана подписки
	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		s.log.Warn("Invalid UUID format for plan ID: %s", req.PlanID)
		return domain.Subscription{}, repository.ErrInvalidData
	}

	plan, err := s.subscriptionRepo.GetPlanByID(ctx, planID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription plan not found: %s", req.PlanID)
		} else {
			s.log.Error("Error fetching subscription plan: %v", err)
		}
		return domain.Subscription{}, err
	}

	// Проверка активности плана
	if !plan.Active {
		s.log.Warn("Subscription plan is not active: %s", req.PlanID)
		return domain.Subscription{}, domain.ErrInvalidOperation
	}

	// Создаем новую подписку
	now := time.Now()
	subscription := domain.Subscription{
		ID:                 uuid.New(),
		CustomerID:         customerID,
		PlanID:             planID,
		Status:             domain.SubscriptionStatusActive,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   calculatePeriodEnd(now, plan.Interval, plan.IntervalCount),
		CancelAtPeriodEnd:  false,
		CreatedAt:          now,
		UpdatedAt:          now,
		Metadata:           req.Metadata,
	}

	// Если указан пробный период
	if req.TrialPeriodDays > 0 {
		subscription.Status = domain.SubscriptionStatusTrialing
		subscription.TrialStart = &now
		trialEnd := now.AddDate(0, 0, req.TrialPeriodDays)
		subscription.TrialEnd = &trialEnd
		subscription.CurrentPeriodEnd = trialEnd
	}

	// Если указан метод оплаты по умолчанию
	if req.DefaultPaymentMethodID != "" {
		subscription.DefaultPaymentMethodID = req.DefaultPaymentMethodID
	}

	// Сохраняем подписку
	createdSubscription, err := s.subscriptionRepo.Create(ctx, subscription)
	if err != nil {
		s.log.Error("Failed to create subscription: %v", err)
		return domain.Subscription{}, err
	}

	s.log.Info("Created subscription with ID: %s", createdSubscription.ID)
	return createdSubscription, nil
}

// GetByID возвращает подписку по ID
func (s *subscriptionService) GetByID(ctx context.Context, id string) (domain.Subscription, error) {
	s.log.Debug("Getting subscription by ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Subscription{}, repository.ErrInvalidData
	}

	subscription, err := s.subscriptionRepo.GetByID(ctx, uuidID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription not found: %s", id)
		} else {
			s.log.Error("Error fetching subscription: %v", err)
		}
		return domain.Subscription{}, err
	}

	return subscription, nil
}

// GetAll возвращает все подписки
func (s *subscriptionService) GetAll(ctx context.Context) ([]domain.Subscription, error) {
	s.log.Debug("Getting all subscriptions")

	subscriptions, err := s.subscriptionRepo.GetAll(ctx)
	if err != nil {
		s.log.Error("Failed to get subscriptions: %v", err)
		return nil, err
	}

	return subscriptions, nil
}

// GetByCustomerID возвращает подписки клиента
func (s *subscriptionService) GetByCustomerID(ctx context.Context, customerID string) ([]domain.Subscription, error) {
	s.log.Debug("Getting subscriptions for customer: %s", customerID)

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

	subscriptions, err := s.subscriptionRepo.GetByCustomerID(ctx, uuidCustomerID)
	if err != nil {
		s.log.Error("Failed to get subscriptions for customer: %v", err)
		return nil, err
	}

	return subscriptions, nil
}

// Cancel отменяет подписку
func (s *subscriptionService) Cancel(ctx context.Context, id string, cancelAtPeriodEnd bool) (domain.Subscription, error) {
	s.log.Debug("Cancelling subscription with ID: %s, cancelAtPeriodEnd: %v", id, cancelAtPeriodEnd)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Subscription{}, repository.ErrInvalidData
	}

	subscription, err := s.subscriptionRepo.GetByID(ctx, uuidID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription not found: %s", id)
		} else {
			s.log.Error("Error fetching subscription: %v", err)
		}
		return domain.Subscription{}, err
	}

	// Проверяем, что подписку можно отменить
	if subscription.Status == domain.SubscriptionStatusCanceled {
		return subscription, nil // Уже отменена
	}

	now := time.Now()
	subscription.UpdatedAt = now
	subscription.CancelAtPeriodEnd = cancelAtPeriodEnd

	if !cancelAtPeriodEnd {
		subscription.Status = domain.SubscriptionStatusCanceled
		subscription.CanceledAt = &now
	}

	err = s.subscriptionRepo.Update(ctx, subscription)
	if err != nil {
		s.log.Error("Failed to update subscription: %v", err)
		return domain.Subscription{}, err
	}

	s.log.Info("Cancelled subscription with ID: %s", id)
	return subscription, nil
}

// Pause приостанавливает подписку
func (s *subscriptionService) Pause(ctx context.Context, id string) (domain.Subscription, error) {
	s.log.Debug("Pausing subscription with ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Subscription{}, repository.ErrInvalidData
	}

	subscription, err := s.subscriptionRepo.GetByID(ctx, uuidID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription not found: %s", id)
		} else {
			s.log.Error("Error fetching subscription: %v", err)
		}
		return domain.Subscription{}, err
	}

	// Проверяем, что подписку можно приостановить
	if subscription.Status != domain.SubscriptionStatusActive {
		s.log.Warn("Cannot pause subscription that is not active: %s, status: %s", id, subscription.Status)
		return domain.Subscription{}, domain.ErrInvalidOperation
	}

	subscription.Status = domain.SubscriptionStatusPaused
	subscription.UpdatedAt = time.Now()

	err = s.subscriptionRepo.Update(ctx, subscription)
	if err != nil {
		s.log.Error("Failed to update subscription: %v", err)
		return domain.Subscription{}, err
	}

	s.log.Info("Paused subscription with ID: %s", id)
	return subscription, nil
}

// Resume возобновляет подписку
func (s *subscriptionService) Resume(ctx context.Context, id string) (domain.Subscription, error) {
	s.log.Debug("Resuming subscription with ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Subscription{}, repository.ErrInvalidData
	}

	subscription, err := s.subscriptionRepo.GetByID(ctx, uuidID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription not found: %s", id)
		} else {
			s.log.Error("Error fetching subscription: %v", err)
		}
		return domain.Subscription{}, err
	}

	// Проверяем, что подписку можно возобновить
	if subscription.Status != domain.SubscriptionStatusPaused {
		s.log.Warn("Cannot resume subscription that is not paused: %s, status: %s", id, subscription.Status)
		return domain.Subscription{}, domain.ErrInvalidOperation
	}

	subscription.Status = domain.SubscriptionStatusActive
	subscription.UpdatedAt = time.Now()

	err = s.subscriptionRepo.Update(ctx, subscription)
	if err != nil {
		s.log.Error("Failed to update subscription: %v", err)
		return domain.Subscription{}, err
	}

	s.log.Info("Resumed subscription with ID: %s", id)
	return subscription, nil
}

// CreatePlan создает новый план подписки
func (s *subscriptionService) CreatePlan(ctx context.Context, req domain.SubscriptionPlanRequest) (domain.SubscriptionPlan, error) {
	s.log.Debug("Creating subscription plan: %s", req.Name)

	// Создаем новый план подписки
	plan := domain.SubscriptionPlan{
		ID:              uuid.New(),
		Name:            req.Name,
		Amount:          req.Amount,
		Currency:        req.Currency,
		Interval:        req.Interval,
		IntervalCount:   req.IntervalCount,
		TrialPeriodDays: req.TrialPeriodDays,
		Active:          true,
		Metadata:        req.Metadata,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Сохраняем план подписки
	createdPlan, err := s.subscriptionRepo.CreatePlan(ctx, plan)
	if err != nil {
		s.log.Error("Failed to create subscription plan: %v", err)
		return domain.SubscriptionPlan{}, err
	}

	s.log.Info("Created subscription plan with ID: %s", createdPlan.ID)
	return createdPlan, nil
}

// GetPlanByID возвращает план подписки по ID
func (s *subscriptionService) GetPlanByID(ctx context.Context, id string) (domain.SubscriptionPlan, error) {
	s.log.Debug("Getting subscription plan by ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.SubscriptionPlan{}, repository.ErrInvalidData
	}

	plan, err := s.subscriptionRepo.GetPlanByID(ctx, uuidID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription plan not found: %s", id)
		} else {
			s.log.Error("Error fetching subscription plan: %v", err)
		}
		return domain.SubscriptionPlan{}, err
	}

	return plan, nil
}

// GetAllPlans возвращает все планы подписок
func (s *subscriptionService) GetAllPlans(ctx context.Context) ([]domain.SubscriptionPlan, error) {
	s.log.Debug("Getting all subscription plans")

	plans, err := s.subscriptionRepo.GetAllPlans(ctx)
	if err != nil {
		s.log.Error("Failed to get subscription plans: %v", err)
		return nil, err
	}

	return plans, nil
}

// UpdatePlan обновляет план подписки
func (s *subscriptionService) UpdatePlan(ctx context.Context, id string, req domain.SubscriptionPlanRequest) (domain.SubscriptionPlan, error) {
	s.log.Debug("Updating subscription plan with ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.SubscriptionPlan{}, repository.ErrInvalidData
	}

	plan, err := s.subscriptionRepo.GetPlanByID(ctx, uuidID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription plan not found: %s", id)
		} else {
			s.log.Error("Error fetching subscription plan: %v", err)
		}
		return domain.SubscriptionPlan{}, err
	}

	// Обновляем поля плана
	plan.Name = req.Name
	plan.Active = req.Active
	if req.Metadata != nil {
		plan.Metadata = req.Metadata
	}
	plan.UpdatedAt = time.Now()

	err = s.subscriptionRepo.UpdatePlan(ctx, plan)
	if err != nil {
		s.log.Error("Failed to update subscription plan: %v", err)
		return domain.SubscriptionPlan{}, err
	}

	s.log.Info("Updated subscription plan with ID: %s", id)
	return plan, nil
}

// DeletePlan удаляет план подписки
func (s *subscriptionService) DeletePlan(ctx context.Context, id string) error {
	s.log.Debug("Deleting subscription plan with ID: %s", id)

	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return repository.ErrInvalidData
	}

	// Проверяем существование плана
	_, err = s.subscriptionRepo.GetPlanByID(ctx, uuidID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription plan not found: %s", id)
		} else {
			s.log.Error("Error fetching subscription plan: %v", err)
		}
		return err
	}

	err = s.subscriptionRepo.DeletePlan(ctx, uuidID)
	if err != nil {
		s.log.Error("Failed to delete subscription plan: %v", err)
		return err
	}

	s.log.Info("Deleted subscription plan with ID: %s", id)
	return nil
}

// Вспомогательные функции

// calculatePeriodEnd вычисляет дату окончания периода подписки
func calculatePeriodEnd(startDate time.Time, interval domain.SubscriptionInterval, intervalCount int) time.Time {
	if intervalCount <= 0 {
		intervalCount = 1
	}

	switch interval {
	case domain.SubscriptionIntervalDay:
		return startDate.AddDate(0, 0, intervalCount)
	case domain.SubscriptionIntervalWeek:
		return startDate.AddDate(0, 0, 7*intervalCount)
	case domain.SubscriptionIntervalMonth:
		return startDate.AddDate(0, intervalCount, 0)
	case domain.SubscriptionIntervalYear:
		return startDate.AddDate(intervalCount, 0, 0)
	default:
		return startDate.AddDate(0, 1, 0) // По умолчанию 1 месяц
	}
}
