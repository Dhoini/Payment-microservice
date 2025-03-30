package main

import (
	"context"
	"github.com/Dhoini/Payment-microservice/config"
	"github.com/Dhoini/Payment-microservice/internal/api/rest/middleware"
	"github.com/Dhoini/Payment-microservice/internal/kafka"
	"github.com/Dhoini/Payment-microservice/internal/kafka/producer"
	"github.com/Dhoini/Payment-microservice/internal/metrics"
	"github.com/Dhoini/Payment-microservice/internal/repository/postgres"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var log *logger.Logger

func init() {
	// Загружаем переменные окружения
	if err := godotenv.Load(); err != nil {
		// Пропускаем ошибку, если .env файл не найден
	}

	// Инициализация логгера
	logLevel := logger.INFO
	if os.Getenv("DEBUG") == "true" {
		logLevel = logger.DEBUG
	}
	log = logger.New(logLevel)
}

func main() {
	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load configuration: %v", err)
	}

	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализация Prometheus
	promRegistry := prometheus.NewRegistry()
	paymentMetrics := metrics.NewPaymentMetrics(promRegistry, log)
	systemMetrics := metrics.NewSystemMetrics(promRegistry, log)

	// Запускаем сбор системных метрик
	systemMetrics.StartRecording(15 * time.Second)
	defer systemMetrics.Stop()

	// Подключение к базе данных
	dbPool, err := postgres.NewConnection(ctx, cfg.Database.GetDSN(), log)
	if err != nil {
		log.Fatal("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	// Инициализация Kafka продюсера
	kafkaConfig := kafka.NewConfig([]string{"kafka:9092"})
	saramaConfig := kafka.NewSaramaConfig(kafkaConfig, log)

	kafkaProducer, err := sarama.NewSyncProducer(kafkaConfig.Brokers, saramaConfig)
	if err != nil {
		log.Fatal("Failed to create Kafka producer: %v", err)
	}
	defer kafkaProducer.Close()

	paymentProducer := producer.NewKafkaPaymentProducer(kafkaProducer, log)

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

	// Prometheus метрики
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{})))

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
			customers.DELETE("/:id", customerHandler.DeleteCustomer)
		}

		// В будущем можно добавить другие API
	}

	// Создание HTTP сервера
	port := cfg.Server.Port
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
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

	shutdownTimeout := time.Duration(cfg.Server.ShutdownTimeout) * time.Second
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatal("Server forced to shutdown: %v", err)
	}

	log.Info("Server stopped gracefully")
}

// HealthCheck обработчик для проверки работоспособности сервиса
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "OK",
		"time":   time.Now().Format(time.RFC3339),
	})
}
