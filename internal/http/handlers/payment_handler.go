package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Dhoini/Payment-microservice/internal/middleware"
	"github.com/Dhoini/Payment-microservice/internal/models"
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

// --- DTO запроса ---
type CreateSubscriptionRequest struct {
	PlanID    string `json:"plan_id" validate:"required"`
	UserEmail string `json:"user_email" validate:"required,email"`
}

// --- DTO ответа ---
type CreateSubscriptionResponse struct {
	SubscriptionID string `json:"subscription_id"`
	Status         string `json:"status"`
	ClientSecret   string `json:"client_secret,omitempty"`
	CreatedAt      string `json:"created_at"`
}

type SubscriptionResponse struct {
	SubscriptionID   string     `json:"subscription_id"`
	UserID           string     `json:"user_id"`
	PlanID           string     `json:"plan_id"`
	Status           string     `json:"status"`
	StripeCustomerID string     `json:"stripe_customer_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	CanceledAt       *time.Time `json:"canceled_at,omitempty"`
}

// --- Обработчики ---

// CreateSubscription обрабатывает POST /api/v1/subscriptions
func (h *PaymentHandler) CreateSubscription(c *gin.Context) {
	ctx := c.Request.Context()
	// Используем базовый логгер хендлера
	h.log.Infow("Handler CreateSubscription started")

	// Шаг 1: Получаем UserID из контекста
	userIDValue, exists := c.Get(string(middleware.ContextUserIDKey))
	if !exists {
		h.log.Errorw("UserID not found in context after auth middleware")
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Unauthorized: User ID missing"}, http.StatusUnauthorized)
		c.Abort()
		return
	}
	userID := userIDValue.(string)

	// Шаг 2: Получаем ключ идемпотентности
	idempotencyKey := c.GetHeader("Idempotency-Key")
	h.log.Debugw("Received CreateSubscription request. UserID: %s, IdempotencyKey: %s", userID, idempotencyKey)

	// Шаг 3: Декодируем и валидируем тело запроса
	requestBody, err := req.HandleBody[CreateSubscriptionRequest](&c.Writer, c.Request, h.log) // Передаем базовый логгер
	if err != nil {
		// HandleBody уже отправил ответ и залогировал ошибку
		c.Abort()
		return
	}

	// Шаг 4: Подготовка ввода для сервиса
	input := services.CreateSubscriptionInput{
		UserID:         userID,
		PlanID:         requestBody.PlanID,
		UserEmail:      requestBody.UserEmail,
		IdempotencyKey: idempotencyKey,
	}

	// Шаг 5: Вызов сервисного слоя
	output, err := h.service.CreateSubscription(ctx, input)
	if err != nil {
		// Логируем ошибку с ID пользователя
		h.log.Errorw("Service failed to create subscription for UserID: %s. Error: %v", userID, err)
		statusCode, errMsg := mapErrorToHTTPStatus(err)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: errMsg}, statusCode)
		c.Abort()
		return
	}

	// Шаг 6: Формирование успешного ответа
	response := CreateSubscriptionResponse{
		SubscriptionID: output.Subscription.SubscriptionID,
		Status:         output.Subscription.Status,
		ClientSecret:   output.ClientSecret,
		CreatedAt:      output.Subscription.CreatedAt.Format(time.RFC3339),
	}

	res.JsonResponse(c.Writer, response, http.StatusCreated)
	h.log.Infow("Handler CreateSubscription finished successfully. UserID: %s, SubscriptionID: %s", userID, response.SubscriptionID)
}

// GetSubscription обрабатывает GET /api/v1/subscriptions/:subscription_id
func (h *PaymentHandler) GetSubscription(c *gin.Context) {
	ctx := c.Request.Context()
	h.log.Infow("Handler GetSubscription started")

	userIDValue, exists := c.Get(string(middleware.ContextUserIDKey))
	if !exists {
		h.log.Errorw("UserID not found in context")
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Unauthorized"}, http.StatusUnauthorized)
		c.Abort()
		return
	}
	userID := userIDValue.(string)
	subscriptionID := c.Param("subscription_id")

	h.log.Infow("Processing GetSubscription request. UserID: %s, SubscriptionID: %s", userID, subscriptionID)

	if subscriptionID == "" {
		h.log.Warnw("Missing subscription ID in request path. UserID: %s", userID)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Missing subscription ID"}, http.StatusBadRequest)
		c.Abort()
		return
	}

	subscription, err := h.service.GetSubscriptionByID(ctx, userID, subscriptionID)
	if err != nil {
		h.log.Warnw("Service failed to get subscription. UserID: %s, SubscriptionID: %s, Error: %v", userID, subscriptionID, err)
		statusCode, errMsg := mapErrorToHTTPStatus(err)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: errMsg}, statusCode)
		c.Abort()
		return
	}

	response := mapModelToSubscriptionResponse(subscription)
	res.JsonResponse(c.Writer, response, http.StatusOK)
	h.log.Infow("Handler GetSubscription finished successfully. UserID: %s, SubscriptionID: %s", userID, subscriptionID)
}

// GetUserSubscriptions обрабатывает GET /api/v1/users/:user_id/subscriptions
func (h *PaymentHandler) GetUserSubscriptions(c *gin.Context) {
	ctx := c.Request.Context()
	h.log.Infow("Handler GetUserSubscriptions started")

	requesterUserIDValue, exists := c.Get(string(middleware.ContextUserIDKey))
	if !exists {
		h.log.Errorw("Requester UserID not found in context")
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Unauthorized"}, http.StatusUnauthorized)
		c.Abort()
		return
	}
	requesterUserID := requesterUserIDValue.(string)
	targetUserID := c.Param("user_id")

	h.log.Infow("Processing GetUserSubscriptions. RequesterID: %s, TargetID: %s", requesterUserID, targetUserID)

	if targetUserID == "" {
		h.log.Warnw("Missing target user ID in request path. RequesterID: %s", requesterUserID)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Missing user ID"}, http.StatusBadRequest)
		c.Abort()
		return
	}

	// !!! ВАЖНО: Проверка прав доступа !!!
	if requesterUserID != targetUserID {
		h.log.Warnw("Forbidden access attempt in GetUserSubscriptions. RequesterID: %s, TargetID: %s", requesterUserID, targetUserID)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Forbidden"}, http.StatusForbidden)
		c.Abort()
		return
	}
	userIDToFetch := targetUserID

	subscriptions, err := h.service.GetSubscriptionsByUserID(ctx, userIDToFetch)
	if err != nil {
		h.log.Errorw("Service failed to get user subscriptions. UserID: %s, Error: %v", userIDToFetch, err)
		statusCode, errMsg := mapErrorToHTTPStatus(err)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: errMsg}, statusCode)
		c.Abort()
		return
	}

	response := make([]SubscriptionResponse, len(subscriptions))
	for i, sub := range subscriptions {
		response[i] = mapModelToSubscriptionResponse(&sub)
	}

	res.JsonResponse(c.Writer, response, http.StatusOK)
	h.log.Infow("Handler GetUserSubscriptions finished successfully. UserID: %s, Count: %d", userIDToFetch, len(response))
}

// CancelSubscription обрабатывает DELETE /api/v1/subscriptions/:subscription_id
func (h *PaymentHandler) CancelSubscription(c *gin.Context) {
	ctx := c.Request.Context()
	h.log.Infow("Handler CancelSubscription started")

	userIDValue, exists := c.Get(string(middleware.ContextUserIDKey))
	if !exists {
		h.log.Errorw("UserID not found in context")
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Unauthorized"}, http.StatusUnauthorized)
		c.Abort()
		return
	}
	userID := userIDValue.(string)
	subscriptionID := c.Param("subscription_id")
	idempotencyKey := c.GetHeader("Idempotency-Key")

	h.log.Infow("Processing CancelSubscription. UserID: %s, SubscriptionID: %s, IdempotencyKey: %s", userID, subscriptionID, idempotencyKey)

	if subscriptionID == "" {
		h.log.Warnw("Missing subscription ID in request path. UserID: %s", userID)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Missing subscription ID"}, http.StatusBadRequest)
		c.Abort()
		return
	}

	err := h.service.CancelSubscription(ctx, userID, subscriptionID, idempotencyKey)
	if err != nil {
		h.log.Warnw("Service failed to cancel subscription. UserID: %s, SubscriptionID: %s, Error: %v", userID, subscriptionID, err)
		statusCode, errMsg := mapErrorToHTTPStatus(err)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: errMsg}, statusCode)
		c.Abort()
		return
	}

	res.JsonResponse(c.Writer, map[string]string{"message": "Subscription cancellation initiated successfully"}, http.StatusOK)
	h.log.Infow("Handler CancelSubscription finished successfully. UserID: %s, SubscriptionID: %s", userID, subscriptionID)
}

// mapModelToSubscriptionResponse (без изменений)
func mapModelToSubscriptionResponse(sub *models.Subscription) SubscriptionResponse {
	if sub == nil {
		return SubscriptionResponse{}
	}
	return SubscriptionResponse{
		SubscriptionID:   sub.SubscriptionID,
		UserID:           sub.UserID,
		PlanID:           sub.PlanID,
		Status:           sub.Status,
		StripeCustomerID: sub.StripeCustomerID,
		CreatedAt:        sub.CreatedAt,
		UpdatedAt:        sub.UpdatedAt,
		ExpiresAt:        sub.ExpiresAt,
		CanceledAt:       sub.CanceledAt,
	}
}

// mapErrorToHTTPStatus (без изменений)
func mapErrorToHTTPStatus(err error) (statusCode int, message string) {
	switch {
	case errors.Is(err, services.ErrSubscriptionNotFound):
		return http.StatusNotFound, "Subscription not found"
	case errors.Is(err, services.ErrUserNotFound):
		return http.StatusNotFound, "User not found"
	case errors.Is(err, services.ErrPaymentFailed):
		return http.StatusUnprocessableEntity, "Payment processing failed"
	case errors.Is(err, services.ErrStripeClient):
		return http.StatusInternalServerError, "Payment provider error"
	case errors.Is(err, services.ErrInternalServer):
		return http.StatusInternalServerError, "Internal server error"
	default:
		return http.StatusInternalServerError, "An unexpected error occurred"
	}
}
