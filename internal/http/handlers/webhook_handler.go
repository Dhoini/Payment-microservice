package handlers

import (
	"encoding/json" // Для разбора данных события
	"errors"
	"io"
	"net/http"

	"github.com/Dhoini/Payment-microservice/internal/config" // Нужен для доступа к webhookSecret
	"github.com/Dhoini/Payment-microservice/internal/services"
	"github.com/Dhoini/Payment-microservice/pkg/logger" // Ваш логгер
	"github.com/Dhoini/Payment-microservice/pkg/res"    // Ваш пакет для ответов (используем для ошибок)

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v78"         // Основной пакет Stripe
	"github.com/stripe/stripe-go/v78/webhook" // Пакет для обработки вебхуков
)

const (
	// Ограничение на размер тела запроса вебхука (Stripe рекомендует ~65kb)
	maxRequestBodySize = int64(65536)
)

// WebhookHandler обрабатывает входящие вебхуки от Stripe.
type WebhookHandler struct {
	service       *services.PaymentService
	log           *logger.Logger
	webhookSecret string // Секретный ключ для проверки подписи вебхука (whsec_...)
}

// NewWebhookHandler создает новый экземпляр WebhookHandler.
func NewWebhookHandler(cfg *config.Config, service *services.PaymentService, log *logger.Logger) (*WebhookHandler, error) {
	// Проверяем, что секрет вебхука задан в конфигурации
	if cfg.Stripe.WebhookSecret == "" {
		log.Errorw("Stripe webhook secret is not configured in config.Stripe.WebhookSecret")
		return nil, errors.New("stripe webhook secret is not configured")
	}
	return &WebhookHandler{
		service:       service,
		log:           log, // Добавляем контекст логгеру
		webhookSecret: cfg.Stripe.WebhookSecret,
	}, nil
}

// HandleStripeWebhook - обработчик для Gin, принимающий вебхуки Stripe.
func (h *WebhookHandler) HandleStripeWebhook(c *gin.Context) {
	ctx := c.Request.Context()

	// 1. Чтение тела запроса с ограничением размера
	// Важно: читаем тело ОДИН РАЗ, так как чтение его "потребляет".
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodySize)
	payload, err := io.ReadAll(c.Request.Body)
	// Не забываем закрыть тело запроса
	//goland:noinspection GoUnhandledErrorResult
	defer c.Request.Body.Close()

	if err != nil {
		h.log.Errorw("Failed to read webhook request body", "error", err)
		// Используем ваш pkg/res для ответа об ошибке
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Cannot read request body"}, http.StatusBadRequest) // Используем 400, т.к. проблема с запросом
		c.Abort()                                                                                               // Прерываем обработку в Gin
		return
	}

	// 2. Получение заголовка с подписью Stripe
	sigHeader := c.GetHeader("Stripe-Signature")
	if sigHeader == "" {
		h.log.Warnw("Missing Stripe-Signature header")
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Missing Stripe-Signature header"}, http.StatusBadRequest)
		c.Abort()
		return
	}

	// 3. Верификация подписи и парсинг события
	// Используем секретный ключ, полученный из конфигурации
	event, err := webhook.ConstructEvent(payload, sigHeader, h.webhookSecret)
	if err != nil {
		h.log.Errorw("Webhook signature verification failed", "error", err)
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Webhook signature verification failed"}, http.StatusBadRequest) // Неверная подпись - плохой запрос
		c.Abort()
		return
	}

	// Логируем успешное получение и верификацию
	h.log.Infow("Received verified Stripe event", "eventID", event.ID, "eventType", event.Type)

	// 4. Извлечение данных и вызов сервиса
	// Метод сервиса HandleWebhookEvent ожидает: ctx, eventType, stripeSubscriptionID, data
	// Попытаемся извлечь ID подписки и передать объект данных.

	var subID string
	var rawData map[string]interface{} // Передаем сырые данные объекта в сервис

	// Пытаемся разобрать event.Data.Raw, чтобы получить доступ к полям объекта
	if err := json.Unmarshal(event.Data.Raw, &rawData); err != nil {
		h.log.Errorw("Failed to unmarshal event.Data.Raw", "error", err, "eventID", event.ID, "eventType", event.Type)
		// Если не можем разобрать данные, скорее всего, дальнейшая обработка невозможна
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Failed to parse event data"}, http.StatusInternalServerError)
		c.Abort()
		return
	}

	// Пытаемся найти ID подписки для частых типов событий
	// Для событий invoice.* ID подписки обычно в поле "subscription"
	if subVal, ok := rawData["subscription"].(string); ok && subVal != "" {
		subID = subVal
	} else if idVal, ok := rawData["id"].(string); ok {
		// Для событий customer.subscription.* или самого объекта подписки, ID может быть в "id"
		// Добавим проверку, что объект действительно подписка (если возможно)
		if objectType, ok := rawData["object"].(string); ok && objectType == "subscription" {
			subID = idVal
		} else if objectType == "invoice" {
			// Если это invoice, но subscription не нашли выше, может его там нет (разовый платеж?)
			h.log.Infow("Event is invoice, but subscription ID not found directly", "eventID", event.ID, "invoiceID", idVal)
		} else {
			// ID найден, но тип объекта не 'subscription'. Это может быть Customer, PaymentIntent и т.д.
			// В таких случаях subID остается пустым, если только сервис не ожидает ID другого объекта.
			h.log.Debugw("Found ID in event data, but object is not a subscription", "eventID", event.ID, "objectType", objectType, "objectID", idVal)
		}
	}

	if subID == "" {
		// Если не смогли извлечь ID подписки автоматически
		h.log.Warnw("Could not reliably determine Stripe Subscription ID from webhook event data", "eventID", event.ID, "eventType", event.Type)
		// Сервис должен быть готов обработать событие без subID, если это применимо для данного eventType
	} else {
		h.log.Debugw("Determined Stripe Subscription ID for event", "eventID", event.ID, "eventType", event.Type, "subscriptionID", subID)
	}

	// 5. Вызов метода сервиса для обработки логики события
	// Передаем тип события, найденный (или пустой) ID подписки и разобранные данные объекта
	err = h.service.HandleWebhookEvent(ctx, event.Type, subID, rawData)
	if err != nil {
		// Логируем ошибку из сервисного слоя
		h.log.Errorw("Error processing webhook event in service", "error", err, "eventID", event.ID, "eventType", event.Type)

		// Отвечаем Stripe ошибкой сервера. Stripe попытается повторить отправку.
		// Если ошибка в нашей логике постоянная, это приведет к повторным ошибкам.
		// В реальном приложении может потребоваться более умная обработка ошибок.
		res.JsonResponse(c.Writer, res.ErrorResponse{Error: "Internal server error processing webhook"}, http.StatusInternalServerError)
		c.Abort()
		return
	}

	// 6. Отправка успешного ответа Stripe (200 OK)
	// Важно ответить быстро, чтобы Stripe не считал доставку неуспешной.
	h.log.Infow("Successfully processed webhook event", "eventID", event.ID, "eventType", event.Type)
	// Не используем res.JsonResponse, так как тело ответа не нужно.
	c.Status(http.StatusOK)
}
