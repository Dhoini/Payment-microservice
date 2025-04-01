package handlers

import (
	"errors"
	"github.com/Dhoini/Payment-microservice/internal/middleware"
	"net/http"
	"time"

	"github.com/gin-gonic/gin" // <-- Используем Gin

	"github.com/Dhoini/Payment-microservice/internal/services"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/Dhoini/Payment-microservice/pkg/req"
	"github.com/Dhoini/Payment-microservice/pkg/res"
)

// PaymentHandler обрабатывает HTTP запросы, связанные с подписками (для Gin).
type PaymentHandler struct {
	service *services.PaymentService
	log     *logger.Logger
}

// NewPaymentHandler создает новый экземпляр PaymentHandler.
func NewPaymentHandler(service *services.PaymentService, log *logger.Logger) *PaymentHandler {
	return &PaymentHandler{
		service: service,
		log:     log,
	}
}

// --- DTO (без изменений) ---
type CreateSubscriptionRequest struct {
	UserID    string `json:"user_id" validate:"required"`
	PlanID    string `json:"plan_id" validate:"required"`
	UserEmail string `json:"user_email" validate:"required,email"`
}

type CreateSubscriptionResponse struct {
	SubscriptionID string `json:"subscription_id"`
	Status         string `json:"status"`
	ClientSecret   string `json:"client_secret,omitempty"`
	CreatedAt      string `json:"created_at"`
}

// --- Обработчики (адаптированы под Gin) ---

// CreateSubscription обрабатывает POST /subscriptions
func (h *PaymentHandler) CreateSubscription(c *gin.Context) { // <-- Сигнатура Gin
	ctx := c.Request.Context() // Получаем контекст из запроса Gin
	h.log.Infow("CreateSubscription handler started")

	idempotencyKey := c.GetHeader("Idempotency-Key") // <-- Получаем заголовок через Gin

	// Шаг 1: Декодируем тело запроса
	// Используем c.Request.Body, который является io.ReadCloser
	requestBody, err := req.Decode[CreateSubscriptionRequest](c.Request.Body)
	if err != nil {
		h.log.Errorw("Failed to decode request body", "error", err)
		// Используем pkg/res, передавая c.Writer
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Invalid request format"}, http.StatusUnprocessableEntity)
		c.Abort() // Останавливаем обработку в Gin
		return
	}

	// Шаг 2: Валидируем тело запроса
	err = req.IsValid(requestBody)
	if err != nil {
		h.log.Errorw("Request body validation failed", "error", err)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Invalid request data", Details: err.Error()}, http.StatusUnprocessableEntity)
		c.Abort()
		return
	}

	// TODO: Получить UserID из контекста Gin (после middleware аутентификации)
	// userIDFromToken, exists := c.Get("userID")
	// if !exists { /* обработать ошибку */ }
	// if requestBody.UserID != userIDFromToken.(string) { ... }

	input := services.CreateSubscriptionInput{
		UserID:         requestBody.UserID, // Пока берем из тела
		PlanID:         requestBody.PlanID,
		UserEmail:      requestBody.UserEmail,
		IdempotencyKey: idempotencyKey,
	}

	output, err := h.service.CreateSubscription(ctx, input)
	if err != nil {
		h.log.Errorw("Service failed to create subscription", "error", err)
		if errors.Is(err, services.ErrPaymentFailed) {
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Payment processing failed"}, http.StatusConflict)
		} else {
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Failed to create subscription"}, http.StatusInternalServerError)
		}
		c.Abort()
		return
	}

	response := CreateSubscriptionResponse{
		SubscriptionID: output.Subscription.SubscriptionID,
		Status:         output.Subscription.Status,
		ClientSecret:   output.ClientSecret,
		CreatedAt:      output.Subscription.CreatedAt.Format(time.RFC3339),
	}

	// Используем pkg/res для успешного ответа
	res.JsonResponse(c.Writer, response, http.StatusCreated)
	h.log.Infow("CreateSubscription handler finished successfully")
}

// GetSubscription обрабатывает GET /subscriptions/:subscription_id
func (h *PaymentHandler) GetSubscription(c *gin.Context) { // <-- Сигнатура Gin
	ctx := c.Request.Context()
	subscriptionID := c.Param("subscription_id") // <-- Получаем параметр URL через Gin

	// TODO: Получить userID из контекста Gin
	// userIDValue, _ := c.Get("userID")
	// userID := userIDValue.(string)
	userID := "user-123" // !!! ЗАГЛУШКА !!!

	h.log.Infow("GetSubscription handler started", "subscriptionID", subscriptionID, "userID", userID)

	if subscriptionID == "" {
		h.log.Warnw("Missing subscription ID in request path")
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Missing subscription ID"}, http.StatusBadRequest)
		c.Abort()
		return
	}

	subscription, err := h.service.GetSubscriptionByID(ctx, userID, subscriptionID)
	if err != nil {
		h.log.Warnw("Service failed to get subscription", "error", err, "subscriptionID", subscriptionID)
		if errors.Is(err, services.ErrSubscriptionNotFound) {
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Subscription not found"}, http.StatusNotFound)
		} else {
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Failed to retrieve subscription"}, http.StatusInternalServerError)
		}
		c.Abort()
		return
	}

	res.JsonResponse(c.Writer, subscription, http.StatusOK)
	h.log.Infow("GetSubscription handler finished successfully")
}

// GetUserSubscriptions обрабатывает GET /users/:user_id/subscriptions
func (h *PaymentHandler) GetUserSubscriptions(c *gin.Context) { // <-- Сигнатура Gin
	ctx := c.Request.Context()
	targetUserID := c.Param("user_id") // <-- Получаем параметр URL через Gin

	// TODO: Получить requesterUserID из контекста Gin
	// requesterUserIDValue, _ := c.Get("userID")
	// requesterUserID := requesterUserIDValue.(string)
	// TODO: Проверка прав: if targetUserID != requesterUserID { c.AbortWithStatusJSON(http.StatusForbidden, ...); return }
	userID := targetUserID // !!! Упрощение !!!

	h.log.Infow("GetUserSubscriptions handler started", "userID", userID)

	if userID == "" {
		h.log.Warnw("Missing user ID in request path")
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Missing user ID"}, http.StatusBadRequest)
		c.Abort()
		return
	}

	subscriptions, err := h.service.GetSubscriptionsByUserID(ctx, userID)
	if err != nil {
		h.log.Errorw("Service failed to get user subscriptions", "error", err, "userID", userID)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Failed to retrieve subscriptions"}, http.StatusInternalServerError)
		c.Abort()
		return
	}

	res.JsonResponse(c.Writer, subscriptions, http.StatusOK)
	h.log.Infow("GetUserSubscriptions handler finished successfully", "count", len(subscriptions))
}

// CancelSubscription обрабатывает DELETE /subscriptions/:subscription_id
func (h *PaymentHandler) CancelSubscription(c *gin.Context) { // <-- Сигнатура Gin
	ctx := c.Request.Context()
	subscriptionID := c.Param("subscription_id") // <-- Получаем параметр URL через Gin
	idempotencyKey := c.GetHeader("Idempotency-Key")

	userID, exists := c.Get(middleware.ContextUserIDKey)
	if !exists {
		// Этого не должно произойти, если middleware отработал правильно
		h.log.Errorw("UserID not found in context after auth middleware")
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Internal server error"}, http.StatusInternalServerError)
		c.Abort()
		return
	}
	userIDStr := userID.(string) // Приводим к строке

	h.log.Infow("CancelSubscription handler started", "subscriptionID", subscriptionID, "userID", userID)

	if subscriptionID == "" {
		h.log.Warnw("Missing subscription ID in request path")
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Missing subscription ID"}, http.StatusBadRequest)
		c.Abort()
		return
	}

	err := h.service.CancelSubscription(ctx, userIDStr, subscriptionID, idempotencyKey)
	if err != nil {
		h.log.Warnw("Service failed to cancel subscription", "error", err, "subscriptionID", subscriptionID)
		if errors.Is(err, services.ErrSubscriptionNotFound) {
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Subscription not found"}, http.StatusNotFound)
		} else {
			res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Failed to cancel subscription"}, http.StatusInternalServerError)
		}
		c.Abort()
		return
	}

	res.JsonResponse(c.Writer, map[string]string{"message": "Subscription cancellation initiated successfully"}, http.StatusOK)
	h.log.Infow("CancelSubscription handler finished successfully")
}
