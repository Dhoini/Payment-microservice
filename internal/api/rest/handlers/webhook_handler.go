package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/internal/integration/stripe"
	"github.com/Dhoini/Payment-microservice/internal/service"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// WebhookHandler обработчик для вебхуков
type WebhookHandler struct {
	stripeWebhookHandler *stripe.WebhookHandler
	paymentService       service.PaymentService
	customerService      service.CustomerService
	log                  *logger.Logger
}

// NewWebhookHandler создает новый обработчик вебхуков
func NewWebhookHandler(stripeWebhookHandler *stripe.WebhookHandler, paymentService service.PaymentService, customerService service.CustomerService, log *logger.Logger) *WebhookHandler {
	return &WebhookHandler{
		stripeWebhookHandler: stripeWebhookHandler,
		paymentService:       paymentService,
		customerService:      customerService,
		log:                  log,
	}
}

// HandleStripeWebhook обрабатывает вебхуки от Stripe
func (h *WebhookHandler) HandleStripeWebhook(c *gin.Context) {
	// Сохраняем тело запроса
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.log.Error("Failed to read webhook body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read webhook body"})
		return
	}

	// Восстанавливаем тело для дальнейшего использования
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	// Проверяем подпись вебхука
	_, err = h.stripeWebhookHandler.VerifySignature(c.Request)
	if err != nil {
		h.log.Error("Failed to verify webhook signature: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to verify webhook signature"})
		return
	}

	// Создаем адаптеры для обработчиков событий
	paymentEventHandler := &stripePaymentEventHandler{
		paymentService: h.paymentService,
		log:            h.log,
	}

	customerEventHandler := &stripeCustomerEventHandler{
		customerService: h.customerService,
		log:             h.log,
	}

	// Обрабатываем вебхук
	h.stripeWebhookHandler.HandleWebhook(c.Writer, c.Request, paymentEventHandler, customerEventHandler)
}

// stripePaymentEventHandler адаптер для обработки событий платежей
type stripePaymentEventHandler struct {
	paymentService service.PaymentService
	log            *logger.Logger
}

// HandlePaymentSucceeded обрабатывает успешный платеж
func (h *stripePaymentEventHandler) HandlePaymentSucceeded(paymentID uuid.UUID, externalID string, amount float64) error {
	h.log.Info("Handling payment succeeded event for payment: %s", paymentID)

	// Обновляем статус платежа на "completed"
	_, err := h.paymentService.UpdateStatus(context.Background(), paymentID.String(), domain.PaymentStatusCompleted)
	if err != nil {
		h.log.Error("Failed to update payment status: %v", err)
		return err
	}

	return nil
}

// HandlePaymentFailed обрабатывает неудачный платеж
func (h *stripePaymentEventHandler) HandlePaymentFailed(paymentID uuid.UUID, externalID string, errorMessage string) error {
	h.log.Info("Handling payment failed event for payment: %s", paymentID)

	// Получаем платеж
	payment, err := h.paymentService.GetByID(context.Background(), paymentID.String())
	if err != nil {
		h.log.Error("Failed to get payment: %v", err)
		return err
	}

	// Обновляем ошибку и статус
	payment.ErrorMessage = errorMessage

	// Обновляем статус платежа на "failed"
	_, err = h.paymentService.UpdateStatus(context.Background(), paymentID.String(), domain.PaymentStatusFailed)
	if err != nil {
		h.log.Error("Failed to update payment status: %v", err)
		return err
	}

	return nil
}

// HandlePaymentCanceled обрабатывает отмененный платеж
func (h *stripePaymentEventHandler) HandlePaymentCanceled(paymentID uuid.UUID, externalID string) error {
	h.log.Info("Handling payment canceled event for payment: %s", paymentID)

	// Обновляем статус платежа на "failed"
	_, err := h.paymentService.UpdateStatus(context.Background(), paymentID.String(), domain.PaymentStatusFailed)
	if err != nil {
		h.log.Error("Failed to update payment status: %v", err)
		return err
	}

	return nil
}

// HandlePaymentRefunded обрабатывает возврат платежа
func (h *stripePaymentEventHandler) HandlePaymentRefunded(paymentID uuid.UUID, externalID string, amount float64) error {
	h.log.Info("Handling payment refunded event for payment: %s", paymentID)

	// Обновляем статус платежа на "refunded"
	_, err := h.paymentService.UpdateStatus(context.Background(), paymentID.String(), domain.PaymentStatusRefunded)
	if err != nil {
		h.log.Error("Failed to update payment status: %v", err)
		return err
	}

	return nil
}

// stripeCustomerEventHandler адаптер для обработки событий клиентов
type stripeCustomerEventHandler struct {
	customerService service.CustomerService
	log             *logger.Logger
}

// HandleCustomerCreated обрабатывает создание клиента
func (h *stripeCustomerEventHandler) HandleCustomerCreated(customerID uuid.UUID, externalID string) error {
	h.log.Info("Handling customer created event for customer: %s", customerID)

	// Получаем клиента
	customer, err := h.customerService.GetByID(context.Background(), customerID.String())
	if err != nil {
		h.log.Error("Failed to get customer: %v", err)
		return err
	}

	// Обновляем externalID если его нет
	if customer.ExternalID == "" {
		// Создаем запрос на обновление
		updateRequest := domain.CustomerRequest{
			Email:      customer.Email,
			Name:       customer.Name,
			Phone:      customer.Phone,
			ExternalID: externalID,
			Metadata:   customer.Metadata,
		}

		// Обновляем клиента
		_, err := h.customerService.Update(context.Background(), customerID.String(), updateRequest)
		if err != nil {
			h.log.Error("Failed to update customer: %v", err)
			return err
		}
	}

	return nil
}

// HandleCustomerUpdated обрабатывает обновление клиента
func (h *stripeCustomerEventHandler) HandleCustomerUpdated(customerID uuid.UUID, externalID string) error {
	h.log.Info("Handling customer updated event for customer: %s", customerID)
	// В большинстве случаев ничего делать не нужно, так как Stripe обычно
	// обновляет клиента в ответ на наши действия
	return nil
}

// HandleCustomerDeleted обрабатывает удаление клиента
func (h *stripeCustomerEventHandler) HandleCustomerDeleted(customerID uuid.UUID, externalID string) error {
	h.log.Info("Handling customer deleted event for customer: %s", customerID)
	// В зависимости от логики бизнеса, можно либо удалить клиента,
	// либо просто отметить его как удаленный в Stripe

	// В данном примере мы просто обновляем информацию о клиенте
	customer, err := h.customerService.GetByID(context.Background(), customerID.String())
	if err != nil {
		h.log.Error("Failed to get customer: %v", err)
		return err
	}

	// Добавляем метку удаления в метаданные
	if customer.Metadata == nil {
		customer.Metadata = make(map[string]string)
	}
	customer.Metadata["stripe_deleted"] = "true"

	// Создаем запрос на обновление
	updateRequest := domain.CustomerRequest{
		Email:      customer.Email,
		Name:       customer.Name,
		Phone:      customer.Phone,
		ExternalID: customer.ExternalID,
		Metadata:   customer.Metadata,
	}

	// Обновляем клиента
	_, err = h.customerService.Update(context.Background(), customerID.String(), updateRequest)
	if err != nil {
		h.log.Error("Failed to update customer: %v", err)
		return err
	}

	return nil
}
