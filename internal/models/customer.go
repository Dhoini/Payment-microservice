package models

import (
	"time"
)

type Customer struct {
	UserID           string    `db:"user_id" json:"user_id"`
	StripeCustomerID string    `db:"stripe_customer_id" json:"stripe_customer_id"`
	Email            string    `db:"email" json:"email"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

// NewCustomer создает нового Customer с заданными параметрами
func NewCustomer(userID, stripeCustomerID, email string) *Customer {
	now := time.Now()
	return &Customer{
		UserID:           userID,
		StripeCustomerID: stripeCustomerID,
		Email:            email,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
