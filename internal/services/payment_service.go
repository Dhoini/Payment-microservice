package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/config"
	"github.com/Dhoini/Payment-microservice/internal/kafka"
	"github.com/Dhoini/Payment-microservice/internal/models"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/internal/stripe"
	"github.com/Dhoini/Payment-microservice/pkg/logger" // <-- Используем ваш логгер
	// "github.com/google/uuid"
)

var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrUserNotFound         = errors.New("user not found")
	ErrPaymentFailed        = errors.New("payment failed")
	ErrInternalServer       = errors.New("internal server error")
)

// PaymentService обрабатывает бизнес-логику, связанную с платежами и подписками.
type PaymentService struct {
	cfg           *config.Config
	subRepo       repository.SubscriptionRepository
	stripeClient  stripe.Client
	kafkaProducer kafka.Producer
	log           *logger.Logger // <-- Используем ваш логгер
}

// NewPaymentService создает новый экземпляр PaymentService.
func NewPaymentService(
	cfg *config.Config,
	subRepo repository.SubscriptionRepository,
	stripeClient stripe.Client,
	kafkaProducer kafka.Producer,
	log *logger.Logger, // <-- Используем ваш логгер
) *PaymentService {
	return &PaymentService{
		cfg:           cfg,
		subRepo:       subRepo,
		stripeClient:  stripeClient,
		kafkaProducer: kafkaProducer,
		log:           log,
	}
}

// CreateSubscriptionInput ... (входные данные для создания подписки)
type CreateSubscriptionInput struct {
	UserID         string
	PlanID         string
	UserEmail      string
	IdempotencyKey string
}

// CreateSubscriptionOutput ... (выходные данные после создания подписки)
type CreateSubscriptionOutput struct {
	Subscription *models.Subscription
	ClientSecret string
}

// CreateSubscription создает новую подписку для пользователя.
func (s *PaymentService) CreateSubscription(ctx context.Context, input CreateSubscriptionInput) (*CreateSubscriptionOutput, error) {
	s.log.Infow("Attempting to create subscription", "userID", input.UserID, "planID", input.PlanID)

	// --- Получение или создание клиента Stripe ---
	if input.UserEmail == "" {
		s.log.Errorw("User email is required to create Stripe customer")
		input.UserEmail = fmt.Sprintf("%s@example.com", input.UserID) // !!! ЗАГЛУШКА !!!
		s.log.Warnw("Using placeholder email for Stripe customer creation", "userID", input.UserID, "email", input.UserEmail)
	}
	stripeCustomerID, err := s.stripeClient.GetOrCreateCustomer(ctx, input.UserID, input.UserEmail)
	if err != nil {
		s.log.Errorw("Failed to get or create Stripe customer", "error", err)
		return nil, fmt.Errorf("stripe customer error: %w", err)
	}
	s.log.Infow("Got Stripe customer ID", "stripeCustomerID", stripeCustomerID)

	// --- Создание подписки в Stripe ---
	stripeSubID, clientSecret, err := s.stripeClient.CreateSubscription(ctx, stripeCustomerID, input.PlanID, input.IdempotencyKey)
	if err != nil {
		s.log.Errorw("Failed to create Stripe subscription", "error", err)
		// TODO: Более детальная обработка ошибок Stripe (карта отклонена и т.д.) -> вернуть ErrPaymentFailed?
		return nil, fmt.Errorf("stripe subscription creation failed: %w", err)
	}
	s.log.Infow("Stripe subscription created", "stripeSubscriptionID", stripeSubID)

	// --- Сохранение подписки в БД ---
	now := time.Now()
	newSub := &models.Subscription{
		SubscriptionID:   stripeSubID, // Используем Stripe ID как основной ID
		UserID:           input.UserID,
		PlanID:           input.PlanID,
		StripeCustomerID: stripeCustomerID,
		Status:           "pending", // Начальный статус, изменится после вебхука
		CreatedAt:        now,
		UpdatedAt:        now,
		// ExpiresAt и CanceledAt остаются nil
	}
	if err := s.subRepo.Create(ctx, newSub); err != nil {
		s.log.Errorw("Failed to save subscription to DB", "error", err)
		// TODO: Попытка отката создания подписки в Stripe? Или фоновая задача реконсиляции.
		return nil, fmt.Errorf("failed to save subscription locally: %w", err)
	}
	s.log.Infow("Subscription saved to DB", "subscriptionID", newSub.SubscriptionID)

	// --- Публикация события в Kafka (асинхронно) ---
	go func(subToPublish *models.Subscription) { // Передаем копию или указатель в горутину
		kafkaCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.kafkaProducer.PublishSubscriptionEvent(kafkaCtx, kafka.TopicSubscriptionCreated, subToPublish); err != nil {
			s.log.Errorw("Failed to publish subscription created event to Kafka", "error", err, "subscriptionID", subToPublish.SubscriptionID)
		} else {
			s.log.Infow("Subscription created event published to Kafka", "subscriptionID", subToPublish.SubscriptionID)
		}
	}(newSub) // Передаем созданную подписку

	// --- Формирование ответа ---
	output := &CreateSubscriptionOutput{
		Subscription: newSub,
		ClientSecret: clientSecret,
	}
	s.log.Infow("Subscription created successfully")
	return output, nil
}

// CancelSubscription отменяет подписку пользователя.
func (s *PaymentService) CancelSubscription(ctx context.Context, userID, subscriptionID, idempotencyKey string) error {
	// IdempotencyKey здесь может быть не так важен, т.к. Stripe API отмены обычно идемпотентен сам
	s.log.Infow("Attempting to cancel subscription", "userID", userID, "subscriptionID", subscriptionID)

	// --- Поиск подписки в БД ---
	sub, err := s.subRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.log.Warnw("Subscription not found in DB for cancellation", "subscriptionID", subscriptionID)
			return ErrSubscriptionNotFound
		}
		s.log.Errorw("Failed to get subscription from DB for cancellation", "error", err)
		return ErrInternalServer
	}

	// --- Проверка владельца и статуса ---
	if sub.UserID != userID {
		s.log.Warnw("User does not own this subscription", "subscriptionID", subscriptionID, "ownerUserID", sub.UserID, "requesterUserID", userID)
		return ErrSubscriptionNotFound // Скрываем детали из соображений безопасности
	}
	if sub.Status == "canceled" {
		s.log.Infow("Subscription already canceled", "subscriptionID", subscriptionID)
		return nil // Идемпотентность
	}

	// --- Отмена подписки в Stripe ---
	err = s.stripeClient.CancelSubscription(ctx, sub.SubscriptionID) // Используем ID подписки
	if err != nil {
		// Ошибка уже логируется внутри stripeClient при вызове logStripeError
		// Но можно добавить контекст здесь, если нужно
		s.log.Errorw("Stripe client failed to cancel subscription", "error", err, "subscriptionID", subscriptionID)
		// Если stripeClient вернул nil при ошибке "уже отменено", то здесь будет nil
		// Если он вернул ошибку, то возвращаем ее дальше
		return fmt.Errorf("stripe subscription cancellation failed: %w", err)
	}
	s.log.Infow("Stripe subscription successfully canceled", "subscriptionID", subscriptionID)

	// --- Обновление статуса в БД ---
	now := time.Now()
	sub.Status = "canceled"
	sub.CanceledAt = &now
	sub.UpdatedAt = now
	if err := s.subRepo.Update(ctx, sub); err != nil {
		s.log.Errorw("Failed to update subscription status to canceled in DB", "error", err, "subscriptionID", subscriptionID)
		// TODO: Критическая ситуация - Stripe отменена, БД нет. Нужна реконсиляция.
		return ErrInternalServer
	}
	s.log.Infow("Subscription status updated to canceled in DB", "subscriptionID", subscriptionID)

	// --- Публикация события в Kafka (асинхронно) ---
	go func(subToPublish *models.Subscription) {
		kafkaCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.kafkaProducer.PublishSubscriptionEvent(kafkaCtx, kafka.TopicSubscriptionCancelled, subToPublish); err != nil {
			s.log.Errorw("Failed to publish subscription cancelled event to Kafka", "error", err, "subscriptionID", subToPublish.SubscriptionID)
		} else {
			s.log.Infow("Subscription cancelled event published to Kafka", "subscriptionID", subToPublish.SubscriptionID)
		}
	}(sub) // Передаем обновленную подписку

	s.log.Infow("Subscription canceled successfully", "subscriptionID", subscriptionID)
	return nil
}

// GetSubscriptionByID получает детали подписки по ID.
func (s *PaymentService) GetSubscriptionByID(ctx context.Context, userID, subscriptionID string) (*models.Subscription, error) {
	s.log.Debugw("Getting subscription by ID", "userID", userID, "subscriptionID", subscriptionID)

	// --- Получение из репозитория ---
	sub, err := s.subRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.log.Warnw("Subscription not found by ID", "subscriptionID", subscriptionID)
			return nil, ErrSubscriptionNotFound
		}
		s.log.Errorw("Failed to get subscription by ID from DB", "error", err, "subscriptionID", subscriptionID)
		return nil, ErrInternalServer
	}

	// --- Проверка владельца ---
	if sub.UserID != userID {
		s.log.Warnw("User attempted to access another user's subscription", "subscriptionID", subscriptionID, "ownerUserID", sub.UserID, "requesterUserID", userID)
		return nil, ErrSubscriptionNotFound // Скрываем факт существования
	}

	s.log.Debugw("Subscription retrieved successfully by ID", "subscriptionID", subscriptionID)
	return sub, nil
}

// GetSubscriptionsByUserID получает все подписки пользователя.
func (s *PaymentService) GetSubscriptionsByUserID(ctx context.Context, userID string) ([]models.Subscription, error) {
	s.log.Debugw("Getting subscriptions by user ID", "userID", userID)

	// --- Получение из репозитория ---
	subs, err := s.subRepo.GetByUserID(ctx, userID)
	// Обрабатываем конец вставки из вашего предыдущего сообщения
	if err != nil {
		// Ошибку ErrNotFound от репозитория не считаем фатальной для списка,
		// просто возвращаем пустой слайс (это не ошибка с точки зрения API).
		if errors.Is(err, repository.ErrNotFound) {
			s.log.Debugw("No subscriptions found for user in DB", "userID", userID)
			return []models.Subscription{}, nil // Возвращаем пустой слайс, а не ошибку
		}
		// Другие ошибки БД логируем и возвращаем как внутреннюю ошибку сервера
		s.log.Errorw("Failed to get subscriptions by user ID from DB", "error", err, "userID", userID)
		return nil, ErrInternalServer
	}
	// КОНЕЦ ДОБАВЛЕННОГО КОДА ДЛЯ GetSubscriptionsByUserID

	s.log.Debugw("Subscriptions retrieved successfully by user ID", "userID", userID, "count", len(subs))
	return subs, nil
}

// HandleWebhookEvent обрабатывает событие от Stripe (вызывается из webhook_handler).
func (s *PaymentService) HandleWebhookEvent(ctx context.Context, eventType string, stripeSubscriptionID string, data map[string]interface{}) error {
	// Вместо With, будем добавлять в каждый вызов:
	s.log.Infow("Handling webhook event", "eventType", eventType, "stripeSubscriptionID", stripeSubscriptionID)

	// Если ID подписки не был извлечен хендлером, но он критичен для события
	if stripeSubscriptionID == "" && s.isSubscriptionEvent(eventType) {
		s.log.Errorw("Missing Stripe Subscription ID for a subscription-related webhook event", "eventType", eventType)
		// Можно вернуть ошибку, если ID обязателен, или попробовать найти его иначе.
		// Пока просто логируем и продолжаем (может, ID не нужен для этого eventType).
		// return errors.New("missing subscription ID for event")
	}

	// 1. Найти подписку по Stripe ID (если ID есть)
	var sub *models.Subscription
	var err error
	if stripeSubscriptionID != "" {
		sub, err = s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubscriptionID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				s.log.Warnw("Received webhook for unknown subscription ID", "eventType", eventType, "stripeSubscriptionID", stripeSubscriptionID)
				// Что делать? Зависит от события.
				// Если это invoice.paid для НОВОЙ подписки, которой еще нет в БД (теоретически возможно при рассинхроне),
				// может, нужно попытаться ее создать? Или просто игнорировать.
				// Пока игнорируем неизвестные подписки.
				return nil // Возвращаем nil, чтобы Stripe не повторял попытку
			}
			s.log.Errorw("Failed to get subscription by Stripe ID from DB", "error", err, "stripeSubscriptionID", stripeSubscriptionID)
			return ErrInternalServer // Возвращаем ошибку, чтобы Stripe ПОВТОРИЛ попытку (проблема с БД)
		}
		s.log.Infow("Found subscription in DB for webhook", "subscriptionID", sub.SubscriptionID, "currentStatus", sub.Status)
	} else {
		s.log.Warnw("Handling webhook event without a resolved Subscription ID", "eventType", eventType)
		// Обработка событий, не связанных напрямую с подпиской (например, customer.created?)
	}

	// 2. Обработать в зависимости от типа события
	needsUpdate := false // Флаг, что нужно обновить подписку в БД
	now := time.Now()

	switch eventType {
	// --- Успешные платежи ---
	case "invoice.paid":
		if sub == nil {
			s.log.Warnw("invoice.paid event received but no corresponding subscription found/resolved", "eventType", eventType)
			return nil // Игнорировать
		}
		s.log.Infow("Processing invoice.paid event", "subscriptionID", sub.SubscriptionID)
		// Обновляем статус на 'active', если он еще не такой
		if sub.Status != "active" {
			sub.Status = "active"
			// Можно извлечь период подписки из `data` (если нужно) и вычислить ExpiresAt
			// expiresAt := s.calculateExpiresAt(data)
			// sub.ExpiresAt = expiresAt
			needsUpdate = true
			s.log.Infow("Subscription status set to active", "subscriptionID", sub.SubscriptionID)
			// TODO: Опубликовать событие в Kafka? (subscription_activated)
		} else {
			s.log.Infow("Subscription already active, processing renewal payment", "subscriptionID", sub.SubscriptionID)
			// Можно обновить ExpiresAt при продлении
			// expiresAt := s.calculateExpiresAt(data)
			// if sub.ExpiresAt == nil || (expiresAt != nil && expiresAt.After(*sub.ExpiresAt)) {
			// 	sub.ExpiresAt = expiresAt
			//  needsUpdate = true
			// }
		}

	// --- Неуспешные платежи ---
	case "invoice.payment_failed":
		if sub == nil {
			s.log.Warnw("invoice.payment_failed event received but no corresponding subscription found/resolved", "eventType", eventType)
			return nil
		}
		s.log.Warnw("Processing invoice.payment_failed event", "subscriptionID", sub.SubscriptionID)
		// Обновляем статус на 'past_due' или 'payment_failed'
		// Зависит от настроек Stripe Dunning (автоматические попытки оплаты)
		if sub.Status != "canceled" { // Не меняем статус, если уже отменена
			sub.Status = "past_due" // Или другой статус, соответствующий неуспешной оплате
			needsUpdate = true
			s.log.Infow("Subscription status set to past_due", "subscriptionID", sub.SubscriptionID)
			// TODO: Опубликовать событие в Kafka? (subscription_payment_failed)
			// TODO: Отправить уведомление пользователю?
		}

	// --- Отмена / Удаление подписки ---
	case "customer.subscription.deleted": // Подписка удалена в Stripe (часто после отмены и окончания периода)
		if sub == nil {
			s.log.Warnw("customer.subscription.deleted event received but no corresponding subscription found/resolved", "eventType", eventType)
			return nil
		}
		s.log.Infow("Processing customer.subscription.deleted event", "subscriptionID", sub.SubscriptionID)
		// Обновляем статус на 'canceled', если еще не отменена
		if sub.Status != "canceled" {
			sub.Status = "canceled"
			if sub.CanceledAt == nil { // Если отмена произошла в Stripe, а не через наш API
				sub.CanceledAt = &now
			}
			needsUpdate = true
			s.log.Infow("Subscription status set to canceled", "subscriptionID", sub.SubscriptionID)
			// TODO: Опубликовать событие в Kafka? (subscription_truly_ended)
		}

	// --- Обновление подписки (например, смена плана, статуса извне) ---
	case "customer.subscription.updated":
		if sub == nil {
			s.log.Warnw("customer.subscription.updated event received but no corresponding subscription found/resolved", "eventType", eventType)
			return nil
		}
		s.log.Infow("Processing customer.subscription.updated event", "subscriptionID", sub.SubscriptionID)
		// Здесь нужно сравнить данные из `data` с текущими данными в `sub`
		// и обновить нужные поля (статус, план, expires_at и т.д.)
		// Пример: обновление статуса
		newStatus, _ := data["status"].(string)
		if newStatus != "" && newStatus != sub.Status {
			s.log.Infow("Subscription status updated via webhook", "subscriptionID", sub.SubscriptionID, "oldStatus", sub.Status, "newStatus", newStatus)
			sub.Status = newStatus
			if newStatus == "canceled" && sub.CanceledAt == nil {
				sub.CanceledAt = &now
			}
			needsUpdate = true
			// TODO: Обновить PlanID, если он изменился?
			// TODO: Опубликовать событие в Kafka? (subscription_updated)
		} else {
			s.log.Infow("No relevant changes detected in customer.subscription.updated event", "subscriptionID", sub.SubscriptionID)
		}

	// TODO: Добавить обработку других важных событий:
	// - checkout.session.completed (если используете Stripe Checkout для создания подписок)
	// - customer.subscription.trial_will_end (уведомление о скором окончании триала)
	// - ... другие события по необходимости ...

	default:
		s.log.Infow("Unhandled event type", "eventType", eventType)
	}

	// 3. Обновить подписку в БД, если были изменения
	if needsUpdate && sub != nil {
		sub.UpdatedAt = now
		if err := s.subRepo.Update(ctx, sub); err != nil {
			s.log.Errorw("Failed to update subscription status in DB after webhook processing", "error", err, "subscriptionID", sub.SubscriptionID)
			return ErrInternalServer // Возвращаем ошибку, чтобы Stripe ПОВТОРИЛ попытку
		}
		s.log.Infow("Subscription updated successfully in DB after webhook", "subscriptionID", sub.SubscriptionID, "newStatus", sub.Status)
	}

	return nil // Возвращаем nil, чтобы подтвердить успешную обработку Stripe
}

// isSubscriptionEvent - простая проверка, относится ли тип события к подписке
// (можно сделать более точной)
func (s *PaymentService) isSubscriptionEvent(eventType string) bool {
	return eventType == "customer.subscription.created" ||
		eventType == "customer.subscription.updated" ||
		eventType == "customer.subscription.deleted" ||
		eventType == "invoice.paid" || // Косвенно относится к подписке
		eventType == "invoice.payment_failed" // Косвенно относится к подписке
}

// calculateExpiresAt - примерная функция для вычисления времени окончания
// (требует разбора данных из события invoice.paid)
// func (s *PaymentService) calculateExpiresAt(data map[string]interface{}) *time.Time {
// 	lines, ok := data["lines"].(map[string]interface{})
// 	if !ok { return nil }
// 	lineData, ok := lines["data"].([]interface{})
// 	if !ok || len(lineData) == 0 { return nil }
// 	firstLine, ok := lineData[0].(map[string]interface{})
// 	if !ok { return nil }
// 	period, ok := firstLine["period"].(map[string]interface{})
// 	if !ok { return nil }
// 	end, ok := period["end"].(float64) // Время в Unix timestamp
// 	if !ok { return nil }
// 	expires := time.Unix(int64(end), 0)
// 	return &expires
// }
