package routes

import (
	"github.com/Dhoini/Payment-microservice/internal/app"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
)

// SetupRoutes настраивает все маршруты API для Gin роутера
func SetupRoutes(router *gin.Engine, app *app.App, log *logger.Logger) {
	// Промежуточное ПО для всех запросов
	router.Use(app.LoggerMiddleware)
	router.Use(gin.Recovery())

	// Группа API
	api := router.Group("/api/v1")
	{
		// Публичные маршруты (без аутентификации)
		// Обработчик вебхуков Stripe
		api.POST("/webhooks/stripe", app.WebhookHandler.HandleStripeWebhook)

		// Здоровье сервиса
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// Защищенные маршруты (требуют аутентификации)
		auth := api.Group("")
		auth.Use(app.AuthMiddleware.RequireAuth())

		// Подписки
		subscriptions := auth.Group("/subscriptions")
		{
			// Создать новую подписку
			subscriptions.POST("", app.PaymentHandler.CreateSubscription)

			// Получить подписку по ID
			subscriptions.GET("/:subscription_id", app.PaymentHandler.GetSubscription)

			// Отменить подписку
			subscriptions.DELETE("/:subscription_id", app.PaymentHandler.CancelSubscription)
		}

		// Подписки пользователя
		users := auth.Group("/users")
		{
			// Получить все подписки пользователя
			users.GET("/:user_id/subscriptions", app.PaymentHandler.GetUserSubscriptions)
		}
	}

	log.Infow("API routes successfully configured")
}
