package repository

import (
	"context"
	"github.com/Dhoini/Payment-microservice/internal/models" // Убедитесь, что путь верный
)

// SubscriptionRepository определяет методы для работы с хранилищем подписок.
type SubscriptionRepository interface {
	// Create сохраняет новую подписку в хранилище.
	Create(ctx context.Context, sub *models.Subscription) error

	// GetByID возвращает подписку по ее ID.
	GetByID(ctx context.Context, subscriptionID string) (*models.Subscription, error)

	// GetByUserID возвращает все активные подписки пользователя.
	GetByUserID(ctx context.Context, userID string) ([]models.Subscription, error)

	// Update обновляет данные существующей подписки (например, статус или время отмены).
	Update(ctx context.Context, sub *models.Subscription) error

	// GetByStripeSubscriptionID возвращает подписку по её Stripe ID. (понадобится для вебхуков)
	GetByStripeSubscriptionID(ctx context.Context, stripeSubscriptionID string) (*models.Subscription, error)

	// Возможно, понадобятся другие методы, например:
	// FindActiveByUserID(ctx context.Context, userID string) (*models.Subscription, error)
	// Delete(ctx context.Context, subscriptionID string) error
}
