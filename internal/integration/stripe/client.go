package stripe

import (
	"github.com/Dhoini/Payment-microservice/pkg/logger"
)

// Client представляет клиент для работы с API Stripe
type Client struct {
	baseURL    string
	apiKey     string
	webhookKey string
	log        *logger.Logger
}

// Config конфигурация для клиента Stripe
type Config struct {
	APIKey     string
	WebhookKey string
	IsTest     bool
}

// NewClient создает новый клиент Stripe
func NewClient(cfg Config, log *logger.Logger) *Client {
	baseURL := "https://api.stripe.com/v1"
	if cfg.IsTest {
		baseURL = "https://api.stripe.com/v1/test"
	}

	return &Client{
		baseURL:    baseURL,
		apiKey:     cfg.APIKey,
		webhookKey: cfg.WebhookKey,
		log:        log,
	}
}

// GetBaseURL возвращает базовый URL для API Stripe
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// GetAPIKey возвращает API ключ Stripe
func (c *Client) GetAPIKey() string {
	return c.apiKey
}

// GetWebhookKey возвращает ключ для верификации webhook-ов Stripe
func (c *Client) GetWebhookKey() string {
	return c.webhookKey
}
