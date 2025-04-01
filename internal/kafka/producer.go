package kafka

import (
	"context"
	"encoding/json" // Для маршалинга данных в JSON
	"errors"        // Для проверки ошибок
	"fmt"
	"time" // Для таймаутов

	"github.com/Dhoini/Payment-microservice/internal/models" // Ваша модель подписки
	"github.com/Dhoini/Payment-microservice/pkg/logger"      // Ваш логгер

	"github.com/segmentio/kafka-go" // Библиотека Kafka
)

// Constants for Kafka topics used by the payment service
// (Интерфейс и константы можно вынести в отдельный файл, например, internal/kafka/kafka.go)
const (
	TopicSubscriptionCreated   = "subscription_created"
	TopicSubscriptionCancelled = "subscription_cancelled"
	// Добавьте другие топики при необходимости
)

// Producer определяет интерфейс для публикации сообщений в Kafka.
type Producer interface {
	// PublishSubscriptionEvent отправляет событие, связанное с подпиской.
	// Ключ сообщения (Key) используется Kafka для партиционирования.
	// Часто используют UserID или SubscriptionID как ключ.
	PublishSubscriptionEvent(ctx context.Context, topic string, subscription *models.Subscription) error
	// Close закрывает соединение продюсера Kafka.
	Close() error
}

// kafkaProducer реализует интерфейс Producer, используя segmentio/kafka-go.
type kafkaProducer struct {
	writer *kafka.Writer  // Объект для записи сообщений
	log    *logger.Logger // Ваш логгер
}

// NewKafkaProducer создает и настраивает новый продюсер Kafka.
func NewKafkaProducer(brokers []string, log *logger.Logger) (Producer, error) {
	// Проверяем, что список брокеров не пуст
	if len(brokers) == 0 {
		log.Errorw("Kafka brokers list is empty in config, cannot create producer")
		return nil, errors.New("kafka brokers are not configured")
	}

	// Настраиваем Kafka Writer
	// Balancer: &kafka.LeastBytes{} - распределяет сообщения по партициям с наименьшей нагрузкой.
	// RequiredAcks: kafka.RequireOne - ждать подтверждения только от лидера партиции (хороший баланс между скоростью и надежностью).
	//               kafka.RequireAll - ждать подтверждения от всех реплик (макс. надежность, медленнее).
	// Async: true - можно включить для асинхронной отправки (быстрее), но требует более сложной обработки ошибок (здесь отключено для простоты).
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...), // Подключаемся к списку брокеров
		Balancer:     &kafka.LeastBytes{},   // Балансировщик нагрузки
		RequiredAcks: kafka.RequireOne,      // Уровень подтверждения записи
		BatchSize:    100,                   // Размер пакета сообщений (уменьшает кол-во запросов)
		BatchTimeout: 10 * time.Millisecond, // Таймаут для накопления пакета
		WriteTimeout: 10 * time.Second,      // Таймаут на операцию записи
		ReadTimeout:  10 * time.Second,      // Таймаут на операцию чтения (для RequiredAcks > 0)
	}

	log.Infow("Kafka producer initialized", "brokers", brokers)

	// Можно добавить .Named("KafkaProducer") или аналогичный метод вашего логгера
	return &kafkaProducer{
		writer: writer,
		log:    log,
	}, nil
}

// PublishSubscriptionEvent преобразует данные подписки в JSON и отправляет в указанный топик Kafka.
func (k *kafkaProducer) PublishSubscriptionEvent(ctx context.Context, topic string, subscription *models.Subscription) error {
	// Используем SubscriptionID как ключ сообщения. Это гарантирует, что все события
	// для одной и той же подписки попадут в одну и ту же партицию Kafka,
	// сохраняя порядок обработки для этой подписки (если консьюмер один на партицию).
	// Альтернатива: использовать UserID, чтобы сгруппировать события по пользователю.
	messageKey := []byte(subscription.SubscriptionID)

	// Преобразуем структуру подписки в JSON для тела сообщения.
	messageValue, err := json.Marshal(subscription)
	if err != nil {
		k.log.Errorw("Failed to marshal subscription data to JSON for Kafka", "error", err, "subscriptionID", subscription.SubscriptionID, "topic", topic)
		return fmt.Errorf("kafka: failed to marshal message data: %w", err)
	}

	// Создаем сообщение Kafka.
	message := kafka.Message{
		Topic: topic,        // Указываем топик
		Key:   messageKey,   // Ключ для партиционирования
		Value: messageValue, // Тело сообщения (JSON)
		Time:  time.Now(),   // Время создания сообщения
	}

	// Отправляем сообщение в Kafka.
	// Используем контекст с таймаутом, чтобы избежать зависания.
	writeCtx, cancel := context.WithTimeout(ctx, 15*time.Second) // Таймаут на запись
	defer cancel()

	err = k.writer.WriteMessages(writeCtx, message)
	if err != nil {
		// Проверяем ошибку таймаута контекста
		if errors.Is(err, context.DeadlineExceeded) {
			k.log.Errorw("Kafka write timeout exceeded", "error", err, "topic", topic, "subscriptionID", subscription.SubscriptionID)
			return fmt.Errorf("kafka: write timeout: %w", err)
		}
		// Другие ошибки записи
		k.log.Errorw("Failed to write message to Kafka", "error", err, "topic", topic, "subscriptionID", subscription.SubscriptionID)
		return fmt.Errorf("kafka: failed to write message: %w", err)
	}

	k.log.Infow("Successfully published message to Kafka", "topic", topic, "subscriptionID", subscription.SubscriptionID, "key", string(messageKey))
	return nil
}

// Close закрывает соединение Kafka Writer.
// Этот метод важно вызвать при завершении работы приложения (graceful shutdown).
func (k *kafkaProducer) Close() error {
	k.log.Infow("Closing Kafka producer writer...")
	err := k.writer.Close()
	if err != nil {
		k.log.Errorw("Failed to close Kafka writer", "error", err)
		return fmt.Errorf("kafka: failed to close writer: %w", err)
	}
	k.log.Infow("Kafka producer writer closed successfully")
	return nil
}
