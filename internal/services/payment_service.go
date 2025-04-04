package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http" // <-- Добавлен импорт для http.StatusTooManyRequests
	"time"

	"github.com/Dhoini/Payment-microservice/internal/config"
	"github.com/Dhoini/Payment-microservice/internal/kafka"
	"github.com/Dhoini/Payment-microservice/internal/models"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/internal/stripe"
	"github.com/Dhoini/Payment-microservice/pkg/logger"

	"github.com/cenkalti/backoff/v4"
	stripego "github.com/stripe/stripe-go/v78"
)

// Определяем строковые константы для типов ошибок, которые нам нужны
const (
	// Используем stripego.ErrorType как тип для ясности
	// stripego.ErrorType("...") - это просто приведение строки к типу stripego.ErrorType
	StripeErrorTypeRateLimit      stripego.ErrorType = "rate_limit_error" // Обычно Rate Limit лучше ловить по коду 429
	StripeErrorTypeAPIConnection  stripego.ErrorType = "api_connection_error"
	StripeErrorTypeAPI            stripego.ErrorType = "api_error" // Эта константа уже есть в stripego, но для единообразия можно определить свою
	StripeErrorTypeAuthentication stripego.ErrorType = "authentication_error"
	StripeErrorTypeCard           stripego.ErrorType = "card_error"
	StripeErrorTypeInvalidRequest stripego.ErrorType = "invalid_request_error"
	StripeErrorTypePermission     stripego.ErrorType = "permission_error"
	StripeErrorTypeIdempotency    stripego.ErrorType = "idempotency_error"
)

// --- Определения кастомных ошибок сервиса ---
var (
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrUserNotFound         = errors.New("user not found")            // Если сервис проверяет пользователя
	ErrPaymentFailed        = errors.New("payment processing failed") // Общая ошибка платежа
	ErrStripeClient         = errors.New("stripe client error")       // Ошибка взаимодействия со Stripe
	ErrInternalServer       = errors.New("internal server error")     // Общая внутренняя ошибка
	ErrInvalidInput         = errors.New("invalid input data")        // Ошибка валидации входных данных
)

type CreateSubscriptionInput struct {
	UserID         string
	PlanID         string
	UserEmail      string
	IdempotencyKey string
}

type CreateSubscriptionOutput struct {
	Subscription *models.Subscription
	ClientSecret string
}

type PaymentService struct {
	cfg           *config.Config
	subRepo       repository.SubscriptionRepository
	customerRepo  repository.CustomerRepository
	stripeClient  stripe.Client
	kafkaProducer kafka.Producer // Может быть nil, если Kafka недоступен
	log           *logger.Logger
}

// NewPaymentService конструктор сервиса
func NewPaymentService(
	cfg *config.Config,
	subRepo repository.SubscriptionRepository,
	stripeClient stripe.Client,
	kafkaProducer kafka.Producer, // Принимаем интерфейс, может быть nil
	log *logger.Logger,
) *PaymentService {
	// Логируем, если Kafka продюсер не инициализирован
	if kafkaProducer == nil {
		log.Warnw("Kafka producer is nil, event publishing will be skipped.")
	}
	return &PaymentService{
		cfg:           cfg,
		subRepo:       subRepo,
		stripeClient:  stripeClient,
		kafkaProducer: kafkaProducer,
		log:           log,
	}
}

// CreateSubscription основной метод создания подписки (без retry логики здесь)
// Retry логика может быть добавлена выше (в хендлерах) или остаться в CreateSubscriptionWithRetry
func (s *PaymentService) CreateSubscription(ctx context.Context, input CreateSubscriptionInput) (*CreateSubscriptionOutput, error) {
	// Базовая валидация на уровне сервиса
	if input.UserID == "" || input.PlanID == "" || input.UserEmail == "" {
		s.log.Warnw("CreateSubscription called with invalid input. UserID: %s, PlanID: %s, Email: %s", input.UserID, input.PlanID, input.UserEmail)
		return nil, ErrInvalidInput // Возвращаем ошибку валидации
	}

	s.log.Infow("Starting CreateSubscription process. UserID: %s, PlanID: %s", input.UserID, input.PlanID)
	startTime := time.Now()

	// Получаем или создаем клиента Stripe
	// Используем errgroup для параллельного выполнения, если это имеет смысл (здесь нет)
	stripeCustomerID, err := s.stripeClient.GetOrCreateCustomer(ctx, input.UserID, input.UserEmail)
	if err != nil {
		// Оборачиваем ошибку Stripe для консистентности
		s.log.Errorw("Failed to get or create Stripe customer. UserID: %s, Error: %v", input.UserID, err)
		return nil, fmt.Errorf("%w: failed to process customer: %v", ErrStripeClient, err)
	}
	s.log.Debugw("Stripe customer processed. UserID: %s, StripeCustomerID: %s", input.UserID, stripeCustomerID)

	// Создаем подписку в Stripe
	stripeSubID, clientSecret, err := s.stripeClient.CreateSubscription(ctx, stripeCustomerID, input.PlanID, input.IdempotencyKey)
	if err != nil {
		// Логируем детали ошибки Stripe
		s.trackStripeError(err, input)
		// Оборачиваем ошибку
		// Проверяем специфичные ошибки, которые могут быть важны для клиента
		var stripeErr *stripego.Error
		if errors.As(err, &stripeErr) {
			if stripeErr.Type == StripeErrorTypeCard || stripeErr.Type == StripeErrorTypeInvalidRequest {
				// Ошибки карты или неверного запроса часто означают проблемы с данными клиента
				return nil, fmt.Errorf("%w: %s", ErrPaymentFailed, stripeErr.Msg)
			}
		}
		// Общая ошибка Stripe
		return nil, fmt.Errorf("%w: failed to create subscription: %v", ErrStripeClient, err)
	}

	duration := time.Since(startTime)
	s.log.Infow("Stripe subscription created successfully. UserID: %s, PlanID: %s, StripeSubID: %s, DurationMs: %d",
		input.UserID, input.PlanID, stripeSubID, duration.Milliseconds())

	// Создаем модель подписки для сохранения и отправки события
	// ВАЖНО: Статус из Stripe может быть не 'active' сразу (например, 'incomplete')
	// Нужно получить актуальный статус из Stripe или обрабатывать вебхуки
	// Пока оставляем 'incomplete' как более безопасный дефолт после создания с default_incomplete
	// или получаем статус из Stripe ответа, если он там есть.
	// stripe.Client.CreateSubscription должен возвращать статус.
	// Предположим, что stripe.Client.CreateSubscription возвращает и статус:
	// stripeSubID, clientSecret, stripeStatus, err := s.stripeClient.CreateSubscription(...)
	subscriptionStatus := "incomplete" // Безопасное значение по умолчанию
	// TODO: Получить реальный статус из ответа Stripe, если stripeClient его возвращает

	subscription := &models.Subscription{
		SubscriptionID:   stripeSubID, // Используем ID из Stripe
		UserID:           input.UserID,
		PlanID:           input.PlanID,
		Status:           subscriptionStatus, // Используем статус из Stripe
		StripeCustomerID: stripeCustomerID,
		// CreatedAt и UpdatedAt будут установлены репозиторием при сохранении
	}

	// Опционально: Синхронное сохранение в БД (если нужно)
	// err = s.subRepo.Create(ctx, subscription)
	// if err != nil {
	//     s.log.Errorw("Failed to save subscription to local DB synchronously. UserID: %s, StripeSubID: %s, Error: %v", input.UserID, stripeSubID, err)
	//     // Решить, является ли это фатальной ошибкой для потока
	//     // Можно попытаться откатить Stripe (сложно) или просто вернуть ошибку
	//     return nil, fmt.Errorf("%w: failed to save subscription locally: %v", ErrInternalServer, err)
	// }
	// s.log.Infow("Subscription saved to local DB synchronously. UserID: %s, StripeSubID: %s", input.UserID, stripeSubID)

	// Асинхронная отправка события в Kafka (если продюсер доступен)
	if s.kafkaProducer != nil {
		// Запускаем в горутине, чтобы не блокировать ответ
		go s.publishSubscriptionEvent(context.WithoutCancel(ctx), subscription) // Используем новый контекст для горутины
	}

	return &CreateSubscriptionOutput{
		Subscription: subscription, // Возвращаем модель с ID и статусом
		ClientSecret: clientSecret,
	}, nil
}

// CreateSubscriptionWithRetry - обертка с логикой повторных попыток (можно оставить)
func (s *PaymentService) CreateSubscriptionWithRetry(ctx context.Context, input CreateSubscriptionInput) (*CreateSubscriptionOutput, error) {
	var output *CreateSubscriptionOutput
	var lastErr error // Сохраняем последнюю ошибку для логирования

	operation := func() error {
		var err error
		// Вызываем основной метод без retry внутри него
		output, err = s.CreateSubscription(ctx, input)
		lastErr = err // Сохраняем ошибку
		if err != nil {
			if isRetryableStripeError(err) {
				s.log.Warnw("Retryable Stripe error occurred, retrying... UserID: %s, Error: %v", input.UserID, err)
				return err // Возвращаем ошибку, чтобы backoff сработал
			}
			// Ошибка неretryable, прекращаем попытки
			s.log.Warnw("Non-retryable error occurred, stopping retries. UserID: %s, Error: %v", input.UserID, err)
			return backoff.Permanent(err)
		}
		// Успех
		return nil
	}

	// Настройка backoff
	bo := backoff.NewExponentialBackOff()
	bo.MaxInterval = 15 * time.Second   // Максимальный интервал между попытками
	bo.MaxElapsedTime = 1 * time.Minute // Максимальное общее время на попытки
	bo.Reset()                          // Сброс перед использованием

	// Запуск retry
	err := backoff.Retry(operation, bo)

	// Если после всех попыток осталась ошибка
	if err != nil {
		s.log.Errorw("Failed to create subscription after all retries. UserID: %s, LastError: %v",
			input.UserID,
			lastErr, // Логируем последнюю полученную ошибку
		)
		// Возвращаем последнюю ошибку, обернутую или как есть
		return nil, lastErr
	}

	// Успех после ретраев (или с первой попытки)
	return output, nil
}

// publishSubscriptionEvent отправляет событие в Kafka
func (s *PaymentService) publishSubscriptionEvent(ctx context.Context, subscription *models.Subscription) {
	// Проверяем, инициализирован ли продюсер
	if s.kafkaProducer == nil {
		s.log.Warnw("Kafka producer not available, skipping event publishing for SubscriptionID: %s", subscription.SubscriptionID)
		return
	}

	// Используем контекст с таймаутом для операции Kafka
	kafkaCtx, cancel := context.WithTimeout(ctx, 10*time.Second) // Увеличил таймаут
	defer cancel()

	err := s.kafkaProducer.PublishSubscriptionEvent(kafkaCtx, kafka.TopicSubscriptionCreated, subscription)
	if err != nil {
		// Логируем ошибку, но не прерываем основной поток
		s.log.Errorw("Failed to publish subscription created event. SubscriptionID: %s, Error: %v",
			subscription.SubscriptionID, err)
		// TODO: Рассмотреть механизм retry или отправки в dead-letter queue для Kafka
	} else {
		s.log.Infow("Subscription created event published successfully. SubscriptionID: %s", subscription.SubscriptionID)
	}
}

// trackStripeError логирует детали ошибки Stripe
func (s *PaymentService) trackStripeError(err error, input CreateSubscriptionInput) {
	var stripeErr *stripego.Error
	if errors.As(err, &stripeErr) {
		// Используем строковые константы для типов
		errorType := stripeErr.Type
		logLevel := s.log.Warnw // По умолчанию - Warning

		// Ошибки API, Connection, Authentication, RateLimit - обычно более серьезные
		if errorType == StripeErrorTypeAPI ||
			errorType == StripeErrorTypeAPIConnection ||
			errorType == StripeErrorTypeAuthentication ||
			(stripeErr.HTTPStatusCode == http.StatusTooManyRequests) { // Явная проверка Rate Limit по коду
			logLevel = s.log.Errorw
		}

		logLevel("Stripe API error occurred during subscription creation. UserID: %s, PlanID: %s, Type: %s, Code: %s, Param: %s, Msg: %s, RequestID: %s, StatusCode: %d",
			input.UserID,
			input.PlanID,
			string(errorType),      // Приводим тип к строке
			string(stripeErr.Code), // Приводим код к строке
			stripeErr.Param,
			stripeErr.Msg,
			stripeErr.RequestID,
			stripeErr.HTTPStatusCode,
		)
	} else {
		// Логируем не-Stripe ошибку, если она произошла во время операции Stripe
		s.log.Errorw("Non-Stripe error during Stripe operation. UserID: %s, PlanID: %s, Error: %v", input.UserID, input.PlanID, err)
	}
}

// isRetryableStripeError проверяет, является ли ошибка Stripe подходящей для повторной попытки
func isRetryableStripeError(err error) bool {
	var stripeErr *stripego.Error
	if errors.As(err, &stripeErr) {
		// Rate Limit (часто лучше ловить по коду 429)
		if stripeErr.HTTPStatusCode == http.StatusTooManyRequests {
			return true
		}
		// Ошибки соединения API
		if stripeErr.Type == StripeErrorTypeAPIConnection {
			return true
		}
		// Некоторые ошибки API сервера Stripe (5xx) могут быть временными
		if stripeErr.HTTPStatusCode >= 500 && stripeErr.HTTPStatusCode != http.StatusNotImplemented { // 501 обычно не retryable
			return true
		}
	}
	// Ошибка блокировки (Idempotency error) - не retryable с тем же ключом
	// if errors.Is(err, &stripe.IdempotencyError{}) { // Пример проверки специфичной ошибки библиотеки
	//     return false
	// }

	return false
}

// --- Методы для Get/Cancel ---

// GetSubscriptionByID получает подписку по ID, проверяя принадлежность пользователю
func (s *PaymentService) GetSubscriptionByID(ctx context.Context, userID, subscriptionID string) (*models.Subscription, error) {
	s.log.Infow("Fetching subscription by ID. UserID: %s, SubscriptionID: %s", userID, subscriptionID)
	sub, err := s.subRepo.GetByID(ctx, subscriptionID) // Получаем из репозитория
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) { // Используем ошибку из репозитория
			s.log.Warnw("Subscription not found in repository. SubscriptionID: %s", subscriptionID)
			return nil, ErrSubscriptionNotFound
		}
		s.log.Errorw("Failed to get subscription from repository. SubscriptionID: %s, Error: %v", subscriptionID, err)
		return nil, fmt.Errorf("%w: %v", ErrInternalServer, err) // Оборачиваем внутреннюю ошибку
	}

	// Проверка принадлежности подписки пользователю
	if sub.UserID != userID {
		s.log.Warnw("User attempted to access subscription belonging to another user. RequesterID: %s, OwnerID: %s, SubscriptionID: %s",
			userID, sub.UserID, subscriptionID)
		// Важно не раскрывать информацию о существовании подписки, возвращаем NotFound
		return nil, ErrSubscriptionNotFound
	}

	s.log.Infow("Subscription retrieved successfully. UserID: %s, SubscriptionID: %s", userID, subscriptionID)
	return sub, nil
}

// GetSubscriptionsByUserID получает все подписки пользователя
func (s *PaymentService) GetSubscriptionsByUserID(ctx context.Context, userID string) ([]models.Subscription, error) {
	s.log.Infow("Fetching subscriptions for UserID: %s", userID)
	subs, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		// Ошибка репозитория (кроме NotFound, т.к. пустой список - не ошибка)
		s.log.Errorw("Failed to get subscriptions from repository for UserID: %s. Error: %v", userID, err)
		return nil, fmt.Errorf("%w: %v", ErrInternalServer, err)
	}
	s.log.Infow("Subscriptions retrieved for UserID: %s. Count: %d", userID, len(subs))
	return subs, nil
}

// CancelSubscription отменяет подписку
func (s *PaymentService) CancelSubscription(ctx context.Context, userID, subscriptionID, idempotencyKey string) error {
	s.log.Infow("Attempting to cancel subscription. UserID: %s, SubscriptionID: %s", userID, subscriptionID)

	// 1. Получить подписку из нашей БД, чтобы проверить владельца
	sub, err := s.subRepo.GetByID(ctx, subscriptionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.log.Warnw("Subscription to cancel not found in repository. SubscriptionID: %s", subscriptionID)
			return ErrSubscriptionNotFound
		}
		s.log.Errorw("Failed to get subscription before cancellation. SubscriptionID: %s, Error: %v", subscriptionID, err)
		return fmt.Errorf("%w: failed to verify subscription owner: %v", ErrInternalServer, err)
	}

	// 2. Проверить владельца
	if sub.UserID != userID {
		s.log.Warnw("User attempted to cancel subscription belonging to another user. RequesterID: %s, OwnerID: %s, SubscriptionID: %s",
			userID, sub.UserID, subscriptionID)
		return ErrSubscriptionNotFound // Возвращаем NotFound из соображений безопасности
	}

	// 3. Проверить статус (можно ли отменить?)
	if sub.Status == "canceled" {
		s.log.Warnw("Attempted to cancel an already canceled subscription. UserID: %s, SubscriptionID: %s", userID, subscriptionID)
		return nil // Считаем операцию успешной, если уже отменена
	}

	// 4. Отменить подписку в Stripe
	// TODO: Добавить передачу idempotencyKey в Stripe клиент, если API Stripe поддерживает это для отмены
	err = s.stripeClient.CancelSubscription(ctx, subscriptionID)
	if err != nil {
		// Логируем ошибку Stripe
		s.trackStripeError(err, CreateSubscriptionInput{UserID: userID, PlanID: sub.PlanID}) // Передаем данные для логирования
		s.log.Errorw("Stripe failed to cancel subscription. UserID: %s, SubscriptionID: %s, Error: %v", userID, subscriptionID, err)
		return fmt.Errorf("%w: failed to cancel stripe subscription: %v", ErrStripeClient, err)
	}
	s.log.Infow("Subscription successfully canceled in Stripe. UserID: %s, SubscriptionID: %s", userID, subscriptionID)

	// 5. Обновить статус в локальной БД (или дождаться вебхука)
	// Если полагаемся на вебхуки, этот шаг не нужен.
	// Если обновляем здесь, нужно быть готовым к рассогласованию, если вебхук придет позже/раньше.
	// Пример синхронного обновления:
	// now := time.Now()
	// sub.Status = "canceled"
	// sub.CanceledAt = &now
	// sub.UpdatedAt = now
	// err = s.subRepo.Update(ctx, sub)
	// if err != nil {
	//     s.log.Errorw("Failed to update local subscription status after cancellation. UserID: %s, SubscriptionID: %s, Error: %v", userID, subscriptionID, err)
	//     // Ошибка некритична для пользователя, но требует мониторинга
	// } else {
	//     s.log.Infow("Local subscription status updated to 'canceled'. UserID: %s, SubscriptionID: %s", userID, subscriptionID)
	// }

	// 6. Отправить событие об отмене в Kafka (если нужно)
	if s.kafkaProducer != nil {
		// Создаем модель для события (может отличаться от основной)
		canceledEventSub := *sub // Копируем
		now := time.Now()
		canceledEventSub.Status = "canceled"
		canceledEventSub.CanceledAt = &now                                           // Устанавливаем время для события
		go s.publishSubscriptionEvent(context.WithoutCancel(ctx), &canceledEventSub) // Используем копию
	}

	return nil
}

// HandleWebhookEvent обрабатывает события из вебхуков Stripe
func (s *PaymentService) HandleWebhookEvent(ctx context.Context, eventType stripego.EventType, eventSubscriptionID string, data map[string]interface{}) error {
	// eventSubscriptionID из хендлера может быть неточным для invoice.* событий,
	// лучше извлекать ID из самого объекта `data`.

	s.log.Infow("Handling webhook event. Type: %s", eventType)

	switch eventType {

	case "customer.subscription.created":
		// Часто это событие приходит ПОСЛЕ вашего CreateSubscription.
		// Может использоваться для дополнительной синхронизации или если подписки создаются только через Stripe UI.
		subID := getStringValue(data, "id")
		status := getStringValue(data, "status")
		s.log.Infow("Webhook 'customer.subscription.created' received. StripeSubID: %s, Status: %s", subID, status)
		// Можно найти подписку по ID и обновить статус, если он отличается от того, что записали при создании.
		// Либо просто игнорировать, если создание идет через API сервиса.
		_, err := s.findAndUpdateSubscriptionStatus(ctx, subID, status, data) // Пример вызова хелпера
		if err != nil && !errors.Is(err, ErrSubscriptionNotFound) {           // Игнорируем NotFound, если подписку еще не успели создать локально
			return fmt.Errorf("failed processing subscription.created: %w", err)
		}

	case "customer.subscription.updated":
		subID := getStringValue(data, "id")
		status := getStringValue(data, "status")
		s.log.Infow("Webhook 'customer.subscription.updated' received. StripeSubID: %s, Status: %s", subID, status)
		if subID == "" {
			s.log.Errorw("StripeSubscriptionID missing in customer.subscription.updated event data")
			return nil // Не можем обработать без ID
		}

		_, err := s.findAndUpdateSubscriptionStatus(ctx, subID, status, data)
		if err != nil {
			// Если подписка не найдена, это может быть проблемой
			if errors.Is(err, ErrSubscriptionNotFound) {
				s.log.Errorw("Received update for non-existent local subscription. StripeSubID: %s", subID)
				return nil // Не повторять попытку для несуществующей подписки
			}
			return fmt.Errorf("failed processing subscription.updated: %w", err)
		}

	case "customer.subscription.deleted":
		subID := getStringValue(data, "id")
		status := getStringValue(data, "status") // Обычно 'canceled'
		s.log.Infow("Webhook 'customer.subscription.deleted' (canceled) received. StripeSubID: %s, Status: %s", subID, status)
		if subID == "" {
			s.log.Errorw("StripeSubscriptionID missing in customer.subscription.deleted event data")
			return nil
		}

		sub, err := s.findAndUpdateSubscriptionStatus(ctx, subID, "canceled", data) // Принудительно ставим 'canceled'
		if err != nil {
			if errors.Is(err, ErrSubscriptionNotFound) {
				s.log.Errorw("Received deletion for non-existent local subscription. StripeSubID: %s", subID)
				return nil
			}
			return fmt.Errorf("failed processing subscription.deleted: %w", err)
		}

		// Отправка события об отмене, если еще не отправляли из CancelSubscription
		if s.kafkaProducer != nil && sub != nil {
			// Создаем копию для события
			eventSub := *sub
			eventSub.Status = "canceled"    // Убедимся, что статус верный
			if eventSub.CanceledAt == nil { // Установим время, если его нет
				now := time.Now()
				eventSub.CanceledAt = &now
			}
			go s.publishSubscriptionEvent(context.WithoutCancel(ctx), &eventSub)
		}

	case "customer.subscription.trial_will_end":
		subID := getStringValue(data, "id")
		userID := "" // Попробуем получить UserID
		if sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, subID); err == nil {
			userID = sub.UserID
		}
		trialEndDate := getTimeValueFromUnix(data, "trial_end")
		s.log.Infow("Webhook 'customer.subscription.trial_will_end' received. StripeSubID: %s, UserID: %s, TrialEnd: %s", subID, userID, trialEndDate)

		// TODO: Отправить уведомление пользователю
		//if s.notificationSvc != nil && userID != "" {
		//    go s.notificationSvc.SendTrialEndingNotification(userID, subID, trialEndDate)
		//}

	case "invoice.payment_succeeded":
		invoiceID := getStringValue(data, "id")
		subID := getStringValue(data, "subscription")
		customerID := getStringValue(data, "customer") // Stripe Customer ID
		periodEnd := getTimeValueFromUnix(data, "period_end")

		s.log.Infow("Webhook 'invoice.payment_succeeded' received. InvoiceID: %s, StripeSubID: %s, CustomerID: %s", invoiceID, subID, customerID)

		if subID == "" {
			s.log.Infow("Invoice %s is not related to a subscription, skipping.", invoiceID)
			return nil // Не ошибка, просто инвойс не для подписки
		}

		sub, err := s.findAndUpdateSubscriptionStatus(ctx, subID, "active", data) // Оплата прошла -> статус должен быть active
		if err != nil {
			if errors.Is(err, ErrSubscriptionNotFound) {
				s.log.Errorw("Received successful payment for non-existent local subscription. StripeSubID: %s", subID)
				return nil
			}
			return fmt.Errorf("failed processing invoice.payment_succeeded for sub %s: %w", subID, err)
		}
		// Дополнительно можно обновить локальный expires_at, если он используется
		if sub != nil && !periodEnd.IsZero() {
			updated := false
			if sub.ExpiresAt == nil || sub.ExpiresAt.Before(periodEnd) {
				sub.ExpiresAt = &periodEnd
				updated = true
			}
			if updated {
				if err := s.subRepo.Update(ctx, sub); err != nil {
					s.log.Errorw("Failed to update expires_at after successful payment. SubscriptionID: %s, Error: %v", sub.SubscriptionID, err)
					// Не фатально, но стоит залогировать
				} else {
					s.log.Infow("Subscription expires_at updated. SubscriptionID: %s, New ExpiresAt: %s", sub.SubscriptionID, periodEnd)
				}
			}
		}

	case "invoice.payment_failed":
		invoiceID := getStringValue(data, "id")
		subID := getStringValue(data, "subscription")
		customerID := getStringValue(data, "customer")
		attemptCount := getInt64Value(data, "attempt_count")

		s.log.Warnw("Webhook 'invoice.payment_failed' received. InvoiceID: %s, StripeSubID: %s, CustomerID: %s, Attempt: %d", invoiceID, subID, customerID, attemptCount)

		if subID == "" {
			s.log.Infow("Failed Invoice %s is not related to a subscription, skipping.", invoiceID)
			return nil
		}

		// Определяем статус на основе конфигурации Stripe (dunning)
		// Stripe может сам перевести подписку в 'past_due', 'unpaid', или 'canceled'
		// Безопаснее всего - обновить статус на основе поля 'status' из самой подписки, если оно есть в data,
		// или установить 'past_due' как индикатор проблемы.
		newStatus := "past_due" // Статус по умолчанию при ошибке оплаты

		_, err := s.findAndUpdateSubscriptionStatus(ctx, subID, newStatus, data) // Обновляем на 'past_due'
		if err != nil {
			if errors.Is(err, ErrSubscriptionNotFound) {
				s.log.Errorw("Received failed payment for non-existent local subscription. StripeSubID: %s", subID)
				return nil
			}
			return fmt.Errorf("failed processing invoice.payment_failed for sub %s: %w", subID, err)
		}

		// TODO: Отправить уведомление пользователю о проблеме с оплатой
		// if s.notificationSvc != nil && sub != nil {
		//     nextAttemptTime := time.Unix(nextPaymentAttemptUnix, 0)
		//     go s.notificationSvc.SendPaymentFailedNotification(sub.UserID, subID, nextAttemptTime)
		// }

	default:
		s.log.Infow("Unhandled webhook event type received: %s", eventType)
	}

	// Возвращаем nil, чтобы Stripe не повторял отправку успешно обработанного (или проигнорированного) события.
	// Если произошла временная ошибка (например, БД недоступна), можно вернуть ошибку,
	// чтобы Stripe попытался отправить вебхук позже.
	return nil
}

// --- Вспомогательные функции ---

// findAndUpdateSubscriptionStatus находит подписку по Stripe ID и обновляет ее статус и другие поля.
// newStatus - желаемый статус, который будет установлен.
// data - данные из объекта события Stripe (обычно объект subscription или invoice).
func (s *PaymentService) findAndUpdateSubscriptionStatus(ctx context.Context, stripeSubscriptionID, newStatus string, data map[string]interface{}) (*models.Subscription, error) {
	if stripeSubscriptionID == "" {
		return nil, fmt.Errorf("stripeSubscriptionID is empty")
	}

	// 1. Найти подписку в локальной БД
	sub, err := s.subRepo.GetByStripeSubscriptionID(ctx, stripeSubscriptionID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.log.Warnw("Subscription not found in local DB by StripeSubID: %s", stripeSubscriptionID)
			return nil, ErrSubscriptionNotFound // Возвращаем кастомную ошибку
		}
		s.log.Errorw("Failed to get subscription from repository by StripeSubID: %s. Error: %v", stripeSubscriptionID, err)
		return nil, fmt.Errorf("%w: repository error: %v", ErrInternalServer, err)
	}

	// 2. Подготовить обновления
	needsUpdate := false
	now := time.Now()

	// Обновляем статус, если он отличается или если пришел статус отмены
	if sub.Status != newStatus || newStatus == "canceled" {
		sub.Status = newStatus
		needsUpdate = true
		s.log.Infow("Updating subscription status. StripeSubID: %s, NewStatus: %s", stripeSubscriptionID, newStatus)
	}

	// Обновляем ID плана, если он изменился (из subscription.updated)
	// Путь к ID плана: plan -> id или items -> data -> [0] -> price -> id
	newPlanID := extractPlanIDFromWebhookData(data)
	if newPlanID != "" && sub.PlanID != newPlanID {
		sub.PlanID = newPlanID
		needsUpdate = true
		s.log.Infow("Updating subscription plan ID. StripeSubID: %s, NewPlanID: %s", stripeSubscriptionID, newPlanID)
	}

	// Обновляем время окончания текущего периода (из subscription.updated или invoice.paid)
	currentPeriodEnd := getTimeValueFromUnix(data, "current_period_end")
	if !currentPeriodEnd.IsZero() {
		// Обновляем ExpiresAt, если оно не установлено или раньше новой даты
		if sub.ExpiresAt == nil || sub.ExpiresAt.Before(currentPeriodEnd) {
			sub.ExpiresAt = &currentPeriodEnd
			needsUpdate = true
			s.log.Infow("Updating subscription expires_at. StripeSubID: %s, NewExpiresAt: %s", stripeSubscriptionID, currentPeriodEnd)
		}
	}

	// Обновляем время фактической отмены (из subscription.updated/deleted)
	canceledAt := getTimeValueFromUnix(data, "canceled_at")
	if !canceledAt.IsZero() && (sub.CanceledAt == nil || !sub.CanceledAt.Equal(canceledAt)) {
		sub.CanceledAt = &canceledAt
		sub.Status = "canceled" // Убедимся, что статус тоже "canceled"
		needsUpdate = true
		s.log.Infow("Updating subscription canceled_at. StripeSubID: %s, CanceledAt: %s", stripeSubscriptionID, canceledAt)
	}

	// Если были изменения, обновляем запись в БД
	if needsUpdate {
		sub.UpdatedAt = now // Устанавливаем время обновления
		err = s.subRepo.Update(ctx, sub)
		if err != nil {
			s.log.Errorw("Failed to update subscription in repository. StripeSubID: %s, Error: %v", stripeSubscriptionID, err)
			return sub, fmt.Errorf("%w: failed to save subscription update: %v", ErrInternalServer, err)
		}
		s.log.Infow("Subscription updated successfully in local DB. StripeSubID: %s", stripeSubscriptionID)
	} else {
		s.log.Infow("No updates needed for subscription in local DB. StripeSubID: %s", stripeSubscriptionID)
	}

	return sub, nil
}

// extractPlanIDFromWebhookData пытается извлечь Price ID из данных вебхука.
func extractPlanIDFromWebhookData(data map[string]interface{}) string {
	// Попробовать извлечь из 'plan.id' (старый формат)
	if plan, ok := data["plan"].(map[string]interface{}); ok {
		if planID, ok := plan["id"].(string); ok && planID != "" {
			return planID
		}
	}
	// Попробовать извлечь из 'items.data[0].price.id' (новый формат)
	if items, ok := data["items"].(map[string]interface{}); ok {
		if itemsData, ok := items["data"].([]interface{}); ok && len(itemsData) > 0 {
			if firstItem, ok := itemsData[0].(map[string]interface{}); ok {
				if price, ok := firstItem["price"].(map[string]interface{}); ok {
					if priceID, ok := price["id"].(string); ok && priceID != "" {
						return priceID
					}
				}
			}
		}
	}
	return "" // Не найден
}
func (s *PaymentService) CreateCustomer(ctx context.Context, userID, email string) (*models.Customer, error) {
	// Проверяем, существует ли уже customer
	if existing, err := s.customerRepo.GetByUserID(ctx, userID); err == nil {
		return existing, nil
	} else if err != repository.ErrCustomerNotFound {
		return nil, fmt.Errorf("failed to check existing customer: %w", err)
	}

	// Создаем customer в Stripe
	stripeCustomerID, err := s.stripeClient.CreateCustomer(ctx, userID, email)
	if err != nil {
		return nil, fmt.Errorf("failed to create stripe customer: %w", err)
	}

	// Создаем customer в локальной БД
	customer := models.NewCustomer(userID, stripeCustomerID, email)
	if err := s.customerRepo.Create(ctx, customer); err != nil {
		// Логируем ошибку, но не удаляем customer из Stripe,
		// так как он может быть использован позже
		s.log.Errorw("Failed to create customer in local DB",
			"error", err,
			"userID", userID,
			"stripeCustomerID", stripeCustomerID)
		return nil, fmt.Errorf("failed to create customer in local DB: %w", err)
	}

	return customer, nil
}

func (s *PaymentService) GetOrCreateCustomer(ctx context.Context, userID, email string) (*models.Customer, error) {
	// Пытаемся найти существующего customer
	customer, err := s.customerRepo.GetByUserID(ctx, userID)
	if err == nil {
		return customer, nil
	}
	if err != repository.ErrCustomerNotFound {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	// Если не нашли, создаем нового
	return s.CreateCustomer(ctx, userID, email)
}

func (s *PaymentService) UpdateCustomerEmail(ctx context.Context, userID, newEmail string) error {
	customer, err := s.customerRepo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get customer: %w", err)
	}

	// Обновляем email в локальной БД
	customer.Email = newEmail
	customer.UpdatedAt = time.Now()

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return fmt.Errorf("failed to update customer: %w", err)
	}

	return nil
}

// getStringValue безопасно извлекает строковое значение из map[string]interface{}.
func getStringValue(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

// getInt64Value безопасно извлекает int64 значение из map[string]interface{}.
// Stripe часто возвращает числа как float64, даже если они целые.
func getInt64Value(data map[string]interface{}, key string) int64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case json.Number: // Если используется json.Unmarshal с UseNumber()
			i, err := v.Int64()
			if err == nil {
				return i
			}
		}
	}
	return 0
}

// getFloat64Value безопасно извлекает float64 значение.
func getFloat64Value(data map[string]interface{}, key string) float64 {
	if val, ok := data[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case json.Number:
			f, err := v.Float64()
			if err == nil {
				return f
			}
		}
	}
	return 0.0
}

// getTimeValueFromUnix безопасно извлекает время из Unix timestamp (int64 или float64).
func getTimeValueFromUnix(data map[string]interface{}, key string) time.Time {
	unixTimestamp := getInt64Value(data, key)
	if unixTimestamp > 0 {
		return time.Unix(unixTimestamp, 0).UTC() // Возвращаем в UTC
	}
	return time.Time{} // Возвращаем нулевое время, если ключ не найден или 0
}
