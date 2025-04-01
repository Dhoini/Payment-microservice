package repository

import (
	"context"
	"database/sql"
	"errors" // Добавлено для определения ErrNotFound
	"fmt"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/models" // Убедитесь, что путь верный
	"github.com/Dhoini/Payment-microservice/pkg/logger"      // Ваш логгер
	"github.com/jmoiron/sqlx"                                // sqlx для работы с БД
)

// ErrNotFound стандартная ошибка для случаев, когда запись не найдена.
var ErrNotFound = errors.New("record not found")

// postgresSubscriptionRepo реализует SubscriptionRepository для PostgreSQL.
type postgresSubscriptionRepo struct {
	db  *sqlx.DB       // Подключение к БД через sqlx
	log *logger.Logger // Ваш логгер
}

// NewPostgresSubscriptionRepository создает новый экземпляр репозитория для PostgreSQL.
func NewPostgresSubscriptionRepository(db *sqlx.DB, log *logger.Logger) SubscriptionRepository {
	return &postgresSubscriptionRepo{
		db:  db,
		log: log,
	}
}

// Create сохраняет новую подписку в базе данных.
func (r *postgresSubscriptionRepo) Create(ctx context.Context, sub *models.Subscription) error {
	// Добавляем время создания и обновления перед вставкой
	now := time.Now()
	sub.CreatedAt = now
	sub.UpdatedAt = now

	query := `
        INSERT INTO subscriptions (
            subscription_id, user_id, plan_id, status, stripe_customer_id,
            created_at, updated_at, expires_at, canceled_at
        ) VALUES (
            :subscription_id, :user_id, :plan_id, :status, :stripe_customer_id,
            :created_at, :updated_at, :expires_at, :canceled_at
        )`
	// Используем NamedExecContext для удобного маппинга полей структуры на параметры запроса
	_, err := r.db.NamedExecContext(ctx, query, sub)
	if err != nil {
		r.log.Errorw("Failed to create subscription in DB", "error", err, "subscriptionID", sub.SubscriptionID, "userID", sub.UserID)
		// TODO: Обработать специфические ошибки БД (например, дубликат ключа), если нужно
		return fmt.Errorf("repository: failed to create subscription: %w", err)
	}

	r.log.Debugw("Successfully created subscription in DB", "subscriptionID", sub.SubscriptionID, "userID", sub.UserID)
	return nil
}

// GetByID возвращает подписку по ее ID (предполагаем, что это Stripe Subscription ID).
func (r *postgresSubscriptionRepo) GetByID(ctx context.Context, subscriptionID string) (*models.Subscription, error) {
	var sub models.Subscription
	query := `
        SELECT subscription_id, user_id, plan_id, status, stripe_customer_id,
               created_at, updated_at, expires_at, canceled_at
        FROM subscriptions
        WHERE subscription_id = $1`

	err := r.db.GetContext(ctx, &sub, query, subscriptionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Warnw("Subscription not found by ID", "subscriptionID", subscriptionID)
			return nil, ErrNotFound // Возвращаем стандартную ошибку
		}
		r.log.Errorw("Failed to get subscription by ID from DB", "error", err, "subscriptionID", subscriptionID)
		return nil, fmt.Errorf("repository: failed to get subscription by ID: %w", err)
	}

	r.log.Debugw("Successfully retrieved subscription by ID", "subscriptionID", sub.SubscriptionID)
	return &sub, nil
}

// GetByUserID возвращает все подписки пользователя.
// Примечание: часто требуется возвращать только *активные* подписки,
// этот метод возвращает все. Возможно, понадобится доп. метод или фильтр.
func (r *postgresSubscriptionRepo) GetByUserID(ctx context.Context, userID string) ([]models.Subscription, error) {
	var subs []models.Subscription
	query := `
        SELECT subscription_id, user_id, plan_id, status, stripe_customer_id,
               created_at, updated_at, expires_at, canceled_at
        FROM subscriptions
        WHERE user_id = $1
        ORDER BY created_at DESC` // Сортируем по убыванию даты создания

	err := r.db.SelectContext(ctx, &subs, query, userID)
	if err != nil {
		// Ошибку sql.ErrNoRows не считаем критической для списка, вернем пустой слайс
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Debugw("No subscriptions found for user ID", "userID", userID)
			return []models.Subscription{}, nil // Возвращаем пустой слайс
		}
		r.log.Errorw("Failed to get subscriptions by user ID from DB", "error", err, "userID", userID)
		return nil, fmt.Errorf("repository: failed to get subscriptions by user ID: %w", err)
	}

	r.log.Debugw("Successfully retrieved subscriptions by user ID", "userID", userID, "count", len(subs))
	return subs, nil
}

// Update обновляет данные существующей подписки в базе данных.
// Обновляет только изменяемые поля: status, updated_at, expires_at, canceled_at.
func (r *postgresSubscriptionRepo) Update(ctx context.Context, sub *models.Subscription) error {
	// Устанавливаем время обновления
	sub.UpdatedAt = time.Now()

	query := `
        UPDATE subscriptions SET
            status = :status,
            updated_at = :updated_at,
            expires_at = :expires_at,
            canceled_at = :canceled_at
            -- Не обновляем: subscription_id, user_id, plan_id, stripe_customer_id, created_at
        WHERE subscription_id = :subscription_id`

	result, err := r.db.NamedExecContext(ctx, query, sub)
	if err != nil {
		r.log.Errorw("Failed to update subscription in DB", "error", err, "subscriptionID", sub.SubscriptionID)
		return fmt.Errorf("repository: failed to update subscription: %w", err)
	}

	// Проверяем, была ли реально обновлена строка
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// Логируем, но не возвращаем как фатальную ошибку, обновление могло пройти
		r.log.Errorw("Failed to get rows affected after update", "error", err, "subscriptionID", sub.SubscriptionID)
	}
	if rowsAffected == 0 {
		r.log.Warnw("Subscription update affected 0 rows", "subscriptionID", sub.SubscriptionID)
		// Возможно, стоит вернуть ErrNotFound, если обновление несуществующей записи - ошибка
		// return ErrNotFound
	}

	r.log.Debugw("Successfully updated subscription in DB", "subscriptionID", sub.SubscriptionID, "rowsAffected", rowsAffected)
	return nil
}

// GetByStripeSubscriptionID возвращает подписку по ее Stripe ID.
// В нашей модели `SubscriptionID` и есть Stripe Subscription ID.
func (r *postgresSubscriptionRepo) GetByStripeSubscriptionID(ctx context.Context, stripeSubscriptionID string) (*models.Subscription, error) {
	// Этот метод идентичен GetByID, так как мы используем Stripe ID как основной ID.
	// Если бы у вас был отдельный столбец stripe_subscription_id, запрос был бы по нему.
	return r.GetByID(ctx, stripeSubscriptionID)

	/* Пример, если бы был отдельный столбец:
	   var sub models.Subscription
	   query := `SELECT ... FROM subscriptions WHERE stripe_subscription_id = $1`
	   err := r.db.GetContext(ctx, &sub, query, stripeSubscriptionID)
	   if err != nil {
	       if errors.Is(err, sql.ErrNoRows) {
	           r.log.Warnw("Subscription not found by Stripe ID", "stripeSubscriptionID", stripeSubscriptionID)
	           return nil, ErrNotFound
	       }
	       r.log.Errorw("Failed to get subscription by Stripe ID from DB", "error", err, "stripeSubscriptionID", stripeSubscriptionID)
	       return nil, fmt.Errorf("repository: failed to get subscription by Stripe ID: %w", err)
	   }
	   r.log.Debugw("Successfully retrieved subscription by Stripe ID", "stripeSubscriptionID", stripeSubscriptionID, "internalSubscriptionID", sub.SubscriptionID)
	   return &sub, nil
	*/
}
