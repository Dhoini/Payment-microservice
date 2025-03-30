package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/google/uuid"
)

// SubscriptionResponse представляет ответ от API Stripe о подписке
type SubscriptionResponse struct {
	ID                   string                 `json:"id"`
	Object               string                 `json:"object"`
	Customer             string                 `json:"customer"`
	CurrentPeriodStart   int64                  `json:"current_period_start"`
	CurrentPeriodEnd     int64                  `json:"current_period_end"`
	Status               string                 `json:"status"`
	CancelAtPeriodEnd    bool                   `json:"cancel_at_period_end"`
	CanceledAt           *int64                 `json:"canceled_at"`
	DefaultPaymentMethod string                 `json:"default_payment_method"`
	Items                *SubscriptionItemsList `json:"items"`
	Metadata             map[string]string      `json:"metadata"`
	TrialStart           *int64                 `json:"trial_start"`
	TrialEnd             *int64                 `json:"trial_end"`
	Created              int64                  `json:"created"`
	Error                *ErrorResponse         `json:"error,omitempty"`
}

// SubscriptionItemsList представляет список элементов подписки
type SubscriptionItemsList struct {
	Object     string             `json:"object"`
	HasMore    bool               `json:"has_more"`
	TotalCount int                `json:"total_count"`
	URL        string             `json:"url"`
	Data       []SubscriptionItem `json:"data"`
}

// SubscriptionItem представляет элемент подписки
type SubscriptionItem struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Price    *Price `json:"price"`
	Quantity int    `json:"quantity"`
}

// Price представляет цену подписки
type Price struct {
	ID         string            `json:"id"`
	Object     string            `json:"object"`
	Active     bool              `json:"active"`
	Currency   string            `json:"currency"`
	UnitAmount int64             `json:"unit_amount"`
	Recurring  *PriceRecurring   `json:"recurring"`
	ProductID  string            `json:"product"`
	Metadata   map[string]string `json:"metadata"`
}

// PriceRecurring представляет периодичность цены
type PriceRecurring struct {
	Interval        string `json:"interval"`
	IntervalCount   int    `json:"interval_count"`
	TrialPeriodDays *int   `json:"trial_period_days"`
}

// CreateSubscription создает новую подписку в Stripe
func (c *Client) CreateSubscription(ctx context.Context, customerID string, priceID string, metadata map[string]string) (*SubscriptionResponse, error) {
	c.log.Debug("Creating Stripe subscription for customer: %s, price: %s", customerID, priceID)

	// Формируем данные для запроса
	formData := url.Values{}
	formData.Add("customer", customerID)
	formData.Add("items[0][price]", priceID)

	// Добавляем метаданные, если они есть
	if metadata != nil {
		for key, value := range metadata {
			formData.Add(fmt.Sprintf("metadata[%s]", key), value)
		}
	}

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/subscriptions",
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем заголовки
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+c.apiKey)

	// Выполняем запрос
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Парсим ответ
	var subscriptionResp SubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&subscriptionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if subscriptionResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", subscriptionResp.Error.Message)
	}

	c.log.Info("Successfully created Stripe subscription with ID: %s", subscriptionResp.ID)
	return &subscriptionResp, nil
}

// GetSubscription получает подписку из Stripe по ID
func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (*SubscriptionResponse, error) {
	c.log.Debug("Getting Stripe subscription with ID: %s", subscriptionID)

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		c.baseURL+"/subscriptions/"+subscriptionID,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем заголовки
	req.Header.Add("Authorization", "Bearer "+c.apiKey)

	// Выполняем запрос
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Парсим ответ
	var subscriptionResp SubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&subscriptionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if subscriptionResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", subscriptionResp.Error.Message)
	}

	c.log.Debug("Successfully retrieved Stripe subscription: %s", subscriptionResp.ID)
	return &subscriptionResp, nil
}

// CancelSubscription отменяет подписку в Stripe
func (c *Client) CancelSubscription(ctx context.Context, subscriptionID string, cancelAtPeriodEnd bool) (*SubscriptionResponse, error) {
	c.log.Debug("Cancelling Stripe subscription with ID: %s, cancelAtPeriodEnd: %v", subscriptionID, cancelAtPeriodEnd)

	// Формируем данные для запроса
	formData := url.Values{}
	if cancelAtPeriodEnd {
		formData.Add("cancel_at_period_end", "true")
	}

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	var req *http.Request
	var err error

	if cancelAtPeriodEnd {
		// Если отмена в конце периода, используем метод PATCH
		req, err = http.NewRequestWithContext(
			ctx,
			"POST",
			c.baseURL+"/subscriptions/"+subscriptionID,
			strings.NewReader(formData.Encode()),
		)
	} else {
		// Если немедленная отмена, используем метод DELETE
		req, err = http.NewRequestWithContext(
			ctx,
			"DELETE",
			c.baseURL+"/subscriptions/"+subscriptionID,
			nil,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем заголовки
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+c.apiKey)

	// Выполняем запрос
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Парсим ответ
	var subscriptionResp SubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&subscriptionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if subscriptionResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", subscriptionResp.Error.Message)
	}

	c.log.Info("Successfully cancelled Stripe subscription: %s", subscriptionResp.ID)
	return &subscriptionResp, nil
}

// CreatePrice создает новую цену (тариф) в Stripe
func (c *Client) CreatePrice(ctx context.Context, plan domain.SubscriptionPlan) (*Price, error) {
	c.log.Debug("Creating Stripe price for plan: %s", plan.Name)

	// Сначала создаем продукт
	productID, err := c.createProduct(ctx, plan.Name, map[string]string{
		"payment_service_plan_id": plan.ID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	// Преобразуем сумму из рублей в копейки для Stripe
	amountInSmallestUnit := int64(plan.Amount * 100)

	// Формируем данные для запроса
	formData := url.Values{}
	formData.Add("unit_amount", fmt.Sprintf("%d", amountInSmallestUnit))
	formData.Add("currency", plan.Currency)
	formData.Add("product", productID)

	// Настраиваем периодичность
	formData.Add("recurring[interval]", string(plan.Interval))
	if plan.IntervalCount > 1 {
		formData.Add("recurring[interval_count]", fmt.Sprintf("%d", plan.IntervalCount))
	}
	if plan.TrialPeriodDays > 0 {
		formData.Add("recurring[trial_period_days]", fmt.Sprintf("%d", plan.TrialPeriodDays))
	}

	// Добавляем метаданные
	formData.Add("metadata[payment_service_plan_id]", plan.ID.String())
	if plan.Metadata != nil {
		for key, value := range plan.Metadata {
			formData.Add(fmt.Sprintf("metadata[%s]", key), value)
		}
	}

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/prices",
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем заголовки
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+c.apiKey)

	// Выполняем запрос
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Парсим ответ
	var priceResp Price
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.log.Info("Successfully created Stripe price with ID: %s", priceResp.ID)
	return &priceResp, nil
}

// createProduct создает новый продукт в Stripe
func (c *Client) createProduct(ctx context.Context, name string, metadata map[string]string) (string, error) {
	c.log.Debug("Creating Stripe product: %s", name)

	// Формируем данные для запроса
	formData := url.Values{}
	formData.Add("name", name)
	formData.Add("type", "service")

	// Добавляем метаданные
	if metadata != nil {
		for key, value := range metadata {
			formData.Add(fmt.Sprintf("metadata[%s]", key), value)
		}
	}

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/products",
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем заголовки
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+c.apiKey)

	// Выполняем запрос
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Парсим ответ
	var productResp struct {
		ID    string         `json:"id"`
		Error *ErrorResponse `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&productResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if productResp.Error != nil {
		return "", fmt.Errorf("stripe API error: %s", productResp.Error.Message)
	}

	c.log.Info("Successfully created Stripe product with ID: %s", productResp.ID)
	return productResp.ID, nil
}

// ToDomainSubscription преобразует подписку Stripe в доменную модель
func ToDomainSubscription(stripeSubscription *SubscriptionResponse, customerID uuid.UUID, planID uuid.UUID, existingID uuid.UUID) (domain.Subscription, error) {
	var subscriptionID uuid.UUID
	if existingID != uuid.Nil {
		subscriptionID = existingID
	} else {
		subscriptionID = uuid.New()
	}

	// Получаем статус подписки
	status, err := mapStripeSubscriptionStatus(stripeSubscription.Status)
	if err != nil {
		return domain.Subscription{}, err
	}

	// Преобразуем время
	currentPeriodStart := time.Unix(stripeSubscription.CurrentPeriodStart, 0)
	currentPeriodEnd := time.Unix(stripeSubscription.CurrentPeriodEnd, 0)

	// Обработка опциональных полей
	var canceledAt *time.Time
	if stripeSubscription.CanceledAt != nil {
		t := time.Unix(*stripeSubscription.CanceledAt, 0)
		canceledAt = &t
	}

	var trialStart, trialEnd *time.Time
	if stripeSubscription.TrialStart != nil {
		t := time.Unix(*stripeSubscription.TrialStart, 0)
		trialStart = &t
	}
	if stripeSubscription.TrialEnd != nil {
		t := time.Unix(*stripeSubscription.TrialEnd, 0)
		trialEnd = &t
	}

	// Создаем доменную модель
	return domain.Subscription{
		ID:                     subscriptionID,
		CustomerID:             customerID,
		PlanID:                 planID,
		Status:                 status,
		CurrentPeriodStart:     currentPeriodStart,
		CurrentPeriodEnd:       currentPeriodEnd,
		CanceledAt:             canceledAt,
		CancelAtPeriodEnd:      stripeSubscription.CancelAtPeriodEnd,
		TrialStart:             trialStart,
		TrialEnd:               trialEnd,
		DefaultPaymentMethodID: stripeSubscription.DefaultPaymentMethod,
		Metadata:               stripeSubscription.Metadata,
		ExternalID:             uuid.MustParse(stripeSubscription.ID), // В реальном коде нужна обработка ошибок
		CreatedAt:              time.Unix(stripeSubscription.Created, 0),
		UpdatedAt:              time.Now(),
	}, nil
}

// mapStripeSubscriptionStatus преобразует статус подписки Stripe в статус доменной модели
func mapStripeSubscriptionStatus(stripeStatus string) (domain.SubscriptionStatus, error) {
	switch stripeStatus {
	case "active":
		return domain.SubscriptionStatusActive, nil
	case "trialing":
		return domain.SubscriptionStatusTrialing, nil
	case "canceled", "incomplete_expired", "unpaid":
		return domain.SubscriptionStatusCanceled, nil
	case "past_due":
		return domain.SubscriptionStatusPaused, nil
	default:
		return "", fmt.Errorf("unknown subscription status: %s", stripeStatus)
	}
}

// mapDomainSubscriptionInterval преобразует интервал подписки доменной модели в строку для Stripe
func mapDomainSubscriptionInterval(interval domain.SubscriptionInterval) string {
	switch interval {
	case domain.SubscriptionIntervalDay:
		return "day"
	case domain.SubscriptionIntervalWeek:
		return "week"
	case domain.SubscriptionIntervalMonth:
		return "month"
	case domain.SubscriptionIntervalYear:
		return "year"
	default:
		return "month" // По умолчанию месяц
	}
}
