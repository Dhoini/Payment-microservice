package domain

import (
	"github.com/google/uuid"
	"time"
)

// Customer представляет собой модель клиента
type Customer struct {
	ID         uuid.UUID         `json:"id"`
	Email      string            `json:"email"`
	Name       string            `json:"name,omitempty"`
	Phone      string            `json:"phone,omitempty"`
	ExternalID string            `json:"external_id,omitempty"` // ID в системе Stripe, например
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// CustomerRequest представляет запрос на создание/обновление клиента
type CustomerRequest struct {
	Email      string            `json:"email" binding:"required,email"`
	Name       string            `json:"name"`
	Phone      string            `json:"phone"`
	ExternalID string            `json:"external_id,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}
