package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dhoini/Payment-microservice/config"
	"github.com/Dhoini/Payment-microservice/internal/api/rest"
	"github.com/Dhoini/Payment-microservice/internal/kafka"
	"github.com/Dhoini/Payment-microservice/internal/kafka/producer"
	"github.com/Dhoini/Payment-microservice/internal/metrics"
	"github.com/Dhoini/Payment-microservice/internal/repository/postgres"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
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

	// Настройка маршрутизатора
	router := rest.SetupRouter(log, promRegistry)

	// Создание и запуск HTTP сервера
	server := rest.NewServer(router, cfg, log)

	// Запуск сервера в горутине
	go func() {
		if err := server.Start(); err != nil {
			log.Fatal("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Останавливаем сервер
	shutdownTimeout := time.Duration(cfg.Server.ShutdownTimeout) * time.Second
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatal("Server forced to shutdown: %v", err)
	}

	log.Info("Server stopped gracefully")
}
