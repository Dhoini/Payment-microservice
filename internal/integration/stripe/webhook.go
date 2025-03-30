package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/google/uuid"
	"io"
	"net/http"
)

// WebhookEvent представляет событие от Stripe Webhook
type WebhookEvent struct {
	ID              string                 `json:"id"`
	Object          string                 `json:"object"`
	Type            string                 `json:"type"`
	Created         int64                  `json:"created"`
	Data            map[string]interface{} `json:"data"`
	PendingWebhooks int                    `json:"pending_webhooks"`
	Livemode        bool                   `json:"livemode"`
}

// WebhookHandler обрабатывает webhook события от Stripe
type WebhookHandler struct {
	client *Client
	log    *logger.Logger
}

// PaymentEventHandler интерфейс для обработки событий платежей
type PaymentEventHandler interface {
	HandlePaymentSucceeded(paymentID uuid.UUID, externalID string, amount float64) error
	HandlePaymentFailed(paymentID uuid.UUID, externalID string, errorMessage string) error
	HandlePaymentCanceled(paymentID uuid.UUID, externalID string) error
	HandlePaymentRefunded(paymentID uuid.UUID, externalID string, amount float64) error
}

// CustomerEventHandler интерфейс для обработки событий клиентов
type CustomerEventHandler interface {
	HandleCustomerCreated(customerID uuid.UUID, externalID string) error
	HandleCustomerUpdated(customerID uuid.UUID, externalID string) error
	HandleCustomerDeleted(customerID uuid.UUID, externalID string) error
}

// NewWebhookHandler создает новый обработчик webhook-ов
func NewWebhookHandler(client *Client, log *logger.Logger) *WebhookHandler {
	return &WebhookHandler{
		client: client,
		log:    log,
	}
}

// VerifySignature проверяет подпись webhook-события от Stripe
func (h *WebhookHandler) VerifySignature(r *http.Request) ([]byte, error) {
	// Получаем заголовок с подписью
	stripeSignature := r.Header.Get("Stripe-Signature")
	if stripeSignature == "" {
		return nil, fmt.Errorf("no Stripe signature in request")
	}

	// Читаем тело запроса
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// В реальном проекте здесь должна быть проверка подписи
	// с использованием webhookKey от Stripe
	// Например: stripe.WebhookSignatureVerifier.Verify(payload, stripeSignature, h.client.GetWebhookKey())

	return payload, nil
}

// HandleWebhook обрабатывает webhook-события от Stripe
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request,
	paymentHandler PaymentEventHandler, customerHandler CustomerEventHandler) {
	// Проверяем подпись и получаем тело запроса
	payload, err := h.VerifySignature(r)
	if err != nil {
		h.log.Error("Failed to verify webhook signature: %v", err)
		http.Error(w, "Signature verification failed", http.StatusBadRequest)
		return
	}

	// Парсим событие
	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		h.log.Error("Failed to parse webhook event: %v", err)
		http.Error(w, "Failed to parse webhook event", http.StatusBadRequest)
		return
	}

	// Логируем полученное событие
	h.log.Info("Received Stripe webhook event: %s, type: %s", event.ID, event.Type)

	// Обрабатываем событие в зависимости от типа
	var handlerErr error
	switch event.Type {
	case "payment_intent.succeeded":
		handlerErr = h.handlePaymentIntentSucceeded(event, paymentHandler)
	case "payment_intent.payment_failed":
		handlerErr = h.handlePaymentIntentFailed(event, paymentHandler)
	case "payment_intent.canceled":
		handlerErr = h.handlePaymentIntentCanceled(event, paymentHandler)
	case "charge.refunded":
		handlerErr = h.handleChargeRefunded(event, paymentHandler)
	case "customer.created":
		handlerErr = h.handleCustomerCreated(event, customerHandler)
	case "customer.updated":
		handlerErr = h.handleCustomerUpdated(event, customerHandler)
	case "customer.deleted":
		handlerErr = h.handleCustomerDeleted(event, customerHandler)
	default:
		h.log.Info("Ignored webhook event type: %s", event.Type)
	}

	if handlerErr != nil {
		h.log.Error("Failed to handle webhook event: %v", handlerErr)
		http.Error(w, fmt.Sprintf("Failed to handle webhook event: %v", handlerErr), http.StatusInternalServerError)
		return
	}

	// Отправляем успешный ответ Stripe
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"received": true}`))
}

// extractUUIDFromMetadata извлекает UUID из метаданных
func extractUUIDFromMetadata(metadata map[string]interface{}, key string) (uuid.UUID, error) {
	if metadata == nil {
		return uuid.Nil, fmt.Errorf("metadata is nil")
	}

	valueInterface, ok := metadata[key]
	if !ok {
		return uuid.Nil, fmt.Errorf("key %s not found in metadata", key)
	}

	valueStr, ok := valueInterface.(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("value for key %s is not a string", key)
	}

	id, err := uuid.Parse(valueStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse UUID: %w", err)
	}

	return id, nil
}

// handlePaymentIntentSucceeded обрабатывает успешный платеж
func (h *WebhookHandler) handlePaymentIntentSucceeded(event WebhookEvent, handler PaymentEventHandler) error {
	h.log.Debug("Handling payment_intent.succeeded event")

	// Получаем данные о платеже из события
	paymentIntentData, ok := event.Data["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Получаем ID платежа в Stripe
	paymentIntentID, ok := paymentIntentData["id"].(string)
	if !ok || paymentIntentID == "" {
		return fmt.Errorf("payment intent ID not found in event data")
	}

	// Получаем метаданные платежа
	metadataInterface, ok := paymentIntentData["metadata"]
	if !ok {
		return fmt.Errorf("metadata not found in payment intent")
	}

	metadata, ok := metadataInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("metadata is not a map")
	}

	// Получаем наш внутренний ID платежа из метаданных
	paymentID, err := extractUUIDFromMetadata(metadata, "payment_service_payment_id")
	if err != nil {
		return fmt.Errorf("failed to extract payment ID: %w", err)
	}

	// Получаем сумму платежа
	amountInterface, ok := paymentIntentData["amount"]
	if !ok {
		return fmt.Errorf("amount not found in payment intent")
	}

	// Преобразуем из float64 или int64 в float64
	var amountFloat float64
	switch v := amountInterface.(type) {
	case float64:
		amountFloat = v
	case int64:
		amountFloat = float64(v)
	case json.Number:
		floatVal, err := v.Float64()
		if err != nil {
			return fmt.Errorf("failed to convert amount to float64: %w", err)
		}
		amountFloat = floatVal
	default:
		return fmt.Errorf("amount is not a number")
	}

	// Переводим из копеек в рубли (или соответствующую валюту)
	amountFloat = amountFloat / 100.0

	// Вызываем обработчик
	if handler != nil {
		return handler.HandlePaymentSucceeded(paymentID, paymentIntentID, amountFloat)
	}

	return nil
}

// handlePaymentIntentFailed обрабатывает неудачный платеж
func (h *WebhookHandler) handlePaymentIntentFailed(event WebhookEvent, handler PaymentEventHandler) error {
	h.log.Debug("Handling payment_intent.payment_failed event")

	// Получаем данные о платеже из события
	paymentIntentData, ok := event.Data["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Получаем ID платежа в Stripe
	paymentIntentID, ok := paymentIntentData["id"].(string)
	if !ok || paymentIntentID == "" {
		return fmt.Errorf("payment intent ID not found in event data")
	}

	// Получаем метаданные платежа
	metadataInterface, ok := paymentIntentData["metadata"]
	if !ok {
		return fmt.Errorf("metadata not found in payment intent")
	}

	metadata, ok := metadataInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("metadata is not a map")
	}

	// Получаем наш внутренний ID платежа из метаданных
	paymentID, err := extractUUIDFromMetadata(metadata, "payment_service_payment_id")
	if err != nil {
		return fmt.Errorf("failed to extract payment ID: %w", err)
	}

	// Получаем сообщение об ошибке
	var errorMessage string
	lastPaymentErrorInterface, ok := paymentIntentData["last_payment_error"]
	if ok {
		lastPaymentError, ok := lastPaymentErrorInterface.(map[string]interface{})
		if ok {
			message, ok := lastPaymentError["message"].(string)
			if ok {
				errorMessage = message
			}
		}
	}

	// Вызываем обработчик
	if handler != nil {
		return handler.HandlePaymentFailed(paymentID, paymentIntentID, errorMessage)
	}

	return nil
}

// handlePaymentIntentCanceled обрабатывает отмененный платеж
func (h *WebhookHandler) handlePaymentIntentCanceled(event WebhookEvent, handler PaymentEventHandler) error {
	h.log.Debug("Handling payment_intent.canceled event")

	// Получаем данные о платеже из события
	paymentIntentData, ok := event.Data["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Получаем ID платежа в Stripe
	paymentIntentID, ok := paymentIntentData["id"].(string)
	if !ok || paymentIntentID == "" {
		return fmt.Errorf("payment intent ID not found in event data")
	}

	// Получаем метаданные платежа
	metadataInterface, ok := paymentIntentData["metadata"]
	if !ok {
		return fmt.Errorf("metadata not found in payment intent")
	}

	metadata, ok := metadataInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("metadata is not a map")
	}

	// Получаем наш внутренний ID платежа из метаданных
	paymentID, err := extractUUIDFromMetadata(metadata, "payment_service_payment_id")
	if err != nil {
		return fmt.Errorf("failed to extract payment ID: %w", err)
	}

	// Вызываем обработчик
	if handler != nil {
		return handler.HandlePaymentCanceled(paymentID, paymentIntentID)
	}

	return nil
}

// handleChargeRefunded обрабатывает возврат платежа
func (h *WebhookHandler) handleChargeRefunded(event WebhookEvent, handler PaymentEventHandler) error {
	h.log.Debug("Handling charge.refunded event")

	// Получаем данные о платеже из события
	chargeData, ok := event.Data["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Получаем ID платежа в Stripe
	chargeID, ok := chargeData["id"].(string)
	if !ok || chargeID == "" {
		return fmt.Errorf("charge ID not found in event data")
	}

	// Получаем ID платежного намерения
	paymentIntentID, ok := chargeData["payment_intent"].(string)
	if !ok || paymentIntentID == "" {
		return fmt.Errorf("payment intent ID not found in charge data")
	}

	// Получаем метаданные платежа, для этого нужно запросить payment intent
	paymentIntent, err := h.client.GetPaymentIntent(context.Background(), paymentIntentID)
	if err != nil {
		return fmt.Errorf("failed to get payment intent: %w", err)
	}

	// Получаем наш внутренний ID платежа из метаданных
	paymentIDStr, ok := paymentIntent.Metadata["payment_service_payment_id"]
	if !ok {
		return fmt.Errorf("payment_service_payment_id not found in payment intent metadata")
	}

	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		return fmt.Errorf("failed to parse payment ID: %w", err)
	}

	// Получаем сумму возврата
	amountRefundedInterface, ok := chargeData["amount_refunded"]
	if !ok {
		return fmt.Errorf("amount_refunded not found in charge data")
	}

	// Преобразуем из float64 или int64 в float64
	var amountRefundedFloat float64
	switch v := amountRefundedInterface.(type) {
	case float64:
		amountRefundedFloat = v
	case int64:
		amountRefundedFloat = float64(v)
	case json.Number:
		floatVal, err := v.Float64()
		if err != nil {
			return fmt.Errorf("failed to convert amount_refunded to float64: %w", err)
		}
		amountRefundedFloat = floatVal
	default:
		return fmt.Errorf("amount_refunded is not a number")
	}

	// Переводим из копеек в рубли (или соответствующую валюту)
	amountRefundedFloat = amountRefundedFloat / 100.0

	// Вызываем обработчик
	if handler != nil {
		return handler.HandlePaymentRefunded(paymentID, paymentIntentID, amountRefundedFloat)
	}

	return nil
}

// handleCustomerCreated обрабатывает создание клиента
func (h *WebhookHandler) handleCustomerCreated(event WebhookEvent, handler CustomerEventHandler) error {
	h.log.Debug("Handling customer.created event")

	// Получаем данные о клиенте из события
	customerData, ok := event.Data["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Получаем ID клиента в Stripe
	customerID, ok := customerData["id"].(string)
	if !ok || customerID == "" {
		return fmt.Errorf("customer ID not found in event data")
	}

	// Получаем метаданные клиента
	metadataInterface, ok := customerData["metadata"]
	if !ok {
		return fmt.Errorf("metadata not found in customer data")
	}

	metadata, ok := metadataInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("metadata is not a map")
	}

	// Получаем наш внутренний ID клиента из метаданных
	internalCustomerID, err := extractUUIDFromMetadata(metadata, "payment_service_customer_id")
	if err != nil {
		return fmt.Errorf("failed to extract customer ID: %w", err)
	}

	// Вызываем обработчик
	if handler != nil {
		return handler.HandleCustomerCreated(internalCustomerID, customerID)
	}

	return nil
}

// handleCustomerUpdated обрабатывает обновление клиента
func (h *WebhookHandler) handleCustomerUpdated(event WebhookEvent, handler CustomerEventHandler) error {
	h.log.Debug("Handling customer.updated event")

	// Получаем данные о клиенте из события
	customerData, ok := event.Data["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Получаем ID клиента в Stripe
	customerID, ok := customerData["id"].(string)
	if !ok || customerID == "" {
		return fmt.Errorf("customer ID not found in event data")
	}

	// Получаем метаданные клиента
	metadataInterface, ok := customerData["metadata"]
	if !ok {
		return fmt.Errorf("metadata not found in customer data")
	}

	metadata, ok := metadataInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("metadata is not a map")
	}

	// Получаем наш внутренний ID клиента из метаданных
	internalCustomerID, err := extractUUIDFromMetadata(metadata, "payment_service_customer_id")
	if err != nil {
		return fmt.Errorf("failed to extract customer ID: %w", err)
	}

	// Вызываем обработчик
	if handler != nil {
		return handler.HandleCustomerUpdated(internalCustomerID, customerID)
	}

	return nil
}

// handleCustomerDeleted обрабатывает удаление клиента
func (h *WebhookHandler) handleCustomerDeleted(event WebhookEvent, handler CustomerEventHandler) error {
	h.log.Debug("Handling customer.deleted event")

	// Получаем данные о клиенте из события
	customerData, ok := event.Data["object"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid event data format")
	}

	// Получаем ID клиента в Stripe
	customerID, ok := customerData["id"].(string)
	if !ok || customerID == "" {
		return fmt.Errorf("customer ID not found in event data")
	}

	// Получаем метаданные клиента
	metadataInterface, ok := customerData["metadata"]
	if !ok {
		return fmt.Errorf("metadata not found in customer data")
	}

	metadata, ok := metadataInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("metadata is not a map")
	}

	// Получаем наш внутренний ID клиента из метаданных
	internalCustomerID, err := extractUUIDFromMetadata(metadata, "payment_service_customer_id")
	if err != nil {
		return fmt.Errorf("failed to extract customer ID: %w", err)
	}

	// Вызываем обработчик
	if handler != nil {
		return handler.HandleCustomerDeleted(internalCustomerID, customerID)
	}

	return nil
}
