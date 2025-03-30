package rest

import (
	"github.com/Dhoini/Payment-microservice/config"
	"github.com/Dhoini/Payment-microservice/internal/api/rest/handlers"
	"github.com/Dhoini/Payment-microservice/internal/api/rest/middleware"
	"github.com/Dhoini/Payment-microservice/internal/integration/stripe"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupRouter настраивает маршрутизатор Gin с маршрутами и middleware
func SetupRouter(log *logger.Logger, registry *prometheus.Registry, cfg *config.Config) *gin.Engine { // Инициализация Gin роутера
	r := gin.New()

	// Подключение middleware
	r.Use(middleware.LoggerMiddleware(log))
	r.Use(gin.Recovery())

	// Endpoint для проверки работоспособности сервиса
	r.GET("/health", handlers.HealthCheck)

	// Prometheus метрики
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(registry, promhttp.HandlerOpts{})))

	// Создание Stripe клиента
	stripeConfig := stripe.Config{
		APIKey:     cfg.Stripe.APIKey,
		WebhookKey: cfg.Stripe.WebhookSecret,
		IsTest:     cfg.Stripe.IsTest,
	}
	stripeClient := stripe.NewClient(stripeConfig, log)
	stripeWebhookHandler := stripe.NewWebhookHandler(stripeClient, log)

	// Инициализация обработчиков
	customerHandler := handlers.NewCustomerHandler(log)
	paymentHandler := handlers.NewPaymentHandler(log)
	webhookHandler := handlers.NewWebhookHandler(stripeWebhookHandler, paymentHandler.GetService(), customerHandler.GetService(), log)
	subscriptionHandler := handlers.NewSubscriptionHandler(log, customerHandler.GetService(), paymentHandler.GetService())

	// API для платежей
	v1 := r.Group("/api/v1")
	{
		// Клиенты
		customers := v1.Group("/customers")
		{
			customers.GET("", customerHandler.GetCustomers)
			customers.GET("/:id", customerHandler.GetCustomer)
			customers.POST("", customerHandler.CreateCustomer)
			customers.PUT("/:id", customerHandler.UpdateCustomer)
			customers.DELETE("/:id", customerHandler.DeleteCustomer)
		}

		// Платежи
		payments := v1.Group("/payments")
		{
			payments.GET("", paymentHandler.GetPayments)
			payments.GET("/:id", paymentHandler.GetPayment)
			payments.POST("", paymentHandler.CreatePayment)
			payments.PUT("/:id", paymentHandler.UpdatePayment)
		}

		// Подписки
		subscriptions := v1.Group("/subscriptions")
		{
			subscriptions.GET("", subscriptionHandler.GetSubscriptions)
			subscriptions.GET("/:id", subscriptionHandler.GetSubscription)
			subscriptions.POST("", subscriptionHandler.CreateSubscription)
			subscriptions.POST("/:id/cancel", subscriptionHandler.CancelSubscription)
			subscriptions.POST("/:id/pause", subscriptionHandler.PauseSubscription)
			subscriptions.POST("/:id/resume", subscriptionHandler.ResumeSubscription)
		}
		// Планы подписок
		plans := v1.Group("/subscription-plans")
		{
			plans.GET("", subscriptionHandler.GetPlans)
			plans.GET("/:id", subscriptionHandler.GetPlan)
			plans.POST("", subscriptionHandler.CreatePlan)
			plans.PUT("/:id", subscriptionHandler.UpdatePlan)
			plans.DELETE("/:id", subscriptionHandler.DeletePlan)
		}

	}

	// Вебхуки на корневом уровне роутера
	webhooks := r.Group("/webhooks")
	{
		webhooks.POST("/stripe", webhookHandler.HandleStripeWebhook)
		// Здесь в будущем можно добавить другие маршруты (подписки и т.д.)
	}
	return r
}
