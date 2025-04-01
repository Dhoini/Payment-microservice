package repository

import (
	"context"
	"github.com/Dhoini/Payment-microservice/internal/models"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
)

// CachedSubscriptionRepository реализует SubscriptionRepository с кешированием
type CachedSubscriptionRepository struct {
	repo  SubscriptionRepository
	cache *RedisCacheRepository
	log   *logger.Logger
}

// NewCachedSubscriptionRepository создает новый репозиторий с кешированием
func NewCachedSubscriptionRepository(
	repo SubscriptionRepository,
	cache *RedisCacheRepository,
	log *logger.Logger,
) SubscriptionRepository {
	return &CachedSubscriptionRepository{
		repo:  repo,
		cache: cache,
		log:   log,
	}
}

// Create сохраняет подписку в БД и кеширует ее
func (r *CachedSubscriptionRepository) Create(ctx context.Context, sub *models.Subscription) error {
	// Сначала сохраняем в основное хранилище
	if err := r.repo.Create(ctx, sub); err != nil {
		return err
	}

	// Затем кешируем подписку
	if err := r.cache.CacheSubscription(ctx, sub); err != nil {
		r.log.Warnw("Failed to cache subscription after creation", "error", err, "subscriptionID", sub.SubscriptionID)
		// Продолжаем выполнение, несмотря на ошибку кеширования
	}

	// Инвалидируем кеш списка подписок пользователя
	if err := r.cache.InvalidateUserSubscriptionsCache(ctx, sub.UserID); err != nil {
		r.log.Warnw("Failed to invalidate user subscriptions cache", "error", err, "userID", sub.UserID)
	}

	return nil
}

// GetByID получает подписку по ID (сначала из кеша, потом из БД)
func (r *CachedSubscriptionRepository) GetByID(ctx context.Context, subscriptionID string) (*models.Subscription, error) {
	// Пытаемся получить из кеша
	cachedSub, err := r.cache.GetCachedSubscription(ctx, subscriptionID)
	if err != nil {
		r.log.Warnw("Error getting subscription from cache", "error", err, "subscriptionID", subscriptionID)
		// Продолжаем выполнение при ошибке кеша
	}

	// Если нашли в кеше, возвращаем
	if cachedSub != nil {
		r.log.Debugw("Subscription found in cache", "subscriptionID", subscriptionID)
		return cachedSub, nil
	}

	// Если не нашли в кеше, ищем в БД
	sub, err := r.repo.GetByID(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	// Кешируем найденную подписку
	if sub != nil {
		if err := r.cache.CacheSubscription(ctx, sub); err != nil {
			r.log.Warnw("Failed to cache subscription after fetching", "error", err, "subscriptionID", subscriptionID)
		}
	}

	return sub, nil
}

// GetByUserID возвращает подписки пользователя (сначала из кеша, потом из БД)
func (r *CachedSubscriptionRepository) GetByUserID(ctx context.Context, userID string) ([]models.Subscription, error) {
	// Пытаемся получить из кеша
	cachedSubs, err := r.cache.GetCachedUserSubscriptions(ctx, userID)
	if err != nil {
		r.log.Warnw("Error getting user subscriptions from cache", "error", err, "userID", userID)
		// Продолжаем выполнение при ошибке кеша
	}

	// Если нашли в кеше, возвращаем
	if cachedSubs != nil && len(cachedSubs) > 0 {
		r.log.Debugw("User subscriptions found in cache", "userID", userID, "count", len(cachedSubs))
		return cachedSubs, nil
	}

	// Если не нашли в кеше, ищем в БД
	subs, err := r.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Кешируем найденные подписки
	if len(subs) > 0 {
		if err := r.cache.CacheUserSubscriptions(ctx, userID, subs); err != nil {
			r.log.Warnw("Failed to cache user subscriptions", "error", err, "userID", userID)
		}
	}

	return subs, nil
}

// Update обновляет подписку в БД и кеше
func (r *CachedSubscriptionRepository) Update(ctx context.Context, sub *models.Subscription) error {
	// Сначала обновляем в основном хранилище
	if err := r.repo.Update(ctx, sub); err != nil {
		return err
	}

	// Обновляем кеш подписки
	if err := r.cache.CacheSubscription(ctx, sub); err != nil {
		r.log.Warnw("Failed to update subscription in cache", "error", err, "subscriptionID", sub.SubscriptionID)
	}

	// Инвалидируем кеш списка подписок пользователя
	if err := r.cache.InvalidateUserSubscriptionsCache(ctx, sub.UserID); err != nil {
		r.log.Warnw("Failed to invalidate user subscriptions cache after update", "error", err, "userID", sub.UserID)
	}

	return nil
}

// GetByStripeSubscriptionID получает подписку по Stripe ID (прокси к GetByID)
func (r *CachedSubscriptionRepository) GetByStripeSubscriptionID(ctx context.Context, stripeSubscriptionID string) (*models.Subscription, error) {
	// В нашей реализации это то же самое, что GetByID
	return r.GetByID(ctx, stripeSubscriptionID)
}
