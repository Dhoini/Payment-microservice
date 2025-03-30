package handler

import (
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

// ItemHandler обрабатывает запросы к ресурсу items
type ItemHandler struct {
	service service.ItemService
	log     *logger.Logger
}

// NewItemHandler создает новый обработчик для items
func NewItemHandler(log *logger.Logger) *ItemHandler {
	return &ItemHandler{
		service: service.NewItemService(),
		log:     log,
	}
}

// GetItems возвращает список всех элементов
func (h *ItemHandler) GetItems(c *gin.Context) {
	h.log.Debug("Getting all items")

	items, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		h.log.Error("Failed to get items: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("Successfully retrieved %d items", len(items))
	c.JSON(http.StatusOK, items)
}

// GetItem возвращает элемент по ID
func (h *ItemHandler) GetItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		h.log.Warn("Invalid ID format: %s", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	h.log.Debug("Getting item with ID: %d", id)

	item, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		h.log.Warn("Item not found: %d", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}

	h.log.Info("Successfully retrieved item with ID: %d", id)
	c.JSON(http.StatusOK, item)
}

// CreateItem создает новый элемент
func (h *ItemHandler) CreateItem(c *gin.Context) {
	var item model.Item
	if err := c.ShouldBindJSON(&item); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.log.Debug("Creating new item: %s", item.Name)

	createdItem, err := h.service.Create(c.Request.Context(), item)
	if err != nil {
		h.log.Error("Failed to create item: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("Successfully created item with ID: %d", createdItem.ID)
	c.JSON(http.StatusCreated, createdItem)
}

// UpdateItem обновляет существующий элемент
func (h *ItemHandler) UpdateItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		h.log.Warn("Invalid ID format: %s", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	var item model.Item
	if err := c.ShouldBindJSON(&item); err != nil {
		h.log.Warn("Invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item.ID = id

	h.log.Debug("Updating item with ID: %d", id)

	if err := h.service.Update(c.Request.Context(), item); err != nil {
		h.log.Error("Failed to update item: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("Successfully updated item with ID: %d", id)
	c.JSON(http.StatusOK, gin.H{"message": "item updated successfully"})
}

// DeleteItem удаляет элемент по ID
func (h *ItemHandler) DeleteItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		h.log.Warn("Invalid ID format: %s", c.Param("id"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	h.log.Debug("Deleting item with ID: %d", id)

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		h.log.Error("Failed to delete item: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.log.Info("Successfully deleted item with ID: %d", id)
	c.JSON(http.StatusOK, gin.H{"message": "item deleted successfully"})
}
