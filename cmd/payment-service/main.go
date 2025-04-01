package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/app"
	"github.com/Dhoini/Payment-microservice/internal/config"
	"github.com/Dhoini/Payment-microservice/internal/db"
	paymentgrpc "github.com/Dhoini/Payment-microservice/internal/grpc"
	"github.com/Dhoini/Payment-microservice/internal/http/routes"
	"github.com/Dhoini/Payment-microservice/internal/kafka"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/internal/services"
	"github.com/Dhoini/Payment-microservice/internal/stripe"
	"github.com/Dhoini/Payment-microservice/pkg/logger"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func main() {
	// Инициализируем контекст с возможностью отмены для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализируем логгер
	log := initLogger()

	log.Infow("Payment microservice starting up...")

	// Загружаем конфигурацию
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		log.Fatalw("Failed to load configuration", "error", err)
	}

	// Устанавливаем режим Gin в зависимости от окружения
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Подключаемся к базе данных
	dbClient, err := db.NewDBClient(cfg.Database.DSN, log)
	if err != nil {
		log.Fatalw("Failed to connect to database", "error", err)
	}
	defer func() {
		if err := dbClient.Close(); err != nil {
			log.Errorw("Error closing database connection", "error", err)
		}
	}()
	log.Infow("Database connection established")

	// Инициализируем Redis кеш
	redisCache, err := repository.NewRedisCacheRepository(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		log,
	)
	if err != nil {
		log.Warnw("Failed to initialize Redis cache, continuing without caching", "error", err)
	} else {
		log.Infow("Redis cache initialized successfully")
		defer func() {
			if err := redisCache.Close(); err != nil {
				log.Errorw("Error closing Redis connection", "error", err)
			}
		}()
	}

	// Инициализируем базовый репозиторий
	baseRepo := repository.NewPostgresSubscriptionRepository(dbClient.DB, log)

	// Создаем репозиторий с кешированием, если Redis доступен
	var subscriptionRepo repository.SubscriptionRepository
	if redisCache != nil {
		subscriptionRepo = repository.NewCachedSubscriptionRepository(baseRepo, redisCache, log)
		log.Infow("Using cached subscription repository")
	} else {
		subscriptionRepo = baseRepo
		log.Infow("Using non-cached subscription repository")
	}

	// Инициализируем клиент Stripe
	stripeClient := stripe.NewStripeClient(cfg.Stripe.APIKey, log)

	// Инициализируем Kafka Producer
	kafkaProducer, err := kafka.NewKafkaProducer(cfg.Kafka.Brokers, log)
	if err != nil {
		log.Fatalw("Failed to initialize Kafka producer", "error", err)
	}
	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			log.Errorw("Error closing Kafka producer", "error", err)
		}
	}()
	log.Infow("Kafka producer initialized")

	// Инициализируем service layer
	paymentService := services.NewPaymentService(cfg, subscriptionRepo, stripeClient, kafkaProducer, log)

	// Инициализируем application
	application := app.NewApp(cfg, paymentService, log)

	// Инициализируем HTTP сервер с роутами
	router := gin.New()
	routes.SetupRoutes(router, application, log)
	httpServer := &http.Server{
		Addr:    ":" + cfg.App.Port,
		Handler: router,
	}

	// Запускаем HTTP сервер в горутине
	go func() {
		log.Infow("Starting HTTP server", "port", cfg.App.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalw("Failed to start HTTP server", "error", err)
		}
	}()

	// Инициализируем и запускаем gRPC сервер в горутине
	grpcServer := grpc.NewServer()
	paymentServer := paymentgrpc.NewPaymentServer(paymentService, log)
	paymentgrpc.RegisterPaymentServiceServer(grpcServer, paymentServer)

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatalw("Failed to listen for gRPC", "error", err)
	}

	go func() {
		log.Infow("Starting gRPC server", "port", cfg.GRPC.Port)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalw("Failed to start gRPC server", "error", err)
		}
	}()

	// Ожидаем сигналы для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Infow("Shutdown signal received")

	// Даем 10 секунд на завершение текущих запросов
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	// Останавливаем HTTP сервер
	log.Infow("Shutting down HTTP server")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Errorw("HTTP server shutdown error", "error", err)
	}

	// Останавливаем gRPC сервер
	log.Infow("Shutting down gRPC server")
	grpcServer.GracefulStop()

	log.Infow("All servers stopped. Goodbye!")
}

// initLogger инициализирует новый логгер
func initLogger() *logger.Logger {
	// Определяем уровень логирования на основе переменной окружения
	logLevel := logger.INFO
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = logger.DEBUG
	}

	// Создаем новый логгер
	return logger.New(logLevel)
}
