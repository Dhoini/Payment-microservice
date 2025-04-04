package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/Dhoini/Payment-microservice/internal/models"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/jmoiron/sqlx"
)

var (
	ErrCustomerNotFound = errors.New("customer not found")
	ErrCustomerExists   = errors.New("customer already exists")
)

type CustomerRepository interface {
	Create(ctx context.Context, customer *models.Customer) error
	GetByUserID(ctx context.Context, userID string) (*models.Customer, error)
	GetByStripeID(ctx context.Context, stripeID string) (*models.Customer, error)
	Update(ctx context.Context, customer *models.Customer) error
}

type postgresCustomerRepository struct {
	db  *sqlx.DB
	log *logger.Logger
}

func NewCustomerRepository(db *sqlx.DB, log *logger.Logger) CustomerRepository {
	return &postgresCustomerRepository{
		db:  db,
		log: log,
	}
}

func (r *postgresCustomerRepository) Create(ctx context.Context, customer *models.Customer) error {
	query := `
		INSERT INTO customers (user_id, stripe_customer_id, email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.ExecContext(ctx, query,
		customer.UserID,
		customer.StripeCustomerID,
		customer.Email,
		customer.CreatedAt,
		customer.UpdatedAt,
	)

	if err != nil {
		r.log.Errorw("Failed to create customer", "error", err, "userID", customer.UserID)
		return fmt.Errorf("failed to create customer: %w", err)
	}

	return nil
}

func (r *postgresCustomerRepository) GetByUserID(ctx context.Context, userID string) (*models.Customer, error) {
	var customer models.Customer

	query := `
		SELECT user_id, stripe_customer_id, email, created_at, updated_at
		FROM customers
		WHERE user_id = $1
	`

	err := r.db.GetContext(ctx, &customer, query, userID)
	if err != nil {
		r.log.Errorw("Failed to get customer by userID", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	return &customer, nil
}

func (r *postgresCustomerRepository) GetByStripeID(ctx context.Context, stripeID string) (*models.Customer, error) {
	var customer models.Customer

	query := `
		SELECT user_id, stripe_customer_id, email, created_at, updated_at
		FROM customers
		WHERE stripe_customer_id = $1
	`

	err := r.db.GetContext(ctx, &customer, query, stripeID)
	if err != nil {

		r.log.Errorw("Failed to get customer by stripeID", "error", err, "stripeID", stripeID)
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	return &customer, nil
}

func (r *postgresCustomerRepository) Update(ctx context.Context, customer *models.Customer) error {
	query := `
		UPDATE customers
		SET email = $1, updated_at = $2
		WHERE user_id = $3
	`

	result, err := r.db.ExecContext(ctx, query,
		customer.Email,
		customer.UpdatedAt,
		customer.UserID,
	)

	if err != nil {
		r.log.Errorw("Failed to update customer", "error", err, "userID", customer.UserID)
		return fmt.Errorf("failed to update customer: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrCustomerNotFound
	}

	return nil
}
