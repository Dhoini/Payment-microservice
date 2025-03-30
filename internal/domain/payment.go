package domain

import (
	"github.com/google/uuid"
	"time"
)

// PaymentStatus статус платежа
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusRefunded  PaymentStatus = "refunded"
)

// Payment представляет собой модель платежа
type Payment struct {
	ID            uuid.UUID         `json:"id"`
	CustomerID    uuid.UUID         `json:"customer_id"`
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	Description   string            `json:"description,omitempty"`
	Status        PaymentStatus     `json:"status"`
	MethodID      uuid.UUID         `json:"method_id,omitempty"`
	MethodType    string            `json:"method_type,omitempty"`
	TransactionID string            `json:"transaction_id,omitempty"`
	ReceiptURL    string            `json:"receipt_url,omitempty"`
	ErrorMessage  string            `json:"error_message,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// PaymentRequest представляет запрос на создание платежа
type PaymentRequest struct {
	CustomerID  string            `json:"customer_id" binding:"required,uuid4"`
	Amount      float64           `json:"amount" binding:"required,gt=0"`
	Currency    string            `json:"currency" binding:"required,len=3"`
	Description string            `json:"description"`
	MethodID    string            `json:"method_id,omitempty" binding:"omitempty,uuid4"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
