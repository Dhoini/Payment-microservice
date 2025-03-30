package handlers

import (
	"net/http"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/internal/service"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
)

// PaymentHandler обработчик для платежей
type PaymentHandler struct {
	service service.PaymentService
	log     *logger.Logger
}

// NewPaymentHandler создает новый обработчик платежей
func NewPaymentHandler(log *logger.Logger) *PaymentHandler {
	customerRepo := repository.NewInMemoryCustomerRepository(log)
	paymentRepo := repository.NewInMemoryPaymentRepository(log)
	svc := service.NewPaymentService(paymentRepo, customerRepo, log)

	return &PaymentHandler{
		service: svc,
		log:     log,
	}
}

// GetPayments возвращает список всех платежей
func (h *PaymentHandler) GetPayments(c *gin.Context) {
	// Проверяем, если есть параметр запроса customer_id
	customerID := c.Query("customer_id")
	if customerID != "" {
		h.getPaymentsByCustomerID(c, customerID)
		return
	}

	payments, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		h.log.Error("Failed to get payments: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get payments"})
		return
	}

	h.log.Info("Returned %d payments", len(payments))
	c.JSON(http.StatusOK, payments)
}

// getPaymentsByCustomerID возвращает платежи по ID клиента
func (h *PaymentHandler) getPaymentsByCustomerID(c *gin.Context, customerID string) {
	payments, err := h.service.GetByCustomerID(c.Request.Context(), customerID)
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

		h.log.Error("Failed to get payments for customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get payments for customer"})
		return
	}

	h.log.Info("Returned %d payments for customer %s", len(payments), customerID)
	c.JSON(http.StatusOK, payments)
}

// GetPayment возвращает платеж по ID
func (h *PaymentHandler) GetPayment(c *gin.Context) {
	id := c.Param("id")

	payment, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Payment not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID format"})
			return
		}

		h.log.Error("Failed to get payment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get payment"})
		return
	}

	h.log.Info("Returned payment with ID: %s", id)
	c.JSON(http.StatusOK, payment)
}

// CreatePayment создает новый платеж
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req domain.PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payment, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Customer not found: %s", req.CustomerID)
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid data in request")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid data in request"})
			return
		}

		h.log.Error("Failed to create payment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create payment"})
		return
	}

	h.log.Info("Created payment with ID: %s", payment.ID)
	c.JSON(http.StatusCreated, payment)
}

// UpdatePayment обновляет статус платежа
func (h *PaymentHandler) UpdatePayment(c *gin.Context) {
	id := c.Param("id")

	type UpdatePaymentRequest struct {
		Status string `json:"status" binding:"required"`
	}

	var req UpdatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверяем, что статус допустимый
	var status domain.PaymentStatus
	switch domain.PaymentStatus(req.Status) {
	case domain.PaymentStatusPending, domain.PaymentStatusCompleted, domain.PaymentStatusFailed, domain.PaymentStatusRefunded:
		status = domain.PaymentStatus(req.Status)
	default:
		h.log.Warn("Invalid payment status: %s", req.Status)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment status"})
		return
	}

	payment, err := h.service.UpdateStatus(c.Request.Context(), id, status)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Payment not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payment ID format"})
			return
		}

		h.log.Error("Failed to update payment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update payment"})
		return
	}

	h.log.Info("Updated payment with ID: %s to status: %s", id, status)
	c.JSON(http.StatusOK, payment)
}
