package domain

import (
	"github.com/google/uuid"
	"time"
)

// SubscriptionStatus статус подписки
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusTrialing SubscriptionStatus = "trialing"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusPaused   SubscriptionStatus = "paused"
)

// SubscriptionInterval период подписки
type SubscriptionInterval string

const (
	SubscriptionIntervalDay   SubscriptionInterval = "day"
	SubscriptionIntervalWeek  SubscriptionInterval = "week"
	SubscriptionIntervalMonth SubscriptionInterval = "month"
	SubscriptionIntervalYear  SubscriptionInterval = "year"
)

// Subscription представляет собой модель подписки
type Subscription struct {
	ID                     uuid.UUID          `json:"id"`
	CustomerID             uuid.UUID          `json:"customer_id"`
	PlanID                 uuid.UUID          `json:"plan_id"`
	Status                 SubscriptionStatus `json:"status"`
	CurrentPeriodStart     time.Time          `json:"current_period_start"`
	CurrentPeriodEnd       time.Time          `json:"current_period_end"`
	CanceledAt             *time.Time         `json:"canceled_at,omitempty"`
	CancelAtPeriodEnd      bool               `json:"cancel_at_period_end"`
	TrialStart             *time.Time         `json:"trial_start,omitempty"`
	TrialEnd               *time.Time         `json:"trial_end,omitempty"`
	DefaultPaymentMethodID string             `json:"default_payment_method_id,omitempty"`
	Metadata               map[string]string  `json:"metadata,omitempty"`
	ExternalID             uuid.UUID          `json:"external_id,omitempty"` // ID в Stripe, например
	CreatedAt              time.Time          `json:"created_at"`
	UpdatedAt              time.Time          `json:"updated_at"`
}

// SubscriptionPlan представляет собой план подписки
type SubscriptionPlan struct {
	ID              uuid.UUID            `json:"id"`
	Name            string               `json:"name"`
	Amount          float64              `json:"amount"`
	Currency        string               `json:"currency"`
	Interval        SubscriptionInterval `json:"interval"`
	IntervalCount   int                  `json:"interval_count"`
	TrialPeriodDays int                  `json:"trial_period_days,omitempty"`
	Active          bool                 `json:"active"`
	ExternalID      uuid.UUID            `json:"external_id,omitempty"`
	Metadata        map[string]string    `json:"metadata,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

// SubscriptionRequest представляет запрос на создание подписки
type SubscriptionRequest struct {
	CustomerID             uuid.UUID         `json:"customer_id" binding:"required"`
	PlanID                 uuid.UUID         `json:"plan_id" binding:"required"`
	TrialPeriodDays        int               `json:"trial_period_days,omitempty"`
	DefaultPaymentMethodID string            `json:"default_payment_method_id,omitempty"`
	Metadata               map[string]string `json:"metadata,omitempty"`
}
