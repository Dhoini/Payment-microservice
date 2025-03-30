package domain

import (
	"time"

	"github.com/google/uuid"
)

// WebhookEventType тип события вебхука
type WebhookEventType string

const (
	// События платежей
	WebhookEventTypePaymentCreated   WebhookEventType = "payment.created"
	WebhookEventTypePaymentUpdated   WebhookEventType = "payment.updated"
	WebhookEventTypePaymentCompleted WebhookEventType = "payment.completed"
	WebhookEventTypePaymentFailed    WebhookEventType = "payment.failed"
	WebhookEventTypePaymentRefunded  WebhookEventType = "payment.refunded"

	// События подписок
	WebhookEventTypeSubscriptionCreated  WebhookEventType = "subscription.created"
	WebhookEventTypeSubscriptionUpdated  WebhookEventType = "subscription.updated"
	WebhookEventTypeSubscriptionCanceled WebhookEventType = "subscription.canceled"
	WebhookEventTypeSubscriptionRenewed  WebhookEventType = "subscription.renewed"

	// События клиентов
	WebhookEventTypeCustomerCreated WebhookEventType = "customer.created"
	WebhookEventTypeCustomerUpdated WebhookEventType = "customer.updated"
	WebhookEventTypeCustomerDeleted WebhookEventType = "customer.deleted"
)

// WebhookEventStatus статус обработки события
type WebhookEventStatus string

const (
	WebhookEventStatusPending   WebhookEventStatus = "pending"
	WebhookEventStatusProcessed WebhookEventStatus = "processed"
	WebhookEventStatusFailed    WebhookEventStatus = "failed"
)

// WebhookEvent представляет событие вебхука
type WebhookEvent struct {
	ID           uuid.UUID          `json:"id"`
	ExternalID   string             `json:"external_id"` // ID события в платежной системе
	Type         WebhookEventType   `json:"type"`
	Status       WebhookEventStatus `json:"status"`
	Payload      []byte             `json:"payload"`
	ResourceID   string             `json:"resource_id"` // ID ресурса, к которому относится событие
	Provider     string             `json:"provider"`    // Название платежной системы
	AttemptCount int                `json:"attempt_count"`
	LastAttempt  *time.Time         `json:"last_attempt,omitempty"`
	ProcessedAt  *time.Time         `json:"processed_at,omitempty"`
	ErrorMessage string             `json:"error_message,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// PaymentWebhookEvent представляет событие платежа для обработки
type PaymentWebhookEvent struct {
	PaymentID    uuid.UUID        `json:"payment_id"`
	Type         WebhookEventType `json:"type"`
	Amount       float64          `json:"amount,omitempty"`
	Currency     string           `json:"currency,omitempty"`
	ExternalID   string           `json:"external_id"`
	Status       PaymentStatus    `json:"status,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
	Timestamp    time.Time        `json:"timestamp"`
}

// SubscriptionWebhookEvent представляет событие подписки для обработки
type SubscriptionWebhookEvent struct {
	SubscriptionID   uuid.UUID          `json:"subscription_id"`
	Type             WebhookEventType   `json:"type"`
	ExternalID       string             `json:"external_id"`
	Status           SubscriptionStatus `json:"status,omitempty"`
	CurrentPeriodEnd *time.Time         `json:"current_period_end,omitempty"`
	CanceledAt       *time.Time         `json:"canceled_at,omitempty"`
	Timestamp        time.Time          `json:"timestamp"`
}

// CustomerWebhookEvent представляет событие клиента для обработки
type CustomerWebhookEvent struct {
	CustomerID uuid.UUID        `json:"customer_id"`
	Type       WebhookEventType `json:"type"`
	ExternalID string           `json:"external_id"`
	Timestamp  time.Time        `json:"timestamp"`
}

// WebhookEventRequest представляет запрос на создание вебхук события
type WebhookEventRequest struct {
	ExternalID string           `json:"external_id"`
	Type       WebhookEventType `json:"type"`
	Provider   string           `json:"provider"`
	ResourceID string           `json:"resource_id"`
	Payload    []byte           `json:"payload"`
}
