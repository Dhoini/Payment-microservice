// cmd/server/main.go
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/handler"
	"github.com/Dhoini/Payment-microservice/internal/middleware"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var log *logger.Logger

func init() {
	// Инициализация логгера
	logLevel := logger.INFO
	if os.Getenv("DEBUG") == "true" {
		logLevel = logger.DEBUG
	}
	log = logger.New(logLevel)
}

func main() {
	// Загрузка переменных окружения
	if err := godotenv.Load("./configs/.env"); err != nil {
		log.Warn("No .env file found, using system environment variables")
	}

	// Установка режима Gin
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Инициализация Gin роутера - отключаем встроенный логгер
	r := gin.New()

	// Подключение middleware
	r.Use(middleware.LoggerMiddleware(log))
	r.Use(gin.Recovery())

	// Регистрация маршрутов
	r.GET("/health", healthCheck)

	// API для платежей
	v1 := r.Group("/api/v1")
	{
		// Платежи
		payments := v1.Group("/payments")
		{
			paymentHandler := handler.NewPaymentHandler(log)
			payments.GET("", paymentHandler.GetPayments)
			payments.GET("/:id", paymentHandler.GetPayment)
			payments.POST("", paymentHandler.CreatePayment)
			payments.PUT("/:id", paymentHandler.UpdatePayment)
		}

		// Клиенты
		customers := v1.Group("/customers")
		{
			customerHandler := handler.NewCustomerHandler(log)
			customers.GET("", customerHandler.GetCustomers)
			customers.GET("/:id", customerHandler.GetCustomer)
			customers.POST("", customerHandler.CreateCustomer)
			customers.PUT("/:id", customerHandler.UpdateCustomer)
		}

		// Подписки
		subscriptions := v1.Group("/subscriptions")
		{
			subscriptionHandler := handler.NewSubscriptionHandler(log)
			subscriptions.GET("", subscriptionHandler.GetSubscriptions)
			subscriptions.GET("/:id", subscriptionHandler.GetSubscription)
			subscriptions.POST("", subscriptionHandler.CreateSubscription)
			subscriptions.PUT("/:id", subscriptionHandler.UpdateSubscription)
			subscriptions.DELETE("/:id", subscriptionHandler.CancelSubscription)
		}
	}

	// Создание HTTP сервера
	port := getEnv("PORT", "8080")
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Запуск сервера в горутине
	log.Info("Starting server on port %s", port)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Server is shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: %v", err)
	}

	log.Info("Server stopped gracefully")
}

// HealthCheck обработчик для проверки работоспособности сервиса
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "OK",
	})
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
