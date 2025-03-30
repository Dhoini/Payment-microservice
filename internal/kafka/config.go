package kafka

import (
	"github.com/Dhoini/Payment-microservice/pkg/logger"
	"github.com/IBM/sarama"
)

// Config конфигурация для Kafka
type Config struct {
	Brokers  []string
	Producer ProducerConfig
	Consumer ConsumerConfig
}

// ProducerConfig конфигурация для продюсера
type ProducerConfig struct {
	MaxMessageBytes  int
	Compression      sarama.CompressionCodec
	RequiredAcks     sarama.RequiredAcks
	FlushMaxMessages int
}

// ConsumerConfig конфигурация для консьюмера
type ConsumerConfig struct {
	Group              string
	InitialOffset      int64
	OffsetReset        string
	SessionTimeout     int
	HeartbeatInterval  int
	RebalanceStrategy  string
	MaxProcessingTime  int
	ReturnErrors       bool
	IsolationLevel     sarama.IsolationLevel
	MaxWaitTime        int
	MinBytes           int
	MaxBytes           int
	EnableAutoCommit   bool
	AutoCommitInterval int
}

// NewConfig создает новую конфигурацию Kafka
func NewConfig(brokers []string) *Config {
	return &Config{
		Brokers: brokers,
		Producer: ProducerConfig{
			MaxMessageBytes:  1000000,
			Compression:      sarama.CompressionSnappy,
			RequiredAcks:     sarama.WaitForAll,
			FlushMaxMessages: 100,
		},
		Consumer: ConsumerConfig{
			Group:              "payment-service",
			InitialOffset:      sarama.OffsetNewest,
			OffsetReset:        "latest",
			SessionTimeout:     30000,
			HeartbeatInterval:  3000,
			RebalanceStrategy:  "range",
			MaxProcessingTime:  100,
			ReturnErrors:       true,
			IsolationLevel:     sarama.ReadCommitted,
			MaxWaitTime:        250,
			MinBytes:           1,
			MaxBytes:           10e6,
			EnableAutoCommit:   true,
			AutoCommitInterval: 1000,
		},
	}
}

// NewSaramaConfig создает новую конфигурацию Sarama
func NewSaramaConfig(cfg *Config, log *logger.Logger) *sarama.Config {
	saramaConfig := sarama.NewConfig()

	// Версия Kafka
	saramaConfig.Version = sarama.V3_3_0_0

	// Настройки продюсера
	saramaConfig.Producer.MaxMessageBytes = cfg.Producer.MaxMessageBytes
	saramaConfig.Producer.Compression = cfg.Producer.Compression
	saramaConfig.Producer.RequiredAcks = cfg.Producer.RequiredAcks
	saramaConfig.Producer.Flush.MaxMessages = cfg.Producer.FlushMaxMessages
	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true

	// Настройки консьюмера
	saramaConfig.Consumer.Group.Session.Timeout = 10000
	saramaConfig.Consumer.Group.Heartbeat.Interval = 3000
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaConfig.Consumer.Offsets.AutoCommit.Enable = true
	saramaConfig.Consumer.Return.Errors = true

	return saramaConfig
}
