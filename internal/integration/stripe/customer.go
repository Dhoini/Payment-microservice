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

// CustomerResponse представляет ответ от API Stripe при работе с клиентом
type CustomerResponse struct {
	ID          string            `json:"id"`
	Object      string            `json:"object"`
	Email       string            `json:"email"`
	Name        string            `json:"name"`
	Phone       string            `json:"phone"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	Created     int64             `json:"created"`
	Deleted     bool              `json:"deleted,omitempty"`
	Error       *ErrorResponse    `json:"error,omitempty"`
}

// ErrorResponse представляет ошибку от API Stripe
type ErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code"`
	Param   string `json:"param"`
}

// CreateCustomer создает нового клиента в Stripe
func (c *Client) CreateCustomer(ctx context.Context, customer domain.Customer) (*CustomerResponse, error) {
	c.log.Debug("Creating Stripe customer for %s", customer.Email)

	// Формируем данные для запроса
	formData := url.Values{}
	formData.Add("email", customer.Email)
	if customer.Name != "" {
		formData.Add("name", customer.Name)
	}
	if customer.Phone != "" {
		formData.Add("phone", customer.Phone)
	}

	// Добавляем метаданные, если они есть
	if customer.Metadata != nil {
		for key, value := range customer.Metadata {
			formData.Add(fmt.Sprintf("metadata[%s]", key), value)
		}
	}

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/customers",
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
	var customerResp CustomerResponse
	if err := json.NewDecoder(resp.Body).Decode(&customerResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if customerResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", customerResp.Error.Message)
	}

	c.log.Info("Successfully created Stripe customer with ID: %s", customerResp.ID)
	return &customerResp, nil
}

// GetCustomer получает клиента из Stripe по ID
func (c *Client) GetCustomer(ctx context.Context, stripeID string) (*CustomerResponse, error) {
	c.log.Debug("Getting Stripe customer with ID: %s", stripeID)

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		c.baseURL+"/customers/"+stripeID,
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
	var customerResp CustomerResponse
	if err := json.NewDecoder(resp.Body).Decode(&customerResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if customerResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", customerResp.Error.Message)
	}

	c.log.Debug("Successfully retrieved Stripe customer: %s", customerResp.ID)
	return &customerResp, nil
}

// UpdateCustomer обновляет клиента в Stripe
func (c *Client) UpdateCustomer(ctx context.Context, stripeID string, customer domain.Customer) (*CustomerResponse, error) {
	c.log.Debug("Updating Stripe customer with ID: %s", stripeID)

	// Формируем данные для запроса
	formData := url.Values{}
	if customer.Email != "" {
		formData.Add("email", customer.Email)
	}
	if customer.Name != "" {
		formData.Add("name", customer.Name)
	}
	if customer.Phone != "" {
		formData.Add("phone", customer.Phone)
	}

	// Добавляем метаданные, если они есть
	if customer.Metadata != nil {
		for key, value := range customer.Metadata {
			formData.Add(fmt.Sprintf("metadata[%s]", key), value)
		}
	}

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/customers/"+stripeID,
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
	var customerResp CustomerResponse
	if err := json.NewDecoder(resp.Body).Decode(&customerResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if customerResp.Error != nil {
		return nil, fmt.Errorf("stripe API error: %s", customerResp.Error.Message)
	}

	c.log.Info("Successfully updated Stripe customer: %s", customerResp.ID)
	return &customerResp, nil
}

// DeleteCustomer удаляет клиента из Stripe
func (c *Client) DeleteCustomer(ctx context.Context, stripeID string) error {
	c.log.Debug("Deleting Stripe customer with ID: %s", stripeID)

	// Создаем HTTP клиент
	httpClient := &http.Client{}

	// Создаем запрос
	req, err := http.NewRequestWithContext(
		ctx,
		"DELETE",
		c.baseURL+"/customers/"+stripeID,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем заголовки
	req.Header.Add("Authorization", "Bearer "+c.apiKey)

	// Выполняем запрос
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Парсим ответ для проверки на ошибки
	var respData struct {
		Deleted bool           `json:"deleted"`
		Error   *ErrorResponse `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Проверяем на ошибки
	if respData.Error != nil {
		return fmt.Errorf("stripe API error: %s", respData.Error.Message)
	}

	if !respData.Deleted {
		return fmt.Errorf("failed to delete customer, but no error was returned")
	}

	c.log.Info("Successfully deleted Stripe customer: %s", stripeID)
	return nil
}
