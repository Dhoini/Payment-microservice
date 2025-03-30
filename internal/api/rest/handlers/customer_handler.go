package handlers

import (
	"net/http"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/repository"
	"github.com/Dhoini/Payment-microservice/internal/service"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
)

// CustomerHandler обработчик для клиентов
type CustomerHandler struct {
	service service.CustomerService
	log     *logger.Logger
}

// NewCustomerHandler создает новый обработчик клиентов
func NewCustomerHandler(log *logger.Logger) *CustomerHandler {
	repo := repository.NewInMemoryCustomerRepository(log)
	svc := service.NewCustomerService(repo, log)

	return &CustomerHandler{
		service: svc,
		log:     log,
	}
}

// GetCustomers возвращает список всех клиентов
func (h *CustomerHandler) GetCustomers(c *gin.Context) {
	customers, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		h.log.Error("Failed to get customers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get customers"})
		return
	}

	h.log.Info("Returned %d customers", len(customers))
	c.JSON(http.StatusOK, customers)
}

// GetCustomer возвращает клиента по ID
func (h *CustomerHandler) GetCustomer(c *gin.Context) {
	id := c.Param("id")

	customer, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Customer not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID format"})
			return
		}

		h.log.Error("Failed to get customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get customer"})
		return
	}

	h.log.Info("Returned customer with ID: %s", id)
	c.JSON(http.StatusOK, customer)
}

// CreateCustomer создает нового клиента
func (h *CustomerHandler) CreateCustomer(c *gin.Context) {
	var req domain.CustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customer, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		if err == repository.ErrDuplicate {
			h.log.Warn("Customer with this email already exists")
			c.JSON(http.StatusConflict, gin.H{"error": "Customer with this email already exists"})
			return
		}

		h.log.Error("Failed to create customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create customer"})
		return
	}

	h.log.Info("Created customer with ID: %s", customer.ID)
	c.JSON(http.StatusCreated, customer)
}

// UpdateCustomer обновляет существующего клиента
func (h *CustomerHandler) UpdateCustomer(c *gin.Context) {
	id := c.Param("id")

	var req domain.CustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("Invalid request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	customer, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Customer not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID format"})
			return
		}

		if err == repository.ErrDuplicate {
			h.log.Warn("Customer with this email already exists")
			c.JSON(http.StatusConflict, gin.H{"error": "Customer with this email already exists"})
			return
		}

		h.log.Error("Failed to update customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update customer"})
		return
	}

	h.log.Info("Updated customer with ID: %s", customer.ID)
	c.JSON(http.StatusOK, customer)
}

// DeleteCustomer удаляет клиента
func (h *CustomerHandler) DeleteCustomer(c *gin.Context) {
	id := c.Param("id")

	err := h.service.Delete(c.Request.Context(), id)
	if err != nil {
		if err == repository.ErrNotFound {
			h.log.Warn("Customer not found: %s", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
			return
		}

		if err == repository.ErrInvalidData {
			h.log.Warn("Invalid UUID format: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID format"})
			return
		}

		h.log.Error("Failed to delete customer: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete customer"})
		return
	}

	h.log.Info("Deleted customer with ID: %s", id)
	c.JSON(http.StatusOK, gin.H{"message": "Customer deleted successfully"})
}
