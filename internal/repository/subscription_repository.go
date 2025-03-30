package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
)

// InMemorySubscriptionRepository реализация репозитория подписок в памяти
type InMemorySubscriptionRepository struct {
	subscriptions map[uuid.UUID]domain.Subscription
	plans         map[uuid.UUID]domain.SubscriptionPlan
	mutex         sync.RWMutex
	log           *logger.Logger
}

// NewInMemorySubscriptionRepository создает новый репозиторий подписок в памяти
func NewInMemorySubscriptionRepository(log *logger.Logger) *InMemorySubscriptionRepository {
	return &InMemorySubscriptionRepository{
		subscriptions: make(map[uuid.UUID]domain.Subscription),
		plans:         make(map[uuid.UUID]domain.SubscriptionPlan),
		log:           log,
	}
}

// Методы для работы с подписками

// GetAll возвращает все подписки
func (r *InMemorySubscriptionRepository) GetAll(ctx context.Context) ([]domain.Subscription, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	subscriptions := make([]domain.Subscription, 0, len(r.subscriptions))
	for _, subscription := range r.subscriptions {
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

// GetByID возвращает подписку по ID
func (r *InMemorySubscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	subscription, exists := r.subscriptions[id]
	if !exists {
		return domain.Subscription{}, ErrNotFound
	}

	return subscription, nil
}

// GetByCustomerID возвращает подписки по ID клиента
func (r *InMemorySubscriptionRepository) GetByCustomerID(ctx context.Context, customerID uuid.UUID) ([]domain.Subscription, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var subscriptions []domain.Subscription
	for _, subscription := range r.subscriptions {
		if subscription.CustomerID == customerID {
			subscriptions = append(subscriptions, subscription)
		}
	}

	return subscriptions, nil
}

// Create создает новую подписку
func (r *InMemorySubscriptionRepository) Create(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Проверяем существование плана
	_, exists := r.plans[subscription.PlanID]
	if !exists {
		return domain.Subscription{}, ErrNotFound
	}

	subscription.CreatedAt = time.Now()
	subscription.UpdatedAt = time.Now()

	r.subscriptions[subscription.ID] = subscription

	return subscription, nil
}

// Update обновляет существующую подписку
func (r *InMemorySubscriptionRepository) Update(ctx context.Context, subscription domain.Subscription) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	_, exists := r.subscriptions[subscription.ID]
	if !exists {
		return ErrNotFound
	}

	subscription.UpdatedAt = time.Now()
	r.subscriptions[subscription.ID] = subscription

	return nil
}

// Delete удаляет подписку
func (r *InMemorySubscriptionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.subscriptions[id]; !exists {
		return ErrNotFound
	}

	delete(r.subscriptions, id)

	return nil
}

// Методы для работы с планами подписок

// GetAllPlans возвращает все планы подписок
func (r *InMemorySubscriptionRepository) GetAllPlans(ctx context.Context) ([]domain.SubscriptionPlan, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	plans := make([]domain.SubscriptionPlan, 0, len(r.plans))
	for _, plan := range r.plans {
		plans = append(plans, plan)
	}

	return plans, nil
}

// GetPlanByID возвращает план подписки по ID
func (r *InMemorySubscriptionRepository) GetPlanByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPlan, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	plan, exists := r.plans[id]
	if !exists {
		return domain.SubscriptionPlan{}, ErrNotFound
	}

	return plan, nil
}

// CreatePlan создает новый план подписки
func (r *InMemorySubscriptionRepository) CreatePlan(ctx context.Context, plan domain.SubscriptionPlan) (domain.SubscriptionPlan, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	r.plans[plan.ID] = plan

	return plan, nil
}

// UpdatePlan обновляет существующий план подписки
func (r *InMemorySubscriptionRepository) UpdatePlan(ctx context.Context, plan domain.SubscriptionPlan) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	existingPlan, exists := r.plans[plan.ID]
	if !exists {
		return ErrNotFound
	}

	// Сохраняем неизменяемые параметры
	plan.Amount = existingPlan.Amount
	plan.Currency = existingPlan.Currency
	plan.Interval = existingPlan.Interval
	plan.IntervalCount = existingPlan.IntervalCount
	plan.TrialPeriodDays = existingPlan.TrialPeriodDays
	plan.CreatedAt = existingPlan.CreatedAt

	plan.UpdatedAt = time.Now()

	r.plans[plan.ID] = plan

	return nil
}

// DeletePlan удаляет план подписки
func (r *InMemorySubscriptionRepository) DeletePlan(ctx context.Context, id uuid.UUID) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.plans[id]; !exists {
		return ErrNotFound
	}

	// Проверяем, что нет активных подписок на этот план
	for _, subscription := range r.subscriptions {
		if subscription.PlanID == id &&
			(subscription.Status == domain.SubscriptionStatusActive ||
				subscription.Status == domain.SubscriptionStatusTrialing) {
			return ErrInvalidOperation
		}
	}

	delete(r.plans, id)

	return nil
}

// Дополнительные методы для PostgreSQL репозитория

// PostgresSubscriptionRepository реализация репозитория подписок через PostgreSQL
type PostgresSubscriptionRepository struct {
	db  *pgxpool.Pool
	log *logger.Logger
}

// NewPostgresSubscriptionRepository создает новый репозиторий подписок через PostgreSQL
func NewPostgresSubscriptionRepository(db *pgxpool.Pool, log *logger.Logger) *PostgresSubscriptionRepository {
	return &PostgresSubscriptionRepository{
		db:  db,
		log: log,
	}
}

// GetAll возвращает все подписки из базы данных
func (r *PostgresSubscriptionRepository) GetAll(ctx context.Context) ([]domain.Subscription, error) {
	query := `
		SELECT 
			id, customer_id, plan_id, status, 
			current_period_start, current_period_end, 
			canceled_at, cancel_at_period_end,
			trial_start, trial_end, 
			default_payment_method_id, metadata,
			external_id, created_at, updated_at
		FROM subscriptions
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []domain.Subscription
	for rows.Next() {
		var subscription domain.Subscription
		var metadataBytes []byte
		var externalIDStr string
		var canceledAt, trialStart, trialEnd *time.Time

		err := rows.Scan(
			&subscription.ID,
			&subscription.CustomerID,
			&subscription.PlanID,
			&subscription.Status,
			&subscription.CurrentPeriodStart,
			&subscription.CurrentPeriodEnd,
			&canceledAt,
			&subscription.CancelAtPeriodEnd,
			&trialStart,
			&trialEnd,
			&subscription.DefaultPaymentMethodID,
			&metadataBytes,
			&externalIDStr,
			&subscription.CreatedAt,
			&subscription.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}

		// Преобразуем опциональные поля
		subscription.CanceledAt = canceledAt
		subscription.TrialStart = trialStart
		subscription.TrialEnd = trialEnd

		// Преобразуем JSON метаданные
		if len(metadataBytes) > 0 {
			// В реальном коде используйте json.Unmarshal
			subscription.Metadata = make(map[string]string)
		}

		// Преобразуем UUID из строки
		if externalIDStr != "" {
			externalID, err := uuid.Parse(externalIDStr)
			if err == nil {
				subscription.ExternalID = externalID
			}
		}

		subscriptions = append(subscriptions, subscription)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subscriptions: %w", err)
	}

	return subscriptions, nil
}

// GetByID возвращает подписку по ID из базы данных
func (r *PostgresSubscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Subscription, error) {
	query := `
		SELECT 
			id, customer_id, plan_id, status, 
			current_period_start, current_period_end, 
			canceled_at, cancel_at_period_end,
			trial_start, trial_end, 
			default_payment_method_id, metadata,
			external_id, created_at, updated_at
		FROM subscriptions
		WHERE id = $1
	`

	var subscription domain.Subscription
	var metadataBytes []byte
	var externalIDStr string
	var canceledAt, trialStart, trialEnd *time.Time

	err := r.db.QueryRow(ctx, query, id).Scan(
		&subscription.ID,
		&subscription.CustomerID,
		&subscription.PlanID,
		&subscription.Status,
		&subscription.CurrentPeriodStart,
		&subscription.CurrentPeriodEnd,
		&canceledAt,
		&subscription.CancelAtPeriodEnd,
		&trialStart,
		&trialEnd,
		&subscription.DefaultPaymentMethodID,
		&metadataBytes,
		&externalIDStr,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, ErrNotFound
		}
		return domain.Subscription{}, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Преобразуем опциональные поля
	subscription.CanceledAt = canceledAt
	subscription.TrialStart = trialStart
	subscription.TrialEnd = trialEnd

	// Преобразуем JSON метаданные
	if len(metadataBytes) > 0 {
		// В реальном коде используйте json.Unmarshal
		subscription.Metadata = make(map[string]string)
	}

	// Преобразуем UUID из строки
	if externalIDStr != "" {
		externalID, err := uuid.Parse(externalIDStr)
		if err == nil {
			subscription.ExternalID = externalID
		}
	}

	return subscription, nil
}

// GetByCustomerID возвращает подписки по ID клиента из базы данных
func (r *PostgresSubscriptionRepository) GetByCustomerID(ctx context.Context, customerID uuid.UUID) ([]domain.Subscription, error) {
	query := `
		SELECT 
			id, customer_id, plan_id, status, 
			current_period_start, current_period_end, 
			canceled_at, cancel_at_period_end,
			trial_start, trial_end, 
			default_payment_method_id, metadata,
			external_id, created_at, updated_at
		FROM subscriptions
		WHERE customer_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer rows.Close()

	var subscriptions []domain.Subscription
	for rows.Next() {
		var subscription domain.Subscription
		var metadataBytes []byte
		var externalIDStr string
		var canceledAt, trialStart, trialEnd *time.Time

		err := rows.Scan(
			&subscription.ID,
			&subscription.CustomerID,
			&subscription.PlanID,
			&subscription.Status,
			&subscription.CurrentPeriodStart,
			&subscription.CurrentPeriodEnd,
			&canceledAt,
			&subscription.CancelAtPeriodEnd,
			&trialStart,
			&trialEnd,
			&subscription.DefaultPaymentMethodID,
			&metadataBytes,
			&externalIDStr,
			&subscription.CreatedAt,
			&subscription.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription: %w", err)
		}

		// Преобразуем опциональные поля
		subscription.CanceledAt = canceledAt
		subscription.TrialStart = trialStart
		subscription.TrialEnd = trialEnd

		// Преобразуем JSON метаданные
		if len(metadataBytes) > 0 {
			// В реальном коде используйте json.Unmarshal
			subscription.Metadata = make(map[string]string)
		}

		// Преобразуем UUID из строки
		if externalIDStr != "" {
			externalID, err := uuid.Parse(externalIDStr)
			if err == nil {
				subscription.ExternalID = externalID
			}
		}

		subscriptions = append(subscriptions, subscription)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subscriptions: %w", err)
	}

	return subscriptions, nil
}

// Create создает новую подписку в базе данных
func (r *PostgresSubscriptionRepository) Create(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error) {
	query := `
		INSERT INTO subscriptions (
			id, customer_id, plan_id, status, 
			current_period_start, current_period_end, 
			canceled_at, cancel_at_period_end,
			trial_start, trial_end, 
			default_payment_method_id, metadata,
			external_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
		RETURNING id, created_at, updated_at
	`

	// Преобразуем метаданные в JSON
	metadataBytes := []byte("{}")
	// В реальном коде используйте json.Marshal(subscription.Metadata)

	var externalIDStr *string
	if subscription.ExternalID != uuid.Nil {
		str := subscription.ExternalID.String()
		externalIDStr = &str
	}

	err := r.db.QueryRow(
		ctx,
		query,
		subscription.ID,
		subscription.CustomerID,
		subscription.PlanID,
		subscription.Status,
		subscription.CurrentPeriodStart,
		subscription.CurrentPeriodEnd,
		subscription.CanceledAt,
		subscription.CancelAtPeriodEnd,
		subscription.TrialStart,
		subscription.TrialEnd,
		subscription.DefaultPaymentMethodID,
		metadataBytes,
		externalIDStr,
		time.Now(),
		time.Now(),
	).Scan(
		&subscription.ID,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// Проверяем код ошибки на нарушение внешнего ключа
			if pgErr.Code == "23503" {
				return domain.Subscription{}, ErrNotFound
			}
		}
		return domain.Subscription{}, fmt.Errorf("failed to create subscription: %w", err)
	}

	return subscription, nil
}

// Update обновляет существующую подписку в базе данных
func (r *PostgresSubscriptionRepository) Update(ctx context.Context, subscription domain.Subscription) error {
	query := `
		UPDATE subscriptions
		SET 
			status = $1,
			current_period_start = $2,
			current_period_end = $3,
			canceled_at = $4,
			cancel_at_period_end = $5,
			trial_start = $6,
			trial_end = $7,
			default_payment_method_id = $8,
			metadata = $9,
			external_id = $10,
			updated_at = $11
		WHERE id = $12
	`

	// Преобразуем метаданные в JSON
	metadataBytes := []byte("{}")
	// В реальном коде используйте json.Marshal(subscription.Metadata)

	var externalIDStr *string
	if subscription.ExternalID != uuid.Nil {
		str := subscription.ExternalID.String()
		externalIDStr = &str
	}

	result, err := r.db.Exec(
		ctx,
		query,
		subscription.Status,
		subscription.CurrentPeriodStart,
		subscription.CurrentPeriodEnd,
		subscription.CanceledAt,
		subscription.CancelAtPeriodEnd,
		subscription.TrialStart,
		subscription.TrialEnd,
		subscription.DefaultPaymentMethodID,
		metadataBytes,
		externalIDStr,
		time.Now(),
		subscription.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete удаляет подписку из базы данных
func (r *PostgresSubscriptionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM subscriptions WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// GetAllPlans возвращает все планы подписок из базы данных
func (r *PostgresSubscriptionRepository) GetAllPlans(ctx context.Context) ([]domain.SubscriptionPlan, error) {
	query := `
		SELECT 
			id, name, amount, currency, interval, interval_count,
			trial_period_days, active, external_id,
			metadata, created_at, updated_at
		FROM subscription_plans
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscription plans: %w", err)
	}
	defer rows.Close()

	var plans []domain.SubscriptionPlan
	for rows.Next() {
		var plan domain.SubscriptionPlan
		var metadataBytes []byte
		var externalIDStr string
		var intervalStr string

		err := rows.Scan(
			&plan.ID,
			&plan.Name,
			&plan.Amount,
			&plan.Currency,
			&intervalStr,
			&plan.IntervalCount,
			&plan.TrialPeriodDays,
			&plan.Active,
			&externalIDStr,
			&metadataBytes,
			&plan.CreatedAt,
			&plan.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subscription plan: %w", err)
		}

		// Преобразуем интервал из строки в enum
		switch intervalStr {
		case "day":
			plan.Interval = domain.SubscriptionIntervalDay
		case "week":
			plan.Interval = domain.SubscriptionIntervalWeek
		case "month":
			plan.Interval = domain.SubscriptionIntervalMonth
		case "year":
			plan.Interval = domain.SubscriptionIntervalYear
		default:
			plan.Interval = domain.SubscriptionIntervalMonth
		}

		// Преобразуем JSON метаданные
		if len(metadataBytes) > 0 {
			// В реальном коде используйте json.Unmarshal
			plan.Metadata = make(map[string]string)
		}

		// Преобразуем UUID из строки
		if externalIDStr != "" {
			externalID, err := uuid.Parse(externalIDStr)
			if err == nil {
				plan.ExternalID = externalID
			}
		}

		plans = append(plans, plan)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subscription plans: %w", err)
	}

	return plans, nil
}

// GetPlanByID возвращает план подписки по ID из базы данных
func (r *PostgresSubscriptionRepository) GetPlanByID(ctx context.Context, id uuid.UUID) (domain.SubscriptionPlan, error) {
	query := `
		SELECT 
			id, name, amount, currency, interval, interval_count,
			trial_period_days, active, external_id,
			metadata, created_at, updated_at
		FROM subscription_plans
		WHERE id = $1
	`

	var plan domain.SubscriptionPlan
	var metadataBytes []byte
	var externalIDStr string
	var intervalStr string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&plan.ID,
		&plan.Name,
		&plan.Amount,
		&plan.Currency,
		&intervalStr,
		&plan.IntervalCount,
		&plan.TrialPeriodDays,
		&plan.Active,
		&externalIDStr,
		&metadataBytes,
		&plan.CreatedAt,
		&plan.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SubscriptionPlan{}, ErrNotFound
		}
		return domain.SubscriptionPlan{}, fmt.Errorf("failed to get subscription plan: %w", err)
	}

	// Преобразуем интервал из строки в enum
	switch intervalStr {
	case "day":
		plan.Interval = domain.SubscriptionIntervalDay
	case "week":
		plan.Interval = domain.SubscriptionIntervalWeek
	case "month":
		plan.Interval = domain.SubscriptionIntervalMonth
	case "year":
		plan.Interval = domain.SubscriptionIntervalYear
	default:
		plan.Interval = domain.SubscriptionIntervalMonth
	}

	// Преобразуем JSON метаданные
	if len(metadataBytes) > 0 {
		// В реальном коде используйте json.Unmarshal
		plan.Metadata = make(map[string]string)
	}

	// Преобразуем UUID из строки
	if externalIDStr != "" {
		externalID, err := uuid.Parse(externalIDStr)
		if err == nil {
			plan.ExternalID = externalID
		}
	}

	return plan, nil
}

// CreatePlan создает новый план подписки в базе данных
func (r *PostgresSubscriptionRepository) CreatePlan(ctx context.Context, plan domain.SubscriptionPlan) (domain.SubscriptionPlan, error) {
	query := `
		INSERT INTO subscription_plans (
			id, name, amount, currency, interval, interval_count,
			trial_period_days, active, external_id,
			metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		RETURNING id, created_at, updated_at
	`

	// Преобразуем интервал в строку
	var intervalStr string
	switch plan.Interval {
	case domain.SubscriptionIntervalDay:
		intervalStr = "day"
	case domain.SubscriptionIntervalWeek:
		intervalStr = "week"
	case domain.SubscriptionIntervalMonth:
		intervalStr = "month"
	case domain.SubscriptionIntervalYear:
		intervalStr = "year"
	default:
		intervalStr = "month"
	}

	// Преобразуем метаданные в JSON
	metadataBytes := []byte("{}")
	// В реальном коде используйте json.Marshal(plan.Metadata)

	var externalIDStr *string
	if plan.ExternalID != uuid.Nil {
		str := plan.ExternalID.String()
		externalIDStr = &str
	}

	err := r.db.QueryRow(
		ctx,
		query,
		plan.ID,
		plan.Name,
		plan.Amount,
		plan.Currency,
		intervalStr,
		plan.IntervalCount,
		plan.TrialPeriodDays,
		plan.Active,
		externalIDStr,
		metadataBytes,
		time.Now(),
		time.Now(),
	).Scan(
		&plan.ID,
		&plan.CreatedAt,
		&plan.UpdatedAt,
	)

	if err != nil {
		return domain.SubscriptionPlan{}, fmt.Errorf("failed to create subscription plan: %w", err)
	}

	return plan, nil
}

// UpdatePlan обновляет существующий план подписки в базе данных
func (r *PostgresSubscriptionRepository) UpdatePlan(ctx context.Context, plan domain.SubscriptionPlan) error {
	query := `
		UPDATE subscription_plans
		SET 
			name = $1,
			active = $2,
			metadata = $3,
			external_id = $4,
			updated_at = $5
		WHERE id = $6
	`

	// Преобразуем метаданные в JSON
	metadataBytes := []byte("{}")
	// В реальном коде используйте json.Marshal(plan.Metadata)

	var externalIDStr *string
	if plan.ExternalID != uuid.Nil {
		str := plan.ExternalID.String()
		externalIDStr = &str
	}

	result, err := r.db.Exec(
		ctx,
		query,
		plan.Name,
		plan.Active,
		metadataBytes,
		externalIDStr,
		time.Now(),
		plan.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update subscription plan: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// DeletePlan удаляет план подписки из базы данных
func (r *PostgresSubscriptionRepository) DeletePlan(ctx context.Context, id uuid.UUID) error {
	// Проверяем, что нет активных подписок на этот план
	checkQuery := `
		SELECT COUNT(*) 
		FROM subscriptions 
		WHERE plan_id = $1 
		AND (status = 'active' OR status = 'trialing')
	`

	var count int
	err := r.db.QueryRow(ctx, checkQuery, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for active subscriptions: %w", err)
	}

	if count > 0 {
		return ErrInvalidOperation
	}

	// Удаляем план
	query := `DELETE FROM subscription_plans WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete subscription plan: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
