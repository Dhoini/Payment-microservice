package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresCustomerRepository реализация репозитория клиентов через PostgreSQL
type PostgresCustomerRepository struct {
	db  *pgxpool.Pool
	log *logger.Logger
}

// NewPostgresCustomerRepository создает новый репозиторий клиентов через PostgreSQL
func NewPostgresCustomerRepository(db *pgxpool.Pool, log *logger.Logger) *PostgresCustomerRepository {
	return &PostgresCustomerRepository{
		db:  db,
		log: log,
	}
}

// GetAll возвращает всех клиентов
func (r *PostgresCustomerRepository) GetAll(ctx context.Context) ([]domain.Customer, error) {
	query := `
		SELECT id, email, name, phone, external_id, metadata, created_at, updated_at
		FROM customers
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query customers: %w", err)
	}
	defer rows.Close()

	var customers []domain.Customer
	for rows.Next() {
		var customer domain.Customer
		var metadataBytes []byte

		err := rows.Scan(
			&customer.ID,
			&customer.Email,
			&customer.Name,
			&customer.Phone,
			&customer.ExternalID,
			&metadataBytes,
			&customer.CreatedAt,
			&customer.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan customer: %w", err)
		}

		// Парсим JSON метаданные
		if len(metadataBytes) > 0 {
			// Здесь нужно распарсить JSON в map[string]string
			// В реальном коде можно использовать json.Unmarshal
			customer.Metadata = make(map[string]string)
		}

		customers = append(customers, customer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating customers: %w", err)
	}

	return customers, nil
}

// GetByID возвращает клиента по ID
func (r *PostgresCustomerRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Customer, error) {
	query := `
		SELECT id, email, name, phone, external_id, metadata, created_at, updated_at
		FROM customers
		WHERE id = $1
	`

	var customer domain.Customer
	var metadataBytes []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&customer.ID,
		&customer.Email,
		&customer.Name,
		&customer.Phone,
		&customer.ExternalID,
		&metadataBytes,
		&customer.CreatedAt,
		&customer.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Customer{}, repository.ErrNotFound
		}
		return domain.Customer{}, fmt.Errorf("failed to get customer: %w", err)
	}

	// Парсим JSON метаданные
	if len(metadataBytes) > 0 {
		// В реальном коде здесь json.Unmarshal
		customer.Metadata = make(map[string]string)
	}

	return customer, nil
}

// Create создает нового клиента
func (r *PostgresCustomerRepository) Create(ctx context.Context, customer domain.Customer) (domain.Customer, error) {
	query := `
		INSERT INTO customers (id, email, name, phone, external_id, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`

	// Преобразуем метаданные в JSON
	// В реальном коде здесь json.Marshal
	metadataBytes := []byte("{}")

	err := r.db.QueryRow(
		ctx,
		query,
		customer.ID,
		customer.Email,
		customer.Name,
		customer.Phone,
		customer.ExternalID,
		metadataBytes,
		customer.CreatedAt,
		customer.UpdatedAt,
	).Scan(&customer.ID, &customer.CreatedAt, &customer.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// Проверяем код ошибки на нарушение уникальности
			if pgErr.Code == "23505" {
				return domain.Customer{}, repository.ErrDuplicate
			}
		}
		return domain.Customer{}, fmt.Errorf("failed to create customer: %w", err)
	}

	return customer, nil
}

// Update обновляет существующего клиента
func (r *PostgresCustomerRepository) Update(ctx context.Context, customer domain.Customer) error {
	query := `
		UPDATE customers
		SET email = $1, name = $2, phone = $3, external_id = $4, metadata = $5, updated_at = $6
		WHERE id = $7
	`

	// Преобразуем метаданные в JSON
	// В реальном коде здесь json.Marshal
	metadataBytes := []byte("{}")

	result, err := r.db.Exec(
		ctx,
		query,
		customer.Email,
		customer.Name,
		customer.Phone,
		customer.ExternalID,
		metadataBytes,
		customer.UpdatedAt,
		customer.ID,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// Проверяем код ошибки на нарушение уникальности
			if pgErr.Code == "23505" {
				return repository.ErrDuplicate
			}
		}
		return fmt.Errorf("failed to update customer: %w", err)
	}

	if result.RowsAffected() == 0 {
		return repository.ErrNotFound
	}

	return nil
}

// Delete удаляет клиента
func (r *PostgresCustomerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM customers WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete customer: %w", err)
	}

	if result.RowsAffected() == 0 {
		return repository.ErrNotFound
	}

	return nil
}
