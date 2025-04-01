package app

import (
	"github.com/Dhoini/Payment-microservice/internal/config"
	"github.com/Dhoini/Payment-microservice/internal/http/handlers"
	"github.com/Dhoini/Payment-microservice/internal/middleware"
	"github.com/Dhoini/Payment-microservice/internal/services"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
)

// App представляет собой контейнер для всех компонентов приложения
type App struct {
	Config           *config.Config
	PaymentService   *services.PaymentService
	PaymentHandler   *handlers.PaymentHandler
	WebhookHandler   *handlers.WebhookHandler
	AuthMiddleware   *middleware.AuthMiddleware
	LoggerMiddleware gin.HandlerFunc
	Logger           *logger.Logger
}

// NewApp создает и инициализирует новый экземпляр приложения
func NewApp(cfg *config.Config, paymentService *services.PaymentService, log *logger.Logger) *App {
	// Инициализируем обработчики HTTP
	paymentHandler := handlers.NewPaymentHandler(paymentService, log)

	// Инициализируем обработчик вебхуков
	webhookHandler, err := handlers.NewWebhookHandler(cfg, paymentService, log)
	if err != nil {
		log.Fatalw("Failed to initialize webhook handler", "error", err)
	}

	// Инициализируем middleware аутентификации
	authMiddleware, err := middleware.NewAuthMiddleware(cfg, log)
	if err != nil {
		log.Fatalw("Failed to initialize auth middleware", "error", err)
	}

	// Инициализируем middleware логирования
	loggerMiddleware := middleware.RequestLogger(log)

	return &App{
		Config:           cfg,
		PaymentService:   paymentService,
		PaymentHandler:   paymentHandler,
		WebhookHandler:   webhookHandler,
		AuthMiddleware:   authMiddleware,
		LoggerMiddleware: loggerMiddleware,
		Logger:           log,
	}
}
