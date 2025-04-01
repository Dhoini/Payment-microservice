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
	"github.com/Dhoini/Payment-microservice/internal/interceptors" // <-- Импорт пакета интерцепторов
	"github.com/Dhoini/Payment-microservice/internal/kafka"
	"github.com/Dhoini/Payment-microservice/internal/middleware" // <-- Импорт для валидатора и ключа
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/internal/services"
	"github.com/Dhoini/Payment-microservice/internal/stripe"
	"github.com/Dhoini/Payment-microservice/pkg/logger"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection" // Для дебаггинга gRPC через grpcurl/Evans
)

func main() {
	// Инициализируем контекст с возможностью отмены для graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализируем логгер
	log := initLogger()

	log.Infow("Payment microservice starting up...")

	// Загружаем конфигурацию
	cfg, err := config.LoadConfig("config.yml") // или .env, если используете его
	if err != nil {
		log.Fatalw("Failed to load configuration", "error", err)
	}
	// Проверка наличия секрета JWT
	if cfg.Auth.JWTSecret == "" || cfg.Auth.JWTSecret == "YourVerySecretKeyHere" {
		log.Warnw("JWT Secret is not set or is using the default placeholder!")
	}
	// Проверка наличия ключей Stripe
	if cfg.Stripe.APIKey == "" || cfg.Stripe.APIKey == "sk_test_YourSecretKeyHere" {
		log.Warnw("Stripe API Key is not set or is using the default placeholder!")
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
		// Не фатально, но предупреждаем
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
	baseRepo := repository.NewPostgresSubscriptionRepository(dbClient.DB(), log) // Передаем DBClient

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
		// Можно сделать не фатальным, если отправка событий не критична для основного флоу
		log.Errorw("Failed to initialize Kafka producer, continuing without event publishing", "error", err)
		// kafkaProducer = &kafka.NoOpProducer{} // Заглушка, если нужно
	} else {
		log.Infow("Kafka producer initialized")
		defer func() {
			if err := kafkaProducer.Close(); err != nil {
				log.Errorw("Error closing Kafka producer", "error", err)
			}
		}()
	}

	// Инициализируем service layer
	paymentService := services.NewPaymentService(cfg, subscriptionRepo, stripeClient, kafkaProducer, log)

	// Инициализируем application (для HTTP)
	// Создаем валидатор токенов
	validator := &middleware.DefaultTokenValidator{
		Secret: []byte(cfg.Auth.JWTSecret),
	}
	application := app.NewApp(cfg, paymentService, log, validator) // Передаем валидатор

	// Инициализируем HTTP сервер с роутами
	router := gin.New() // Используем gin.New() для большего контроля над middleware
	// Добавляем middleware логирования и восстановления Gin
	router.Use(application.LoggerMiddleware) // Логгер запросов
	router.Use(gin.Recovery())               // Восстановление после паник
	// Настраиваем маршруты
	routes.SetupRoutes(router, application, log) // Передаем application

	httpServer := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Запускаем HTTP сервер в горутине
	go func() {
		log.Infow("Starting HTTP server", "port", cfg.App.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalw("Failed to start HTTP server", "error", err)
		}
	}()

	// --- Настройка gRPC сервера ---
	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatalw("Failed to listen for gRPC", "error", err)
	}

	// Создаем интерцептор аутентификации
	authInterceptor := interceptors.NewAuthInterceptor(log, validator)

	// Настраиваем логирование для gRPC (пример)
	// loggerOpts := []grpcMw.Option{
	// 	grpcMw.WithLogOnEvents(grpcMw.StartCall, grpcMw.FinishCall),
	// }

	// Создаем gRPC сервер с интерцепторами
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			// grpcMw.UnaryServerInterceptor(interceptorLogger(log), loggerOpts...), // Пример интерцептора логирования
			authInterceptor.Unary(), // <-- Наш интерцептор аутентификации
			// Добавьте другие интерцепторы здесь, если нужно
		),
		// grpc.StreamInterceptor(...) // Для потоковых интерцепторов
	)

	// Регистрируем сервис
	paymentServer := paymentgrpc.NewPaymentServer(paymentService, log)
	paymentgrpc.RegisterPaymentServiceServer(grpcServer, paymentServer)

	// Включаем gRPC Reflection для дебаггинга (удобно с grpcurl/Evans)
	// Отключите в production, если не требуется
	reflection.Register(grpcServer)
	log.Infow("gRPC reflection service registered")

	// Запускаем gRPC сервер в горутине
	go func() {
		log.Infow("Starting gRPC server", "port", cfg.GRPC.Port)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalw("Failed to start gRPC server", "error", err)
		}
	}()

	// --- Graceful Shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Infow("Shutdown signal received")

	// Даем 10 секунд на завершение текущих запросов
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Останавливаем HTTP сервер
	log.Infow("Shutting down HTTP server")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Errorw("HTTP server shutdown error", "error", err)
	} else {
		log.Infow("HTTP server gracefully stopped")
	}

	// Останавливаем gRPC сервер
	log.Infow("Shutting down gRPC server")
	grpcServer.GracefulStop() // GracefulStop ждет завершения текущих RPC
	log.Infow("gRPC server gracefully stopped")

	log.Infow("Cleanup finished. Goodbye!")
}

// initLogger инициализирует новый логгер (без изменений)
func initLogger() *logger.Logger {
	logLevel := logger.INFO
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = logger.DEBUG
	}
	return logger.New(logLevel)
}

/*
// Пример адаптера для grpc-ecosystem/go-grpc-middleware/logging
// (Требует установки пакета: go get github.com/grpc-ecosystem/go-grpc-middleware/v2)
func interceptorLogger(l *logger.Logger) grpcMw.Logger {
	return grpcMw.LoggerFunc(func(ctx context.Context, level grpcMw.Level, msg string, fields ...any) {
		logFields := make([]interface{}, 0, len(fields))
		for i := 0; i < len(fields); i += 2 {
			// Преобразуем в формат ключ-значение, как ожидает наш логгер
			if i+1 < len(fields) {
				logFields = append(logFields, fields[i], fields[i+1])
			} else {
				logFields = append(logFields, fields[i], "INVALID_FIELD") // Обработка нечетного числа полей
			}
		}

		switch level {
		case grpcMw.LevelDebug:
			l.Debugw(msg, logFields...)
		case grpcMw.LevelInfo:
			l.Infow(msg, logFields...)
		case grpcMw.LevelWarn:
			l.Warnw(msg, logFields...)
		case grpcMw.LevelError:
			l.Errorw(msg, logFields...)
		default:
			l.Errorw("Unknown log level", "level", level, "message", msg)
		}
	})
}
*/
