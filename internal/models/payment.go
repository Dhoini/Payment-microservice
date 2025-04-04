package models

import "time"

// Subscription представляет подписку пользователя в системе.
type Subscription struct {
	SubscriptionID   string     `db:"subscription_id" json:"subscription_id"`       // ID подписки (может быть из Stripe)
	UserID           string     `db:"user_id" json:"user_id"`                       // ID пользователя, которому принадлежит подписка
	PlanID           string     `db:"plan_id" json:"plan_id"`                       // ID тарифного плана
	Status           string     `db:"status" json:"status"`                         // Статус подписки (e.g., active, canceled, past_due)
	StripeCustomerID string     `db:"stripe_customer_id" json:"stripe_customer_id"` // ID клиента в Stripe
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`                 // Время создания записи
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`                 // Время последнего обновления записи
	ExpiresAt        *time.Time `db:"expires_at" json:"expires_at,omitempty"`       // Время окончания подписки (если применимо)
	CanceledAt       *time.Time `db:"canceled_at" json:"canceled_at,omitempty"`     // Время отмены подписки
}
