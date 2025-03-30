package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Dhoini/Payment-microservice/internal/domain"
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/IBM/sarama"
)

const (
	TopicPaymentCreated  = "payment.created"
	TopicPaymentUpdated  = "payment.updated"
	TopicPaymentComplete = "payment.complete"
	TopicPaymentFailed   = "payment.failed"
)

// PaymentEvent представляет событие платежа для Kafka
type PaymentEvent struct {
	ID          string               `json:"id"`
	CustomerID  string               `json:"customer_id"`
	Amount      float64              `json:"amount"`
	Currency    string               `json:"currency"`
	Status      domain.PaymentStatus `json:"status"`
	Description string               `json:"description,omitempty"`
	Timestamp   time.Time            `json:"timestamp"`
}

// PaymentProducer интерфейс для отправки событий платежей
type PaymentProducer interface {
	PublishPaymentCreated(ctx context.Context, payment domain.Payment) error
	PublishPaymentUpdated(ctx context.Context, payment domain.Payment) error
	PublishPaymentCompleted(ctx context.Context, payment domain.Payment) error
	PublishPaymentFailed(ctx context.Context, payment domain.Payment) error
	Close() error
}

type kafkaPaymentProducer struct {
	producer sarama.SyncProducer
	log      *logger.Logger
}

// NewKafkaPaymentProducer создает новый продюсер событий платежей
func NewKafkaPaymentProducer(producer sarama.SyncProducer, log *logger.Logger) PaymentProducer {
	return &kafkaPaymentProducer{
		producer: producer,
		log:      log,
	}
}

// PublishPaymentCreated публикует событие о создании платежа
func (p *kafkaPaymentProducer) PublishPaymentCreated(ctx context.Context, payment domain.Payment) error {
	return p.publishEvent(ctx, TopicPaymentCreated, payment)
}

// PublishPaymentUpdated публикует событие об обновлении платежа
func (p *kafkaPaymentProducer) PublishPaymentUpdated(ctx context.Context, payment domain.Payment) error {
	return p.publishEvent(ctx, TopicPaymentUpdated, payment)
}

// PublishPaymentCompleted публикует событие о завершении платежа
func (p *kafkaPaymentProducer) PublishPaymentCompleted(ctx context.Context, payment domain.Payment) error {
	return p.publishEvent(ctx, TopicPaymentComplete, payment)
}

// PublishPaymentFailed публикует событие о неудачном платеже
func (p *kafkaPaymentProducer) PublishPaymentFailed(ctx context.Context, payment domain.Payment) error {
	return p.publishEvent(ctx, TopicPaymentFailed, payment)
}

// publishEvent публикует событие платежа в Kafka
func (p *kafkaPaymentProducer) publishEvent(ctx context.Context, topic string, payment domain.Payment) error {
	event := PaymentEvent{
		ID:          payment.ID.String(),
		CustomerID:  payment.CustomerID.String(),
		Amount:      payment.Amount,
		Currency:    payment.Currency,
		Status:      payment.Status,
		Description: payment.Description,
		Timestamp:   time.Now(),
	}

	messageValue, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal payment event: %w", err)
	}

	message := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(payment.ID.String()),
		Value: sarama.ByteEncoder(messageValue),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("event_type"),
				Value: []byte(topic),
			},
		},
		Timestamp: time.Now(),
	}

	partition, offset, err := p.producer.SendMessage(message)
	if err != nil {
		return fmt.Errorf("failed to publish payment event: %w", err)
	}

	p.log.Info("Published payment event to topic %s: partition=%d offset=%d",
		topic, partition, offset)

	return nil
}

// Close закрывает продюсер
func (p *kafkaPaymentProducer) Close() error {
	return p.producer.Close()
}
