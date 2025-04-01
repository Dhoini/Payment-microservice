package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/models"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/redis/go-redis/v9"
)

const (
	// Префиксы ключей для различных типов данных
	subscriptionKeyPrefix      = "subscription:"
	userSubscriptionsKeyPrefix = "user_subscriptions:"

	// TTL для кэша
	defaultCacheTTL = 15 * time.Minute
)

// RedisCacheRepository реализует кеширование для репозиториев с использованием Redis
type RedisCacheRepository struct {
	client *redis.Client
	log    *logger.Logger
}

// NewRedisCacheRepository создает новый экземпляр Redis репозитория
func NewRedisCacheRepository(redisAddr, redisPassword string, redisDB int, log *logger.Logger) (*RedisCacheRepository, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Проверяем соединение с Redis
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Errorw("Failed to connect to Redis", "error", err)
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Infow("Connected to Redis successfully", "addr", redisAddr)
	return &RedisCacheRepository{
		client: client,
		log:    log,
	}, nil
}

// Close закрывает соединение с Redis
func (r *RedisCacheRepository) Close() error {
	return r.client.Close()
}

// CacheSubscription кеширует подписку в Redis
func (r *RedisCacheRepository) CacheSubscription(ctx context.Context, sub *models.Subscription) error {
	key := fmt.Sprintf("%s%s", subscriptionKeyPrefix, sub.SubscriptionID)

	data, err := json.Marshal(sub)
	if err != nil {
		r.log.Errorw("Failed to marshal subscription for caching", "error", err, "subscriptionID", sub.SubscriptionID)
		return fmt.Errorf("failed to marshal subscription: %w", err)
	}

	if err := r.client.Set(ctx, key, data, defaultCacheTTL).Err(); err != nil {
		r.log.Errorw("Failed to cache subscription in Redis", "error", err, "subscriptionID", sub.SubscriptionID)
		return fmt.Errorf("failed to cache subscription: %w", err)
	}

	r.log.Debugw("Subscription cached successfully", "subscriptionID", sub.SubscriptionID)
	return nil
}

// GetCachedSubscription получает подписку из кеша
func (r *RedisCacheRepository) GetCachedSubscription(ctx context.Context, subscriptionID string) (*models.Subscription, error) {
	key := fmt.Sprintf("%s%s", subscriptionKeyPrefix, subscriptionID)

	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Ключ не найден в кеше
			r.log.Debugw("Subscription not found in cache", "subscriptionID", subscriptionID)
			return nil, nil // Возвращаем nil вместо ошибки
		}
		r.log.Errorw("Error getting subscription from Redis", "error", err, "subscriptionID", subscriptionID)
		return nil, fmt.Errorf("failed to get subscription from cache: %w", err)
	}

	var sub models.Subscription
	if err := json.Unmarshal(data, &sub); err != nil {
		r.log.Errorw("Failed to unmarshal cached subscription", "error", err, "subscriptionID", subscriptionID)
		return nil, fmt.Errorf("failed to unmarshal cached subscription: %w", err)
	}

	r.log.Debugw("Subscription retrieved from cache", "subscriptionID", subscriptionID)
	return &sub, nil
}

// DeleteCachedSubscription удаляет подписку из кеша
func (r *RedisCacheRepository) DeleteCachedSubscription(ctx context.Context, subscriptionID string) error {
	key := fmt.Sprintf("%s%s", subscriptionKeyPrefix, subscriptionID)

	if err := r.client.Del(ctx, key).Err(); err != nil {
		r.log.Errorw("Failed to delete subscription from cache", "error", err, "subscriptionID", subscriptionID)
		return fmt.Errorf("failed to delete subscription from cache: %w", err)
	}

	r.log.Debugw("Subscription deleted from cache", "subscriptionID", subscriptionID)
	return nil
}

// CacheUserSubscriptions кеширует список подписок пользователя
func (r *RedisCacheRepository) CacheUserSubscriptions(ctx context.Context, userID string, subs []models.Subscription) error {
	key := fmt.Sprintf("%s%s", userSubscriptionsKeyPrefix, userID)

	data, err := json.Marshal(subs)
	if err != nil {
		r.log.Errorw("Failed to marshal user subscriptions for caching", "error", err, "userID", userID)
		return fmt.Errorf("failed to marshal user subscriptions: %w", err)
	}

	if err := r.client.Set(ctx, key, data, defaultCacheTTL).Err(); err != nil {
		r.log.Errorw("Failed to cache user subscriptions in Redis", "error", err, "userID", userID)
		return fmt.Errorf("failed to cache user subscriptions: %w", err)
	}

	r.log.Debugw("User subscriptions cached successfully", "userID", userID, "count", len(subs))
	return nil
}

// GetCachedUserSubscriptions получает список подписок пользователя из кеша
func (r *RedisCacheRepository) GetCachedUserSubscriptions(ctx context.Context, userID string) ([]models.Subscription, error) {
	key := fmt.Sprintf("%s%s", userSubscriptionsKeyPrefix, userID)

	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Ключ не найден в кеше
			r.log.Debugw("User subscriptions not found in cache", "userID", userID)
			return nil, nil // Возвращаем nil вместо ошибки
		}
		r.log.Errorw("Error getting user subscriptions from Redis", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to get user subscriptions from cache: %w", err)
	}

	var subs []models.Subscription
	if err := json.Unmarshal(data, &subs); err != nil {
		r.log.Errorw("Failed to unmarshal cached user subscriptions", "error", err, "userID", userID)
		return nil, fmt.Errorf("failed to unmarshal cached user subscriptions: %w", err)
	}

	r.log.Debugw("User subscriptions retrieved from cache", "userID", userID, "count", len(subs))
	return subs, nil
}

// InvalidateUserSubscriptionsCache удаляет кеш подписок пользователя
func (r *RedisCacheRepository) InvalidateUserSubscriptionsCache(ctx context.Context, userID string) error {
	key := fmt.Sprintf("%s%s", userSubscriptionsKeyPrefix, userID)

	if err := r.client.Del(ctx, key).Err(); err != nil {
		r.log.Errorw("Failed to invalidate user subscriptions cache", "error", err, "userID", userID)
		return fmt.Errorf("failed to invalidate user subscriptions cache: %w", err)
	}

	r.log.Debugw("User subscriptions cache invalidated", "userID", userID)
	return nil
}
