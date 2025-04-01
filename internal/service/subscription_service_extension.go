package service

import (
	"context"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
)

// SubscriptionServiceExtension предоставляет расширенные функции для сервиса подписок
type SubscriptionServiceExtension interface {
	// RenewSubscription продлевает подписку на следующий период
	RenewSubscription(ctx context.Context, id string) (domain.Subscription, error)

	// CheckSubscriptionsForRenewal проверяет подписки для продления
	CheckSubscriptionsForRenewal(ctx context.Context) error

	// GetSubscriptionWithPlan возвращает подписку с информацией о плане
	GetSubscriptionWithPlan(ctx context.Context, id string) (domain.Subscription, domain.SubscriptionPlan, error)

	// GetCustomerSubscriptionsWithPlans возвращает подписки клиента с информацией о планах
	GetCustomerSubscriptionsWithPlans(ctx context.Context, customerID string) ([]domain.Subscription, []domain.SubscriptionPlan, error)

	// CalculateNextPaymentDate вычисляет дату следующего платежа для подписки
	CalculateNextPaymentDate(ctx context.Context, id string) (*time.Time, error)
}

// subscriptionServiceExtension реализация расширенных функций сервиса подписок
type subscriptionServiceExtension struct {
	subscriptionRepo SubscriptionRepository
	customerRepo     repository.CustomerRepository
	paymentRepo      repository.PaymentRepository
	log              *logger.Logger
}

// NewSubscriptionServiceExtension создает новый расширенный сервис подписок
func NewSubscriptionServiceExtension(
	subscriptionRepo SubscriptionRepository,
	customerRepo repository.CustomerRepository,
	paymentRepo repository.PaymentRepository,
	log *logger.Logger,
) SubscriptionServiceExtension {
	return &subscriptionServiceExtension{
		subscriptionRepo: subscriptionRepo,
		customerRepo:     customerRepo,
		paymentRepo:      paymentRepo,
		log:              log,
	}
}

// RenewSubscription продлевает подписку на следующий период
func (s *subscriptionServiceExtension) RenewSubscription(ctx context.Context, id string) (domain.Subscription, error) {
	s.log.Debug("Renewing subscription with ID: %s", id)

	// Преобразуем строку в UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Subscription{}, repository.ErrInvalidData
	}

	// Получаем подписку
	subscription, err := s.subscriptionRepo.GetByID(ctx, uuidID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription not found: %s", id)
		} else {
			s.log.Error("Error fetching subscription: %v", err)
		}
		return domain.Subscription{}, err
	}

	// Проверяем, что подписку можно продлить
	if subscription.Status != domain.SubscriptionStatusActive &&
		subscription.Status != domain.SubscriptionStatusTrialing {
		s.log.Warn("Cannot renew subscription that is not active or in trial: %s, status: %s", id, subscription.Status)
		return domain.Subscription{}, domain.ErrInvalidOperation
	}

	// Получаем план подписки
	plan, err := s.subscriptionRepo.GetPlanByID(ctx, subscription.PlanID)
	if err != nil {
		s.log.Error("Failed to get subscription plan: %v", err)
		return domain.Subscription{}, err
	}

	// Устанавливаем новый период
	if subscription.Status == domain.SubscriptionStatusTrialing {
		// Если подписка была в пробном периоде, переводим ее в активную
		subscription.Status = domain.SubscriptionStatusActive
		subscription.TrialEnd = nil
	}

	// Устанавливаем новые даты периода
	subscription.CurrentPeriodStart = subscription.CurrentPeriodEnd
	subscription.CurrentPeriodEnd = calculatePeriodEnd(
		subscription.CurrentPeriodStart,
		plan.Interval,
		plan.IntervalCount,
	)

	// Обновляем подписку
	subscription.UpdatedAt = time.Now()
	err = s.subscriptionRepo.Update(ctx, subscription)
	if err != nil {
		s.log.Error("Failed to update subscription: %v", err)
		return domain.Subscription{}, err
	}

	s.log.Info("Renewed subscription with ID: %s", id)
	return subscription, nil
}

// CheckSubscriptionsForRenewal проверяет подписки для продления
func (s *subscriptionServiceExtension) CheckSubscriptionsForRenewal(ctx context.Context) error {
	s.log.Debug("Checking subscriptions for renewal")

	// Получаем все подписки
	subscriptions, err := s.subscriptionRepo.GetAll(ctx)
	if err != nil {
		s.log.Error("Failed to get subscriptions: %v", err)
		return err
	}

	// Текущее время
	now := time.Now()

	// Проверяем каждую подписку
	for _, subscription := range subscriptions {
		// Проверяем, что подписка активна и срок действия истек
		if (subscription.Status == domain.SubscriptionStatusActive ||
			subscription.Status == domain.SubscriptionStatusTrialing) &&
			subscription.CurrentPeriodEnd.Before(now) {

			// Если подписка отменена в конце периода, то меняем статус на отмененный
			if subscription.CancelAtPeriodEnd {
				subscription.Status = domain.SubscriptionStatusCanceled
				canceledAt := time.Now()
				subscription.CanceledAt = &canceledAt
				subscription.UpdatedAt = time.Now()

				err := s.subscriptionRepo.Update(ctx, subscription)
				if err != nil {
					s.log.Error("Failed to update subscription status to canceled: %v", err)
					continue
				}

				s.log.Info("Subscription %s has been canceled", subscription.ID)
				continue
			}

			// Иначе продлеваем подписку
			_, err := s.RenewSubscription(ctx, subscription.ID.String())
			if err != nil {
				s.log.Error("Failed to renew subscription %s: %v", subscription.ID, err)
				continue
			}
		}
	}

	s.log.Info("Finished checking subscriptions for renewal")
	return nil
}

// GetSubscriptionWithPlan возвращает подписку с информацией о плане
func (s *subscriptionServiceExtension) GetSubscriptionWithPlan(ctx context.Context, id string) (domain.Subscription, domain.SubscriptionPlan, error) {
	s.log.Debug("Getting subscription with plan for ID: %s", id)

	// Преобразуем строку в UUID
	uuidID, err := uuid.Parse(id)
	if err != nil {
		s.log.Warn("Invalid UUID format: %s", id)
		return domain.Subscription{}, domain.SubscriptionPlan{}, repository.ErrInvalidData
	}

	// Получаем подписку
	subscription, err := s.subscriptionRepo.GetByID(ctx, uuidID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Subscription not found: %s", id)
		} else {
			s.log.Error("Error fetching subscription: %v", err)
		}
		return domain.Subscription{}, domain.SubscriptionPlan{}, err
	}

	// Получаем план подписки
	plan, err := s.subscriptionRepo.GetPlanByID(ctx, subscription.PlanID)
	if err != nil {
		s.log.Error("Failed to get subscription plan: %v", err)
		return domain.Subscription{}, domain.SubscriptionPlan{}, err
	}

	return subscription, plan, nil
}

// GetCustomerSubscriptionsWithPlans возвращает подписки клиента с информацией о планах
func (s *subscriptionServiceExtension) GetCustomerSubscriptionsWithPlans(ctx context.Context, customerID string) ([]domain.Subscription, []domain.SubscriptionPlan, error) {
	s.log.Debug("Getting subscriptions with plans for customer: %s", customerID)

	// Преобразуем строку в UUID
	uuidCustomerID, err := uuid.Parse(customerID)
	if err != nil {
		s.log.Warn("Invalid UUID format for customer ID: %s", customerID)
		return nil, nil, repository.ErrInvalidData
	}

	// Проверяем существование клиента
	_, err = s.customerRepo.GetByID(ctx, uuidCustomerID)
	if err != nil {
		if err == repository.ErrNotFound {
			s.log.Warn("Customer not found: %s", customerID)
		} else {
			s.log.Error("Error fetching customer: %v", err)
		}
		return nil, nil, err
	}

	// Получаем подписки клиента
	subscriptions, err := s.subscriptionRepo.GetByCustomerID(ctx, uuidCustomerID)
	if err != nil {
		s.log.Error("Failed to get subscriptions for customer: %v", err)
		return nil, nil, err
	}

	// Получаем планы подписок
	plans := make([]domain.SubscriptionPlan, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		plan, err := s.subscriptionRepo.GetPlanByID(ctx, subscription.PlanID)
		if err != nil {
			s.log.Error("Failed to get subscription plan: %v", err)
			return nil, nil, err
		}
		plans = append(plans, plan)
	}

	return subscriptions, plans, nil
}

// CalculateNextPaymentDate вычисляет дату следующего платежа для подписки
func (s *subscriptionServiceExtension) CalculateNextPaymentDate(ctx context.Context, id string) (*time.Time, error) {
	s.log.Debug("Calculating next payment date for subscription: %s", id)

	// Получаем подписку с планом
	subscription, plan, err := s.GetSubscriptionWithPlan(ctx, id)
	if err != nil {
		return nil, err
	}

	// Проверяем статус подписки
	if subscription.Status != domain.SubscriptionStatusActive &&
		subscription.Status != domain.SubscriptionStatusTrialing {
		s.log.Warn("Cannot calculate next payment date for inactive subscription: %s", id)
		return nil, domain.ErrInvalidOperation
	}

	// Если подписка отменена в конце периода, то платежа не будет
	if subscription.CancelAtPeriodEnd {
		return nil, nil
	}

	// Если подписка в пробном периоде, то следующий платеж будет в конце пробного периода
	if subscription.Status == domain.SubscriptionStatusTrialing && subscription.TrialEnd != nil {
		return subscription.TrialEnd, nil
	}

	// Иначе возвращаем дату окончания текущего периода
	nextPaymentDate := subscription.CurrentPeriodEnd
	return &nextPaymentDate, nil
}
