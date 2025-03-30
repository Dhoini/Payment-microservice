package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Dhoini/Payment-microservice/internal/domain"
)

// PaymentIntentResponse представляет ответ PaymentIntent от API Stripe
type PaymentIntentResponse struct {
	ID                 string            `json:"id"`
	Object             string            `json:"object"`
	Amount             int64             `json:"amount"`
	AmountReceived     int64             `json:"amount_received"`
	ClientSecret       string            `json:"client_secret"`
	Currency           string            `json:"currency"`
	Customer           string            `json:"customer"`
	Description        string            `json:"description"`
	Status             string            `json:"status"`
	PaymentMethod      string            `json:"payment_method"`
	PaymentMethodTypes []string          `json:"payment_method_types"`
	ReceiptURL         string            `json:"receipt_url"`
	LastPaymentError   *PaymentError     `json:"last_payment_error,omitempty"`
	Metadata           map[string]string `json:"metadata"`
	Created            int64             `json:"created"`
	Error              *ErrorResponse    `json:"error,omitempty"`
}

// PaymentError представляет ошибку платежа Stripe
type PaymentError struct {
	Type        string `json:"type"`
	Message     string `json:"message"`
	Code        string `json:"code"`
	DeclineCode string `json:"decline_code"`
}

// CreatePaymentIntent создает намерение платежа в Stripe
func (c *Client) CreatePaymentIntent(ctx context.Context, payment domain.Payment) (*PaymentIntentResponse, error) {
	c.log.Debug("Creating Stripe payment intent for customer: %s", payment.CustomerID)

	// Преобразуем сумму из рублей в копейки для Stripe (Stripe работает в наименьших единицах валюты)
	amountInSmallestUnit := int64(payment.Amount * 100)

	// Формируем данные для запроса
	formData := url.Values{}
	formData.Add("amount", fmt.Sprintf("%d", amountInSmallestUnit))
	formData.Add("currency", payment.Currency)

	if payment.CustomerID.String() != "" {
		if payment.Metadata != nil && payment.Metadata["stripe_customer_id"] != "" {
			formData.Add("customer", payment.Metadata["stripe_customer_id"])
		}
	}

	if payment.Description != "" {
		formData.Add("description", payment.Description)
	}

	// Добавляем метаданные, если они есть
	if payment.Metadata != nil {
		for key, value := range payment.Metadata {
			formData.Add(fmt.Sprintf("metadata[%s]", key), value)
		}
	}

	// Добавляем идентификатор платежной системы
	formData.Add("metadata[payment_id]", payment.ID.String())

	// Добавляем типы платежных методов
	formData.Add("payment_method_types[]", "card")

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/payment_intents",
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
	var paymentIntentResp PaymentIntentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentIntentResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if paymentIntentResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", paymentIntentResp.Error.Message)
	}

	c.log.Info("Successfully created Stripe payment intent with ID: %s, status: %s",
		paymentIntentResp.ID, paymentIntentResp.Status)
	return &paymentIntentResp, nil
}

// GetPaymentIntent получает намерение платежа из Stripe
func (c *Client) GetPaymentIntent(ctx context.Context, paymentIntentID string) (*PaymentIntentResponse, error) {
	c.log.Debug("Getting Stripe payment intent with ID: %s", paymentIntentID)

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		c.baseURL+"/payment_intents/"+paymentIntentID,
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
	var paymentIntentResp PaymentIntentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentIntentResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if paymentIntentResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", paymentIntentResp.Error.Message)
	}

	c.log.Debug("Successfully retrieved Stripe payment intent: %s, status: %s",
		paymentIntentResp.ID, paymentIntentResp.Status)
	return &paymentIntentResp, nil
}

// CancelPaymentIntent отменяет намерение платежа в Stripe
func (c *Client) CancelPaymentIntent(ctx context.Context, paymentIntentID string) (*PaymentIntentResponse, error) {
	c.log.Debug("Cancelling Stripe payment intent with ID: %s", paymentIntentID)

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/payment_intents/"+paymentIntentID+"/cancel",
		nil,
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
	var paymentIntentResp PaymentIntentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentIntentResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if paymentIntentResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", paymentIntentResp.Error.Message)
	}

	c.log.Info("Successfully cancelled Stripe payment intent: %s, status: %s",
		paymentIntentResp.ID, paymentIntentResp.Status)
	return &paymentIntentResp, nil
}

// CreateRefund создает возврат платежа в Stripe
func (c *Client) CreateRefund(ctx context.Context, paymentIntentID string, amount float64) (*RefundResponse, error) {
	c.log.Debug("Creating refund for payment intent: %s", paymentIntentID)

	// Преобразуем сумму из рублей в копейки для Stripe
	amountInSmallestUnit := int64(amount * 100)

	// Формируем данные для запроса
	formData := url.Values{}
	formData.Add("payment_intent", paymentIntentID)

	if amount > 0 {
		formData.Add("amount", fmt.Sprintf("%d", amountInSmallestUnit))
	}

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/refunds",
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
	var refundResp RefundResponse
	if err := json.NewDecoder(resp.Body).Decode(&refundResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if refundResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", refundResp.Error.Message)
	}

	c.log.Info("Successfully created refund: %s for payment intent: %s",
		refundResp.ID, paymentIntentID)
	return &refundResp, nil
}

// RefundResponse представляет ответ о возврате от API Stripe
type RefundResponse struct {
	ID            string            `json:"id"`
	Object        string            `json:"object"`
	Amount        int64             `json:"amount"`
	Currency      string            `json:"currency"`
	PaymentIntent string            `json:"payment_intent"`
	Status        string            `json:"status"`
	Reason        string            `json:"reason"`
	Metadata      map[string]string `json:"metadata"`
	Created       int64             `json:"created"`
	Error         *ErrorResponse    `json:"error,omitempty"`
}

// StripeStatus преобразует статус платежа Stripe в статус системы
func StripeStatus(stripeStatus string) domain.PaymentStatus {
	switch stripeStatus {
	case "succeeded":
		return domain.PaymentStatusCompleted
	case "canceled":
		return domain.PaymentStatusFailed
	case "processing":
		return domain.PaymentStatusPending
	default:
		return domain.PaymentStatusPending
	}
}
