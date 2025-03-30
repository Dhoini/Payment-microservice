package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewConnection создает новое подключение к PostgreSQL
func NewConnection(ctx context.Context, connString string, log *logger.Logger) (*pgxpool.Pool, error) {
	log.Info("Connecting to PostgreSQL")

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %w", err)
	}

	// Настраиваем пул соединений
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Проверяем подключение
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Info("Successfully connected to PostgreSQL")
	return pool, nil
}
