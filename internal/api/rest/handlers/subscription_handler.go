package handlers

import (
	"net/http"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/internal/service"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
)

// SubscriptionHandler обработчик для подписок
type SubscriptionHandler struct {
	subscriptionSvc service.SubscriptionService
	customerSvc     service.CustomerService
	planSvc         service.SubscriptionService
	log             *logger.Logger
}

// NewSubscriptionHandler создает новый обработчик подписок
func NewSubscriptionHandler(log *logger.Logger) *SubscriptionHandler {
	customerRepo := repository.NewInMemoryCustomerRepository(log)
	paymentRepo := repository.NewInMemoryPaymentRepository(log)
	subscriptionRepo := repository.NewInMemorySubscriptionRepository(log)

	customerSvc := service.NewCustomerService(customerRepo, log)
	svc := service.NewSubscriptionService(
		subscriptionRepo,
		customerRepo,
		paymentRepo,
		log,
	)

	return &SubscriptionHandler{
		subscriptionSvc: svc,
		customerSvc:     customerSvc,
		planSvc:         svc,
		log:             log,
	}
}

// CreateSubscription создает новую подписку
func (h *SubscriptionHandler) CreateSubscription(c *gin.Context) {
	var req domain.SubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subscription, err := h.subscriptionSvc.Create(c.Request.Context(), req)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Customer or plan not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer or plan not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid data in request")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data in request"})
			return
		}

		if err == domain.ErrInvalidOperation {
			h.log.Warn("Invalid operation: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid operation"})
			return
		}

		h.log.Error("Failed to create subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subscription"})
		return
	}

	h.log.Info("Created subscription with ID: %s", subscription.ID)
	c.JSON(http.StatusCreated, subscription)
}

// GetSubscription возвращает подписку по ID
func (h *SubscriptionHandler) GetSubscription(c *gin.Context) {
	id := c.Param("id")

	subscription, err := h.subscriptionSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Subscription not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID format"})
			return
		}

		h.log.Error("Failed to get subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscription"})
		return
	}

	h.log.Info("Returned subscription with ID: %s", id)
	c.JSON(http.StatusOK, subscription)
}

// ListSubscriptions возвращает список подписок
func (h *SubscriptionHandler) ListSubscriptions(c *gin.Context) {
	// Проверяем, если есть параметр запроса customer_id
	customerID := c.Query("customer_id")
	if customerID != "" {
		h.listSubscriptionsByCustomerID(c, customerID)
		return
	}

	subscriptions, err := h.subscriptionSvc.GetAll(c.Request.Context())
	if err != nil {
		h.log.Error("Failed to get subscriptions: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscriptions"})
		return
	}

	h.log.Info("Returned %d subscriptions", len(subscriptions))
	c.JSON(http.StatusOK, subscriptions)
}

// listSubscriptionsByCustomerID возвращает подписки по ID клиента
func (h *SubscriptionHandler) listSubscriptionsByCustomerID(c *gin.Context, customerID string) {
	subscriptions, err := h.subscriptionSvc.GetByCustomerID(c.Request.Context(), customerID)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Customer not found: %s", customerID)
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", customerID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID format"})
			return
		}

		h.log.Error("Failed to get subscriptions for customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscriptions for customer"})
		return
	}

	h.log.Info("Returned %d subscriptions for customer %s", len(subscriptions), customerID)
	c.JSON(http.StatusOK, subscriptions)
}

// CancelSubscription отменяет подписку
func (h *SubscriptionHandler) CancelSubscription(c *gin.Context) {
	id := c.Param("id")

	// Получаем параметр cancel_at_period_end из запроса
	cancelAtPeriodEnd := c.DefaultQuery("cancel_at_period_end", "false") == "true"

	subscription, err := h.subscriptionSvc.Cancel(c.Request.Context(), id, cancelAtPeriodEnd)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Subscription not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID format"})
			return
		}

		if err == domain.ErrInvalidOperation {
			h.log.Warn("Cannot cancel subscription: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot cancel subscription"})
			return
		}

		h.log.Error("Failed to cancel subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel subscription"})
		return
	}

	h.log.Info("Cancelled subscription with ID: %s", id)
	c.JSON(http.StatusOK, subscription)
}

// PauseSubscription приостанавливает подписку
func (h *SubscriptionHandler) PauseSubscription(c *gin.Context) {
	id := c.Param("id")

	subscription, err := h.subscriptionSvc.Pause(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Subscription not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID format"})
			return
		}

		if err == domain.ErrInvalidOperation {
			h.log.Warn("Cannot pause subscription: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot pause subscription"})
			return
		}

		h.log.Error("Failed to pause subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pause subscription"})
		return
	}

	h.log.Info("Paused subscription with ID: %s", id)
	c.JSON(http.StatusOK, subscription)
}

// ResumeSubscription возобновляет подписку
func (h *SubscriptionHandler) ResumeSubscription(c *gin.Context) {
	id := c.Param("id")

	subscription, err := h.subscriptionSvc.Resume(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Subscription not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription ID format"})
			return
		}

		if err == domain.ErrInvalidOperation {
			h.log.Warn("Cannot resume subscription: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot resume subscription"})
			return
		}

		h.log.Error("Failed to resume subscription: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resume subscription"})
		return
	}

	h.log.Info("Resumed subscription with ID: %s", id)
	c.JSON(http.StatusOK, subscription)
}

// GetPlans returns all subscription plans
func (h *SubscriptionHandler) GetPlans(c *gin.Context) {
	plans, err := h.planSvc.GetAllPlans(c.Request.Context())
	if err != nil {
		h.log.Error("Failed to get subscription plans: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscription plans"})
		return
	}

	h.log.Info("Returned %d subscription plans", len(plans))
	c.JSON(http.StatusOK, plans)
}

// GetPlan returns a subscription plan by ID
func (h *SubscriptionHandler) GetPlan(c *gin.Context) {
	id := c.Param("id")

	plan, err := h.planSvc.GetPlanByID(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Subscription plan not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription plan not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription plan ID format"})
			return
		}

		h.log.Error("Failed to get subscription plan: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscription plan"})
		return
	}

	h.log.Info("Returned subscription plan with ID: %s", id)
	c.JSON(http.StatusOK, plan)
}

// CreatePlan creates a new subscription plan
func (h *SubscriptionHandler) CreatePlan(c *gin.Context) {
	var req domain.SubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plan, err := h.planSvc.CreatePlan(c.Request.Context(), req)
	if err != nil {
		if err == repository.ErrDuplicate {
			h.log.Warn("Subscription plan with this name already exists")
			c.JSON(http.StatusConflict, gin.H{"error": "Subscription plan with this name already exists"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid data in request")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data in request"})
			return
		}

		h.log.Error("Failed to create subscription plan: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create subscription plan"})
		return
	}

	h.log.Info("Created subscription plan with ID: %s", plan.ID)
	c.JSON(http.StatusCreated, plan)
}

// UpdatePlan updates an existing subscription plan
func (h *SubscriptionHandler) UpdatePlan(c *gin.Context) {
	id := c.Param("id")

	var req domain.SubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plan, err := h.planSvc.UpdatePlan(c.Request.Context(), id, req)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Subscription plan not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription plan not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format or data: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription plan ID format or data"})
			return
		}

		if err == repository.ErrDuplicate {
			h.log.Warn("Subscription plan with this name already exists")
			c.JSON(http.StatusConflict, gin.H{"error": "Subscription plan with this name already exists"})
			return
		}

		h.log.Error("Failed to update subscription plan: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subscription plan"})
		return
	}

	h.log.Info("Updated subscription plan with ID: %s", plan.ID)
	c.JSON(http.StatusOK, plan)
}

// DeletePlan deletes a subscription plan
func (h *SubscriptionHandler) DeletePlan(c *gin.Context) {
	id := c.Param("id")

	err := h.planSvc.DeletePlan(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Subscription plan not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Subscription plan not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subscription plan ID format"})
			return
		}

		if err == domain.ErrInvalidOperation {
			h.log.Warn("Cannot delete plan with active subscriptions: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete plan with active subscriptions"})
			return
		}

		h.log.Error("Failed to delete subscription plan: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete subscription plan"})
		return
	}

	h.log.Info("Deleted subscription plan with ID: %s", id)
	c.JSON(http.StatusOK, gin.H{"message": "Subscription plan deleted successfully"})
}
