package db

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
	"time"
)

// DBClient представляет клиент для работы с базой данных.
type DBClient struct {
	db  *sqlx.DB
	log *zap.Logger
}

// NewDBClient создает новый экземпляр DBClient.
func NewDBClient(dsn string, log *zap.Logger) (*DBClient, error) {
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Error("Failed to connect to database", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		log.Error("Failed to ping database", zap.Error(err))
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DBClient{db: db, log: log}, nil
}

// Close закрывает соединение с базой данных.
func (dc *DBClient) Close() error {
	err := dc.db.Close()
	if err != nil {
		dc.log.Error("Failed to close database connection", zap.Error(err))
		return fmt.Errorf("failed to close database connection: %w", err)
	}
	return nil
}

// GetSubscription retrieves a subscription from the database.
func (dc *DBClient) GetSubscription(ctx context.Context, userID, subscriptionID string, subscription *Subscription) error {
	query := `
        SELECT subscription_id, plan_id, created_at, canceled_at
        FROM subscriptions
        WHERE user_id = $1 AND subscription_id = $2
    `
	err := dc.db.QueryRowxContext(ctx, ctx, query, userID, subscriptionID).StructScan(subscription)
	if err != nil {
		if err == sql.ErrNoRows {
			dc.log.Warn("Subscription not found", zap.String("user_id", userID), zap.String("subscription_id", subscriptionID))
			return fmt.Errorf("subscription not found")
		}
		dc.log.Error("Failed to get subscription from database", zap.Error(err))
		return fmt.Errorf("failed to get subscription from database: %w", err)
	}
	dc.log.Debug("Subscription retrieved successfully", zap.String("user_id", userID), zap.String("subscription_id", subscriptionID))
	return nil
}

// SaveSubscription saves a subscription to the database.
func (dc *DBClient) SaveSubscription(ctx context.Context, subscription *Subscription) error {
	query := `
        INSERT INTO subscriptions (subscription_id, user_id, plan_id, created_at, canceled_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (subscription_id) DO UPDATE SET
            user_id = $2,
            plan_id = $3,
            created_at = $4,
            canceled_at = $5
    `
	_, err := dc.db.ExecContext(ctx, ctx, query,
		subscription.SubscriptionID, subscription.UserID, subscription.PlanID,
		subscription.CreatedAt, subscription.CanceledAt)
	if err != nil {
		dc.log.Error("Failed to save subscription to database", zap.Error(err))
		return fmt.Errorf("failed to save subscription to database: %w", err)
	}
	dc.log.Debug("Subscription saved successfully", zap.String("subscription_id", subscription.SubscriptionID))
	return nil
}

// UpdateSubscription updates a subscription in the database.
func (dc *DBClient) UpdateSubscription(ctx context.Context, subscriptionID string, canceledAt time.Time) error {
	query := `
        UPDATE subscriptions
        SET canceled_at = $1
        WHERE subscription_id = $2
    `
	res, err := dc.db.ExecContext(ctx, ctx, query, canceledAt, subscriptionID)
	if err != nil {
		dc.log.Error("Failed to update subscription in database", zap.Error(err))
		return fmt.Errorf("failed to update subscription in database: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		dc.log.Error("Failed to get affected rows count", zap.Error(err))
		return fmt.Errorf("failed to get affected rows count: %w", err)
	}

	if rowsAffected == 0 {
		dc.log.Warn("No rows updated", zap.String("subscription_id", subscriptionID))
		return fmt.Errorf("no such subscription: %s", subscriptionID)
	}
	dc.log.Debug("Subscription updated successfully", zap.String("subscription_id", subscriptionID))

	return nil
}

func (dc *DBClient) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	tx, err := dc.db.Beginx()
	if err != nil {
		dc.log.Error("Failed to begin transaction", zap.Error(err))
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	dc.log.Debug("Transaction started")
	return tx, nil
}

func (dc *DBClient) CommitTx(ctx context.Context, tx *sqlx.Tx) error {
	err := tx.Commit()
	if err != nil {
		dc.log.Error("Failed to commit transaction", zap.Error(err))
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	dc.log.Debug("Transaction committed")
	return nil
}

func (dc *DBClient) RollbackTx(ctx context.Context, tx *sqlx.Tx) error {
	err := tx.Rollback()
	if err != nil {
		dc.log.Error("Failed to rollback transaction", zap.Error(err))
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}
	dc.log.Debug("Transaction rolled back")
	return nil
}

type Subscription struct {
	SubscriptionID string    `db:"subscription_id"`
	UserID         string    `db:"user_id"`
	PlanID         string    `db:"plan_id"`
	CreatedAt      time.Time `db:"created_at"`
	CanceledAt     time.Time `db:"canceled_at"`
}
