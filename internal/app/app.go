package app

import (
	"github.com/Dhoini/Payment-microservice/internal/config"
	"github.com/Dhoini/Payment-microservice/internal/http/handlers"
	"github.com/Dhoini/Payment-microservice/internal/middleware"
	"github.com/Dhoini/Payment-microservice/internal/services"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
)

type App struct {
	Config           *config.Config
	PaymentService   *services.PaymentService
	PaymentHandler   *handlers.PaymentHandler
	WebhookHandler   *handlers.WebhookHandler
	AuthMiddleware   *middleware.JWTMiddleware
	LoggerMiddleware gin.HandlerFunc
	Logger           *logger.Logger
}

func NewApp(cfg *config.Config, paymentService *services.PaymentService, log *logger.Logger, validator middleware.TokenValidator) *App {
	paymentHandler := handlers.NewPaymentHandler(paymentService, log)

	webhookHandler, err := handlers.NewWebhookHandler(cfg, paymentService, log)
	if err != nil {
		log.Fatalw("Failed to initialize webhook handler", "error", err)
	}

	authMiddleware := middleware.NewJWTMiddleware(cfg, log, validator)

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
