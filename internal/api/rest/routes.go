package rest

import (
	"github.com/Dhoini/Payment-microservice/internal/api/rest/handlers"
	"github.com/Dhoini/Payment-microservice/internal/api/rest/middleware"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupRouter настраивает маршрутизатор Gin с маршрутами и middleware
func SetupRouter(log *logger.Logger, registry *prometheus.Registry) *gin.Engine {
	// Инициализация Gin роутера
	r := gin.New()

	// Подключение middleware
	r.Use(middleware.LoggerMiddleware(log))
	r.Use(gin.Recovery())

	// Endpoint для проверки работоспособности сервиса
	r.GET("/health", handlers.HealthCheck)

	// Prometheus метрики
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(registry, promhttp.HandlerOpts{})))

	// Инициализация обработчиков
	customerHandler := handlers.NewCustomerHandler(log)
	paymentHandler := handlers.NewPaymentHandler(log)

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

		// Здесь в будущем можно добавить другие маршруты (подписки и т.д.)
	}

	return r
}
