package stripe

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dhoini/Payment-microservice/pkg/logger"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/client"
)

const (
	// Ключ метаданных для связи Stripe Customer с вашим UserID
	metadataUserIDKey = "user_id"
)

// Client определяет методы для взаимодействия со Stripe API.
type Client interface {
	// CreateCustomer создает нового клиента в Stripe и возвращает его Stripe ID.
	CreateCustomer(ctx context.Context, userID, email string) (string, error)

	// GetOrCreateCustomer ищет клиента по userID, если не находит - создает нового.
	GetOrCreateCustomer(ctx context.Context, userID, email string) (string, error)

	// CreateSubscription создает подписку в Stripe для клиента.
	// Возвращает Stripe Subscription ID и Client Secret для первого платежа (если нужен).
	CreateSubscription(ctx context.Context, stripeCustomerID, planID, idempotencyKey string) (stripeSubscriptionID, clientSecret string, err error)

	// CancelSubscription отменяет подписку в Stripe.
	CancelSubscription(ctx context.Context, stripeSubscriptionID string) error
}

// stripeClient реализует интерфейс Client.
type stripeClient struct {
	client *client.API    // Клиент Stripe SDK
	log    *logger.Logger // Используем ваш кастомный логгер
}

// NewStripeClient создает новый экземпляр клиента Stripe.
func NewStripeClient(apiKey string, log *logger.Logger) Client {
	sc := &client.API{}
	sc.Init(apiKey, nil) // Инициализируем клиент Stripe с API ключом
	return &stripeClient{
		client: sc,
		log:    log,
	}
}

// CreateCustomer создает нового клиента в Stripe.
func (sc *stripeClient) CreateCustomer(ctx context.Context, userID, email string) (string, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			metadataUserIDKey: userID,
		},
	}
	params.Context = ctx

	cus, err := sc.client.Customers.New(params)
	if err != nil {
		logStripeError(sc.log, "CreateCustomer", err)
		return "", fmt.Errorf("stripe: failed to create customer: %w", err)
	}

	sc.log.Infow("Stripe customer created", "stripeCustomerID", cus.ID, "userID", userID)
	return cus.ID, nil
}

// GetOrCreateCustomer ищет клиента по userID в метаданных, если не находит - создает нового.
func (sc *stripeClient) GetOrCreateCustomer(ctx context.Context, userID, email string) (string, error) {
	sc.log.Debugw("Searching for Stripe customer using Search API", "userID", userID)

	// 1. Ищем клиента по метаданным (user_id) через Search API
	searchQuery := fmt.Sprintf("metadata['%s']:'%s'", metadataUserIDKey, userID)
	searchParams := &stripe.CustomerSearchParams{
		SearchParams: stripe.SearchParams{
			Query:   searchQuery,
			Limit:   stripe.Int64(1),
			Context: ctx,
		},
	}

	// Выполняем поиск через sc.client.Search.Customers
	customers := sc.client.Customers.Search(searchParams)

	if customers.Next() {
		// Клиент найден
		customer := customers.Customer()
		sc.log.Infow("Found existing Stripe customer via Search", "stripeCustomerID", customer.ID, "userID", userID)
		return customer.ID, nil
	}

	// Проверяем ошибки итератора
	if err := customers.Err(); err != nil {
		logStripeError(sc.log, "SearchCustomers", err)
		var stripeErr *stripe.Error
		if errors.As(err, &stripeErr) {
			if stripeErr.Type == stripe.ErrorTypeInvalidRequest {
				return "", fmt.Errorf("stripe: failed to search customer (invalid search query?): %w", err)
			}
		} else {
			return "", fmt.Errorf("stripe: failed to search customer (unknown error): %w", err)
		}
		sc.log.Warnw("Non-fatal error during customer search, proceeding to create", "error", err)
	}

	// 2. Клиент не найден или произошла некритичная ошибка поиска - создаем нового
	sc.log.Infow("Stripe customer not found via Search, creating new one", "userID", userID)
	return sc.CreateCustomer(ctx, userID, email)
}

// CreateSubscription создает подписку в Stripe для указанного клиента и плана.
func (sc *stripeClient) CreateSubscription(ctx context.Context, stripeCustomerID, planID, idempotencyKey string) (string, string, error) {
	params := &stripe.SubscriptionParams{
		Customer: stripe.String(stripeCustomerID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(planID),
			},
		},
		PaymentBehavior: stripe.String("default_incomplete"), //SubscriptionPaymentBehaviorDefaultIncomplete
		Params: stripe.Params{
			IdempotencyKey: stripe.String(idempotencyKey),
			Context:        ctx,
		},
	}
	// Используем AddExpand для получения PaymentIntent
	params.AddExpand("latest_invoice.payment_intent")

	// Создаем подписку через sc.client.Subscriptions.New
	subscription, err := sc.client.Subscriptions.New(params)
	if err != nil {
		logStripeError(sc.log, "CreateSubscription", err)
		return "", "", fmt.Errorf("stripe: failed to create subscription: %w", err)
	}

	sc.log.Infow("Stripe subscription created", "stripeSubscriptionID", subscription.ID, "status", string(subscription.Status))

	// Извлекаем client_secret
	clientSecret := ""
	if subscription.LatestInvoice != nil && subscription.LatestInvoice.PaymentIntent != nil {
		clientSecret = subscription.LatestInvoice.PaymentIntent.ClientSecret
		sc.log.Debugw("Retrieved client secret from payment intent", "stripeSubscriptionID", subscription.ID, "paymentIntentID", subscription.LatestInvoice.PaymentIntent.ID)
	} else {
		sc.log.Warnw("No payment intent or client secret found in created subscription", "stripeSubscriptionID", subscription.ID, "status", string(subscription.Status))
	}

	return subscription.ID, clientSecret, nil
}

// CancelSubscription отменяет подписку в Stripe немедленно.
func (sc *stripeClient) CancelSubscription(ctx context.Context, stripeSubscriptionID string) error {
	params := &stripe.SubscriptionCancelParams{
		Params: stripe.Params{
			Context: ctx,
		},
	}

	// Отменяем подписку через sc.client.Subscriptions.Cancel
	_, err := sc.client.Subscriptions.Cancel(stripeSubscriptionID, params)
	if err != nil {
		// Обрабатываем случай, если подписка уже удалена
		stripeErr, ok := err.(*stripe.Error)
		if ok && stripeErr.Code == stripe.ErrorCodeResourceMissing {
			sc.log.Warnw("Attempted to cancel already canceled/missing Stripe subscription", "stripeSubscriptionID", stripeSubscriptionID)
			return nil
		}
		logStripeError(sc.log, "CancelSubscription", err)
		return fmt.Errorf("stripe: failed to cancel subscription: %w", err)
	}

	sc.log.Infow("Stripe subscription canceled", "stripeSubscriptionID", stripeSubscriptionID)
	return nil
}

// logStripeError - вспомогательная функция для логирования деталей ошибки Stripe.
func logStripeError(log *logger.Logger, operation string, err error) {
	var stripeErr *stripe.Error
	if errors.As(err, &stripeErr) {
		log.Errorw("Stripe API error",
			"operation", operation,
			"type", string(stripeErr.Type),
			"code", string(stripeErr.Code),
			"param", stripeErr.Param,
			"message", stripeErr.Msg,
			"request_id", stripeErr.RequestID,
			"status_code", stripeErr.HTTPStatusCode,
		)
	} else {
		log.Errorw("Non-Stripe error during Stripe operation",
			"operation", operation,
			"error", err,
		)
	}
}
